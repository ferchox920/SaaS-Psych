package clinicalairun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const (
	StatusRunning              = "running"
	StatusSucceeded            = "succeeded"
	StatusFailed               = "failed"
	StatusCancelled            = "cancelled"
	SourceFormulationSnapshot  = "formulation_snapshot"
	SourceFormulationAnchor    = "formulation_anchor"
	SourceSessionReport        = "session_report"
	SourceClinicalEvidence     = "clinical_evidence"
	SourceClinicalEvent        = "clinical_event"
	SourceClinicalProcess      = "clinical_process"
	SourceClinicalHypothesis   = "clinical_hypothesis"
	SourceClinicalTarget       = "clinical_target"
	SourceClinicalGoal         = "clinical_goal"
	SourceGoalIndicator        = "goal_indicator"
	SourceTherapeuticRationale = "therapeutic_rationale"
	SourceGIRA                 = "gira"
	SourceGIRAPhase            = "gira_phase"
	SourceApproachDefinition   = "therapeutic_approach_definition"
	SourceTechniqueDefinition  = "therapeutic_technique_definition"
)

type Source struct {
	ID            uuid.UUID `json:"id,omitempty"`
	TenantID      uuid.UUID `json:"tenant_id"`
	AIRunID       uuid.UUID `json:"ai_run_id"`
	SourceType    string    `json:"source_type"`
	SourceID      uuid.UUID `json:"source_id"`
	SourceVersion *int      `json:"source_version,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

type Run struct {
	ID                uuid.UUID       `json:"id"`
	TenantID          uuid.UUID       `json:"tenant_id"`
	ClientID          uuid.UUID       `json:"client_id"`
	AppointmentID     *uuid.UUID      `json:"appointment_id,omitempty"`
	ClinicalSessionID *uuid.UUID      `json:"clinical_session_id,omitempty"`
	CreatedByUserID   uuid.UUID       `json:"created_by_user_id"`
	Provider          string          `json:"provider"`
	Model             string          `json:"model"`
	Operation         string          `json:"operation"`
	PromptName        string          `json:"prompt_name"`
	PromptVersion     string          `json:"prompt_version"`
	AppVersion        string          `json:"app_version"`
	BuildRevision     string          `json:"build_revision"`
	ParametersJSON    json.RawMessage `json:"parameters_json"`
	InputHash         string          `json:"input_hash"`
	ContextHash       string          `json:"context_hash"`
	OutputHash        *string         `json:"output_hash,omitempty"`
	Status            string          `json:"status"`
	StartedAt         time.Time       `json:"started_at"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	ErrorCode         *string         `json:"error_code,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Sources           []Source        `json:"sources,omitempty"`
}
type StartInput struct {
	TenantID, ClientID, CreatedByUserID                   uuid.UUID
	AppointmentID, ClinicalSessionID                      *uuid.UUID
	Provider, Model, Operation, PromptName, PromptVersion string
	Parameters                                            any
	Input, Context                                        any
	Sources                                               []Source
}
type Repository interface {
	Start(context.Context, Run) (Run, error)
	Finish(context.Context, uuid.UUID, uuid.UUID, string, *string, *string, time.Time) (Run, error)
	ListSources(context.Context, uuid.UUID, uuid.UUID) ([]Source, error)
}
type Metrics interface{ RecordClinicalAIRun(string, string) }
type Service struct {
	repo                      Repository
	metrics                   Metrics
	now                       func() time.Time
	appVersion, buildRevision string
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }, appVersion: "development", buildRevision: "unknown"}
}
func (s *Service) WithMetrics(metrics Metrics) *Service { s.metrics = metrics; return s }
func (s *Service) WithBuildInfo(appVersion, buildRevision string) *Service {
	if strings.TrimSpace(appVersion) != "" {
		s.appVersion = strings.TrimSpace(appVersion)
	}
	if strings.TrimSpace(buildRevision) != "" {
		s.buildRevision = strings.TrimSpace(buildRevision)
	}
	return s
}
func (s *Service) Start(ctx context.Context, input StartInput) (Run, error) {
	if input.TenantID == uuid.Nil || input.ClientID == uuid.Nil || input.CreatedByUserID == uuid.Nil {
		return Run{}, domainerrors.NewValidation("tenant_id, client_id and created_by_user_id are required")
	}
	fields := []string{input.Provider, input.Model, input.Operation, input.PromptName, input.PromptVersion}
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return Run{}, domainerrors.NewValidation("AI run provenance fields are required")
		}
	}
	parameters, err := json.Marshal(input.Parameters)
	if err != nil {
		return Run{}, domainerrors.NewValidation("parameters must be JSON serializable")
	}
	now := s.now()
	sources, err := CanonicalSources(input.TenantID, input.Sources)
	if err != nil {
		return Run{}, err
	}
	contextHash := Hash(input.Context)
	if input.Sources != nil {
		contextHash = HashSources(sources)
	}
	run := Run{ID: uuid.New(), TenantID: input.TenantID, ClientID: input.ClientID, AppointmentID: input.AppointmentID, ClinicalSessionID: input.ClinicalSessionID, CreatedByUserID: input.CreatedByUserID, Provider: input.Provider, Model: input.Model, Operation: input.Operation, PromptName: input.PromptName, PromptVersion: input.PromptVersion, AppVersion: s.appVersion, BuildRevision: s.buildRevision, ParametersJSON: parameters, InputHash: Hash(input.Input), ContextHash: contextHash, Status: StatusRunning, StartedAt: now, CreatedAt: now, UpdatedAt: now, Sources: sources}
	out, err := s.repo.Start(ctx, run)
	if err == nil && s.metrics != nil {
		s.metrics.RecordClinicalAIRun(input.Operation, StatusRunning)
	}
	return out, err
}
func (s *Service) ListSources(ctx context.Context, tenantID, runID uuid.UUID) ([]Source, error) {
	return s.repo.ListSources(ctx, tenantID, runID)
}
func CanonicalSources(tenantID uuid.UUID, sources []Source) ([]Source, error) {
	out := append([]Source(nil), sources...)
	seen := make(map[string]struct{}, len(out))
	for i := range out {
		out[i].TenantID = tenantID
		if out[i].SourceID == uuid.Nil {
			return nil, domainerrors.NewValidation("AI run source_id is required")
		}
		switch out[i].SourceType {
		case SourceFormulationSnapshot, SourceFormulationAnchor, SourceSessionReport, SourceClinicalEvidence, SourceClinicalEvent, SourceClinicalProcess, SourceClinicalHypothesis, SourceClinicalTarget, SourceClinicalGoal, SourceGoalIndicator, SourceTherapeuticRationale, SourceGIRA, SourceGIRAPhase, SourceApproachDefinition, SourceTechniqueDefinition:
		default:
			return nil, domainerrors.NewValidation("invalid AI run source_type")
		}
		version := 0
		if out[i].SourceVersion != nil {
			version = *out[i].SourceVersion
			if version < 1 {
				return nil, domainerrors.NewValidation("AI run source_version must be positive")
			}
		}
		key := fmt.Sprintf("%s/%s/%d", out[i].SourceType, out[i].SourceID, version)
		if _, ok := seen[key]; ok {
			return nil, domainerrors.NewValidation("duplicate AI run source")
		}
		seen[key] = struct{}{}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.SourceType != b.SourceType {
			return a.SourceType < b.SourceType
		}
		if a.SourceID != b.SourceID {
			return a.SourceID.String() < b.SourceID.String()
		}
		av, bv := 0, 0
		if a.SourceVersion != nil {
			av = *a.SourceVersion
		}
		if b.SourceVersion != nil {
			bv = *b.SourceVersion
		}
		return av < bv
	})
	return out, nil
}
func HashSources(sources []Source) string {
	type canonical struct {
		SourceType    string `json:"source_type"`
		SourceID      string `json:"source_id"`
		SourceVersion *int   `json:"source_version"`
	}
	items := make([]canonical, len(sources))
	for i, source := range sources {
		items[i] = canonical{source.SourceType, source.SourceID.String(), source.SourceVersion}
	}
	return Hash(items)
}
func (s *Service) Succeed(ctx context.Context, tenantID, runID uuid.UUID, output any) (Run, error) {
	hash := Hash(output)
	out, err := s.repo.Finish(ctx, tenantID, runID, StatusSucceeded, &hash, nil, s.now())
	if err == nil && s.metrics != nil {
		s.metrics.RecordClinicalAIRun(out.Operation, StatusSucceeded)
	}
	return out, err
}
func (s *Service) Fail(ctx context.Context, tenantID, runID uuid.UUID, code string, cancelled bool) (Run, error) {
	status := StatusFailed
	if cancelled {
		status = StatusCancelled
	}
	code = strings.TrimSpace(code)
	if code == "" {
		code = "unknown"
	}
	out, err := s.repo.Finish(ctx, tenantID, runID, status, nil, &code, s.now())
	if err == nil && s.metrics != nil {
		s.metrics.RecordClinicalAIRun(out.Operation, status)
	}
	return out, err
}
func (s *Service) RecordPersistedCompletion(operation, status string) {
	if s != nil && s.metrics != nil {
		s.metrics.RecordClinicalAIRun(operation, status)
	}
}
func Hash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
