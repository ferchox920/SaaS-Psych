package longitudinal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/approvedcontext"
	"sessionflow/apps/api/internal/usecase/clinicalairun"
	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
)

type Repository interface {
	SessionAnalysis(context.Context, uuid.UUID, uuid.UUID) (SessionAnalysis, error)
	FindOpenDiff(context.Context, uuid.UUID, uuid.UUID, int64) (Diff, error)
	CreateDiff(context.Context, CreateDiffInput) (Diff, error)
	ListEvidence(context.Context, uuid.UUID, uuid.UUID) ([]Evidence, error)
	ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]Event, error)
	ListProcesses(context.Context, uuid.UUID, uuid.UUID) ([]Process, error)
	ListHypotheses(context.Context, uuid.UUID, uuid.UUID) ([]Hypothesis, error)
	State(context.Context, uuid.UUID, uuid.UUID) (State, error)
	ListDiffs(context.Context, uuid.UUID, uuid.UUID) ([]Diff, error)
	GetDiff(context.Context, uuid.UUID, uuid.UUID) (Diff, error)
	Decide(context.Context, DecisionInput) (Diff, error)
	Merge(context.Context, MergeInput) (Diff, error)
}
type ClinicalAccess interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type ApprovedContext interface {
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (approvedcontext.ApprovedClinicalContext, error)
}
type Provider interface {
	InterpretLongitudinal(context.Context, string, InterpreterInput, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error)
}
type Metrics interface {
	RecordLongitudinalAnalysis(string, time.Duration)
	RecordClinicalDiffCreated(string)
	RecordClinicalDiffDecision(string, string)
	RecordClinicalDiffMerge(string)
}
type Auditor interface {
	RecordDomainEvent(context.Context, uuid.UUID, uuid.UUID, string, string, *uuid.UUID, map[string]any) error
}

type InterpreterInput struct {
	SchemaVersion   string                                  `json:"schema_version"`
	SessionReport   json.RawMessage                         `json:"session_report"`
	ApprovedContext approvedcontext.ApprovedClinicalContext `json:"approved_context"`
	CurrentState    State                                   `json:"current_longitudinal_state"`
}

type Service struct {
	repo                Repository
	access              ClinicalAccess
	approved            ApprovedContext
	runs                *clinicalairun.Service
	provider            Provider
	auditor             Auditor
	metrics             Metrics
	providerName, model string
	parameters          map[string]any
	now                 func() time.Time
}

