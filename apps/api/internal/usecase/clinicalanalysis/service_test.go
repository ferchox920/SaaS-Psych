package clinicalanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

type testProvider struct {
	output Result
	review ReviewResult
	err    error
	before func()
}

func TestLiveAnalysisRejectsOversizedAdjacentClinicalText(t *testing.T) {
	base := AnalyzeLiveInput{TenantID: uuid.New(), ActorUserID: uuid.New(), AppointmentID: uuid.New(), Request: LiveRequest{Fragment: "Fragmento ficticio"}}
	for _, tc := range []struct {
		name string
		edit func(*AnalyzeLiveInput)
	}{
		{"previous intervention", func(input *AnalyzeLiveInput) { input.Request.PreviousIntervention = strings.Repeat("a", 2001) }},
		{"patient response", func(input *AnalyzeLiveInput) { input.Request.PatientResponse = strings.Repeat("a", 2001) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.edit(&input)
			if err := validateAnalyzeInput(input); !errors.Is(err, domainerrors.ErrValidation) {
				t.Fatalf("oversized text err=%v", err)
			}
		})
	}
}

func (p testProvider) Health(context.Context) (ProviderStatus, error) {
	return ProviderStatus{Available: true}, nil
}
func (p testProvider) ListModels(context.Context) ([]ModelInfo, error) { return nil, nil }
func (p testProvider) WarmModel(context.Context) error                 { return nil }
func (p testProvider) UnloadModel(context.Context) error               { return nil }
func (p testProvider) ReviewSession(context.Context, string, ReviewRequest, func(GenerationProgress)) (ProviderOutput, error) {
	if p.before != nil {
		p.before()
	}
	if p.err != nil {
		return ProviderOutput{}, p.err
	}
	raw, _ := json.Marshal(p.review)
	return ProviderOutput{JSON: raw, Metrics: GenerationMetrics{FirstToken: time.Second, Total: 3 * time.Second}}, nil
}

func TestReviewGuardsUnknownEvidenceAndRisk(t *testing.T) {
	category := "posible autolesión"
	result := applyReviewGuards(ReviewRequest{SessionText: "Ficticio."}, ReviewResult{
		Mode: "review", EmergingFormulation: "Provisional", Evidence: []Evidence{{SourceID: "invented", Kind: EpistemicFact, Summary: "No permitido"}, {SourceID: "current_session", Kind: EpistemicFact, Summary: "Dato"}},
		NextFocus: []string{"Interpretar"}, Risk: Risk{Detected: true, Category: &category},
	})
	if len(result.Evidence) != 1 || result.Evidence[0].SourceID != "current_session" {
		t.Fatalf("unexpected evidence: %+v", result.Evidence)
	}
	if !result.Risk.RequiresHumanAssessment || len(result.NextFocus) != 1 || !strings.Contains(result.NextFocus[0], "Evaluar riesgo") {
		t.Fatalf("risk guard not applied: %+v", result)
	}
}
func (p testProvider) AnalyzeLive(_ context.Context, _ string, _ LiveRequest, progress func(GenerationProgress)) (ProviderOutput, error) {
	if p.before != nil {
		p.before()
	}
	if p.err != nil {
		return ProviderOutput{}, p.err
	}
	if progress != nil {
		progress(GenerationProgress{GeneratedCharacters: 10})
	}
	raw, _ := json.Marshal(p.output)
	return ProviderOutput{JSON: raw, Metrics: GenerationMetrics{FirstToken: time.Second, Total: 2 * time.Second}}, nil
}

type analysisRunRepo struct {
	run          clinicalairun.Run
	finishStatus string
}

func (r *analysisRunRepo) Start(_ context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	r.run = run
	return run, nil
}
func (r *analysisRunRepo) Finish(_ context.Context, _, _ uuid.UUID, status string, outputHash, errorCode *string, at time.Time) (clinicalairun.Run, error) {
	r.finishStatus = status
	r.run.Status = status
	r.run.OutputHash = outputHash
	r.run.ErrorCode = errorCode
	r.run.CompletedAt = &at
	return r.run, nil
}
func (r *analysisRunRepo) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]clinicalairun.Source, error) {
	return append([]clinicalairun.Source(nil), r.run.Sources...), nil
}

type analysisMemory struct {
	runID    *uuid.UUID
	approved clinicalmemory.ApprovedContext
}

