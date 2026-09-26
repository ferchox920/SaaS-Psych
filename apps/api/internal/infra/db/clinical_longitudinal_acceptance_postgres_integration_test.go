package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	approvedcontext "sessionflow/apps/api/internal/usecase/approvedcontext"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type longitudinalAcceptanceAccess struct{ allowed bool }

func (a longitudinalAcceptanceAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

type longitudinalAcceptanceProvider struct {
	pool    *pgxpool.Pool
	tenant  uuid.UUID
	output  []byte
	err     error
	called  int
	running bool
	inputs  []longitudinal.InterpreterInput
}

func (p *longitudinalAcceptanceProvider) InterpretLongitudinal(_ context.Context, _ string, input longitudinal.InterpreterInput, _ func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	p.called++
	p.inputs = append(p.inputs, input)
	var status string
	if err := p.pool.QueryRow(context.Background(), `SELECT status FROM clinical_ai_runs WHERE tenant_id=$1 AND operation='longitudinal_interpretation' ORDER BY created_at DESC,id DESC LIMIT 1`, p.tenant).Scan(&status); err == nil && status == clinicalairun.StatusRunning {
		p.running = true
	}
	if p.err != nil {
		return clinicalanalysis.ProviderOutput{}, p.err
	}
	return clinicalanalysis.ProviderOutput{JSON: p.output}, nil
}

type longitudinalAcceptanceFixture struct {
	tenant, user, client, appointment, session, report uuid.UUID
	reportJSON                                         json.RawMessage
}

func newLongitudinalAcceptanceFixture(t *testing.T, pool *pgxpool.Pool) longitudinalAcceptanceFixture {
	t.Helper()
	ctx := context.Background()
	f := longitudinalAcceptanceFixture{tenant: uuid.New(), user: uuid.New(), client: uuid.New(), appointment: uuid.New(), session: uuid.New(), report: uuid.New()}
	f.reportJSON = json.RawMessage(`{"schema_version":"session-report-v1.1","summary":"Caso ficticio de aceptación.","facts":[{"id":"fact-001","statement":"La persona informó una mejora sostenida.","category":"reported_change"},{"id":"fact-002","statement":"La persona describió temor al rechazo.","category":"patient_report"},{"id":"fact-003","statement":"La persona describió una regla moral contra confrontar.","category":"patient_report"},{"id":"fact-004","statement":"La persona agregó contexto nuevo sin cambiar hipótesis existentes.","category":"patient_report"}],"relevant_changes":[],"interventions":[],"patient_responses":[{"id":"response-001","response_type":"correction","description":"La persona corrigió la interpretación previa."}],"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":[],"safety_signals":[],"open_questions":[],"longitudinal_candidates":[]}`)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'stage 2b.1 acceptance')`, f.tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, f.user, f.tenant, f.user.String()+"@stage2b1.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Fictitious longitudinal client')`, f.client, f.tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW()-INTERVAL '1 hour',NOW(),'completed')`, f.appointment, f.tenant, f.client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW()-INTERVAL '1 hour',NOW())`, f.session, f.tenant, f.client, f.appointment, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO session_reports(id,tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,1,'session-report-v1.1','approved',$4,$5,$5,NOW())`, f.report, f.tenant, f.session, f.reportJSON, f.user)
	return f
}

func (f longitudinalAcceptanceFixture) addApprovedSessionReport(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	appointment, session, report := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','completed')`, appointment, f.tenant, f.client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW(),NOW()+INTERVAL '1 hour')`, session, f.tenant, f.client, appointment, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO session_reports(id,tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,1,'session-report-v1.1','approved',$4,$5,$5,NOW())`, report, f.tenant, session, f.reportJSON, f.user)
	return session, report
}

func newLongitudinalAcceptanceService(pool *pgxpool.Pool, provider longitudinal.Provider, allowed bool) (*longitudinal.Service, *ClinicalLongitudinalRepository, *ClinicalAIRunRepository) {
	access := longitudinalAcceptanceAccess{allowed: allowed}
	longitudinalRepo := NewClinicalLongitudinalRepository(pool)
	runRepo := NewClinicalAIRunRepository(pool)
	runs := clinicalairun.NewService(runRepo).WithBuildInfo("stage-2b.1-test", "acceptance")
	approved := approvedcontext.NewService(NewClinicalMemoryRepository(pool), NewSessionReportRepository(pool), access)
	service := longitudinal.NewService(longitudinalRepo, access, approved, runs, provider, nil, "ollama", "fixture", map[string]any{"temperature": 0.1})
	return service, longitudinalRepo, runRepo
}

