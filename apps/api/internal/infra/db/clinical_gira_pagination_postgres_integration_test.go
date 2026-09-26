package db

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestLongitudinalGIRAPagesPostgresIntegration(t *testing.T) {
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
	ids, phases := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}, []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO giras(id,tenant_id,client_id,process_id,gira_version,title,summary,created_by_user_id) VALUES($1,$2,$3,$4,$5,'GIRA ficticia','GIRA ficticia',$6)`, id, f.tenant, f.client, process, i+1, f.user)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_phases(id,tenant_id,client_id,gira_id,position,title,description,created_by_user_id) VALUES($1,$2,$3,$4,1,'Fase ficticia','Fase ficticia',$5)`, phases[i], f.tenant, f.client, id, f.user)
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO giras(tenant_id,client_id,process_id,gira_version,title,summary,created_by_user_id) VALUES($1,$2,$3,1,'Otra GIRA','Otra GIRA',$4)`, f.tenant, otherClient, otherProcess, f.user)
	targetID, goalID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,$4,'Target','Target','other',$5)`, targetID, f.tenant, f.client, process, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,created_by_user_id) VALUES($1,$2,$3,$4,'Goal','Goal','other','high',$5)`, goalID, f.tenant, f.client, process, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_targets(tenant_id,client_id,gira_id,target_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[2], targetID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_phase_goals(tenant_id,client_id,phase_id,goal_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, phases[0], goalID)
	rationaleID, evidenceID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO therapeutic_rationales(id,tenant_id,client_id,process_id,target_id,goal_id,approach_slug,approach_version,rationale,expected_effect,grounding_status,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6,'act',1,'Rationale ficticio','Efecto ficticio','exploration_needed',$7)`, rationaleID, f.tenant, f.client, process, targetID, goalID, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5)`, evidenceID, f.tenant, f.client, f.report, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO therapeutic_rationale_evidence(tenant_id,client_id,rationale_id,evidence_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, rationaleID, evidenceID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_rationales(tenant_id,client_id,gira_id,rationale_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[0], rationaleID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO gira_phase_rationales(tenant_id,client_id,phase_id,rationale_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, phases[0], rationaleID)
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListGIRAsPage(ctx, f.tenant, f.client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first GIRA page=%#v err=%v", first, err)
	}
	if len(first[0].TargetIDs) != 1 || first[0].TargetIDs[0] != targetID || len(first[0].Phases) != 1 || first[0].Phases[0].ID != phases[2] || len(first[1].TargetIDs) != 0 {
		t.Fatalf("first GIRA associations leaked or missing: %#v", first)
	}
	second, err := repo.ListGIRAsPage(ctx, f.tenant, f.client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second GIRA page=%#v err=%v", second, err)
	}
	if len(second[0].Phases) != 1 || second[0].Phases[0].ID != phases[0] || len(second[0].Phases[0].GoalIDs) != 1 || second[0].Phases[0].GoalIDs[0] != goalID || len(second[0].Phases[0].RationaleIDs) != 1 || second[0].Phases[0].RationaleIDs[0] != rationaleID || len(second[0].TargetIDs) != 0 || len(second[0].Rationales) != 1 || second[0].Rationales[0].ID != rationaleID || len(second[0].Rationales[0].EvidenceIDs) != 1 || second[0].Rationales[0].EvidenceIDs[0] != evidenceID {
		t.Fatalf("second GIRA associations leaked or missing: %#v", second)
	}
}
