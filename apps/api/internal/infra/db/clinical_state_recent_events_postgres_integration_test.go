package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type stateSQLCapture struct{ queries []pgx.TraceQueryStartData }

func (c *stateSQLCapture) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	c.queries = append(c.queries, data)
	return ctx
}
func (*stateSQLCapture) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestStateRecentEventsBoundsSQLBeforeLoadingEvidencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	capture := &stateSQLCapture{}
	config, err := pgxpool.ParseConfig(postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = capture
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	f := newLongitudinalAcceptanceFixture(t, pool)
	evidenceID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5)`, evidenceID, f.tenant, f.client, f.report, f.user)
	base := time.Now().UTC().Truncate(time.Second)
	approvedIDs := make([]uuid.UUID, 21)
	for i := range approvedIDs {
		id := uuid.New()
		approvedIDs[i] = id
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,created_by_user_id) VALUES($1,$2,$3,'other','Aprobado ficticio','Aprobado ficticio',$4,$5)`, id, f.tenant, f.client, base.Add(time.Duration(i)*time.Minute), f.user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, id, evidenceID)
		mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_events SET approval_status='approved',approved_by_user_id=$3,approved_at=NOW() WHERE tenant_id=$1 AND id=$2`, f.tenant, id, f.user)
	}
	for i := 0; i < 25; i++ {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(tenant_id,client_id,event_type,title,description,observed_at,created_by_user_id) VALUES($1,$2,'other','Propuesto ficticio','Propuesto ficticio',$3,$4)`, f.tenant, f.client, base.Add(time.Duration(21+i)*time.Minute), f.user)
	}
	before := len(capture.queries)
	state, err := NewClinicalLongitudinalRepository(pool).State(ctx, f.tenant, f.client)
	if err != nil || len(state.RecentEvents) != 20 || state.RecentEvents[0].ID != approvedIDs[20] || state.RecentEvents[19].ID != approvedIDs[1] {
		t.Fatalf("recent approved events=%#v err=%v", state.RecentEvents, err)
	}
	var eventQuery pgx.TraceQueryStartData
	for _, query := range capture.queries[before:] {
		if strings.Contains(query.SQL, "FROM clinical_events e") && strings.Contains(query.SQL, "ORDER BY e.observed_at") {
			eventQuery = query
		}
	}
	if !strings.Contains(eventQuery.SQL, "approval_status='approved'") || !strings.Contains(eventQuery.SQL, "LIMIT $3") || len(eventQuery.Args) != 4 || eventQuery.Args[2] != 20 || eventQuery.Args[3] != 0 {
		t.Fatalf("recent events must filter and limit in PostgreSQL, query=%q args=%v", eventQuery.SQL, eventQuery.Args)
	}
}

func TestStateFiltersVisibleSectionsBeforeLoadingAssociationsPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	capture := &stateSQLCapture{}
	config, err := pgxpool.ParseConfig(postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = capture
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	f := newLongitudinalAcceptanceFixture(t, pool)
	before := len(capture.queries)
	if _, err := NewClinicalLongitudinalRepository(pool).State(ctx, f.tenant, f.client); err != nil {
		t.Fatal(err)
	}
	checks := []struct{ table, filter string }{
		{"FROM clinical_processes p", "p.approval_status='approved'"},
		{"FROM clinical_hypotheses h", "h.approval_status='approved' AND h.process_id IS NULL"},
		{"FROM clinical_evidence e", "e.status='active'"},
		{"FROM clinical_diffs d", "d.status IN ('draft','pending_review','partially_reviewed','approved')"},
	}
	for _, check := range checks {
		found := false
		for _, query := range capture.queries[before:] {
			if strings.Contains(query.SQL, check.table) && strings.Contains(query.SQL, check.filter) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("State must filter %s in PostgreSQL: missing %q", check.table, check.filter)
		}
	}
}

func TestStateLoadsOperationsOnlyForOpenProposalsPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	capture := &stateSQLCapture{}
	config, err := pgxpool.ParseConfig(postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = capture
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	f := newLongitudinalAcceptanceFixture(t, pool)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, f.tenant, f.client)
	openID, closedID := uuid.New(), uuid.New()
	for _, item := range []struct {
		id     uuid.UUID
		status string
	}{{openID, "pending_review"}, {closedID, "rejected"}} {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,status,base_state_version,created_by_user_id) VALUES($1,$2,$3,$4,0,$5)`, item.id, f.tenant, f.client, item.status, f.user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_diff_operations(tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,original_proposal) VALUES($1,$2,$3,1,'create_process',$4,'{}')`, f.tenant, f.client, item.id, uuid.New())
	}
	before := len(capture.queries)
	state, err := NewClinicalLongitudinalRepository(pool).State(ctx, f.tenant, f.client)
	if err != nil || len(state.OpenProposals) != 1 || state.OpenProposals[0].ID != openID || len(state.OpenProposals[0].Operations) != 1 {
		t.Fatalf("open proposals=%#v err=%v", state.OpenProposals, err)
	}
	var operationsQuery pgx.TraceQueryStartData
	for _, query := range capture.queries[before:] {
		if strings.Contains(query.SQL, "FROM clinical_diff_operations o") {
			operationsQuery = query
		}
	}
	if !strings.Contains(operationsQuery.SQL, "d.id=ANY($3::uuid[])") || len(operationsQuery.Args) != 3 {
		t.Fatalf("operation load must be scoped to open proposal IDs, query=%q args=%v", operationsQuery.SQL, operationsQuery.Args)
	}
}