func longitudinalStateFingerprint(t *testing.T, pool *pgxpool.Pool, tenant, client uuid.UUID) string {
	t.Helper()
	var fingerprint string
	query := `SELECT jsonb_build_object(
		'evidence',jsonb_build_array((SELECT count(*) FROM clinical_evidence WHERE tenant_id=$1 AND client_id=$2),(SELECT COALESCE(sum(version),0) FROM clinical_evidence WHERE tenant_id=$1 AND client_id=$2)),
		'events',jsonb_build_array((SELECT count(*) FROM clinical_events WHERE tenant_id=$1 AND client_id=$2),(SELECT COALESCE(sum(version),0) FROM clinical_events WHERE tenant_id=$1 AND client_id=$2)),
		'processes',jsonb_build_array((SELECT count(*) FROM clinical_processes WHERE tenant_id=$1 AND client_id=$2),(SELECT COALESCE(sum(version),0) FROM clinical_processes WHERE tenant_id=$1 AND client_id=$2)),
		'hypotheses',jsonb_build_array((SELECT count(*) FROM clinical_hypotheses WHERE tenant_id=$1 AND client_id=$2),(SELECT COALESCE(sum(version),0) FROM clinical_hypotheses WHERE tenant_id=$1 AND client_id=$2)),
		'event_evidence',(SELECT count(*) FROM clinical_event_evidence WHERE tenant_id=$1 AND client_id=$2),
		'process_events',(SELECT count(*) FROM clinical_process_events WHERE tenant_id=$1 AND client_id=$2),
		'hypothesis_evidence',(SELECT count(*) FROM clinical_hypothesis_evidence WHERE tenant_id=$1 AND client_id=$2),
		'head',COALESCE((SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2),0),
		'transitions',(SELECT count(*) FROM clinical_longitudinal_transitions WHERE tenant_id=$1 AND client_id=$2)
	)::text`
	if err := pool.QueryRow(context.Background(), query, tenant, client).Scan(&fingerprint); err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func stage2BArtifactFingerprint(t *testing.T, pool *pgxpool.Pool, tenant, client uuid.UUID) string {
	t.Helper()
	var fingerprint string
	query := `SELECT jsonb_build_object(
		'evidence',(SELECT count(*) FROM clinical_evidence WHERE tenant_id=$1 AND client_id=$2),
		'events',(SELECT count(*) FROM clinical_events WHERE tenant_id=$1 AND client_id=$2),
		'processes',(SELECT count(*) FROM clinical_processes WHERE tenant_id=$1 AND client_id=$2),
		'hypotheses',(SELECT count(*) FROM clinical_hypotheses WHERE tenant_id=$1 AND client_id=$2),
		'heads',(SELECT count(*) FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2),
		'diffs',(SELECT count(*) FROM clinical_diffs WHERE tenant_id=$1 AND client_id=$2),
		'operations',(SELECT count(*) FROM clinical_diff_operations WHERE tenant_id=$1 AND client_id=$2)
	)::text`
	if err := pool.QueryRow(context.Background(), query, tenant, client).Scan(&fingerprint); err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func startAcceptanceRun(t *testing.T, pool *pgxpool.Pool, f longitudinalAcceptanceFixture) uuid.UUID {
	t.Helper()
	run := uuid.New()
	hash := "0000000000000000000000000000000000000000000000000000000000000000"
	mustExecIntegrationSQL(t, pool, context.Background(), `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,input_hash,context_hash,status,started_at) VALUES($1,$2,$3,$4,$5,$6,'ollama','fixture','longitudinal_interpretation','clinical-longitudinal-interpreter','clinical-longitudinal-interpreter-v1',$7,$7,'running',NOW())`, run, f.tenant, f.client, f.appointment, f.session, f.user, hash)
	return run
}

func mergeAcceptanceOperations(t *testing.T, pool *pgxpool.Pool, f longitudinalAcceptanceFixture, operations []longitudinal.Operation) longitudinal.Diff {
	t.Helper()
	ctx := context.Background()
	repo := NewClinicalLongitudinalRepository(pool)
	state, err := repo.State(ctx, f.tenant, f.client)
	if err != nil {
		t.Fatal(err)
	}
	run := startAcceptanceRun(t, pool, f)
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.tenant, ClientID: f.client, SessionID: f.session, ReportID: f.report, RunID: run, ActorID: f.user, BaseStateVersion: state.StateVersion, Operations: operations, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range diff.Operations {
		diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	diff, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: diff.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return diff
}

func encodeAcceptanceProposal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func acceptanceIntPointer(value int) *int { return &value }

func TestReviewedApprovedDiffDoesNotMutateLongitudinalStateBeforeMerge(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	repo := NewClinicalLongitudinalRepository(pool)
	run := startAcceptanceRun(t, pool, f)
	evidenceID := uuid.New()
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.tenant, ClientID: f.client, SessionID: f.session, ReportID: f.report, RunID: run, ActorID: f.user, BaseStateVersion: 0, Operations: []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", 1)})
	if err != nil {
		t.Fatal(err)
	}
	beforeReview := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
	diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: diff.ID, OperationID: diff.Operations[0].ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
	if err != nil || diff.Status != "approved" {
		t.Fatalf("reviewed diff=%#v err=%v", diff, err)
	}
	afterReview := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
	if beforeReview != afterReview {
		t.Fatalf("human review mutated longitudinal state\nbefore=%s\nafter=%s", beforeReview, afterReview)
	}
}

func TestLongitudinalAnalyzeCreatesDiffWithoutMutatingState(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	evidenceID, eventID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	result := longitudinal.InterpreterResult{Operations: []longitudinal.ProposedOperation{
		{ID: "evidence", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{ID: "event", OperationType: "create_event", TargetEntityID: &eventID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Mejora informada", Description: "Cambio informado.", ObservedAt: now, EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "process", OperationType: "create_process", TargetEntityID: &processID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso longitudinal", Description: "Seguimiento ficticio.", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "link", OperationType: "link_event_process", TargetEntityID: &processID, ExpectedEntityVersion: acceptanceIntPointer(1), Proposal: encodeAcceptanceProposal(t, longitudinal.LinkEventProcessProposal{EventID: eventID, ProcessID: processID, EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "hypothesis", OperationType: "create_hypothesis", TargetEntityID: &hypothesisID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "La mejora podría sostenerse.", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	}, Uncertainties: []longitudinal.Uncertainty{}}
	provider := &longitudinalAcceptanceProvider{pool: pool, tenant: f.tenant, output: encodeAcceptanceProposal(t, result)}
	service, repo, runRepo := newLongitudinalAcceptanceService(pool, provider, true)
	initial := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
	out, err := service.Analyze(ctx, f.tenant, f.session, f.user)
	if err != nil {
		t.Fatal(err)
	}
	if provider.called != 1 || !provider.running || len(provider.inputs) != 1 || provider.inputs[0].SchemaVersion != longitudinal.PromptVersion || provider.inputs[0].ApprovedContext.ClientID != f.client || len(provider.inputs[0].SessionReport) == 0 || out.Diff.Status != "pending_review" || out.Diff.SourceAIRunID == nil {
		t.Fatalf("provider/run/diff boundary not observed: called=%d running=%t out=%#v", provider.called, provider.running, out)
	}
	if afterAnalyze := longitudinalStateFingerprint(t, pool, f.tenant, f.client); initial != afterAnalyze {
		t.Fatalf("Analyze mutated state\nbefore=%s\nafter=%s", initial, afterAnalyze)
	}
	for _, op := range out.Diff.Operations {
		out.Diff, err = service.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: out.Diff.ID, OperationID: op.ID, ActorID: f.user, ExpectedDiffRevision: out.Diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if afterReview := longitudinalStateFingerprint(t, pool, f.tenant, f.client); initial != afterReview {
		t.Fatalf("review mutated state\nbefore=%s\nafter=%s", initial, afterReview)
	}
	merged, err := service.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: out.Diff.ID, ActorID: f.user, ExpectedDiffRevision: out.Diff.Revision})
	if err != nil || merged.Status != "merged" {
		t.Fatalf("merge=%#v err=%v", merged, err)
	}
	if afterMerge := longitudinalStateFingerprint(t, pool, f.tenant, f.client); initial == afterMerge {
		t.Fatal("human merge did not mutate state")
	}
	state, err := repo.State(ctx, f.tenant, f.client)
	if err != nil || len(state.ActiveEvidence) != 1 || len(state.RecentEvents) != 1 || len(state.Processes) != 1 || len(state.Processes[0].Hypotheses) != 1 {
		t.Fatalf("merged state=%#v err=%v", state, err)
	}
	runSources, err := runRepo.ListSources(ctx, f.tenant, *out.Diff.SourceAIRunID)
	if err != nil || len(runSources) != 1 || runSources[0].SourceType != clinicalairun.SourceSessionReport {
		t.Fatalf("initial longitudinal sources=%#v err=%v", runSources, err)
	}

	secondSession, _ := f.addApprovedSessionReport(t, pool)
	provider.output = []byte(`{"operations":[],"uncertainties":[{"type":"insufficient_evidence","question":"No hay cambio adicional verificable.","evidence_ids":[]}]}`)
	provider.running = false
	second, err := service.Analyze(ctx, f.tenant, secondSession, f.user)
	if err != nil || second.Diff.SourceAIRunID == nil || !provider.running || len(provider.inputs) != 2 || len(provider.inputs[1].CurrentState.ActiveEvidence) != 1 || len(provider.inputs[1].CurrentState.RecentEvents) != 1 || len(provider.inputs[1].CurrentState.Processes) != 1 {
		t.Fatalf("second analysis=%#v running=%t err=%v", second, provider.running, err)
	}
	sources, err := runRepo.ListSources(ctx, f.tenant, *second.Diff.SourceAIRunID)
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]bool{clinicalairun.SourceSessionReport: false, clinicalairun.SourceClinicalEvidence: false, clinicalairun.SourceClinicalEvent: false, clinicalairun.SourceClinicalProcess: false, clinicalairun.SourceClinicalHypothesis: false}
	for _, source := range sources {
		if _, ok := wantTypes[source.SourceType]; ok && source.SourceVersion != nil && *source.SourceVersion > 0 {
			wantTypes[source.SourceType] = true
		}
	}
	for sourceType, found := range wantTypes {
		if !found {
			t.Fatalf("missing versioned longitudinal AI source %s: %#v", sourceType, sources)
		}
	}
}

func TestLongitudinalAIRunFailureCancellationAndInvalidOutput(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	cases := []struct {
		name, expectedStatus, expectedCode string
		providerErr                        error
		output                             []byte
	}{
		{name: "provider_failure", expectedStatus: "failed", expectedCode: "provider_error", providerErr: errors.New("provider unavailable")},
		{name: "cancellation", expectedStatus: "cancelled", expectedCode: "cancelled", providerErr: context.Canceled},
		{name: "invalid_output", expectedStatus: "failed", expectedCode: "invalid_model_output", output: []byte(`{"operations":"invalid","uncertainties":[]}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			f := newLongitudinalAcceptanceFixture(t, pool)
			provider := &longitudinalAcceptanceProvider{pool: pool, tenant: f.tenant, output: tc.output, err: tc.providerErr}
			service, _, _ := newLongitudinalAcceptanceService(pool, provider, true)
			before := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
			if _, err = service.Analyze(ctx, f.tenant, f.session, f.user); err == nil {
				t.Fatal("expected analysis failure")
			}
			var status, code string
			var diffs int
			if err = pool.QueryRow(ctx, `SELECT status,error_code FROM clinical_ai_runs WHERE tenant_id=$1 AND operation='longitudinal_interpretation' ORDER BY created_at DESC,id DESC LIMIT 1`, f.tenant).Scan(&status, &code); err != nil {
				t.Fatal(err)
			}
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_diffs WHERE tenant_id=$1 AND client_id=$2`, f.tenant, f.client).Scan(&diffs); err != nil {
				t.Fatal(err)
			}
			if !provider.running || status != tc.expectedStatus || code != tc.expectedCode || diffs != 0 || before != longitudinalStateFingerprint(t, pool, f.tenant, f.client) {
				t.Fatalf("running=%t status=%s code=%s diffs=%d", provider.running, status, code, diffs)
			}
		})
	}
}

func TestClinicalLongitudinalMergeRollsBackAfterPartialApplication(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	repo := NewClinicalLongitudinalRepository(pool)
	processID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,approved_by_user_id,approved_at,opened_at) VALUES($1,$2,$3,'Existing process','Original description','approved','active',$4,$4,NOW(),NOW())`, processID, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id,revision) VALUES($1,$2,0)`, f.tenant, f.client)
	run := startAcceptanceRun(t, pool, f)
	evidenceID := uuid.New()
	wrongVersion := 99
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.tenant, ClientID: f.client, SessionID: f.session, ReportID: f.report, RunID: run, ActorID: f.user, BaseStateVersion: 0, Operations: []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "update_process", TargetEntityID: processID, ExpectedEntityVersion: &wrongVersion, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.UpdateProcessProposal{Title: "Should roll back", Description: "Should never persist.", EvidenceIDs: []uuid.UUID{evidenceID}})},
	}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", 2)})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range diff.Operations {
		diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: diff.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("merge error=%v", err)
	}
	var evidenceCount, transitionCount int
	var title, description, diffStatus string
	var processVersion int
	var head int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_evidence WHERE tenant_id=$1 AND id=$2`, f.tenant, evidenceID).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT title,description,version FROM clinical_processes WHERE tenant_id=$1 AND id=$2`, f.tenant, processID).Scan(&title, &description, &processVersion); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2`, f.tenant, f.client).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM clinical_diffs WHERE tenant_id=$1 AND id=$2`, f.tenant, diff.ID).Scan(&diffStatus); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_longitudinal_transitions WHERE tenant_id=$1 AND diff_id=$2`, f.tenant, diff.ID).Scan(&transitionCount); err != nil {
		t.Fatal(err)
	}
	if evidenceCount != 0 || title != "Existing process" || description != "Original description" || processVersion != 1 || head != 0 || diffStatus != "approved" || transitionCount != 0 {
		t.Fatalf("partial effects remained: evidence=%d process=%q/%q v%d head=%d diff=%s transitions=%d", evidenceCount, title, description, processVersion, head, diffStatus, transitionCount)
	}
}

func TestHypothesisCanPreserveSupportingAndContradictingEvidence(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	e1, e2, e3, processID, h1, h2 := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: e1, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_evidence", TargetEntityID: e2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-002", EpistemicType: "patient_report"})},
		{OperationType: "create_evidence", TargetEntityID: e3, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-003", EpistemicType: "patient_report"})},
		{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso con explicaciones competidoras", Description: "Fixture ficticio.", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{e1}})},
		{OperationType: "create_hypothesis", TargetEntityID: h1, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "Miedo al rechazo.", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{e1, e2}, ContradictingEvidenceIDs: []uuid.UUID{e3}})},
		{OperationType: "create_hypothesis", TargetEntityID: h2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "Regla moral contra confrontar.", ConfidenceLevel: "red", SupportingEvidenceIDs: []uuid.UUID{e3}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	})
	repo := NewClinicalLongitudinalRepository(pool)
	state, err := repo.State(ctx, f.tenant, f.client)
	if err != nil || len(state.Processes) != 1 || len(state.Processes[0].Hypotheses) != 2 {
		t.Fatalf("competing hypotheses state=%#v err=%v", state, err)
	}
	var first longitudinal.Hypothesis
	for _, hypothesis := range state.Processes[0].Hypotheses {
		if hypothesis.ID == h1 {
			first = hypothesis
		}
	}
	if first.ID == uuid.Nil || len(first.SupportingEvidence) != 2 || len(first.ContradictingEvidence) != 1 {
		t.Fatalf("supporting/contradicting evidence was not preserved: %#v", first)
	}
	processHistory, err := repo.GetProcessHistory(ctx, f.tenant, f.client, processID)
	if err != nil || len(processHistory.Transitions) != 1 || processHistory.Transitions[0].MergedByUserID == nil || *processHistory.Transitions[0].MergedByUserID != f.user || len(processHistory.Process.Hypotheses) != 2 {
		t.Fatalf("process history=%#v err=%v", processHistory, err)
	}
	hypothesisHistory, err := repo.GetHypothesisHistory(ctx, f.tenant, f.client, h1)
	if err != nil || len(hypothesisHistory.Transitions) != 1 || len(hypothesisHistory.Hypothesis.SupportingEvidence) != 2 || len(hypothesisHistory.Hypothesis.ContradictingEvidence) != 1 {
		t.Fatalf("hypothesis history=%#v err=%v", hypothesisHistory, err)
	}
}

func TestHypothesisLifecycleAndEvidenceDoesNotAutoChangeConfidence(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	evidenceID, laterEvidenceID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Lifecycle process", Description: "Fixture.", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{evidenceID}})},
		{OperationType: "create_hypothesis", TargetEntityID: hypothesisID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "Lifecycle hypothesis.", ConfidenceLevel: "red", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	})
	transition := func(kind string, expected int) {
		mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: kind, TargetEntityID: hypothesisID, ExpectedEntityVersion: &expected, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.TransitionHypothesisProposal{EvidenceIDs: []uuid.UUID{evidenceID}})}})
	}
	assertHypothesis := func(version int, confidence, status string) {
		items, e := NewClinicalLongitudinalRepository(pool).ListHypotheses(ctx, f.tenant, f.client)
		if e != nil || len(items) != 1 || items[0].Version != version || items[0].ConfidenceLevel != confidence || items[0].ClinicalStatus != status {
			t.Fatalf("hypothesis after transition=%#v err=%v", items, e)
		}
	}
	transition("strengthen_hypothesis", 1)
	assertHypothesis(2, "yellow", "active")
	transition("strengthen_hypothesis", 2)
	assertHypothesis(3, "green", "active")
	transition("weaken_hypothesis", 3)
	assertHypothesis(4, "yellow", "weakened")
	transition("weaken_hypothesis", 4)
	assertHypothesis(5, "red", "weakened")
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: laterEvidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-004", EpistemicType: "patient_report"})}})
	assertHypothesis(5, "red", "weakened")
	transition("retire_hypothesis", 5)
	assertHypothesis(6, "red", "retired")
	history, err := NewClinicalLongitudinalRepository(pool).GetHypothesisHistory(ctx, f.tenant, f.client, hypothesisID)
	if err != nil || len(history.Transitions) != 6 {
		t.Fatalf("hypothesis lifecycle history=%#v err=%v", history, err)
	}
}

func TestProcessCloseReopenLifecyclePreservesHistory(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	evidenceID, processID := uuid.New(), uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Closable process", Description: "Fixture.", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{evidenceID}})},
	})
	version := 1
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "close_process", TargetEntityID: processID, ExpectedEntityVersion: &version, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.TransitionProcessProposal{EvidenceIDs: []uuid.UUID{evidenceID}})}})
	items, err := NewClinicalLongitudinalRepository(pool).ListProcesses(ctx, f.tenant, f.client)
	if err != nil || len(items) != 1 || items[0].ClinicalStatus != "closed" || items[0].Version != 2 || items[0].ClosedAt == nil {
		t.Fatalf("closed process=%#v err=%v", items, err)
	}
	version = 2
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "reopen_process", TargetEntityID: processID, ExpectedEntityVersion: &version, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.TransitionProcessProposal{EvidenceIDs: []uuid.UUID{evidenceID}, ReopenStatus: "active"})}})
	items, err = NewClinicalLongitudinalRepository(pool).ListProcesses(ctx, f.tenant, f.client)
	if err != nil || items[0].ClinicalStatus != "active" || items[0].Version != 3 || items[0].ClosedAt != nil || items[0].OpenedAt == nil {
		t.Fatalf("reopened process=%#v err=%v", items, err)
	}
	history, err := NewClinicalLongitudinalRepository(pool).GetProcessHistory(ctx, f.tenant, f.client, processID)
	if err != nil || len(history.Transitions) != 3 || history.Transitions[1].Action != "close_process" || history.Transitions[2].Action != "reopen_process" {
		t.Fatalf("process lifecycle history=%#v err=%v", history, err)
	}
}

func TestLongitudinalFixturesAThroughGUseRealService(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	raw, err := os.ReadFile("../../usecase/longitudinal/testdata/longitudinal_eval_fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name          string                           `json:"name"`
		Operations    []longitudinal.ProposedOperation `json:"operations"`
		Uncertainties []longitudinal.Uncertainty       `json:"uncertainties"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 7 {
		t.Fatalf("fixtures=%d", len(fixtures))
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			pool, e := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
			if e != nil {
				t.Fatal(e)
			}
			defer pool.Close()
			f := newLongitudinalAcceptanceFixture(t, pool)
			fixture.Operations = seedAcceptanceFixtureState(t, pool, f, fixture.Name, fixture.Operations)
			output, _ := json.Marshal(longitudinal.InterpreterResult{Operations: fixture.Operations, Uncertainties: fixture.Uncertainties})
			provider := &longitudinalAcceptanceProvider{pool: pool, tenant: f.tenant, output: output}
			service, _, _ := newLongitudinalAcceptanceService(pool, provider, true)
			before := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
			analysis, e := service.Analyze(ctx, f.tenant, f.session, f.user)
			if e != nil {
				t.Fatal(e)
			}
			if !provider.running || len(provider.inputs) != 1 || provider.inputs[0].SchemaVersion != longitudinal.PromptVersion || provider.inputs[0].ApprovedContext.ClientID != f.client || analysis.Diff.Status != "pending_review" || len(analysis.Diff.Operations) != len(fixture.Operations) || before != longitudinalStateFingerprint(t, pool, f.tenant, f.client) {
				t.Fatalf("fixture did not traverse run/diff boundary safely: %#v", analysis)
			}
			var status string
			if e = pool.QueryRow(ctx, `SELECT status FROM clinical_ai_runs WHERE tenant_id=$1 AND id=$2`, f.tenant, *analysis.Diff.SourceAIRunID).Scan(&status); e != nil || status != "succeeded" {
				t.Fatalf("run status=%s err=%v", status, e)
			}
			switch fixture.Name {
			case "A_existing_process_link":
				assertOperationKinds(t, analysis.Diff, []string{"link_event_process"}, []string{"create_process"})
			case "B_possible_new_process":
				assertOperationKinds(t, analysis.Diff, []string{"create_process"}, nil)
			case "C_contradicted_hypothesis":
				assertOneOfOperationKinds(t, analysis.Diff, "weaken_hypothesis", "link_contradicting_evidence")
			case "D_insufficient_evidence":
				if len(analysis.Diff.Operations) != 0 || len(analysis.Diff.Uncertainties) == 0 {
					t.Fatalf("insufficient evidence fixture=%#v", analysis.Diff)
				}
			case "E_report_hypothesis_remains_candidate":
				assertOperationKinds(t, analysis.Diff, nil, []string{"create_hypothesis"})
			case "F_patient_correction":
				assertOperationKinds(t, analysis.Diff, []string{"link_contradicting_evidence"}, nil)
			case "G_competing_hypotheses":
				count := 0
				for _, op := range analysis.Diff.Operations {
					if op.OperationType == "create_hypothesis" {
						count++
					}
				}
				if count < 2 {
					t.Fatalf("competing hypothesis count=%d", count)
				}
			}
		})
	}
}

