package db

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestListDiffsBatchesOperationsPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'diff batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id) VALUES($1,$2)`, tenant, client)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,status,base_state_version,created_by_user_id) VALUES($1,$2,$3,'pending_review',0,$4)`, id, tenant, client, user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diff_operations(tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,original_proposal) VALUES($1,$2,$3,$4,'create_process',$5,'{}')`, tenant, client, id, i+1, uuid.New())
	}
	before := len(recorder.Ended())
	diffs, err := NewClinicalLongitudinalRepository(pool).ListDiffs(ctx, tenant, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 3 {
		t.Fatalf("got %d diffs", len(diffs))
	}
	for _, diff := range diffs {
		if len(diff.Operations) != 1 || diff.Operations[0].DiffID != diff.ID {
			t.Fatalf("operations mismatched for %s: %#v", diff.ID, diff.Operations)
		}
	}
	if got := len(recorder.Ended()) - before; got > 2 {
		t.Fatalf("N+1 queries for 3 diffs: got %d, want at most 2", got)
	}
}

func TestLongitudinalEventAndHypothesisListsBatchEvidencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'evidence batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	for i := 0; i < 3; i++ {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,'other','Ficticio','Ficticio',NOW(),'proposed',$3)`, tenant, client, user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(tenant_id,client_id,statement,approval_status,clinical_status,confidence_level,created_by_user_id) VALUES($1,$2,'Ficticio','proposed','active','red',$3)`, tenant, client, user)
	}
	repo := NewClinicalLongitudinalRepository(pool)
	before := len(recorder.Ended())
	events, err := repo.ListEvents(ctx, tenant, client)
	if err != nil || len(events) != 3 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	if got := len(recorder.Ended()) - before; got > 2 {
		t.Errorf("event N+1: %d queries", got)
	}
	before = len(recorder.Ended())
	hypotheses, err := repo.ListHypotheses(ctx, tenant, client)
	if err != nil || len(hypotheses) != 3 {
		t.Fatalf("hypotheses=%d err=%v", len(hypotheses), err)
	}
	if got := len(recorder.Ended()) - before; got > 2 {
		t.Errorf("hypothesis N+1: %d queries", got)
	}
}

func TestLongitudinalProcessListDoesNotQueryPerProcessPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'process batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	for i := 0; i < 3; i++ {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,'Ficticio','Ficticio','proposed','observing',$3)`, tenant, client, user)
	}
	before := len(recorder.Ended())
	processes, err := NewClinicalLongitudinalRepository(pool).ListProcesses(ctx, tenant, client)
	if err != nil || len(processes) != 3 {
		t.Fatalf("processes=%d err=%v", len(processes), err)
	}
	if got := len(recorder.Ended()) - before; got > 10 {
		t.Fatalf("per-process queries: %d for 3 empty processes", got)
	}
}

func TestLongitudinalTargetsBatchLinksPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, process := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'target batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Ficticio','Ficticio','proposed','observing',$4)`, process, tenant, client, user)
	for i := 0; i < 3; i++ {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,'Ficticio','Ficticio','other',$4)`, tenant, client, process, user)
	}
	before := len(recorder.Ended())
	targets, err := NewClinicalLongitudinalRepository(pool).ListTargets(ctx, tenant, client)
	if err != nil || len(targets) != 3 {
		t.Fatalf("targets=%d err=%v", len(targets), err)
	}
	if got := len(recorder.Ended()) - before; got > 2 {
		t.Fatalf("target link N+1: %d queries", got)
	}
}

