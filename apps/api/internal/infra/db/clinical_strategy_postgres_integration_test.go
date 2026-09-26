package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type postgresGIRAProvider struct {
	pool       *pgxpool.Pool
	tenant     uuid.UUID
	sawRunning bool
}

func (p *postgresGIRAProvider) BuildGIRA(_ context.Context, _ string, in longitudinal.GIRAProviderRequest, _ func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	var status string
	if err := p.pool.QueryRow(context.Background(), `SELECT status FROM clinical_ai_runs WHERE tenant_id=$1 AND operation='gira_build' ORDER BY created_at DESC LIMIT 1`, p.tenant).Scan(&status); err == nil && status == "running" {
		p.sawRunning = true
	}
	technique := "behavioral_experiment"
	one := 1
	result := longitudinal.GIRASemanticProposal{
		Targets:        []longitudinal.GIRASemanticTarget{{Ref: "target_1", Title: "Evitación", Description: "Evitación conductual", TargetType: "behavioral_pattern", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}, EventRefs: []string{}}},
		Goals:          []longitudinal.GIRASemanticGoal{{Ref: "goal_1", Title: "Conversación observable", Description: "Iniciar conversación pese a malestar", GoalType: "behavior_change", Priority: "high", TargetRefs: []string{"target_1"}}},
		Indicators:     []longitudinal.GIRASemanticIndicator{{Ref: "indicator_1", GoalRef: "goal_1", Description: "Inicia conversación", IndicatorType: "qualitative"}},
		Rationales:     []longitudinal.GIRASemanticRationale{{Ref: "rationale_1", TargetRef: "target_1", GoalRef: "goal_1", ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &one, Rationale: "La evitación mantiene el patrón", ExpectedEffect: "Aproximación", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}}},
		GIRA:           &longitudinal.GIRASemanticGIRA{Ref: "gira_1", Title: "GIRA", Summary: "Ruta", TargetRefs: []string{"target_1"}, GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}},
		Phases:         []longitudinal.GIRASemanticPhase{{Ref: "phase_1", GIRARef: "gira_1", Position: 1, Title: "Fase", Description: "Fase específica", GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}, IndicatorRefs: []string{"indicator_1"}}},
		IndicatorLinks: []longitudinal.GIRASemanticIndicatorLink{}, Uncertainties: []longitudinal.GIRASemanticUncertainty{},
	}
	return clinicalanalysis.ProviderOutput{JSON: mustJSON(result)}, nil
}
func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }

type strategyFixture struct {
	base              longitudinalAcceptanceFixture
	evidence, process uuid.UUID
}

func newStrategyFixture(t *testing.T, pool *pgxpool.Pool) strategyFixture {
	t.Helper()
	f := strategyFixture{base: newLongitudinalAcceptanceFixture(t, pool), evidence: uuid.New(), process: uuid.New()}
	mergeAcceptanceOperations(t, pool, f.base, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: f.evidence, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateEvidenceProposal{SourceItemID: "fact-002", EpistemicType: "patient_report"})},
		{OperationType: "create_process", TargetEntityID: f.process, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Proceso de evitación", Description: "Patrón clínico ficticio", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{f.evidence}})},
	})
	return f
}