func seedAcceptanceFixtureState(t *testing.T, pool *pgxpool.Pool, f longitudinalAcceptanceFixture, name string, operations []longitudinal.ProposedOperation) []longitudinal.ProposedOperation {
	t.Helper()
	if len(operations) == 0 {
		return operations
	}
	e1, e2, eventID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedEvidence := func(two bool) []longitudinal.Operation {
		seed := []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: e1, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-002", EpistemicType: "patient_report"})}}
		if two {
			seed = append(seed, longitudinal.Operation{OperationType: "create_evidence", TargetEntityID: e2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "response-001", EpistemicType: "patient_report"})})
		}
		return seed
	}
	switch name {
	case "A_existing_process_link":
		seed := append(seedEvidence(false),
			longitudinal.Operation{OperationType: "create_event", TargetEntityID: eventID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Cambio", Description: "Cambio aprobado.", ObservedAt: time.Now().UTC(), EvidenceIDs: []uuid.UUID{e1}})},
			longitudinal.Operation{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso existente", Description: "Fixture.", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{e1}})},
		)
		mergeAcceptanceOperations(t, pool, f, seed)
		version := 1
		operations[0].TargetEntityID = &processID
		operations[0].ExpectedEntityVersion = &version
		operations[0].Proposal = encodeAcceptanceProposal(t, longitudinal.LinkEventProcessProposal{EventID: eventID, ProcessID: processID, EvidenceIDs: []uuid.UUID{e1}})
	case "B_possible_new_process":
		mergeAcceptanceOperations(t, pool, f, seedEvidence(false))
		operations[0].Proposal = encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso posible", Description: "Requiere revisión humana.", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{e1}})
	case "C_contradicted_hypothesis", "F_patient_correction":
		seed := append(seedEvidence(true), longitudinal.Operation{OperationType: "create_hypothesis", TargetEntityID: hypothesisID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{Statement: "Hipótesis previa.", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{e1}, ContradictingEvidenceIDs: []uuid.UUID{}})})
		mergeAcceptanceOperations(t, pool, f, seed)
		version := 1
		operations[0].TargetEntityID = &hypothesisID
		operations[0].ExpectedEntityVersion = &version
		if name == "C_contradicted_hypothesis" {
			operations[0].Proposal = encodeAcceptanceProposal(t, longitudinal.TransitionHypothesisProposal{EvidenceIDs: []uuid.UUID{e2}})
		} else {
			operations[0].Proposal = encodeAcceptanceProposal(t, longitudinal.LinkHypothesisEvidenceProposal{HypothesisID: hypothesisID, EvidenceID: e2, EvidenceIDs: []uuid.UUID{e2}})
		}
	case "G_competing_hypotheses":
		seed := append(seedEvidence(true), longitudinal.Operation{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso común", Description: "Fixture.", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{e1}})})
		mergeAcceptanceOperations(t, pool, f, seed)
		evidenceIDs := []uuid.UUID{e1, e2}
		for i := range operations {
			operations[i].Proposal = encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: fmt.Sprintf("Explicación competidora %d.", i+1), ConfidenceLevel: "red", SupportingEvidenceIDs: []uuid.UUID{evidenceIDs[i]}, ContradictingEvidenceIDs: []uuid.UUID{}})
		}
	}
	return operations
}

