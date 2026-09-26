package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	approvedcontext "sessionflow/apps/api/internal/usecase/approvedcontext"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

func TestConcurrentReportApprovalCommitsOneStateAndAuditPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, appointment, session := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'approval race')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Fictitious patient')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW()-INTERVAL '1 hour',NOW(),'scheduled')`, appointment, tenant, client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW()-INTERVAL '1 hour',NOW())`, session, tenant, client, appointment, user)
	repo := NewSessionReportRepository(pool)
	draft, err := repo.CreateDraft(ctx, tenant, session, user, nil, validRepositoryReport())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			ready.Done()
			<-start
			_, approveErr := repo.Approve(ctx, tenant, draft.ID, user, draft.Revision)
			results <- approveErr
		}()
	}
	ready.Wait()
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		switch result := <-results; {
		case result == nil:
			successes++
		case errors.Is(result, domainerrors.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected approval result: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	var approvedCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM session_reports WHERE tenant_id=$1 AND clinical_session_id=$2 AND status='approved'`, tenant, session).Scan(&approvedCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND entity_id=$2 AND action='session_report.approved'`, tenant, draft.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Get(ctx, tenant, draft.ID)
	if err != nil || approvedCount != 1 || auditCount != 1 || stored.Revision != draft.Revision+1 {
		t.Fatalf("approved=%d audit=%d stored=%#v err=%v", approvedCount, auditCount, stored, err)
	}
}

func TestSessionReportVersionApprovalAndLongitudinalIsolationPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, other, user, client, appointment, session := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'report tenant'),($2,'other')`, tenant, other)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@report.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'report client')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW()-INTERVAL '1 hour',NOW(),'scheduled')`, appointment, tenant, client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW()-INTERVAL '1 hour',NOW())`, session, tenant, client, appointment, user)
	memory := NewClinicalMemoryRepository(pool)
	suggestion, err := memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{TenantID: tenant, ClientID: client, AppointmentID: appointment, ActorUserID: user, Mode: "live", PromptVersion: "clinical-live-v1", Result: json.RawMessage(`{"mode":"live"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.DecideSuggestion(ctx, tenant, suggestion.ID, user, "accepted", "", "reviewed"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := memory.CreateSnapshot(ctx, tenant, client, user, "Approved formulation", []clinicalmemory.Anchor{{SourceID: "fact-1", Kind: "fact", Summary: "Direct fact"}, {SourceID: "hyp-1", Kind: "hypothesis", Summary: "Provisional", TrafficLight: stringPointer("yellow"), SourceSuggestionID: &suggestion.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.ApproveSnapshot(ctx, tenant, client, snapshot.ID, user); err != nil {
		t.Fatal(err)
	}
	before := longitudinalFingerprint(t, pool, ctx, tenant, client)
	repo := NewSessionReportRepository(pool)
	document := validRepositoryReport()
	draft, err := repo.CreateDraft(ctx, tenant, session, user, nil, document)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := repo.Approve(ctx, tenant, draft.ID, user, draft.Revision)
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approved=%#v err=%v", approved, err)
	}
	if _, err := repo.Approve(ctx, tenant, draft.ID, user, draft.Revision); err == nil {
		t.Fatal("double approval must conflict")
	}
	runRepo := NewClinicalAIRunRepository(pool)
	runService := clinicalairun.NewService(runRepo)
	run, err := runService.Start(ctx, clinicalairun.StartInput{TenantID: tenant, ClientID: client, AppointmentID: &appointment, ClinicalSessionID: &session, CreatedByUserID: user, Provider: "ollama", Model: "fixture", Operation: "historical_context_test", PromptName: "fixture", PromptVersion: "v1", Parameters: map[string]any{}, Input: "ephemeral", Sources: []clinicalairun.Source{{SourceType: clinicalairun.SourceSessionReport, SourceID: approved.ID, SourceVersion: &approved.Version}}})
	if err != nil {
		t.Fatal(err)
	}
	after := longitudinalFingerprint(t, pool, ctx, tenant, client)
	if before != after {
		t.Fatalf("APPROVING A SESSION REPORT MUTATED LONGITUDINAL FORMULATION\nbefore=%s\nafter=%s", before, after)
	}
	if _, err := repo.Get(ctx, other, draft.ID); err == nil {
		t.Fatal("cross-tenant report read must fail")
	}
	newDocument := document
	newDocument.Summary = "Human corrected summary"
	next, err := repo.Update(ctx, tenant, draft.ID, user, approved.Revision, newDocument)
	if err != nil || next.ID == draft.ID || next.Version != 2 || next.Status != "draft" {
		t.Fatalf("next=%#v err=%v", next, err)
	}
	if _, err := repo.Update(ctx, tenant, next.ID, user, next.Revision+1, newDocument); err == nil {
		t.Fatal("lost update must conflict")
	}
	newDocument.Summary = "Second clinician edit before approval"
	current, err := repo.Update(ctx, tenant, next.ID, user, next.Revision, newDocument)
	if err != nil || current.Revision != next.Revision+1 {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	if _, err := repo.Approve(ctx, tenant, next.ID, user, next.Revision); err == nil {
		t.Fatal("stale report approval must conflict")
	}
	latest, err := repo.Approve(ctx, tenant, next.ID, user, current.Revision)
	if err != nil || latest.Status != "approved" {
		t.Fatal(err)
	}
	old, err := repo.Get(ctx, tenant, draft.ID)
	if err != nil || old.Status != "superseded" || old.ReportJSON == nil {
		t.Fatalf("approved report was overwritten or not superseded: %#v err=%v", old, err)
	}
	historicalSources, err := runService.ListSources(ctx, tenant, run.ID)
	if err != nil || len(historicalSources) != 1 || historicalSources[0].SourceID != old.ID || historicalSources[0].SourceVersion == nil || *historicalSources[0].SourceVersion != old.Version {
		t.Fatalf("historical superseded source lost: %#v err=%v", historicalSources, err)
	}
	if _, err := repo.CreateDraft(ctx, tenant, session, user, nil, document); err != nil {
		t.Fatal(err)
	}
	pending, err := memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{TenantID: tenant, ClientID: client, AppointmentID: appointment, ActorUserID: user, Mode: "live", PromptVersion: "clinical-live-v1", Result: json.RawMessage(`{"mode":"live"}`)})
	if err != nil {
		t.Fatal(err)
	}
	discarded, err := memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{TenantID: tenant, ClientID: client, AppointmentID: appointment, ActorUserID: user, Mode: "live", PromptVersion: "clinical-live-v1", Result: json.RawMessage(`{"mode":"live"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.DecideSuggestion(ctx, tenant, discarded.ID, user, "discarded", "", "not supported"); err != nil {
		t.Fatal(err)
	}
	_ = pending
	if _, err := memory.CreateSnapshot(ctx, tenant, client, user, "Draft must stay excluded", []clinicalmemory.Anchor{{SourceID: "draft-fact", Kind: "fact", Summary: "Draft fact"}, {SourceID: "draft-hyp", Kind: "hypothesis", Summary: "Draft hypothesis", TrafficLight: stringPointer("red")}}); err != nil {
		t.Fatal(err)
	}
	approvedOnly, err := approvedcontext.NewService(memory, repo, dbApprovedContextAccess{}).Get(ctx, tenant, client, user)
	if err != nil {
		t.Fatal(err)
	}
	if approvedOnly.Formulation == nil || approvedOnly.Formulation.SnapshotID != snapshot.ID {
		t.Fatalf("draft formulation leaked: %#v", approvedOnly.Formulation)
	}
	if len(approvedOnly.SessionReports) != 1 || approvedOnly.SessionReports[0].ID != latest.ID {
		t.Fatalf("draft/superseded report leaked: %#v", approvedOnly.SessionReports)
	}
}

type dbApprovedContextAccess struct{}

func (dbApprovedContextAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return true, nil
}
func validRepositoryReport() sessionreport.ReportV1 {
	return sessionreport.ReportV1{SchemaVersion: sessionreport.SchemaVersion, Summary: "Fictitious summary", Facts: []sessionreport.Fact{}, RelevantChanges: []sessionreport.RelevantChange{}, Interventions: []sessionreport.Intervention{}, PatientResponses: []sessionreport.PatientResponse{}, AffectiveNodes: []sessionreport.AffectiveNode{}, InferenceCandidates: []sessionreport.InferenceCandidate{}, HypothesisCandidates: []sessionreport.HypothesisCandidate{}, SafetySignals: []sessionreport.SafetySignal{}, OpenQuestions: []sessionreport.OpenQuestion{}, LongitudinalCandidates: []sessionreport.LongitudinalCandidate{}}
}
func longitudinalFingerprint(t *testing.T, pool *pgxpool.Pool, ctx context.Context, tenantID, clientID uuid.UUID) string {
	t.Helper()
	var value string
	query := `SELECT jsonb_build_object('snapshots',(SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM (SELECT id,version,status,approved_summary FROM clinical_formulation_snapshots WHERE tenant_id=$1 AND client_id=$2 ORDER BY id)x),'anchors',(SELECT COALESCE(jsonb_agg(to_jsonb(y)),'[]') FROM (SELECT a.id,a.snapshot_id,a.source_id,a.kind,a.summary FROM clinical_formulation_anchors a JOIN clinical_formulation_snapshots s ON s.tenant_id=a.tenant_id AND s.id=a.snapshot_id WHERE a.tenant_id=$1 AND s.client_id=$2 ORDER BY a.id)y),'suggestions',(SELECT COALESCE(jsonb_agg(to_jsonb(z)),'[]') FROM (SELECT id,disposition,correction_text FROM clinical_ai_suggestions WHERE tenant_id=$1 AND client_id=$2 ORDER BY id)z))::text`
	if err := pool.QueryRow(ctx, query, tenantID, clientID).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