func TestLongitudinalGoalsBatchIndicatorsPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, process := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'goal batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Ficticio','Ficticio','proposed','observing',$4)`, process, tenant, client, user)
	var firstGoal, firstIndicator uuid.UUID
	for i := 0; i < 3; i++ {
		goal := uuid.New()
		indicator := uuid.New()
		if i == 0 {
			firstGoal, firstIndicator = goal, indicator
		}
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other','medium',$5)`, goal, tenant, client, process, user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goal_indicators(id,tenant_id,client_id,goal_id,description,indicator_type,created_by_user_id) VALUES($1,$2,$3,$4,'Señal ficticia','qualitative',$5)`, indicator, tenant, client, goal, user)
	}
	target, event := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other',$5)`, target, tenant, client, process, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goal_targets(tenant_id,client_id,goal_id,target_id) VALUES($1,$2,$3,$4)`, tenant, client, firstGoal, target)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,$3,'other','Ficticio','Ficticio',NOW(),'proposed',$4)`, event, tenant, client, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_indicator_events(tenant_id,client_id,indicator_id,event_id,relation_type) VALUES($1,$2,$3,$4,'neutral')`, tenant, client, firstIndicator, event)
	before := len(recorder.Ended())
	goals, err := NewClinicalLongitudinalRepository(pool).ListGoals(ctx, tenant, client)
	if err != nil || len(goals) != 3 {
		t.Fatalf("goals=%d err=%v", len(goals), err)
	}
	for _, goal := range goals {
		if len(goal.Indicators) != 1 {
			t.Fatalf("missing indicator on goal %s", goal.ID)
		}
		if goal.ID == firstGoal {
			if len(goal.TargetIDs) != 1 || goal.TargetIDs[0] != target || len(goal.Indicators[0].Links) != 1 || goal.Indicators[0].Links[0].SourceID != event || goal.Indicators[0].Links[0].SourceType != "event" {
				t.Fatalf("batched goal associations lost: %#v", goal)
			}
		} else if len(goal.TargetIDs) != 0 || len(goal.Indicators[0].Links) != 0 {
			t.Fatalf("goal association leaked: %#v", goal)
		}
	}
	if got := len(recorder.Ended()) - before; got > 4 {
		t.Fatalf("goal/indicator N+1: %d queries", got)
	}
}

func TestLongitudinalRationalesBatchLinksPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, process, target, goal := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'rationale batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Ficticio','Ficticio','proposed','observing',$4)`, process, tenant, client, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other',$5)`, target, tenant, client, process, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other','medium',$5)`, goal, tenant, client, process, user)
	for i := 0; i < 3; i++ {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO therapeutic_rationales(tenant_id,client_id,process_id,target_id,goal_id,approach_slug,approach_version,rationale,expected_effect,grounding_status,created_by_user_id) VALUES($1,$2,$3,$4,$5,'act',1,'Ficticio','Ficticio','exploration_needed',$6)`, tenant, client, process, target, goal, user)
	}
	before := len(recorder.Ended())
	rationales, err := NewClinicalLongitudinalRepository(pool).listRationalesByClient(ctx, tenant, client)
	if err != nil || len(rationales) != 3 {
		t.Fatalf("rationales=%d err=%v", len(rationales), err)
	}
	if got := len(recorder.Ended()) - before; got > 2 {
		t.Fatalf("rationale link N+1: %d queries", got)
	}
}

func TestLongitudinalGIRAsBatchPhasesPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = tp.Shutdown(ctx) }()
	pool, err := NewPostgresPoolWithTracing(ctx, postgresIntegrationDatabaseURL(), PoolTracingConfig{Tracer: tp.Tracer("query-count")})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, process := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'gira batch tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id) VALUES($1,$2,$3,'Ficticio','Ficticio','proposed','observing',$4)`, process, tenant, client, user)
	var firstGIRA, firstPhase uuid.UUID
	for i := 1; i <= 3; i++ {
		gira := uuid.New()
		phase := uuid.New()
		if i == 1 {
			firstGIRA, firstPhase = gira, phase
		}
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO giras(id,tenant_id,client_id,process_id,gira_version,title,summary,created_by_user_id) VALUES($1,$2,$3,$4,$5,'Ficticio','Ficticio',$6)`, gira, tenant, client, process, i, user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_phases(id,tenant_id,client_id,gira_id,position,title,description,created_by_user_id) VALUES($1,$2,$3,$4,1,'Ficticio','Ficticio',$5)`, phase, tenant, client, gira, user)
	}
	target, goal := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other',$5)`, target, tenant, client, process, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,$4,'Ficticio','Ficticio','other','medium',$5)`, goal, tenant, client, process, user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_targets(tenant_id,client_id,gira_id,target_id) VALUES($1,$2,$3,$4)`, tenant, client, firstGIRA, target)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_goals(tenant_id,client_id,gira_id,goal_id) VALUES($1,$2,$3,$4)`, tenant, client, firstGIRA, goal)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_phase_goals(tenant_id,client_id,phase_id,goal_id) VALUES($1,$2,$3,$4)`, tenant, client, firstPhase, goal)
	before := len(recorder.Ended())
	giras, err := NewClinicalLongitudinalRepository(pool).ListGIRAs(ctx, tenant, client)
	if err != nil || len(giras) != 3 {
		t.Fatalf("giras=%d err=%v", len(giras), err)
	}
	for _, gira := range giras {
		if len(gira.Phases) != 1 {
			t.Fatalf("missing phase on GIRA %s", gira.ID)
		}
		if gira.ID == firstGIRA {
			if len(gira.TargetIDs) != 1 || gira.TargetIDs[0] != target || len(gira.GoalIDs) != 1 || gira.GoalIDs[0] != goal || len(gira.Phases[0].GoalIDs) != 1 || gira.Phases[0].GoalIDs[0] != goal {
				t.Fatalf("batched GIRA links lost: %#v", gira)
			}
		} else if len(gira.TargetIDs) != 0 || len(gira.GoalIDs) != 0 || len(gira.Phases[0].GoalIDs) != 0 {
			t.Fatalf("GIRA association leaked: %#v", gira)
		}
	}
	if got := len(recorder.Ended()) - before; got > 6 {
		t.Fatalf("GIRA/phase N+1: %d queries", got)
	}
}
