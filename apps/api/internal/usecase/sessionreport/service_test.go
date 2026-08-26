package sessionreport

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
)

func validReport() ReportV1 {
	return ReportV1{SchemaVersion: SchemaVersion, Summary: "Resumen factual ficticio.", Facts: []Fact{{ID: "fact-001", Statement: "La persona informó ansiedad.", Category: "reported_symptom"}}, RelevantChanges: []RelevantChange{}, Interventions: []Intervention{}, PatientResponses: []PatientResponse{}, AffectiveNodes: []AffectiveNode{}, InferenceCandidates: []InferenceCandidate{{ID: "inference-001", Statement: "Podría anticipar conflicto.", EvidenceRefs: []string{"fact-001"}}}, HypothesisCandidates: []HypothesisCandidate{{ID: "hypothesis-001", Statement: "Hipótesis provisional.", TrafficLight: "yellow", EvidenceRefs: []string{"fact-001"}}}, SafetySignals: []SafetySignal{}, OpenQuestions: []OpenQuestion{}, LongitudinalCandidates: []LongitudinalCandidate{}}
}

type reportRepoStub struct {
	details                    SessionDetails
	report                     Report
	created, updated, approved bool
}

func (r *reportRepoStub) SessionDetails(context.Context, uuid.UUID, uuid.UUID) (SessionDetails, error) {
	return r.details, nil
}
func (r *reportRepoStub) CreateDraft(_ context.Context, tenantID, sessionID, actorID uuid.UUID, runID *uuid.UUID, document ReportV1) (Report, error) {
	r.created = true
	raw, _ := json.Marshal(document)
	r.report = Report{ID: uuid.New(), TenantID: tenantID, ClinicalSessionID: sessionID, Version: 1, Revision: 1, SchemaVersion: SchemaVersion, Status: "draft", ReportJSON: raw, CreatedByUserID: actorID, SourceAIRunID: runID}
	return r.report, nil
}
func (r *reportRepoStub) List(context.Context, uuid.UUID, uuid.UUID) ([]Report, error) {
	return []Report{r.report}, nil
}
func (r *reportRepoStub) Get(context.Context, uuid.UUID, uuid.UUID) (Report, error) {
	return r.report, nil
}
func (r *reportRepoStub) Update(_ context.Context, _, _, _ uuid.UUID, expected int, document ReportV1) (Report, error) {
	if expected != r.report.Revision {
		return Report{}, domainerrors.ErrConflict
	}
	r.updated = true
	r.report.Revision++
	raw, _ := json.Marshal(document)
	r.report.ReportJSON = raw
	return r.report, nil
}
func (r *reportRepoStub) Approve(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Report, error) {
	if r.report.Status != "draft" {
		return Report{}, domainerrors.ErrConflict
	}
	r.approved = true
	r.report.Status = "approved"
	return r.report, nil
}

type reportAccess struct{ allowed bool }

