package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func TestClinicalLongitudinalHumanMergePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, appointment, session, run, report := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'longitudinal tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@longitudinal.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Longitudinal fixture')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW()-INTERVAL '1 hour',NOW(),'completed')`, appointment, tenant, client)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW()-INTERVAL '1 hour',NOW())`, session, tenant, client, appointment, user)
	hash := "0000000000000000000000000000000000000000000000000000000000000000"
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,input_hash,context_hash,output_hash,status,started_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,'ollama','fixture','longitudinal_interpretation','clinical-longitudinal-interpreter','clinical-longitudinal-interpreter-v1',$7,$7,$7,'succeeded',NOW(),NOW())`, run, tenant, client, appointment, session, user, hash)
	reportJSON := json.RawMessage(`{"schema_version":"session-report-v1.1","summary":"Ficticio","facts":[{"id":"fact-001","statement":"La persona informó una mejora sostenida.","category":"reported_change"}],"relevant_changes":[],"interventions":[],"patient_responses":[],"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":[{"id":"hypothesis-001","statement":"No debe ser evidencia.","traffic_light":"yellow","evidence_refs":["fact-001"]}],"safety_signals":[],"open_questions":[],"longitudinal_candidates":[]}`)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO session_reports(id,tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,1,'session-report-v1.1','approved',$4,$5,$5,NOW())`, report, tenant, session, reportJSON, user)
	repo := NewClinicalLongitudinalRepository(pool)
	evidenceID, eventID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	observed := time.Now().UTC()
	encode := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	one := 1
	ops := []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: evidenceID, OriginalProposal: encode(longitudinal.CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})},
		{OperationType: "create_event", TargetEntityID: eventID, OriginalProposal: encode(longitudinal.CreateEventProposal{EventType: "reported_change", Title: "Mejora informada", Description: "Cambio informado por la persona.", ObservedAt: observed, EvidenceIDs: []uuid.UUID{evidenceID}})},
		{OperationType: "create_process", TargetEntityID: processID, OriginalProposal: encode(longitudinal.CreateProcessProposal{Title: "Proceso de cambio", Description: "Seguimiento longitudinal.", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{evidenceID}})},
		{OperationType: "link_event_process", TargetEntityID: processID, ExpectedEntityVersion: &one, OriginalProposal: encode(longitudinal.LinkEventProcessProposal{EventID: eventID, ProcessID: processID, EvidenceIDs: []uuid.UUID{evidenceID}})},
		{OperationType: "create_hypothesis", TargetEntityID: hypothesisID, OriginalProposal: encode(longitudinal.CreateHypothesisProposal{ProcessID: &processID, Statement: "La mejora podría sostenerse.", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})},
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_ai_runs SET status='running',output_hash=NULL,completed_at=NULL WHERE tenant_id=$1 AND id=$2`, tenant, run)
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: tenant, ClientID: client, SessionID: session, ReportID: report, RunID: run, ActorID: user, BaseStateVersion: 0, Operations: ops, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM clinical_evidence WHERE tenant_id=$1)+(SELECT count(*) FROM clinical_events WHERE tenant_id=$1)+(SELECT count(*) FROM clinical_processes WHERE tenant_id=$1)+(SELECT count(*) FROM clinical_hypotheses WHERE tenant_id=$1)`, tenant).Scan(&count); err != nil || count != 0 {
		t.Fatalf("AI diff changed longitudinal state before merge: count=%d err=%v", count, err)
	}
	for _, op := range diff.Operations {
		diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if diff.Status != "approved" {
		t.Fatalf("status=%s", diff.Status)
	}
	merged, err := repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: diff.ID, ActorID: user, ExpectedDiffRevision: diff.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Status != "merged" || merged.MergedStateVersion == nil || *merged.MergedStateVersion != 1 {
		t.Fatalf("merged=%#v", merged)
	}
	state, err := repo.State(ctx, tenant, client)
	if err != nil {
		t.Fatal(err)
	}
	if state.StateVersion != 1 || len(state.Processes) != 1 || len(state.ActiveEvidence) != 1 || len(state.RecentEvents) != 1 || len(state.Processes[0].Hypotheses) != 1 {
		t.Fatalf("state=%#v", state)
	}
	again, err := repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: diff.ID, ActorID: user, ExpectedDiffRevision: diff.Revision})
	if err != nil || again.Status != "merged" {
		t.Fatalf("idempotent merge=%#v err=%v", again, err)
	}
	run2 := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,input_hash,context_hash,status,started_at) VALUES($1,$2,$3,$4,$5,$6,'ollama','fixture','longitudinal_interpretation','clinical-longitudinal-interpreter','clinical-longitudinal-interpreter-v1',$7,$7,'running',NOW())`, run2, tenant, client, appointment, session, user, hash)
	two := 2
	original := encode(longitudinal.UpdateProcessProposal{Title: "Título propuesto", Description: "Descripción propuesta.", EvidenceIDs: []uuid.UUID{evidenceID}})
	human := encode(longitudinal.UpdateProcessProposal{Title: "Título corregido por humano", Description: "Descripción corregida.", EvidenceIDs: []uuid.UUID{evidenceID}})
	diff2, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: tenant, ClientID: client, SessionID: session, ReportID: report, RunID: run2, ActorID: user, BaseStateVersion: 1, Operations: []longitudinal.Operation{{OperationType: "update_process", TargetEntityID: processID, ExpectedEntityVersion: &two, OriginalProposal: original}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	diff2, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: tenant, DiffID: diff2.ID, OperationID: diff2.Operations[0].ID, ActorID: user, ExpectedDiffRevision: diff2.Revision, Decision: "modified", Modification: human})
	if err != nil {
		t.Fatal(err)
	}
	if string(diff2.Operations[0].OriginalProposal) == string(diff2.Operations[0].HumanModification) {
		t.Fatal("human correction overwrote original proposal")
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: diff2.ID, ActorID: user, ExpectedDiffRevision: diff2.Revision}); err != nil {
		t.Fatal(err)
	}
	state, err = repo.State(ctx, tenant, client)
	if err != nil {
		t.Fatal(err)
	}
	if state.StateVersion != 2 || state.Processes[0].Title != "Título corregido por humano" {
		t.Fatalf("modified state=%#v", state)
	}
	if _, err = pool.Exec(ctx, `UPDATE clinical_evidence SET statement='Reescritura prohibida' WHERE tenant_id=$1 AND id=$2`, tenant, evidenceID); err == nil {
		t.Fatal("evidence provenance was mutable")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM clinical_evidence WHERE tenant_id=$1 AND id=$2`, tenant, evidenceID); err == nil {
		t.Fatal("evidence hard delete was allowed")
	}
	run3 := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,input_hash,context_hash,status,started_at) VALUES($1,$2,$3,$4,$5,$6,'ollama','fixture','longitudinal_interpretation','clinical-longitudinal-interpreter','clinical-longitudinal-interpreter-v1',$7,$7,'running',NOW())`, run3, tenant, client, appointment, session, user, hash)
	diff3, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: tenant, ClientID: client, SessionID: session, ReportID: report, RunID: run3, ActorID: user, BaseStateVersion: 2, Operations: []longitudinal.Operation{{OperationType: "create_event", TargetEntityID: uuid.New(), OriginalProposal: encode(longitudinal.CreateEventProposal{EventType: "other", Title: "Rechazado", Description: "No debe fusionarse.", ObservedAt: observed, EvidenceIDs: []uuid.UUID{evidenceID}})}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	diff3, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: tenant, DiffID: diff3.ID, OperationID: diff3.Operations[0].ID, ActorID: user, ExpectedDiffRevision: diff3.Revision, Decision: "rejected"})
	if err != nil || diff3.Status != "rejected" {
		t.Fatalf("rejected diff=%#v err=%v", diff3, err)
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: diff3.ID, ActorID: user, ExpectedDiffRevision: diff3.Revision}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("rejected merge err=%v", err)
	}
	run4 := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,input_hash,context_hash,status,started_at) VALUES($1,$2,$3,$4,$5,$6,'ollama','fixture','longitudinal_interpretation','clinical-longitudinal-interpreter','clinical-longitudinal-interpreter-v1',$7,$7,'running',NOW())`, run4, tenant, client, appointment, session, user, hash)
	wrong := 99
	diff4, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: tenant, ClientID: client, SessionID: session, ReportID: report, RunID: run4, ActorID: user, BaseStateVersion: 2, Operations: []longitudinal.Operation{{OperationType: "update_process", TargetEntityID: processID, ExpectedEntityVersion: &wrong, OriginalProposal: original}}, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	diff4, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: tenant, DiffID: diff4.ID, OperationID: diff4.Operations[0].ID, ActorID: user, ExpectedDiffRevision: diff4.Revision, Decision: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: diff4.ID, ActorID: user, ExpectedDiffRevision: diff4.Revision}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("entity version conflict err=%v", err)
	}
	state, err = repo.State(ctx, tenant, client)
	if err != nil || state.StateVersion != 2 || state.Processes[0].Title != "Título corregido por humano" {
		t.Fatalf("entity conflict left partial state=%#v err=%v", state, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinical_evidence(tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,'session_report',$3,1,'hypothesis-001','documented_fact','No debe ser evidencia.','active',$4)`, tenant, client, report, user); err == nil {
		t.Fatal("hypothesis candidate became factual evidence")
	}
	staleID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,status,base_state_version,created_by_user_id) VALUES($1,$2,$3,'approved',0,$4)`, staleID, tenant, client, user)
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: tenant, DiffID: staleID, ActorID: user, ExpectedDiffRevision: 1}); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale merge err=%v", err)
	}
}
