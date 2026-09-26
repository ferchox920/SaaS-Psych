package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLongitudinalProcessPagesPostgresIntegration(t *testing.T) {
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
	otherClient := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Otro')`, otherClient, f.tenant)
	base := time.Now().UTC().Truncate(time.Second)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,created_by_user_id,updated_at) VALUES($1,$2,$3,'Proceso ficticio','Proceso ficticio',$4,$5)`, id, f.tenant, f.client, f.user, base.Add(time.Duration(i)*time.Minute))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_processes(tenant_id,client_id,title,description,created_by_user_id) VALUES($1,$2,'Otro proceso','Otro proceso',$3)`, f.tenant, otherClient, f.user)
	eventID, hypothesisID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,created_by_user_id) VALUES($1,$2,$3,'other','Evento ficticio','Evento ficticio',NOW(),$4)`, eventID, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_process_events(tenant_id,client_id,process_id,event_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[2], eventID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,process_id,statement,confidence_level,created_by_user_id) VALUES($1,$2,$3,$4,'Hipótesis ficticia','yellow',$5)`, hypothesisID, f.tenant, f.client, ids[0], f.user)
	evidenceID, targetID, goalID, rationaleID, giraID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5)`, evidenceID, f.tenant, f.client, f.report, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Target','Target','other',$5)`, targetID, f.tenant, f.client, ids[2], f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_target_evidence(tenant_id,client_id,target_id,evidence_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, targetID, evidenceID)
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_targets SET approval_status='approved',approved_by_user_id=$3,approved_at=NOW() WHERE tenant_id=$1 AND id=$2`, f.tenant, targetID, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,$4,'Goal','Goal','other','high',$5)`, goalID, f.tenant, f.client, ids[2], f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goal_targets(tenant_id,client_id,goal_id,target_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, goalID, targetID)
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_goals SET approval_status='approved',approved_by_user_id=$3,approved_at=NOW() WHERE tenant_id=$1 AND id=$2`, f.tenant, goalID, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO therapeutic_rationales(id,tenant_id,client_id,process_id,target_id,goal_id,approach_slug,approach_version,rationale,expected_effect,grounding_status,approval_status,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,'act',1,'Rationale','Efecto','grounded','approved',$7,$7,NOW())`, rationaleID, f.tenant, f.client, ids[2], targetID, goalID, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO giras(id,tenant_id,client_id,process_id,gira_version,title,summary,approval_status,created_by_user_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,1,'GIRA','GIRA','approved',$5,$5,NOW())`, giraID, f.tenant, f.client, ids[2], f.user)
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListProcessesPage(ctx, f.tenant, f.client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first process page=%#v err=%v", first, err)
	}
	if len(first[0].Events) != 1 || first[0].Events[0].ID != eventID || len(first[1].Events) != 0 || len(first[0].Hypotheses) != 0 || first[0].TherapeuticStrategy == nil {
		t.Fatalf("first process associations leaked or missing: %#v", first)
	}
	strategy := first[0].TherapeuticStrategy
	if len(strategy.Targets) != 1 || strategy.Targets[0].ID != targetID || len(strategy.Targets[0].EvidenceIDs) != 1 || strategy.Targets[0].EvidenceIDs[0] != evidenceID || len(strategy.Goals) != 1 || strategy.Goals[0].ID != goalID || len(strategy.Rationales) != 1 || strategy.Rationales[0].ID != rationaleID || len(strategy.GIRAs) != 1 || strategy.GIRAs[0].ID != giraID {
		t.Fatalf("first process strategy lost: %#v", strategy)
	}
	second, err := repo.ListProcessesPage(ctx, f.tenant, f.client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second process page=%#v err=%v", second, err)
	}
	if len(second[0].Hypotheses) != 1 || second[0].Hypotheses[0].ID != hypothesisID || len(second[0].Events) != 0 || second[0].TherapeuticStrategy == nil {
		t.Fatalf("second process associations leaked or missing: %#v", second)
	}
}