func (a reportAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

type reportProvider struct {
	raw    []byte
	err    error
	called bool
	before func()
}

func (p *reportProvider) GenerateSessionReport(context.Context, string, []byte, map[string]any) (ProviderOutput, error) {
	p.called = true
	if p.before != nil {
		p.before()
	}
	return ProviderOutput{JSON: p.raw}, p.err
}

type reportRunRepo struct {
	run      clinicalairun.Run
	finished string
}

func (r *reportRunRepo) Start(_ context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	r.run = run
	return run, nil
}
func (r *reportRunRepo) Finish(_ context.Context, _, _ uuid.UUID, status string, outputHash, errorCode *string, at time.Time) (clinicalairun.Run, error) {
	r.finished = status
	r.run.Status = status
	r.run.OutputHash = outputHash
	r.run.ErrorCode = errorCode
	r.run.CompletedAt = &at
	return r.run, nil
}
func (r *reportRunRepo) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]clinicalairun.Source, error) {
	return append([]clinicalairun.Source(nil), r.run.Sources...), nil
}
func newReportService(repo *reportRepoStub, provider *reportProvider, runs *reportRunRepo, allowed bool) *Service {
	return NewService(repo, reportAccess{allowed}, clinicalairun.NewService(runs), provider, "ollama", "fixture", map[string]any{"temperature": 0.1})
}
func TestSessionReportSchemaRejectsUnknownAndMissingVersion(t *testing.T) {
	raw, _ := json.Marshal(validReport())
	var object map[string]any
	_ = json.Unmarshal(raw, &object)
	object["unknown"] = "no"
	bad, _ := json.Marshal(object)
	if _, err := DecodeAndValidate(bad); err == nil {
		t.Fatal("unknown fields must fail")
	}
	report := validReport()
	report.SchemaVersion = ""
	if err := Validate(report); err == nil {
		t.Fatal("schema version required")
	}
}
func TestSessionReportKeepsFactsInferenceAndHypothesisSeparated(t *testing.T) {
	report := validReport()
	if err := Validate(report); err != nil {
		t.Fatal(err)
	}
	if report.Facts[0].Statement == report.InferenceCandidates[0].Statement || report.Facts[0].Statement == report.HypothesisCandidates[0].Statement {
		t.Fatal("epistemic categories collapsed")
	}
}
func TestStableItemIDsSurviveEditReorderAndDeletion(t *testing.T) {
	report := validReport()
	report.Facts = append(report.Facts, Fact{ID: "fact-002", Statement: "Segundo hecho", Category: "reported"})
	originalFirst := report.Facts[0].ID
	report.Facts[0].Statement = "Texto editado"
	report.Facts[0], report.Facts[1] = report.Facts[1], report.Facts[0]
	report.Facts = report.Facts[1:]
	if report.Facts[0].ID != originalFirst {
		t.Fatalf("remaining identity changed: %q", report.Facts[0].ID)
	}
	report.Interventions = append(report.Interventions, Intervention{Type: "reflection", Description: "Nueva intervención"})
	AssignMissingIDs(&report)
	if report.Interventions[0].ID != "intervention-001" {
		t.Fatalf("new item ID=%q", report.Interventions[0].ID)
	}
	if err := Validate(report); err != nil {
		t.Fatal(err)
	}
}
func TestStableItemIDsMustBeUnique(t *testing.T) {
	report := validReport()
	report.Facts = append(report.Facts, Fact{ID: report.Facts[0].ID, Statement: "otro", Category: "reported"})
	if err := Validate(report); err == nil {
		t.Fatal("duplicate report item ID must fail")
	}
}
func TestDeletedItemIDIsNotReusedForNewItem(t *testing.T) {
	stored := validReport()
	stored.Facts = append(stored.Facts, Fact{ID: "fact-002", Statement: "second", Category: "reported"})
	incoming := validReport()
	incoming.Facts = []Fact{{Statement: "brand new", Category: "reported"}}
	AssignMissingIDs(&incoming, reportItemIDs(stored)...)
	if incoming.Facts[0].ID != "fact-003" {
		t.Fatalf("deleted identity was reused: %q", incoming.Facts[0].ID)
	}
}
func TestUpdateAllocatesIdentityBeyondStoredReportHistory(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	stored := validReport()
	stored.Facts = append(stored.Facts, Fact{ID: "fact-002", Statement: "second", Category: "reported"})
	raw, _ := json.Marshal(stored)
	repo := &reportRepoStub{details: SessionDetails{ID: session, ClientID: client}, report: Report{ID: uuid.New(), TenantID: tenant, ClinicalSessionID: session, Revision: 1, Status: "draft", ReportJSON: raw}}
	incoming := validReport()
	incoming.Facts = []Fact{{Statement: "new", Category: "reported"}}
	out, err := newReportService(repo, &reportProvider{}, &reportRunRepo{}, true).Update(context.Background(), UpdateInput{TenantID: tenant, ReportID: repo.report.ID, ActorUserID: actor, ExpectedRevision: 1, Report: incoming})
	if err != nil {
		t.Fatal(err)
	}
	var updated ReportV1
	if err := json.Unmarshal(out.ReportJSON, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Facts[0].ID != "fact-003" {
		t.Fatalf("stored item identity reused: %q", updated.Facts[0].ID)
	}
}
func TestLegacySessionReportRemainsReadable(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	legacy := []byte(`{"schema_version":"session-report-v1","summary":"legacy","facts":[],"relevant_changes":[],"interventions":[],"patient_responses":[],"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":[],"safety_signals":[],"open_questions":[],"longitudinal_candidates":[]}`)
	repo := &reportRepoStub{details: SessionDetails{ID: session, ClientID: client}, report: Report{ID: uuid.New(), TenantID: tenant, ClinicalSessionID: session, SchemaVersion: LegacySchemaVersion, Status: "approved", ReportJSON: legacy}}
	out, err := newReportService(repo, &reportProvider{}, &reportRunRepo{}, true).Get(context.Background(), tenant, repo.report.ID, actor)
	if err != nil || string(out.ReportJSON) != string(legacy) {
		t.Fatalf("legacy report not readable: out=%#v err=%v", out, err)
	}
}
func TestGenerateCreatesRunAndDraft(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &reportRepoStub{details: SessionDetails{ID: session, TenantID: tenant, ClientID: client, Status: "completed"}}
	raw, _ := json.Marshal(validReport())
	runs := &reportRunRepo{}
	provider := &reportProvider{raw: raw, before: func() {
		if runs.run.Status != clinicalairun.StatusRunning {
			t.Fatalf("provider called before run: %#v", runs.run)
		}
	}}
	item, err := newReportService(repo, provider, runs, true).Generate(context.Background(), GenerateInput{TenantID: tenant, SessionID: session, ActorUserID: actor, SessionText: "Material clínico ficticio de longitud suficiente."})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.created || item.Status != "draft" || item.SourceAIRunID == nil || runs.finished != clinicalairun.StatusSucceeded {
		t.Fatalf("item=%#v run=%#v", item, runs)
	}
}
func TestSessionReportCancellationPersistsCancelledRun(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &reportRepoStub{details: SessionDetails{ID: session, TenantID: tenant, ClientID: client, Status: "completed"}}
	runs := &reportRunRepo{}
	provider := &reportProvider{err: context.Canceled}
	_, err := newReportService(repo, provider, runs, true).Generate(context.Background(), GenerateInput{TenantID: tenant, SessionID: session, ActorUserID: actor, SessionText: "Material clínico ficticio de longitud suficiente."})
	if !errors.Is(err, context.Canceled) || runs.finished != clinicalairun.StatusCancelled || repo.created {
		t.Fatalf("err=%v run=%s created=%v", err, runs.finished, repo.created)
	}
}
func TestSessionReportProviderFailurePersistsFailedRun(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &reportRepoStub{details: SessionDetails{ID: session, TenantID: tenant, ClientID: client, Status: "completed"}}
	runs := &reportRunRepo{}
	provider := &reportProvider{err: errors.New("provider down")}
	_, err := newReportService(repo, provider, runs, true).Generate(context.Background(), GenerateInput{TenantID: tenant, SessionID: session, ActorUserID: actor, SessionText: "Material clínico ficticio de longitud suficiente."})
	if err == nil || runs.finished != clinicalairun.StatusFailed || repo.created {
		t.Fatalf("err=%v run=%s created=%v", err, runs.finished, repo.created)
	}
}
func TestInvalidInputDoesNotInvokeInference(t *testing.T) {
	repo := &reportRepoStub{}
	provider := &reportProvider{}
	runs := &reportRunRepo{}
	_, err := newReportService(repo, provider, runs, true).Generate(context.Background(), GenerateInput{SessionText: "short"})
	if err == nil || provider.called || runs.run.ID != uuid.Nil {
		t.Fatalf("err=%v provider=%v run=%#v", err, provider.called, runs.run)
	}
}
func TestInvalidOutputFailsRunAndDoesNotPersistReport(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &reportRepoStub{details: SessionDetails{ID: session, ClientID: client, Status: "completed"}}
	provider := &reportProvider{raw: []byte(`{"schema_version":"session-report-v1","summary":"missing sections"}`)}
	runs := &reportRunRepo{}
	_, err := newReportService(repo, provider, runs, true).Generate(context.Background(), GenerateInput{TenantID: tenant, SessionID: session, ActorUserID: actor, SessionText: "Material clínico ficticio de longitud suficiente."})
	if err == nil || repo.created || runs.finished != clinicalairun.StatusFailed {
		t.Fatalf("err=%v created=%v run=%s", err, repo.created, runs.finished)
	}
}
func TestSessionReportCAWriteAndOptimisticConflict(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	raw, _ := json.Marshal(validReport())
	repo := &reportRepoStub{details: SessionDetails{ID: session, ClientID: client, Status: "completed"}, report: Report{ID: uuid.New(), TenantID: tenant, ClinicalSessionID: session, Revision: 2, Status: "draft", ReportJSON: raw}}
	service := newReportService(repo, &reportProvider{}, &reportRunRepo{}, false)
	if _, err := service.Update(context.Background(), UpdateInput{TenantID: tenant, ReportID: repo.report.ID, ActorUserID: actor, ExpectedRevision: 2, Report: validReport()}); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
	service = newReportService(repo, &reportProvider{}, &reportRunRepo{}, true)
	if _, err := service.Update(context.Background(), UpdateInput{TenantID: tenant, ReportID: repo.report.ID, ActorUserID: actor, ExpectedRevision: 1, Report: validReport()}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
}
func TestApprovingSessionReportDoesNotMutateApprovedLongitudinalFormulation(t *testing.T) {
	tenant, session, actor, client := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	state := struct{ Snapshots, Anchors, Suggestions []string }{[]string{"approved-v3"}, []string{"fact-a", "hypothesis-b"}, []string{"accepted-s1", "pending-s2"}}
	before := struct{ Snapshots, Anchors, Suggestions []string }{append([]string{}, state.Snapshots...), append([]string{}, state.Anchors...), append([]string{}, state.Suggestions...)}
	repo := &reportRepoStub{details: SessionDetails{ID: session, ClientID: client}, report: Report{ID: uuid.New(), TenantID: tenant, ClinicalSessionID: session, Status: "draft"}}
	if _, err := newReportService(repo, &reportProvider{}, &reportRunRepo{}, true).Approve(context.Background(), tenant, repo.report.ID, actor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, state) {
		t.Fatalf("approved longitudinal state mutated: before=%#v after=%#v", before, state)
	}
}