func createReviewedStrategyDiff(t *testing.T, pool *pgxpool.Pool, f strategyFixture, ops []longitudinal.Operation) longitudinal.Diff {
	t.Helper()
	ctx := context.Background()
	repo := NewClinicalLongitudinalRepository(pool)
	state, err := repo.State(ctx, f.base.tenant, f.base.client)
	if err != nil {
		t.Fatal(err)
	}
	run := startAcceptanceRun(t, pool, f.base)
	diff, err := repo.CreateDiff(ctx, longitudinal.CreateDiffInput{TenantID: f.base.tenant, ClientID: f.base.client, SessionID: f.base.session, ReportID: f.base.report, RunID: run, ActorID: f.base.user, BaseStateVersion: state.StateVersion, Operations: ops, Uncertainties: []longitudinal.Uncertainty{}, OutputHash: fmt.Sprintf("%064d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range diff.Operations {
		diff, err = repo.Decide(ctx, longitudinal.DecisionInput{TenantID: f.base.tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
	}
	return diff
}

func fullStrategyOperations(t *testing.T, f strategyFixture) ([]longitudinal.Operation, map[string]uuid.UUID) {
	t.Helper()
	ids := map[string]uuid.UUID{"target": uuid.New(), "goal": uuid.New(), "indicator": uuid.New(), "rationale": uuid.New(), "gira": uuid.New(), "phase": uuid.New()}
	technique := "behavioral_experiment"
	version := 1
	ops := []longitudinal.Operation{
		{OperationType: "create_target", TargetEntityID: ids["target"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTargetProposal{ProcessID: f.process, Title: "Evitación de conversaciones", Description: "Evitación ante desaprobación anticipada", TargetType: "behavioral_pattern", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}})},
		{OperationType: "create_goal", TargetEntityID: ids["goal"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGoalProposal{ProcessID: f.process, Title: "Iniciar conversación difícil", Description: "Iniciar y sostener una conversación relevante pese a malestar", GoalType: "behavior_change", Priority: "high", TargetIDs: []uuid.UUID{ids["target"]}})},
		{OperationType: "create_goal_indicator", TargetEntityID: ids["indicator"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGoalIndicatorProposal{GoalID: ids["goal"], Description: "Inicia la conversación y mantiene el límite pese a culpa", IndicatorType: "qualitative"})},
		{OperationType: "create_therapeutic_rationale", TargetEntityID: ids["rationale"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTherapeuticRationaleProposal{ProcessID: f.process, TargetID: ids["target"], GoalID: ids["goal"], ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &version, Rationale: "La evitación alivia a corto plazo y mantiene el patrón", ExpectedEffect: "Aumentar aproximación conductual", GroundingStatus: "grounded", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}})},
		{OperationType: "create_gira", TargetEntityID: ids["gira"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGIRAProposal{ProcessID: f.process, GIRAVersion: 1, Title: "GIRA conductual", Summary: "Ruta focal y revisable", TargetIDs: []uuid.UUID{ids["target"]}, GoalIDs: []uuid.UUID{ids["goal"]}, RationaleIDs: []uuid.UUID{ids["rationale"]}})},
		{OperationType: "create_gira_phase", TargetEntityID: ids["phase"], OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGIRAPhaseProposal{GIRAID: ids["gira"], Position: 1, Title: "Preparación e intervención", Description: "Fase ordenada explícitamente", GoalIDs: []uuid.UUID{ids["goal"]}, RationaleIDs: []uuid.UUID{ids["rationale"]}, IndicatorIDs: []uuid.UUID{ids["indicator"]}})},
	}
	return ops, ids
}

func TestClinicalStrategyReviewMergeProjectionHistoryAndProvenancePostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	ops, ids := fullStrategyOperations(t, f)
	diff := createReviewedStrategyDiff(t, pool, f, ops)
	var before int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_targets WHERE tenant_id=$1 AND id=$2`, f.base.tenant, ids["target"]).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatal("human review mutated strategy before merge")
	}
	repo := NewClinicalLongitudinalRepository(pool)
	diff, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if diff.Status != "merged" {
		t.Fatalf("status=%s", diff.Status)
	}
	state, err := repo.State(ctx, f.base.tenant, f.base.client)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Processes) != 1 || state.Processes[0].TherapeuticStrategy == nil {
		t.Fatalf("state=%#v", state)
	}
	strategy := state.Processes[0].TherapeuticStrategy
	if len(strategy.Targets) != 1 || len(strategy.Goals) != 1 || len(strategy.GIRAs) != 1 || len(strategy.GIRAs[0].Phases) != 1 || len(strategy.GIRAs[0].Phases[0].IndicatorIDs) != 1 {
		t.Fatalf("strategy projection=%#v", strategy)
	}
	for entity, id := range map[string]uuid.UUID{"target": ids["target"], "goal": ids["goal"], "goal_indicator": ids["indicator"], "therapeutic_rationale": ids["rationale"], "gira": ids["gira"], "gira_phase": ids["phase"]} {
		history, e := repo.GetStrategyHistory(ctx, f.base.tenant, f.base.client, entity, id)
		if e != nil || len(history.Transitions) != 1 {
			t.Fatalf("%s history=%#v err=%v", entity, history, e)
		}
	}
	var chainOK bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_targets t JOIN clinical_goal_targets gt ON gt.tenant_id=t.tenant_id AND gt.target_id=t.id JOIN clinical_goals g ON g.tenant_id=gt.tenant_id AND g.id=gt.goal_id JOIN clinical_goal_indicators i ON i.tenant_id=g.tenant_id AND i.goal_id=g.id JOIN therapeutic_rationales r ON r.tenant_id=g.tenant_id AND r.goal_id=g.id AND r.target_id=t.id JOIN therapeutic_technique_definitions td ON td.slug=r.technique_slug AND td.version=r.technique_version AND td.approach_slug=r.approach_slug AND td.approach_version=r.approach_version JOIN gira_rationales gr ON gr.tenant_id=r.tenant_id AND gr.rationale_id=r.id JOIN giras gi ON gi.tenant_id=gr.tenant_id AND gi.id=gr.gira_id JOIN gira_phases p ON p.tenant_id=gi.tenant_id AND p.gira_id=gi.id JOIN gira_phase_indicators pi ON pi.tenant_id=p.tenant_id AND pi.phase_id=p.id AND pi.indicator_id=i.id WHERE t.tenant_id=$1 AND t.id=$2)`, f.base.tenant, ids["target"]).Scan(&chainOK)
	if err != nil || !chainOK {
		t.Fatalf("reverse structural provenance failed err=%v", err)
	}
	second, err := repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: 1})
	if err != nil || second.MergedStateVersion == nil || *second.MergedStateVersion != *diff.MergedStateVersion {
		t.Fatalf("idempotent merge=%#v err=%v", second, err)
	}
}