func assertOperationKinds(t *testing.T, diff longitudinal.Diff, required, forbidden []string) {
	t.Helper()
	kinds := map[string]bool{}
	for _, op := range diff.Operations {
		kinds[op.OperationType] = true
	}
	for _, kind := range required {
		if !kinds[kind] {
			t.Fatalf("missing operation %s in %#v", kind, kinds)
		}
	}
	for _, kind := range forbidden {
		if kinds[kind] {
			t.Fatalf("forbidden operation %s in %#v", kind, kinds)
		}
	}
}

func assertOneOfOperationKinds(t *testing.T, diff longitudinal.Diff, alternatives ...string) {
	t.Helper()
	for _, op := range diff.Operations {
		for _, alternative := range alternatives {
			if op.OperationType == alternative {
				return
			}
		}
	}
	t.Fatalf("none of %v found in %#v", alternatives, diff.Operations)
}

func TestClinicalLongitudinalTenantAndClientIsolationMatrix(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	a1 := newLongitudinalAcceptanceFixture(t, pool)
	b1 := newLongitudinalAcceptanceFixture(t, pool)
	a2Client, a2Appointment, a2Session, a2Report := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Tenant A client 2')`, a2Client, a1.tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','completed')`, a2Appointment, a1.tenant, a2Client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW(),NOW()+INTERVAL '1 hour')`, a2Session, a1.tenant, a2Client, a2Appointment, a1.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO session_reports(id,tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,1,'session-report-v1.1','approved',$4,$5,$5,NOW())`, a2Report, a1.tenant, a2Session, a1.reportJSON, a1.user)

	insertEvidence := func(tenant, client, report, actor uuid.UUID) uuid.UUID {
		id := uuid.New()
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.','active',$5)`, id, tenant, client, report, actor)
		return id
	}
	eA1 := insertEvidence(a1.tenant, a1.client, a1.report, a1.user)
	eA2 := insertEvidence(a1.tenant, a2Client, a2Report, a1.user)
	eB1 := insertEvidence(b1.tenant, b1.client, b1.report, b1.user)
	if items, e := NewClinicalLongitudinalRepository(pool).ListEvidence(ctx, a1.tenant, b1.client); e != nil || len(items) != 0 {
		t.Fatalf("tenant A read tenant B evidence: %#v err=%v", items, e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO clinical_evidence(tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,'session_report',$3,1,'fact-002','patient_report','La persona describió temor al rechazo.','active',$4)`, a1.tenant, a1.client, a2Report, a1.user); e == nil {
		t.Fatal("client A1 sourced evidence from A2 report")
	}
	if _, e := pool.Exec(ctx, `INSERT INTO clinical_evidence(tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,'session_report',$3,1,'fact-002','patient_report','La persona describió temor al rechazo.','active',$4)`, a1.tenant, a1.client, b1.report, a1.user); e == nil {
		t.Fatal("tenant A sourced evidence from tenant B report")
	}

	insertApprovedEvent := func(tenant, client, actor, evidence uuid.UUID) uuid.UUID {
		id := uuid.New()
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, e = tx.Exec(ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,'other','Fixture','Fixture',NOW(),'approved',$4,$4,NOW())`, id, tenant, client, actor); e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4)`, tenant, client, id, evidence); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		return id
	}
	eventA1 := insertApprovedEvent(a1.tenant, a1.client, a1.user, eA1)
	eventA2 := insertApprovedEvent(a1.tenant, a2Client, a1.user, eA2)
	eventB1 := insertApprovedEvent(b1.tenant, b1.client, b1.user, eB1)
	for name, foreignEvidence := range map[string]uuid.UUID{"A2": eA2, "B1": eB1} {
		if _, e := pool.Exec(ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4)`, a1.tenant, a1.client, eventA1, foreignEvidence); e == nil {
			t.Fatalf("Event A1 linked %s evidence", name)
		}
	}
	processA1, processA2, processB1 := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,approved_by_user_id,approved_at,opened_at) VALUES($1,$2,$3,'P','P','approved','active',$4,$4,NOW(),NOW()),($5,$2,$6,'P2','P2','approved','active',$4,$4,NOW(),NOW())`, processA1, a1.tenant, a1.client, a1.user, processA2, a2Client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,approved_by_user_id,approved_at,opened_at) VALUES($1,$2,$3,'PB','PB','approved','active',$4,$4,NOW(),NOW())`, processB1, b1.tenant, b1.client, b1.user)
	for name, foreignEvent := range map[string]uuid.UUID{"A2": eventA2, "B1": eventB1} {
		if _, e := pool.Exec(ctx, `INSERT INTO clinical_process_events(tenant_id,client_id,process_id,event_id) VALUES($1,$2,$3,$4)`, a1.tenant, a1.client, processA1, foreignEvent); e == nil {
			t.Fatalf("Process A1 linked %s event", name)
		}
	}
	if _, e := pool.Exec(ctx, `INSERT INTO clinical_hypotheses(tenant_id,client_id,process_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,'cross client','proposed','active','red',$4,NULL,NULL)`, a1.tenant, a1.client, processA2, a1.user); e == nil {
		t.Fatal("Hypothesis A1 referenced Process A2")
	}
	hypothesisA1 := uuid.New()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,process_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,'H','approved','active','yellow',$5,$5,NOW())`, hypothesisA1, a1.tenant, a1.client, processA1, a1.user); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,'supporting')`, a1.tenant, a1.client, hypothesisA1, eA1); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for name, foreignEvidence := range map[string]uuid.UUID{"A2": eA2, "B1": eB1} {
		if _, e := pool.Exec(ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,'contradicting')`, a1.tenant, a1.client, hypothesisA1, foreignEvidence); e == nil {
			t.Fatalf("Hypothesis A1 linked %s evidence", name)
		}
	}
	if _, e := pool.Exec(ctx, `INSERT INTO clinical_diffs(tenant_id,client_id,clinical_session_id,source_session_report_id,status,base_state_version,created_by_user_id) VALUES($1,$2,$3,$4,'pending_review',0,$5)`, a1.tenant, a1.client, a2Session, a2Report, a1.user); e == nil {
		t.Fatal("Diff A1 referenced A2 session/report")
	}

	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id,revision) VALUES($1,$2,0),($1,$3,0),($4,$5,0) ON CONFLICT DO NOTHING`, a1.tenant, a1.client, a2Client, b1.tenant, b1.client)
	run := startAcceptanceRun(t, pool, a1)
	wrongTargetVersion := 1
	repo := NewClinicalLongitudinalRepository(pool)
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: a1.tenant, ClientID: a1.client, SessionID: a1.session, ReportID: a1.report, RunID: run, ActorID: a1.user, BaseStateVersion: 0, Operations: []longitudinal.Operation{{OperationType: "update_process", TargetEntityID: processA2, ExpectedEntityVersion: &wrongTargetVersion, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.UpdateProcessProposal{Title: "cross", Description: "cross", EvidenceIDs: []uuid.UUID{eA1}})}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", 7)})
	if err != nil {
		t.Fatal(err)
	}
	diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: a1.tenant, DiffID: diff.ID, OperationID: diff.Operations[0].ID, ActorID: a1.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: a1.tenant, DiffID: diff.ID, ActorID: a1.user, ExpectedDiffRevision: diff.Revision}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("cross-client merge err=%v", err)
	}
	var title string
	if err = pool.QueryRow(ctx, `SELECT title FROM clinical_processes WHERE tenant_id=$1 AND id=$2`, a1.tenant, processA2).Scan(&title); err != nil || title != "P2" {
		t.Fatalf("cross-client process mutated title=%s err=%v", title, err)
	}
}

