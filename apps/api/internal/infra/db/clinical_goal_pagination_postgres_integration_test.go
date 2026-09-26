package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLongitudinalGoalPagesPostgresIntegration(t *testing.T) {
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
	otherClient, process, otherProcess := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Otro')`, otherClient, f.tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,created_by_user_id) VALUES($1,$2,$3,'Proceso ficticio','Proceso ficticio',$4),($5,$2,$6,'Otro proceso','Otro proceso',$4)`, process, f.tenant, f.client, f.user, otherProcess, otherClient)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	base := time.Now().UTC().Truncate(time.Second)
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id,created_at) VALUES($1,$2,$3,$4,'Goal ficticio','Goal ficticio','other','high',$5,$6)`, id, f.tenant, f.client, process, f.user, base.Add(time.Duration(i)*time.Minute))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,'Otro goal','Otro goal','other','high',$4)`, f.tenant, otherClient, otherProcess, f.user)
	targetID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Target ficticio','Target ficticio','other',$5)`, targetID, f.tenant, f.client, process, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goal_targets(tenant_id,client_id,goal_id,target_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[0], targetID)
	indicatorID, eventID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goal_indicators(id,tenant_id,client_id,goal_id,description,indicator_type,created_by_user_id) VALUES($1,$2,$3,$4,'Indicador ficticio','qualitative',$5)`, indicatorID, f.tenant, f.client, ids[2], f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,created_by_user_id) VALUES($1,$2,$3,'other','Evento','Evento',NOW(),$4)`, eventID, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_indicator_events(tenant_id,client_id,indicator_id,event_id,relation_type) VALUES($1,$2,$3,$4,'supports_progress')`, f.tenant, f.client, indicatorID, eventID)
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListGoalsPage(ctx, f.tenant, f.client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[0] || first[1].ID != ids[1] {
		t.Fatalf("first goal page=%#v err=%v", first, err)
	}
	if len(first[0].TargetIDs) != 1 || first[0].TargetIDs[0] != targetID || len(first[0].Indicators) != 0 || len(first[1].TargetIDs) != 0 {
		t.Fatalf("first goal associations leaked or missing: %#v", first)
	}
	second, err := repo.ListGoalsPage(ctx, f.tenant, f.client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[2] {
		t.Fatalf("second goal page=%#v err=%v", second, err)
	}
	if len(second[0].Indicators) != 1 || second[0].Indicators[0].ID != indicatorID || len(second[0].Indicators[0].Links) != 1 || second[0].Indicators[0].Links[0].SourceID != eventID || len(second[0].TargetIDs) != 0 {
		t.Fatalf("second goal associations leaked or missing: %#v", second)
	}
}