func TestGIRABuilderPersistsRunSourcesAndDiffOnlyPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	provider := &postgresGIRAProvider{pool: pool, tenant: f.base.tenant}
	service, _, runRepo := newLongitudinalAcceptanceService(pool, nil, true)
	service.WithGIRABuilder(provider)
	out, err := service.BuildGIRA(ctx, f.base.tenant, f.process, f.base.user)
	if err != nil {
		t.Fatal(err)
	}
	if !provider.sawRunning || out.Diff.Status != "pending_review" {
		t.Fatalf("run-before-provider=%v diff=%#v", provider.sawRunning, out.Diff)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_targets WHERE tenant_id=$1 AND client_id=$2`, f.base.tenant, f.base.client).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("GIRA builder directly materialized strategy")
	}
	var runID uuid.UUID
	var status, promptName, promptVersion string
	var parameters json.RawMessage
	if err = pool.QueryRow(ctx, `SELECT id,status,prompt_name,prompt_version,parameters_json FROM clinical_ai_runs WHERE tenant_id=$1 AND operation='gira_build' ORDER BY created_at DESC LIMIT 1`, f.base.tenant).Scan(&runID, &status, &promptName, &promptVersion, &parameters); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || promptName != longitudinal.GIRAPromptName || promptVersion != longitudinal.GIRAPromptVersion {
		t.Fatalf("run status=%s prompt=%s/%s", status, promptName, promptVersion)
	}
	var runMetadata map[string]any
	if err = json.Unmarshal(parameters, &runMetadata); err != nil || runMetadata["generation_repaired"] != false || runMetadata["output_bytes"] == nil {
		t.Fatalf("run metadata=%s err=%v", parameters, err)
	}
	sources, err := runRepo.ListSources(ctx, f.base.tenant, runID)
	if err != nil {
		t.Fatal(err)
	}
	seenProcess, seenEvidence, seenApproach, seenTechnique := false, false, false, false
	for _, source := range sources {
		seenProcess = seenProcess || source.SourceType == "clinical_process" && source.SourceID == f.process
		seenEvidence = seenEvidence || source.SourceType == "clinical_evidence" && source.SourceID == f.evidence
		seenApproach = seenApproach || source.SourceType == "therapeutic_approach_definition"
		seenTechnique = seenTechnique || source.SourceType == "therapeutic_technique_definition"
	}
	if !seenProcess || !seenEvidence || !seenApproach || !seenTechnique {
		t.Fatalf("sources=%#v", sources)
	}
	if out.Diff.ClinicalSessionID != nil || out.Diff.SourceSessionReportID != nil {
		t.Fatalf("strategy diff incorrectly bound to session/report: %#v", out.Diff)
	}
}

func TestClinicalStrategyTransactionalRollbackAndStaleConflictPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	ops, ids := fullStrategyOperations(t, f)
	initial := createReviewedStrategyDiff(t, pool, f, ops)
	repo := NewClinicalLongitudinalRepository(pool)
	initial, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: initial.ID, ActorID: f.base.user, ExpectedDiffRevision: initial.Revision})
	if err != nil {
		t.Fatal(err)
	}
	rollbackTarget := uuid.New()
	stale := 999
	badOps := []longitudinal.Operation{{OperationType: "create_target", TargetEntityID: rollbackTarget, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTargetProposal{ProcessID: f.process, Title: "Debe revertirse", Description: "Target transitorio", TargetType: "other", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}})}, {OperationType: "update_goal", TargetEntityID: ids["goal"], ExpectedEntityVersion: &stale, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.UpdateGoalProposal{Title: "Stale", Description: "No debe persistir", GoalType: "behavior_change", Priority: "high", TargetIDs: []uuid.UUID{ids["target"]}})}}
	diff := createReviewedStrategyDiff(t, pool, f, badOps)
	_, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision})
	if !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_targets WHERE tenant_id=$1 AND id=$2`, f.base.tenant, rollbackTarget).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial strategy object survived rollback")
	}
}