func TestSessionReportApprovalDoesNotMutateStage2BArtifacts(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	appointment, session := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','completed')`, appointment, f.tenant, f.client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW(),NOW()+INTERVAL '1 hour')`, session, f.tenant, f.client, appointment, f.user)
	document := sessionreport.ReportV1{SchemaVersion: sessionreport.SchemaVersion, Summary: "Fictitious report awaiting approval.", Facts: []sessionreport.Fact{{ID: "fact-001", Statement: "Fictitious fact.", Category: "patient_report"}}, RelevantChanges: []sessionreport.RelevantChange{}, Interventions: []sessionreport.Intervention{}, PatientResponses: []sessionreport.PatientResponse{}, AffectiveNodes: []sessionreport.AffectiveNode{}, InferenceCandidates: []sessionreport.InferenceCandidate{}, HypothesisCandidates: []sessionreport.HypothesisCandidate{{ID: "hypothesis-001", Statement: "Candidate only.", TrafficLight: "yellow", EvidenceRefs: []string{"fact-001"}}}, SafetySignals: []sessionreport.SafetySignal{}, OpenQuestions: []sessionreport.OpenQuestion{}, LongitudinalCandidates: []sessionreport.LongitudinalCandidate{}}
	reports := NewSessionReportRepository(pool)
	draft, err := reports.CreateDraft(ctx, f.tenant, session, f.user, nil, document)
	if err != nil {
		t.Fatal(err)
	}
	before := stage2BArtifactFingerprint(t, pool, f.tenant, f.client)
	if _, err = reports.Approve(ctx, f.tenant, draft.ID, f.user, draft.Revision); err != nil {
		t.Fatal(err)
	}
	after := stage2BArtifactFingerprint(t, pool, f.tenant, f.client)
	if before != after {
		t.Fatalf("SessionReport approval mutated Stage 2B artifacts\nbefore=%s\nafter=%s", before, after)
	}
}

