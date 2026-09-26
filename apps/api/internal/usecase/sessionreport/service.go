package sessionreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	"sessionflow/apps/api/internal/usecase/consent"
)

var ErrInvalidOutput = errors.New("invalid session report output")

type Repository interface {
	SessionDetails(context.Context, uuid.UUID, uuid.UUID) (SessionDetails, error)
	CreateDraft(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID, ReportV1) (Report, error)
	List(context.Context, uuid.UUID, uuid.UUID) ([]Report, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Report, error)
	Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, ReportV1) (Report, error)
	Approve(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int) (Report, error)
}
type ClinicalAccess interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type Provider interface {
	GenerateSessionReport(context.Context, string, []byte, map[string]any) (ProviderOutput, error)
}
type Metrics interface {
	RecordSessionReportGeneration(string, time.Duration)
	RecordSessionReportApproval(string)
}
type Service struct {
	consent             consent.Authorizer
	repo                Repository
	access              ClinicalAccess
	runs                *clinicalairun.Service
	provider            Provider
	providerName, model string
	parameters          map[string]any
	metrics             Metrics
}
type GenerateInput struct {
	TenantID, SessionID, ActorUserID uuid.UUID
	SessionText                      string
}
type UpdateInput struct {
	TenantID, ReportID, ActorUserID uuid.UUID
	ExpectedRevision                int
	Report                          ReportV1
}

func NewService(repo Repository, access ClinicalAccess, runs *clinicalairun.Service, provider Provider, providerName, model string, parameters map[string]any) *Service {
	return &Service{repo: repo, access: access, runs: runs, provider: provider, providerName: providerName, model: model, parameters: parameters}
}
func (s *Service) WithMetrics(metrics Metrics) *Service      { s.metrics = metrics; return s }
func (s *Service) WithConsent(a consent.Authorizer) *Service { s.consent = a; return s }