func NewService(repo Repository, access ClinicalAccess, approved ApprovedContext, runs *clinicalairun.Service, provider Provider, auditor Auditor, providerName, model string, parameters map[string]any) *Service {
	return &Service{repo: repo, access: access, approved: approved, runs: runs, provider: provider, auditor: auditor, providerName: providerName, model: model, parameters: parameters, now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) WithMetrics(m Metrics) *Service { s.metrics = m; return s }

func (s *Service) Analyze(ctx context.Context, tenantID, sessionID, actorID uuid.UUID) (out AnalysisOutput, err error) {
	started := s.now()
	defer func() {
		if s.metrics != nil {
			result := "success"
			if err != nil {
				result = "error"
			}
			s.metrics.RecordLongitudinalAnalysis(result, s.now().Sub(started))
		}
	}()
	analysis, err := s.repo.SessionAnalysis(ctx, tenantID, sessionID)
	if err != nil {
		return out, err
	}
	if analysis.Status != "completed" {
		return out, domainerrors.NewValidation("longitudinal analysis requires a completed clinical session")
	}
	if err = s.require(ctx, tenantID, actorID, analysis.ClientID, "treating"); err != nil {
		return out, err
	}
	state, err := s.repo.State(ctx, tenantID, analysis.ClientID)
	if err != nil {
		return out, err
	}
	if existing, e := s.repo.FindOpenDiff(ctx, tenantID, analysis.ReportID, state.StateVersion); e == nil {
		return AnalysisOutput{Diff: existing, Reused: true}, nil
	} else if !errors.Is(e, domainerrors.ErrNotFound) {
		return out, e
	}
	approved, err := s.approved.Get(ctx, tenantID, analysis.ClientID, actorID)
	if err != nil {
		return out, err
	}
	input := InterpreterInput{SchemaVersion: PromptVersion, SessionReport: analysis.ReportJSON, ApprovedContext: approved, CurrentState: state}
	sources := append([]clinicalairun.Source{}, approved.Sources...)
	reportVersion := analysis.ReportVersion
	sources = appendSourceUnique(sources, clinicalairun.Source{TenantID: tenantID, SourceType: clinicalairun.SourceSessionReport, SourceID: analysis.ReportID, SourceVersion: &reportVersion})
	sources = append(sources, stateSources(tenantID, state)...)
	run, err := s.runs.Start(ctx, clinicalairun.StartInput{TenantID: tenantID, ClientID: analysis.ClientID, AppointmentID: analysis.AppointmentID, ClinicalSessionID: &sessionID, CreatedByUserID: actorID, Provider: s.providerName, Model: s.model, Operation: "longitudinal_interpretation", PromptName: PromptName, PromptVersion: PromptVersion, Parameters: s.parameters, Input: input, Context: input, Sources: sources})
	if err != nil {
		return out, fmt.Errorf("start longitudinal AI run: %w", err)
	}
	finished := false
	defer func() {
		if !finished && err != nil {
			cancelled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
			_, _ = s.runs.Fail(context.WithoutCancel(ctx), tenantID, run.ID, longitudinalErrorCode(err), cancelled)
		}
	}()
	providerOut, err := s.provider.InterpretLongitudinal(ctx, SystemPromptV1(), input, nil)
	if err != nil {
		return out, err
	}
	result, err := DecodeInterpreterResult(providerOut.JSON)
	if err != nil {
		return out, fmt.Errorf("invalid longitudinal model output: %w", err)
	}
	operations := make([]Operation, 0, len(result.Operations))
	localIDs := map[string]uuid.UUID{}
	for _, proposal := range result.Operations {
		target := uuid.New()
		if proposal.TargetEntityID != nil {
			target = *proposal.TargetEntityID
		}
		localIDs[proposal.ID] = target
		operations = append(operations, Operation{ID: uuid.New(), TenantID: tenantID, ClientID: analysis.ClientID, Sequence: len(operations) + 1, OperationType: proposal.OperationType, TargetEntityID: target, ExpectedEntityVersion: proposal.ExpectedEntityVersion, OriginalProposal: proposal.Proposal, ReviewStatus: "pending"})
	}
	_ = localIDs // stable reserved ids are carried by target_entity_id; payload references use UUIDs.
	diff, err := s.repo.CreateDiff(ctx, CreateDiffInput{TenantID: tenantID, ClientID: analysis.ClientID, SessionID: sessionID, ReportID: analysis.ReportID, RunID: run.ID, ActorID: actorID, BaseStateVersion: state.StateVersion, Operations: operations, Uncertainties: result.Uncertainties, OutputHash: clinicalairun.Hash(result)})
	if err != nil {
		if errors.Is(err, domainerrors.ErrConflict) {
			if existing, e := s.repo.FindOpenDiff(ctx, tenantID, analysis.ReportID, state.StateVersion); e == nil {
				_, _ = s.runs.Fail(context.WithoutCancel(ctx), tenantID, run.ID, "duplicate_open_diff", true)
				finished = true
				return AnalysisOutput{Diff: existing, Reused: true}, nil
			}
		}
		return out, err
	}
	finished = true
	s.runs.RecordPersistedCompletion("longitudinal_interpretation", clinicalairun.StatusSucceeded)
	if s.metrics != nil {
		s.metrics.RecordClinicalDiffCreated("success")
	}
	return AnalysisOutput{Diff: diff}, nil
}

func (s *Service) ListEvidence(ctx context.Context, t, c, a uuid.UUID) ([]Evidence, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListEvidence(ctx, t, c)
}
func (s *Service) ListEvents(ctx context.Context, t, c, a uuid.UUID) ([]Event, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListEvents(ctx, t, c)
}
func (s *Service) ListProcesses(ctx context.Context, t, c, a uuid.UUID) ([]Process, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListProcesses(ctx, t, c)
}
func (s *Service) ListHypotheses(ctx context.Context, t, c, a uuid.UUID) ([]Hypothesis, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListHypotheses(ctx, t, c)
}
func (s *Service) State(ctx context.Context, t, c, a uuid.UUID) (State, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return State{}, err
	}
	return s.repo.State(ctx, t, c)
}
func (s *Service) ListDiffs(ctx context.Context, t, c, a uuid.UUID) ([]Diff, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListDiffs(ctx, t, c)
}
func (s *Service) GetDiff(ctx context.Context, t, id, a uuid.UUID) (Diff, error) {
	d, err := s.repo.GetDiff(ctx, t, id)
	if err != nil {
		return d, err
	}
	if err = s.require(ctx, t, a, d.ClientID, "treating", "supervisor"); err != nil {
		return Diff{}, err
	}
	return d, nil
}
func (s *Service) Decide(ctx context.Context, in DecisionInput) (Diff, error) {
	d, err := s.repo.GetDiff(ctx, in.TenantID, in.DiffID)
	if err != nil {
		return d, err
	}
	if err = s.require(ctx, in.TenantID, in.ActorID, d.ClientID, "treating"); err != nil {
		return Diff{}, err
	}
	if in.Decision == "modified" {
		if err = ValidateProposal(operationType(d.Operations, in.OperationID), in.Modification); err != nil {
			return Diff{}, err
		}
	} else if len(in.Modification) > 0 {
		return Diff{}, domainerrors.NewValidation("modification is only valid for modified decisions")
	}
	out, err := s.repo.Decide(ctx, in)
	if s.metrics != nil {
		s.metrics.RecordClinicalDiffDecision(in.Decision, result(err))
	}
	return out, err
}
func (s *Service) Merge(ctx context.Context, in MergeInput) (Diff, error) {
	d, err := s.repo.GetDiff(ctx, in.TenantID, in.DiffID)
	if err != nil {
		return d, err
	}
	if in.ActorID == uuid.Nil {
		return Diff{}, domainerrors.ErrForbidden
	}
	if err = s.require(ctx, in.TenantID, in.ActorID, d.ClientID, "treating"); err != nil {
		return Diff{}, err
	}
	out, err := s.repo.Merge(ctx, in)
	if s.metrics != nil {
		s.metrics.RecordClinicalDiffMerge(result(err))
	}
	return out, err
}
func (s *Service) require(ctx context.Context, t, a, c uuid.UUID, rel ...string) error {
	if t == uuid.Nil || a == uuid.Nil || c == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, actor_user_id and client_id are required")
	}
	ok, err := s.access.CanAccessClient(ctx, t, a, c, rel...)
	if err != nil {
		return err
	}
	if !ok {
		return domainerrors.ErrForbidden
	}
	return nil
}
func operationType(ops []Operation, id uuid.UUID) string {
	for _, op := range ops {
		if op.ID == id {
			return op.OperationType
		}
	}
	return ""
}
func result(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}
func longitudinalErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, domainerrors.ErrValidation):
		return "invalid_model_output"
	default:
		return "provider_error"
	}
}
func stateSources(t uuid.UUID, s State) []clinicalairun.Source {
	out := []clinicalairun.Source{}
	add := func(kind string, id uuid.UUID, v int) {
		version := v
		out = append(out, clinicalairun.Source{TenantID: t, SourceType: kind, SourceID: id, SourceVersion: &version})
	}
	for _, e := range s.ActiveEvidence {
		add("clinical_evidence", e.ID, e.Version)
	}
	for _, e := range s.RecentEvents {
		add("clinical_event", e.ID, e.Version)
	}
	for _, p := range s.Processes {
		add("clinical_process", p.ID, p.Version)
		for _, h := range p.Hypotheses {
			add("clinical_hypothesis", h.ID, h.Version)
		}
	}
	for _, h := range s.UnassignedHypotheses {
		add("clinical_hypothesis", h.ID, h.Version)
	}
	return out
}
func appendSourceUnique(in []clinicalairun.Source, candidate clinicalairun.Source) []clinicalairun.Source {
	for _, source := range in {
		if source.SourceType == candidate.SourceType && source.SourceID == candidate.SourceID {
			left, right := 0, 0
			if source.SourceVersion != nil {
				left = *source.SourceVersion
			}
			if candidate.SourceVersion != nil {
				right = *candidate.SourceVersion
			}
			if left == right {
				return in
			}
		}
	}
	return append(in, candidate)
}
func normalizeText(v string) string { return strings.Join(strings.Fields(v), " ") }