func (m *analysisMemory) GetApprovedContext(context.Context, uuid.UUID, uuid.UUID) (clinicalmemory.ApprovedContext, error) {
	if m.approved.SnapshotID == uuid.Nil {
		return clinicalmemory.ApprovedContext{}, domainerrors.ErrNotFound
	}
	return m.approved, nil
}
func TestAnalyzeLivePersistsOnlyActuallyUsedApprovedSources(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	runs := &analysisRunRepo{}
	anchors := make([]clinicalmemory.Anchor, 5)
	for i := range anchors {
		anchors[i] = clinicalmemory.Anchor{ID: uuid.New(), SourceID: uuid.NewString(), Kind: "fact", Summary: "approved"}
	}
	memory := &analysisMemory{approved: clinicalmemory.ApprovedContext{SnapshotID: uuid.New(), Version: 4, ApprovedSummary: "approved", Anchors: anchors}}
	service := NewService(testProvider{output: guardedResult()}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithLongitudinalMemory(memory).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", nil)
	if _, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio suficiente."}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(runs.run.Sources) != 5 {
		t.Fatalf("want snapshot + four used anchors, got %#v", runs.run.Sources)
	}
	for _, source := range runs.run.Sources {
		if source.SourceID == anchors[4].ID {
			t.Fatal("unused fifth anchor was persisted as a source")
		}
	}
}
func TestAnalyzeLiveInputHashExcludesUntrustedLongitudinalContext(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	hashFor := func(untrusted string) string {
		runs := &analysisRunRepo{}
		service := NewService(testProvider{output: guardedResult()}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithLongitudinalMemory(&analysisMemory{}).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", nil)
		_, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio suficiente.", ApprovedSummary: untrusted, RelevantContext: []ContextItem{{SourceID: "browser", Summary: untrusted}}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return runs.run.InputHash
	}
	if hashFor("browser context A") != hashFor("browser context B") {
		t.Fatal("input_hash included longitudinal context that the provider did not use")
	}
}
func (m *analysisMemory) CreateSuggestion(_ context.Context, input clinicalmemory.CreateSuggestionInput) (clinicalmemory.Suggestion, error) {
	m.runID = input.AIRunID
	return clinicalmemory.Suggestion{ID: uuid.New()}, nil
}

func TestAnalyzeLiveCreatesRunBeforeInferenceAndLinksSuggestion(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	runs := &analysisRunRepo{}
	memory := &analysisMemory{}
	provider := testProvider{output: guardedResult(), before: func() {
		if runs.run.Status != clinicalairun.StatusRunning {
			t.Fatalf("provider called before running run persisted: %#v", runs.run)
		}
	}}
	service := NewService(provider, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithLongitudinalMemory(memory).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", map[string]any{"temperature": 0.1})
	output, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio suficiente."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output.AIRunID == nil || memory.runID == nil || *memory.runID != *output.AIRunID || runs.finishStatus != clinicalairun.StatusSucceeded {
		t.Fatalf("output=%#v linked=%v status=%s", output, memory.runID, runs.finishStatus)
	}
	if runs.run.Provider != "ollama" || runs.run.Model != "fixture" || runs.run.PromptVersion != PromptVersion {
		t.Fatalf("provenance=%#v", runs.run)
	}
}
func TestAnalyzeLiveValidationFailureCreatesNoRunAndCallsNoProvider(t *testing.T) {
	runs := &analysisRunRepo{}
	called := false
	service := NewService(testProvider{before: func() { called = true }}, testAppointments{}, testAccess{true}, nil).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", nil)
	_, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{}, nil)
	if err == nil || called || runs.run.ID != uuid.Nil {
		t.Fatalf("err=%v provider=%v run=%#v", err, called, runs.run)
	}
}
func TestAnalyzeLivePersistsFailedAndCancelledRuns(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{errors.New("provider down"), clinicalairun.StatusFailed}, {context.Canceled, clinicalairun.StatusCancelled}} {
		tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		runs := &analysisRunRepo{}
		service := NewService(testProvider{err: tc.err}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", map[string]any{})
		_, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio suficiente."}}, nil)
		if err == nil || runs.finishStatus != tc.want {
			t.Fatalf("err=%v status=%s want=%s", err, runs.finishStatus, tc.want)
		}
		if runs.run.OutputHash != nil {
			t.Fatal("failed run must not have output hash")
		}
	}
}