func TestEvidenceVersionProvenanceRemainsBoundToOriginalReportVersion(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	e1 := uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: e1, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})}})
	reports := NewSessionReportRepository(pool)
	updatedDocument := sessionreport.ReportV1{}
	if err = json.Unmarshal(f.reportJSON, &updatedDocument); err != nil {
		t.Fatal(err)
	}
	updatedDocument.Facts[0].Statement = "La persona informó una mejora con contexto actualizado."
	draftV2, err := reports.Update(ctx, f.tenant, f.report, f.user, 1, updatedDocument)
	if err != nil || draftV2.Version != 2 {
		t.Fatalf("draft v2=%#v err=%v", draftV2, err)
	}
	reportV2, err := reports.Approve(ctx, f.tenant, draftV2.ID, f.user, draftV2.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var sourceID uuid.UUID
	var sourceVersion int
	var sourceItem, statement string
	if err = pool.QueryRow(ctx, `SELECT source_id,source_version,source_item_id,statement FROM clinical_evidence WHERE tenant_id=$1 AND id=$2`, f.tenant, e1).Scan(&sourceID, &sourceVersion, &sourceItem, &statement); err != nil {
		t.Fatal(err)
	}
	if sourceID != f.report || sourceVersion != 1 || sourceItem != "fact-001" || statement != "La persona informó una mejora sostenida." {
		t.Fatalf("v1 evidence changed source=%s v%d item=%s statement=%q", sourceID, sourceVersion, sourceItem, statement)
	}
	e2 := uuid.New()
	fV2 := f
	fV2.report = reportV2.ID
	fV2.reportJSON, _ = json.Marshal(updatedDocument)
	mergeAcceptanceOperations(t, pool, fV2, []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: e2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})}})
	if err = pool.QueryRow(ctx, `SELECT source_id,source_version,source_item_id,statement FROM clinical_evidence WHERE tenant_id=$1 AND id=$2`, f.tenant, e2).Scan(&sourceID, &sourceVersion, &sourceItem, &statement); err != nil {
		t.Fatal(err)
	}
	if sourceID != reportV2.ID || sourceVersion != 2 || sourceItem != "fact-001" || statement != updatedDocument.Facts[0].Statement {
		t.Fatalf("v2 evidence source=%s v%d item=%s statement=%q", sourceID, sourceVersion, sourceItem, statement)
	}
}

func TestEvidenceDuplicateInvalidationAndSupersessionLifecycle(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	e1 := uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: e1, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})}})
	repo := NewClinicalLongitudinalRepository(pool)
	state, _ := repo.State(ctx, f.tenant, f.client)
	run := startAcceptanceRun(t, pool, f)
	duplicate := uuid.New()
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.tenant, ClientID: f.client, SessionID: f.session, ReportID: f.report, RunID: run, ActorID: f.user, BaseStateVersion: state.StateVersion, Operations: []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: duplicate, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", 8)})
	if err != nil {
		t.Fatal(err)
	}
	diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: diff.ID, OperationID: diff.Operations[0].ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: diff.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("duplicate merge err=%v", err)
	}
	var activeCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_evidence WHERE tenant_id=$1 AND client_id=$2 AND source_id=$3 AND source_version=1 AND source_item_id='fact-001' AND status='active'`, f.tenant, f.client, f.report).Scan(&activeCount); err != nil || activeCount != 1 {
		t.Fatalf("active duplicate count=%d err=%v", activeCount, err)
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_diffs SET status='rejected',updated_at=NOW() WHERE tenant_id=$1 AND id=$2`, f.tenant, diff.ID)
	version := 1
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "invalidate_evidence", TargetEntityID: e1, ExpectedEntityVersion: &version, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.InvalidateEvidenceProposal{EvidenceIDs: []uuid.UUID{e1}})}})
	items, err := repo.ListEvidence(ctx, f.tenant, f.client)
	if err != nil || len(items) != 1 || items[0].Status != "invalidated" || items[0].Version != 2 {
		t.Fatalf("invalidated evidence=%#v err=%v", items, err)
	}
	e2 := uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{{OperationType: "create_evidence", TargetEntityID: e2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-002", EpistemicType: "patient_report"})}})
	if _, err = pool.Exec(ctx, `UPDATE clinical_evidence SET status='superseded',version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND id=$2`, f.tenant, e2); err != nil {
		t.Fatal(err)
	}
	var status, item, preservedStatement string
	var evidenceVersion int
	if err = pool.QueryRow(ctx, `SELECT status,version,source_item_id,statement FROM clinical_evidence WHERE tenant_id=$1 AND id=$2`, f.tenant, e2).Scan(&status, &evidenceVersion, &item, &preservedStatement); err != nil {
		t.Fatal(err)
	}
	if status != "superseded" || evidenceVersion != 2 || item != "fact-002" || preservedStatement != "La persona describió temor al rechazo." {
		t.Fatalf("superseded evidence status=%s v%d item=%s statement=%q", status, evidenceVersion, item, preservedStatement)
	}
}

func TestGranularHumanReviewAppliesApprovedAndModifiedButNotRejected(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	repo := NewClinicalLongitudinalRepository(pool)
	run := startAcceptanceRun(t, pool, f)
	evidenceID, eventID, rejectedProcessID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	originalEvent := encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "AI title", Description: "AI description", ObservedAt: now, EvidenceIDs: []uuid.UUID{evidenceID}})
	humanEvent := encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Human title", Description: "Human description", ObservedAt: now, EvidenceIDs: []uuid.UUID{evidenceID}})
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.tenant, ClientID: f.client, SessionID: f.session, ReportID: f.report, RunID: run, ActorID: f.user, BaseStateVersion: 0, Operations: []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_event", TargetEntityID: eventID, OriginalProposal: originalEvent},
		{OperationType: "create_process", TargetEntityID: rejectedProcessID, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Rejected", Description: "Rejected", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{evidenceID}})},
	}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", 9)})
	if err != nil {
		t.Fatal(err)
	}
	before := longitudinalStateFingerprint(t, pool, f.tenant, f.client)
	decisions := []struct {
		decision     string
		modification json.RawMessage
	}{{decision: "approved"}, {decision: "modified", modification: humanEvent}, {decision: "rejected"}}
	for i, op := range diff.Operations {
		diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision, Decision: decisions[i].decision, Modification: decisions[i].modification})
		if err != nil {
			t.Fatal(err)
		}
	}
	if before != longitudinalStateFingerprint(t, pool, f.tenant, f.client) {
		t.Fatal("granular review mutated state before merge")
	}
	if diff.Status != "approved" || diff.Operations[0].ReviewStatus != "approved" || diff.Operations[1].ReviewStatus != "modified" || diff.Operations[2].ReviewStatus != "rejected" || string(diff.Operations[1].OriginalProposal) == string(diff.Operations[1].HumanModification) || diff.Operations[1].ReviewedByUserID == nil || diff.Operations[1].ReviewedAt == nil {
		t.Fatalf("review provenance=%#v", diff)
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: diff.ID, ActorID: f.user, ExpectedDiffRevision: diff.Revision}); err != nil {
		t.Fatal(err)
	}
	var eventTitle, eventDescription string
	var processCount, correctedAudit int
	if err = pool.QueryRow(ctx, `SELECT title,description FROM clinical_events WHERE tenant_id=$1 AND id=$2`, f.tenant, eventID).Scan(&eventTitle, &eventDescription); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_processes WHERE tenant_id=$1 AND id=$2`, f.tenant, rejectedProcessID).Scan(&processCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND action='clinical_event.corrected' AND entity_id=$2`, f.tenant, eventID).Scan(&correctedAudit); err != nil {
		t.Fatal(err)
	}
	if eventTitle != "Human title" || eventDescription != "Human description" || processCount != 0 || correctedAudit != 1 {
		t.Fatalf("merge results title=%q description=%q rejected_process=%d corrected_audit=%d", eventTitle, eventDescription, processCount, correctedAudit)
	}
}