func TestClinicalStrategyIsolationCompatibilityNoDeleteAndHistoricalRegistryPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	otherClient := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Other')`, otherClient, f.base.tenant)
	otherAppointment, otherSession, otherReport := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','completed')`, otherAppointment, f.base.tenant, otherClient)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES($1,$2,$3,$4,$5,'completed',NOW(),NOW()+INTERVAL '1 hour')`, otherSession, f.base.tenant, otherClient, otherAppointment, f.base.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO session_reports(id,tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,1,'session-report-v1.1','approved','{"facts":[{"id":"x","statement":"cross"}]}',$4,$4,NOW())`, otherReport, f.base.tenant, otherSession, f.base.user)
	crossEvidence := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'x','patient_report','cross','active',$5)`, crossEvidence, f.base.tenant, otherClient, otherReport, f.base.user)
	badTarget := uuid.New()
	diff := createReviewedStrategyDiff(t, pool, f, []longitudinal.Operation{{OperationType: "create_target", TargetEntityID: badTarget, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTargetProposal{ProcessID: f.process, Title: "Cross", Description: "Cross client", TargetType: "other", EvidenceIDs: []uuid.UUID{crossEvidence}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}})}})
	repo := NewClinicalLongitudinalRepository(pool)
	_, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision})
	if err == nil {
		t.Fatal("cross-client grounding accepted")
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_diffs SET status='rejected' WHERE tenant_id=$1 AND id=$2`, f.base.tenant, diff.ID)
	ops, ids := fullStrategyOperations(t, f)
	good := createReviewedStrategyDiff(t, pool, f, ops)
	good, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: good.ID, ActorID: f.base.user, ExpectedDiffRevision: good.Revision})
	if err != nil {
		t.Fatal(err)
	}
	expectRejected := func(name string, operations []longitudinal.Operation) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			candidate := createReviewedStrategyDiff(t, pool, f, operations)
			if _, mergeErr := repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: candidate.ID, ActorID: f.base.user, ExpectedDiffRevision: candidate.Revision}); mergeErr == nil {
				t.Fatal("cross-boundary strategy operation was accepted")
			}
			mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_diffs SET status='rejected' WHERE tenant_id=$1 AND id=$2`, f.base.tenant, candidate.ID)
		})
	}

	otherProcess := uuid.New()
	mergeAcceptanceOperations(t, pool, f.base, []longitudinal.Operation{{OperationType: "create_process", TargetEntityID: otherProcess, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateProcessProposal{Title: "Otro proceso", Description: "Mismo cliente, proceso distinto", ClinicalStatus: "active", EvidenceIDs: []uuid.UUID{f.evidence}})}})
	expectRejected("cross_process_goal_target", []longitudinal.Operation{{OperationType: "create_goal", TargetEntityID: uuid.New(), OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGoalProposal{ProcessID: otherProcess, Title: "Cruce", Description: "No debe aceptar target de otro proceso", GoalType: "other", Priority: "medium", TargetIDs: []uuid.UUID{ids["target"]}})}})

	target2, goal2 := uuid.New(), uuid.New()
	secondEntities := createReviewedStrategyDiff(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_target", TargetEntityID: target2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTargetProposal{ProcessID: f.process, Title: "Segundo target", Description: "Target alternativo", TargetType: "other", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}})},
		{OperationType: "create_goal", TargetEntityID: goal2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGoalProposal{ProcessID: f.process, Title: "Segundo goal", Description: "Goal ligado solo al segundo target", GoalType: "other", Priority: "medium", TargetIDs: []uuid.UUID{target2}})},
	})
	secondEntities, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: secondEntities.ID, ActorID: f.base.user, ExpectedDiffRevision: secondEntities.Revision})
	if err != nil {
		t.Fatal(err)
	}
	technique, version := "behavioral_experiment", 1
	expectRejected("cross_goal_rationale", []longitudinal.Operation{{OperationType: "create_therapeutic_rationale", TargetEntityID: uuid.New(), OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTherapeuticRationaleProposal{ProcessID: f.process, TargetID: ids["target"], GoalID: goal2, ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &version, Rationale: "Cruce inválido", ExpectedEffect: "Ninguno", GroundingStatus: "grounded", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}})}})

	rationale2, gira2 := uuid.New(), uuid.New()
	secondGIRA := createReviewedStrategyDiff(t, pool, f, []longitudinal.Operation{
		{OperationType: "create_therapeutic_rationale", TargetEntityID: rationale2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateTherapeuticRationaleProposal{ProcessID: f.process, TargetID: target2, GoalID: goal2, ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &version, Rationale: "Cadena válida alternativa", ExpectedEffect: "Cambio observable", GroundingStatus: "grounded", EvidenceIDs: []uuid.UUID{f.evidence}, HypothesisIDs: []uuid.UUID{}})},
		{OperationType: "create_gira", TargetEntityID: gira2, OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGIRAProposal{ProcessID: f.process, GIRAVersion: 2, Title: "GIRA v2", Summary: "Segunda versión", SupersedesGIRAID: uuidPtr(ids["gira"]), TargetIDs: []uuid.UUID{target2}, GoalIDs: []uuid.UUID{goal2}, RationaleIDs: []uuid.UUID{rationale2}})},
	})
	secondGIRA, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: secondGIRA.ID, ActorID: f.base.user, ExpectedDiffRevision: secondGIRA.Revision})
	if err != nil {
		t.Fatal(err)
	}
	expectRejected("cross_gira_phase", []longitudinal.Operation{{OperationType: "create_gira_phase", TargetEntityID: uuid.New(), OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGIRAPhaseProposal{GIRAID: ids["gira"], Position: 2, Title: "Cruce de GIRA", Description: "No pertenece", GoalIDs: []uuid.UUID{goal2}, RationaleIDs: []uuid.UUID{rationale2}, IndicatorIDs: []uuid.UUID{}})}})
	expectRejected("cross_goal_indicator", []longitudinal.Operation{{OperationType: "create_gira_phase", TargetEntityID: uuid.New(), OriginalProposal: encodeAcceptanceProposal(t, longitudinal.CreateGIRAPhaseProposal{GIRAID: gira2, Position: 2, Title: "Cruce de indicador", Description: "Indicador de otro goal", GoalIDs: []uuid.UUID{goal2}, RationaleIDs: []uuid.UUID{rationale2}, IndicatorIDs: []uuid.UUID{ids["indicator"]}})}})
	if _, err = repo.Merge(ctx, longitudinal.MergeInput{TenantID: uuid.New(), DiffID: good.ID, ActorID: f.base.user, ExpectedDiffRevision: good.Revision}); err == nil {
		t.Fatal("cross-tenant merge accepted")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM clinical_targets WHERE tenant_id=$1 AND id=$2`, f.base.tenant, ids["target"]); err == nil {
		t.Fatal("hard delete was allowed")
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE therapeutic_approach_definitions SET status='active' WHERE slug='cbt' AND version=1`)
	}()
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE therapeutic_approach_definitions SET status='deprecated' WHERE slug='cbt' AND version=1`)
	gira, err := repo.GetGIRA(ctx, f.base.tenant, ids["gira"])
	if err != nil || len(gira.Rationales) != 1 || gira.Rationales[0].ApproachVersion != 1 {
		t.Fatalf("historical registry resolution lost: %#v err=%v", gira, err)
	}
	var compatible bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM therapeutic_rationales r JOIN therapeutic_technique_definitions t ON t.slug=r.technique_slug AND t.version=r.technique_version AND t.approach_slug=r.approach_slug AND t.approach_version=r.approach_version WHERE r.tenant_id=$1 AND r.id=$2)`, f.base.tenant, ids["rationale"]).Scan(&compatible)
	if err != nil || !compatible {
		t.Fatalf("technique compatibility not reconstructible err=%v", err)
	}
}

func uuidPtr(v uuid.UUID) *uuid.UUID { return &v }