func TestReviewSessionCreatesTraceableRunAndSuggestion(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	runs, memory := &analysisRunRepo{}, &analysisMemory{}
	provider := testProvider{review: ReviewResult{Mode: "review", EmergingFormulation: "Formulación ficticia provisional.", EffectiveInterventions: []string{}, WeakOrRiskyInterventions: []string{}, TherapistPatterns: []string{}, NextFocus: []string{}, Evidence: []Evidence{}, Risk: Risk{}}, before: func() {
		if runs.run.Status != clinicalairun.StatusRunning {
			t.Fatalf("review provider called before run: %#v", runs.run)
		}
	}}
	service := NewService(provider, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithLongitudinalMemory(memory).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", map[string]any{"temperature": 0.1})
	output, err := service.ReviewSession(context.Background(), ReviewSessionInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, SessionText: "Material ficticio completo de una sesión para revisión."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output.AIRunID == nil || memory.runID == nil || *memory.runID != *output.AIRunID || runs.finishStatus != clinicalairun.StatusSucceeded {
		t.Fatalf("output=%#v run=%#v", output, runs)
	}
	if runs.run.Operation != "review_session" || runs.run.PromptVersion != ReviewPromptVersion {
		t.Fatalf("provenance=%#v", runs.run)
	}
}
func TestReviewSessionPersistsFailedAndCancelledRuns(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{errors.New("provider down"), clinicalairun.StatusFailed}, {context.Canceled, clinicalairun.StatusCancelled}} {
		tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		runs := &analysisRunRepo{}
		service := NewService(testProvider{err: tc.err}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{true}, nil).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", nil)
		_, err := service.ReviewSession(context.Background(), ReviewSessionInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, SessionText: "Material ficticio completo de una sesión para revisión."}, nil)
		if err == nil || runs.finishStatus != tc.want {
			t.Fatalf("err=%v status=%s want=%s", err, runs.finishStatus, tc.want)
		}
	}
}
func TestReviewValidationFailureCreatesNoRunAndCallsNoProvider(t *testing.T) {
	runs := &analysisRunRepo{}
	called := false
	service := NewService(testProvider{before: func() { called = true }}, testAppointments{}, testAccess{true}, nil).WithRunTracking(clinicalairun.NewService(runs), "ollama", "fixture", nil)
	_, err := service.ReviewSession(context.Background(), ReviewSessionInput{SessionText: "short"}, nil)
	if err == nil || called || runs.run.ID != uuid.Nil {
		t.Fatalf("err=%v provider=%v run=%#v", err, called, runs.run)
	}
}

type testAppointments struct{ appointment domainappointment.Entity }

func (r testAppointments) GetByID(context.Context, uuid.UUID, uuid.UUID) (domainappointment.Entity, error) {
	return r.appointment, nil
}

type testAccess struct{ allowed bool }

func (a testAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

type auditCall struct {
	action   string
	metadata map[string]any
}
type testAuditor struct{ calls []auditCall }

func (a *testAuditor) RecordDomainEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ *uuid.UUID, metadata map[string]any) error {
	a.calls = append(a.calls, auditCall{action: action, metadata: metadata})
	return nil
}

func TestAnalyzeLiveRequiresTreatingAssignment(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	service := NewService(testProvider{output: guardedResult()}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{allowed: false}, nil)
	_, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{
		TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio."},
	}, nil)
	if err != domainerrors.ErrForbidden {
		t.Fatalf("expected forbidden without treating assignment, got %v", err)
	}
}

func TestAnalyzeLiveAuditsWithoutClinicalContent(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	auditor := &testAuditor{}
	service := NewService(testProvider{output: guardedResult()}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{allowed: true}, auditor)
	output, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{
		TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID,
		Request: LiveRequest{Fragment: "Paciente ficticio expresa una duda."},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output.Result.Node == "" || len(auditor.calls) != 2 {
		t.Fatalf("expected result and start/complete audits, got %+v calls=%d", output, len(auditor.calls))
	}
	for _, call := range auditor.calls {
		encoded, _ := json.Marshal(call.metadata)
		if len(encoded) == 0 || containsClinicalText(string(encoded), "Paciente ficticio") {
			t.Fatalf("audit metadata must not contain fragment: %s", encoded)
		}
	}
}

func TestAnalyzeLiveReturnsConfiguredRiskProtocolOnlyOnRisk(t *testing.T) {
	tenantID, actorID, appointmentID, clientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	service := NewService(testProvider{output: guardedResult()}, testAppointments{appointment: domainappointment.Entity{ID: appointmentID, ClientID: clientID}}, testAccess{allowed: true}, nil).WithRiskProtocol("Circuito ficticio configurado por Fernando.")
	output, err := service.AnalyzeLive(context.Background(), AnalyzeLiveInput{TenantID: tenantID, ActorUserID: actorID, AppointmentID: appointmentID, Request: LiveRequest{Fragment: "Caso ficticio: no quiero vivir."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output.RiskProtocol == nil || *output.RiskProtocol != "Circuito ficticio configurado por Fernando." {
		t.Fatalf("missing configured risk protocol: %+v", output)
	}
}

func containsClinicalText(value, clinicalText string) bool {
	for i := 0; i+len(clinicalText) <= len(value); i++ {
		if value[i:i+len(clinicalText)] == clinicalText {
			return true
		}
	}
	return false
}