func TestApprovedEventAndHypothesisRequireEvidenceAtCommit(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	if _, err = pool.Exec(ctx, `INSERT INTO clinical_events(tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,'other','No evidence','No evidence',NOW(),'approved',$3,$3,NOW())`, f.tenant, f.client, f.user); err == nil {
		t.Fatal("approved Event without Evidence committed")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinical_hypotheses(tenant_id,client_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,'No support','approved','active','red',$3,$3,NOW())`, f.tenant, f.client, f.user); err == nil {
		t.Fatal("approved Hypothesis without supporting Evidence committed")
	}
}

func TestLongitudinalStateExclusionMatrixAndDeterminism(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	activeEvidence, approvedEvent, approvedProcess, approvedHypothesis := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mergeAcceptanceOperations(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: activeEvidence, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_event", TargetEntityID: approvedEvent, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Approved event", Description: "Approved event", ObservedAt: time.Now().UTC(), EvidenceIDs: []uuid.UUID{activeEvidence}})},
		{OperationType: "create_process", TargetEntityID: approvedProcess, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Approved process", Description: "Approved process", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{activeEvidence}})},
		{OperationType: "create_hypothesis", TargetEntityID: approvedHypothesis, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &approvedProcess, Statement: "Approved hypothesis", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{activeEvidence}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	})
	invalidatedEvidence := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,version,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-002','patient_report','La persona describió temor al rechazo.','invalidated',2,$5)`, invalidatedEvidence, f.tenant, f.client, f.report, f.user)
	proposedEvent, rejectedEvent := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,$3,'other','Proposed','Proposed',NOW(),'proposed',$4)`, proposedEvent, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id,approved_at) VALUES($1,$2,$3,'other','Rejected','Rejected',NOW(),'rejected',$4,NOW())`, rejectedEvent, f.tenant, f.client, f.user)
	proposedProcess, rejectedProcess := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Proposed','Proposed','proposed','observing',$4)`, proposedProcess, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,approved_at) VALUES($1,$2,$3,'Rejected','Rejected','rejected','observing',$4,NOW())`, rejectedProcess, f.tenant, f.client, f.user)
	proposedHypothesis, rejectedHypothesis := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id) VALUES($1,$2,$3,'Proposed','proposed','active','red',$4)`, proposedHypothesis, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id,approved_at) VALUES($1,$2,$3,'Rejected','rejected','active','red',$4,NOW())`, rejectedHypothesis, f.tenant, f.client, f.user)
	pendingDiff, rejectedDiff, mergedDiff := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,status,base_state_version,created_by_user_id) VALUES($1,$2,$3,'pending_review',1,$4),($5,$2,$3,'rejected',1,$4),($6,$2,$3,'merged',0,$4)`, pendingDiff, f.tenant, f.client, f.user, rejectedDiff, mergedDiff)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diff_operations(tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,original_proposal,review_status,reviewed_by_user_id,reviewed_at) VALUES($1,$2,$3,1,'create_process',$4,$5,'rejected',$6,NOW())`, f.tenant, f.client, rejectedDiff, uuid.New(), encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Rejected op", Description: "Rejected op", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{activeEvidence}}), f.user)
	lowID := uuid.New()
	highID := lowID
	lowID[15] = 0
	highID[15] = 255
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,approved_by_user_id,approved_at,opened_at,created_at,updated_at) VALUES($1,$3,$4,'High UUID','Determinism','approved','active',$5,$5,'2026-01-01','2026-01-01','2026-01-01','2026-01-01'),($2,$3,$4,'Low UUID','Determinism','approved','active',$5,$5,'2026-01-01','2026-01-01','2026-01-01','2026-01-01')`, highID, lowID, f.tenant, f.client, f.user)
	repo := NewClinicalLongitudinalRepository(pool)
	state1, err := repo.State(ctx, f.tenant, f.client)
	if err != nil {
		t.Fatal(err)
	}
	state2, err := repo.State(ctx, f.tenant, f.client)
	if err != nil {
		t.Fatal(err)
	}
	state3, err := repo.State(ctx, f.tenant, f.client)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state1, state2) || !reflect.DeepEqual(state2, state3) {
		t.Fatalf("repeated state differs\n1=%#v\n2=%#v\n3=%#v", state1, state2, state3)
	}
	if len(state1.ActiveEvidence) != 1 || state1.ActiveEvidence[0].ID != activeEvidence || len(state1.RecentEvents) != 1 || state1.RecentEvents[0].ID != approvedEvent || len(state1.OpenProposals) != 1 || state1.OpenProposals[0].ID != pendingDiff {
		t.Fatalf("state exclusion failure: %#v", state1)
	}
	processIDs := map[uuid.UUID]int{}
	for i, process := range state1.Processes {
		processIDs[process.ID] = i
		if process.ID == proposedProcess || process.ID == rejectedProcess {
			t.Fatalf("unapproved process leaked: %#v", process)
		}
	}
	if _, ok := processIDs[approvedProcess]; !ok || processIDs[highID] >= processIDs[lowID] {
		t.Fatalf("deterministic UUID ordering missing: %#v", processIDs)
	}
	for _, hypothesis := range state1.UnassignedHypotheses {
		if hypothesis.ID == proposedHypothesis || hypothesis.ID == rejectedHypothesis {
			t.Fatalf("unapproved hypothesis leaked: %#v", hypothesis)
		}
	}
	if len(state1.Processes[processIDs[approvedProcess]].Hypotheses) != 1 || state1.Processes[processIDs[approvedProcess]].Hypotheses[0].ID != approvedHypothesis {
		t.Fatalf("approved hypothesis missing: %#v", state1.Processes[processIDs[approvedProcess]])
	}
}

func TestLongitudinalProvenanceEndToEndReachesSessionReportSourceAIRun(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	appointment, session := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','completed')`, appointment, f.tenant, f.client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW(),NOW()+INTERVAL '1 hour')`, session, f.tenant, f.client, appointment, f.user)
	runService := clinicalairun.NewService(NewClinicalAIRunRepository(pool)).WithBuildInfo("stage-2b.1-source", "source-build")
	sourceRun, err := runService.Start(ctx, clinicalairun.StartInput{TenantID: f.tenant, ClientID: f.client, AppointmentID: &appointment, ClinicalSessionID: &session, CreatedByUserID: f.user, Provider: "ollama", Model: "source-fixture-model", Operation: "generate_session_report", PromptName: "session-report", PromptVersion: "session-report-v1.1", Parameters: map[string]any{"temperature": 0.1}, Input: map[string]any{"fixture": true}, Context: map[string]any{"fixture": true}, Sources: []clinicalairun.Source{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runService.Succeed(ctx, f.tenant, sourceRun.ID, map[string]any{"report": "fictitious"}); err != nil {
		t.Fatal(err)
	}
	document := sessionreport.ReportV1{SchemaVersion: sessionreport.SchemaVersion, Summary: "Source-run provenance fixture.", Facts: []sessionreport.Fact{{ID: "fact-001", Statement: "La persona informó una mejora con provenance completa.", Category: "patient_report"}}, RelevantChanges: []sessionreport.RelevantChange{}, Interventions: []sessionreport.Intervention{}, PatientResponses: []sessionreport.PatientResponse{}, AffectiveNodes: []sessionreport.AffectiveNode{}, InferenceCandidates: []sessionreport.InferenceCandidate{}, HypothesisCandidates: []sessionreport.HypothesisCandidate{}, SafetySignals: []sessionreport.SafetySignal{}, OpenQuestions: []sessionreport.OpenQuestion{}, LongitudinalCandidates: []sessionreport.LongitudinalCandidate{}}
	reports := NewSessionReportRepository(pool)
	draft, err := reports.CreateDraft(ctx, f.tenant, session, f.user, &sourceRun.ID, document)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reports.Approve(ctx, f.tenant, draft.ID, f.user, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f2 := f
	f2.appointment, f2.session, f2.report = appointment, session, report.ID
	f2.reportJSON, _ = json.Marshal(document)
	evidenceID, eventID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	result := longitudinal.InterpreterResult{Operations: []longitudinal.ProposedOperation{
		{ID: "e", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{ID: "event", OperationType: "create_event", TargetEntityID: &eventID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Provenance event", Description: "Fictitious event.", ObservedAt: time.Now().UTC(), EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "process", OperationType: "create_process", TargetEntityID: &processID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Provenance process", Description: "Fictitious process.", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "process-event", OperationType: "link_event_process", TargetEntityID: &processID, ExpectedEntityVersion: acceptanceIntPointer(1), Proposal: encodeAcceptanceProposal(t, longitudinal.LinkEventProcessProposal{EventID: eventID, ProcessID: processID, EvidenceIDs: []uuid.UUID{evidenceID}})},
		{ID: "hypothesis", OperationType: "create_hypothesis", TargetEntityID: &hypothesisID, Proposal: encodeAcceptanceProposal(t, longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "Provenance hypothesis.", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	}, Uncertainties: []longitudinal.Uncertainty{}}
	output, _ := json.Marshal(result)
	provider := &longitudinalAcceptanceProvider{pool: pool, tenant: f.tenant, output: output}
	service, repo, _ := newLongitudinalAcceptanceService(pool, provider, true)
	analysis, err := service.Analyze(ctx, f.tenant, session, f.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range analysis.Diff.Operations {
		analysis.Diff, err = service.Decide(ctx, longitudinal.DecisionInput{TenantID: f.tenant, DiffID: analysis.Diff.ID, OperationID: op.ID, ActorID: f.user, ExpectedDiffRevision: analysis.Diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	merged, err := service.Merge(ctx, longitudinal.MergeInput{TenantID: f.tenant, DiffID: analysis.Diff.ID, ActorID: f.user, ExpectedDiffRevision: analysis.Diff.Revision})
	if err != nil || merged.Status != "merged" {
		t.Fatalf("merged=%#v err=%v", merged, err)
	}
	var gotHypothesis, gotEvidence, gotReport, gotSourceRun uuid.UUID
	var sourceItem, providerName, model, promptName, promptVersion, appVersion, buildRevision string
	var sourceVersion int
	query := `SELECT h.id,e.id,e.source_item_id,e.source_id,e.source_version,sr.source_ai_run_id,r.provider,r.model,r.prompt_name,r.prompt_version,r.app_version,r.build_revision FROM clinical_hypotheses h JOIN clinical_hypothesis_evidence he ON he.tenant_id=h.tenant_id AND he.hypothesis_id=h.id AND he.relation_type='supporting' JOIN clinical_evidence e ON e.tenant_id=he.tenant_id AND e.id=he.evidence_id JOIN session_reports sr ON sr.tenant_id=e.tenant_id AND sr.id=e.source_id JOIN clinical_ai_runs r ON r.tenant_id=sr.tenant_id AND r.id=sr.source_ai_run_id WHERE h.tenant_id=$1 AND h.id=$2`
	if err = pool.QueryRow(ctx, query, f.tenant, hypothesisID).Scan(&gotHypothesis, &gotEvidence, &sourceItem, &gotReport, &sourceVersion, &gotSourceRun, &providerName, &model, &promptName, &promptVersion, &appVersion, &buildRevision); err != nil {
		t.Fatal(err)
	}
	if gotHypothesis != hypothesisID || gotEvidence != evidenceID || sourceItem != "fact-001" || gotReport != report.ID || sourceVersion != 1 || gotSourceRun != sourceRun.ID || providerName != "ollama" || model != "source-fixture-model" || promptName != "session-report" || promptVersion != "session-report-v1.1" || appVersion != "stage-2b.1-source" || buildRevision != "source-build" {
		t.Fatalf("reverse provenance mismatch h=%s e=%s item=%s report=%s/v%d run=%s provider=%s model=%s prompt=%s/%s build=%s/%s", gotHypothesis, gotEvidence, sourceItem, gotReport, sourceVersion, gotSourceRun, providerName, model, promptName, promptVersion, appVersion, buildRevision)
	}
	state, err := repo.State(ctx, f.tenant, f.client)
	if err != nil || len(state.RecentEvents) != 1 || state.RecentEvents[0].ID != eventID || len(state.Processes) != 1 || state.Processes[0].ID != processID || len(state.Processes[0].Events) != 1 || state.Processes[0].Events[0].ID != eventID || len(state.Processes[0].Hypotheses) != 1 || state.Processes[0].Hypotheses[0].ID != hypothesisID {
		t.Fatalf("forward provenance state=%#v err=%v", state, err)
	}
	history, err := repo.GetHypothesisHistory(ctx, f.tenant, f.client, hypothesisID)
	if err != nil || len(history.Transitions) != 1 || history.Transitions[0].DiffID != analysis.Diff.ID || history.Transitions[0].MergedByUserID == nil || *history.Transitions[0].MergedByUserID != f.user {
		t.Fatalf("diff/review/merge provenance=%#v err=%v", history, err)
	}
}

func TestLongitudinalCoreEntitiesRejectHardDelete(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	evidence, event, process, hypothesis := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.','active',$5)`, evidence, f.tenant, f.client, f.report, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,$3,'other','Proposed','Proposed',NOW(),'proposed',$4)`, event, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Proposed','Proposed','proposed','observing',$4)`, process, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id) VALUES($1,$2,$3,'Proposed','proposed','active','red',$4)`, hypothesis, f.tenant, f.client, f.user)
	for table, id := range map[string]uuid.UUID{"clinical_evidence": evidence, "clinical_events": event, "clinical_processes": process, "clinical_hypotheses": hypothesis} {
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id=$1 AND id=$2", table)
		if _, err = pool.Exec(ctx, query, f.tenant, id); err == nil {
			t.Fatalf("hard delete succeeded for %s", table)
		}
	}
}