func (s *Service) Generate(ctx context.Context, input GenerateInput) (out Report, err error) {
	started := time.Now()
	defer func() {
		if s.metrics != nil {
			result := "success"
			if err != nil {
				result = "error"
			}
			s.metrics.RecordSessionReportGeneration(result, time.Since(started))
		}
	}()
	text := strings.TrimSpace(input.SessionText)
	if length := len([]rune(text)); length < 20 || length > 40000 {
		return Report{}, domainerrors.NewValidation("session_text must contain between 20 and 40000 characters")
	}
	details, err := s.repo.SessionDetails(ctx, input.TenantID, input.SessionID)
	if err != nil {
		return Report{}, err
	}
	if details.Status != "completed" {
		return Report{}, domainerrors.NewValidation("session report generation requires a completed clinical session")
	}
	if err := s.require(ctx, input.TenantID, input.ActorUserID, details.ClientID, "treating"); err != nil {
		return Report{}, err
	}
	if s.consent != nil {
		if _, err = s.consent.Authorize(ctx, input.TenantID, details.ClientID, consent.LocalAI, "session_report_generate", input.SessionID); err != nil {
			return Report{}, err
		}
	}
	run, err := s.runs.Start(ctx, clinicalairun.StartInput{TenantID: input.TenantID, ClientID: details.ClientID, AppointmentID: details.AppointmentID, ClinicalSessionID: &details.ID, CreatedByUserID: input.ActorUserID, Provider: s.providerName, Model: s.model, Operation: "generate_session_report", PromptName: "session-report", PromptVersion: SchemaVersion, Parameters: s.parameters, Input: struct{ SessionText string }{text}, Context: struct {
		SessionID uuid.UUID
		Status    string
	}{details.ID, details.Status}})
	if err != nil {
		return Report{}, fmt.Errorf("start session report AI run: %w", err)
	}
	finished := false
	defer func() {
		if !finished && err != nil {
			cancelled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
			_, _ = s.runs.Fail(context.WithoutCancel(ctx), input.TenantID, run.ID, reportRunErrorCode(err), cancelled)
		}
	}()
	payload, _ := json.Marshal(map[string]any{"schema_version": SchemaVersion, "session_text": text})
	providerOut, err := s.provider.GenerateSessionReport(ctx, SystemPromptV1(), payload, JSONSchemaV1())
	if err != nil {
		return Report{}, err
	}
	report, err := DecodeAndValidate(providerOut.JSON)
	if err != nil {
		return Report{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	if _, err = s.runs.Succeed(ctx, input.TenantID, run.ID, report); err != nil {
		return Report{}, fmt.Errorf("finish session report AI run: %w", err)
	}
	finished = true
	out, err = s.repo.CreateDraft(ctx, input.TenantID, input.SessionID, input.ActorUserID, &run.ID, report)
	return out, err
}
func (s *Service) List(ctx context.Context, tenantID, sessionID, actorID uuid.UUID) ([]Report, error) {
	details, err := s.repo.SessionDetails(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, tenantID, actorID, details.ClientID, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, tenantID, sessionID)
}
func (s *Service) Get(ctx context.Context, tenantID, reportID, actorID uuid.UUID) (Report, error) {
	item, err := s.repo.Get(ctx, tenantID, reportID)
	if err != nil {
		return Report{}, err
	}
	details, err := s.repo.SessionDetails(ctx, tenantID, item.ClinicalSessionID)
	if err != nil {
		return Report{}, err
	}
	if err := s.require(ctx, tenantID, actorID, details.ClientID, "treating", "supervisor"); err != nil {
		return Report{}, err
	}
	return item, nil
}
func (s *Service) Update(ctx context.Context, input UpdateInput) (Report, error) {
	if input.ExpectedRevision < 1 {
		return Report{}, domainerrors.NewValidation("expected_revision must be positive")
	}
	existing, err := s.repo.Get(ctx, input.TenantID, input.ReportID)
	if err != nil {
		return Report{}, err
	}
	details, err := s.repo.SessionDetails(ctx, input.TenantID, existing.ClinicalSessionID)
	if err != nil {
		return Report{}, err
	}
	if err := s.require(ctx, input.TenantID, input.ActorUserID, details.ClientID, "treating"); err != nil {
		return Report{}, err
	}
	var stored ReportV1
	if len(existing.ReportJSON) > 0 {
		if err := json.Unmarshal(existing.ReportJSON, &stored); err != nil {
			return Report{}, fmt.Errorf("decode stored session report identity: %w", err)
		}
	}
	AssignMissingIDs(&input.Report, reportItemIDs(stored)...)
	if err := Validate(input.Report); err != nil {
		return Report{}, err
	}
	return s.repo.Update(ctx, input.TenantID, input.ReportID, input.ActorUserID, input.ExpectedRevision, input.Report)
}
func (s *Service) Approve(ctx context.Context, tenantID, reportID, actorID uuid.UUID, expectedRevision int) (Report, error) {
	if expectedRevision < 1 {
		return Report{}, domainerrors.NewValidation("expected_revision must be positive")
	}
	existing, err := s.repo.Get(ctx, tenantID, reportID)
	if err != nil {
		return Report{}, err
	}
	details, err := s.repo.SessionDetails(ctx, tenantID, existing.ClinicalSessionID)
	if err != nil {
		return Report{}, err
	}
	if err := s.require(ctx, tenantID, actorID, details.ClientID, "treating"); err != nil {
		return Report{}, err
	}
	out, err := s.repo.Approve(ctx, tenantID, reportID, actorID, expectedRevision)
	if s.metrics != nil {
		result := "success"
		if err != nil {
			result = "error"
		}
		s.metrics.RecordSessionReportApproval(result)
	}
	return out, err
}
func (s *Service) require(ctx context.Context, tenantID, actorID, clientID uuid.UUID, relationships ...string) error {
	allowed, err := s.access.CanAccessClient(ctx, tenantID, actorID, clientID, relationships...)
	if err != nil {
		return err
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}
func DecodeAndValidate(raw []byte) (ReportV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var report ReportV1
	if err := decoder.Decode(&report); err != nil {
		return ReportV1{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ReportV1{}, errors.New("report contains trailing data")
	}
	if err := Validate(report); err != nil {
		return ReportV1{}, err
	}
	return report, nil
}
func Validate(r ReportV1) error {
	if err := validateReportStringBounds(r); err != nil {
		return err
	}
	if r.SchemaVersion != SchemaVersion {
		return domainerrors.NewValidation("schema_version must be session-report-v1.1")
	}
	if strings.TrimSpace(r.Summary) == "" {
		return domainerrors.NewValidation("summary is required")
	}
	if r.Facts == nil || r.RelevantChanges == nil || r.Interventions == nil || r.PatientResponses == nil || r.AffectiveNodes == nil || r.InferenceCandidates == nil || r.HypothesisCandidates == nil || r.SafetySignals == nil || r.OpenQuestions == nil || r.LongitudinalCandidates == nil {
		return domainerrors.NewValidation("all session report sections are required")
	}
	counts := []struct{ n, max int }{{len(r.Facts), 20}, {len(r.RelevantChanges), 12}, {len(r.Interventions), 12}, {len(r.PatientResponses), 12}, {len(r.AffectiveNodes), 12}, {len(r.InferenceCandidates), 12}, {len(r.HypothesisCandidates), 12}, {len(r.SafetySignals), 8}, {len(r.OpenQuestions), 12}, {len(r.LongitudinalCandidates), 12}}
	for _, count := range counts {
		if count.n > count.max {
			return domainerrors.NewValidation("session report section exceeds its item limit")
		}
	}
	for _, f := range r.Facts {
		if invalidItemID(f.ID, "fact") || strings.TrimSpace(f.Statement) == "" || strings.TrimSpace(f.Category) == "" {
			return domainerrors.NewValidation("facts require statement and category")
		}
	}
	for _, item := range r.RelevantChanges {
		if invalidItemID(item.ID, "change") || blank(item.Description) || blank(item.Category) {
			return domainerrors.NewValidation("relevant changes require description and category")
		}
	}
	for _, item := range r.Interventions {
		if invalidItemID(item.ID, "intervention") || blank(item.Type) || blank(item.Description) {
			return domainerrors.NewValidation("interventions require type and description")
		}
	}
	for _, item := range r.PatientResponses {
		if invalidItemID(item.ID, "response") || blank(item.ResponseType) || blank(item.Description) {
			return domainerrors.NewValidation("patient responses require response_type and description")
		}
	}
	for _, item := range r.AffectiveNodes {
		if invalidItemID(item.ID, "affect") || blank(item.Description) {
			return domainerrors.NewValidation("affective nodes require description")
		}
	}
	for _, item := range r.InferenceCandidates {
		if invalidItemID(item.ID, "inference") || blank(item.Statement) || item.EvidenceRefs == nil {
			return domainerrors.NewValidation("inference candidates require statement and evidence_refs")
		}
	}
	for _, h := range r.HypothesisCandidates {
		if invalidItemID(h.ID, "hypothesis") || blank(h.Statement) || h.EvidenceRefs == nil || (h.TrafficLight != "green" && h.TrafficLight != "yellow" && h.TrafficLight != "red") {
			return domainerrors.NewValidation("hypotheses require statement and valid traffic_light")
		}
	}
	for _, item := range r.SafetySignals {
		if invalidItemID(item.ID, "safety") || blank(item.Description) || blank(item.Category) || !item.RequiresHumanAssessment {
			return domainerrors.NewValidation("safety signals require description, category and human assessment")
		}
	}
	for _, item := range r.OpenQuestions {
		if invalidItemID(item.ID, "question") || blank(item.Question) {
			return domainerrors.NewValidation("open questions require question")
		}
	}
	for _, candidate := range r.LongitudinalCandidates {
		if invalidItemID(candidate.ID, "longitudinal") || blank(candidate.Rationale) {
			return domainerrors.NewValidation("longitudinal candidates require rationale")
		}
		switch candidate.Operation {
		case "possible_existing_process_update", "possible_new_process", "possible_hypothesis_update", "possible_goal_update", "possible_formulation_update":
		default:
			return domainerrors.NewValidation("invalid longitudinal candidate operation")
		}
	}
	ids := reportItemIDs(r)
	sort.Strings(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			return domainerrors.NewValidation("session report item IDs must be unique")
		}
	}
	// Only observed or reported report items can support an inference or hypothesis.
	// Interpretive items must not recursively become their own evidence.
	evidenceIDs := make(map[string]struct{}, len(r.Facts)+len(r.RelevantChanges)+len(r.PatientResponses)+len(r.AffectiveNodes))
	for _, item := range r.Facts {
		evidenceIDs[item.ID] = struct{}{}
	}
	for _, item := range r.RelevantChanges {
		evidenceIDs[item.ID] = struct{}{}
	}
	for _, item := range r.PatientResponses {
		evidenceIDs[item.ID] = struct{}{}
	}
	for _, item := range r.AffectiveNodes {
		evidenceIDs[item.ID] = struct{}{}
	}
	validateRefs := func(refs []string) error {
		seen := make(map[string]struct{}, len(refs))
		for _, ref := range refs {
			if _, ok := evidenceIDs[ref]; !ok {
				return domainerrors.NewValidation("evidence_refs must identify an observed report item")
			}
			if _, duplicate := seen[ref]; duplicate {
				return domainerrors.NewValidation("evidence_refs must be unique within each item")
			}
			seen[ref] = struct{}{}
		}
		return nil
	}
	for _, item := range r.InferenceCandidates {
		if err := validateRefs(item.EvidenceRefs); err != nil {
			return err
		}
	}
	for _, item := range r.HypothesisCandidates {
		if err := validateRefs(item.EvidenceRefs); err != nil {
			return err
		}
	}
	return nil
}

var itemIDPattern = regexp.MustCompile(`^[a-z]+-[0-9]{3,6}$`)

func invalidItemID(id, prefix string) bool {
	return !itemIDPattern.MatchString(id) || !strings.HasPrefix(id, prefix+"-")
}
func reportItemIDs(r ReportV1) []string {
	ids := make([]string, 0)
	for _, x := range r.Facts {
		ids = append(ids, x.ID)
	}
	for _, x := range r.RelevantChanges {
		ids = append(ids, x.ID)
	}
	for _, x := range r.Interventions {
		ids = append(ids, x.ID)
	}
	for _, x := range r.PatientResponses {
		ids = append(ids, x.ID)
	}
	for _, x := range r.AffectiveNodes {
		ids = append(ids, x.ID)
	}
	for _, x := range r.InferenceCandidates {
		ids = append(ids, x.ID)
	}
	for _, x := range r.HypothesisCandidates {
		ids = append(ids, x.ID)
	}
	for _, x := range r.SafetySignals {
		ids = append(ids, x.ID)
	}
	for _, x := range r.OpenQuestions {
		ids = append(ids, x.ID)
	}
	for _, x := range r.LongitudinalCandidates {
		ids = append(ids, x.ID)
	}
	return ids
}
func AssignMissingIDs(r *ReportV1, reserved ...string) {
	used := map[string]bool{}
	for _, id := range reserved {
		if id != "" {
			used[id] = true
		}
	}
	for _, id := range reportItemIDs(*r) {
		if id != "" {
			used[id] = true
		}
	}
	next := func(prefix string) string {
		for n := 1; ; n++ {
			id := prefix + "-" + fmt.Sprintf("%03d", n)
			if !used[id] {
				used[id] = true
				return id
			}
		}
	}
	for i := range r.Facts {
		if r.Facts[i].ID == "" {
			r.Facts[i].ID = next("fact")
		}
	}
	for i := range r.RelevantChanges {
		if r.RelevantChanges[i].ID == "" {
			r.RelevantChanges[i].ID = next("change")
		}
	}
	for i := range r.Interventions {
		if r.Interventions[i].ID == "" {
			r.Interventions[i].ID = next("intervention")
		}
	}
	for i := range r.PatientResponses {
		if r.PatientResponses[i].ID == "" {
			r.PatientResponses[i].ID = next("response")
		}
	}
	for i := range r.AffectiveNodes {
		if r.AffectiveNodes[i].ID == "" {
			r.AffectiveNodes[i].ID = next("affect")
		}
	}
	for i := range r.InferenceCandidates {
		if r.InferenceCandidates[i].ID == "" {
			r.InferenceCandidates[i].ID = next("inference")
		}
	}
	for i := range r.HypothesisCandidates {
		if r.HypothesisCandidates[i].ID == "" {
			r.HypothesisCandidates[i].ID = next("hypothesis")
		}
	}
	for i := range r.SafetySignals {
		if r.SafetySignals[i].ID == "" {
			r.SafetySignals[i].ID = next("safety")
		}
	}
	for i := range r.OpenQuestions {
		if r.OpenQuestions[i].ID == "" {
			r.OpenQuestions[i].ID = next("question")
		}
	}
	for i := range r.LongitudinalCandidates {
		if r.LongitudinalCandidates[i].ID == "" {
			r.LongitudinalCandidates[i].ID = next("longitudinal")
		}
	}
}
func blank(value string) bool { return strings.TrimSpace(value) == "" }
func reportRunErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrInvalidOutput):
		return "invalid_output"
	default:
		return "generation_failed"
	}
}
