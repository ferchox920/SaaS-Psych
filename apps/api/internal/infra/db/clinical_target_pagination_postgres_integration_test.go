package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLongitudinalTargetPagesPostgresIntegration(t *testing.T) {
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
	base := time.Now().UTC().Truncate(time.Second)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,created_by_user_id,updated_at) VALUES($1,$2,$3,$4,'Target ficticio','Target ficticio','other',$5,$6)`, id, f.tenant, f.client, process, f.user, base.Add(time.Duration(i)*time.Minute))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_targets(tenant_id,client_id,process_id,title,description,target_type,created_by_user_id) VALUES($1,$2,$3,'Otro target','Otro target','other',$4)`, f.tenant, otherClient, otherProcess, f.user)
	evidenceID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5)`, evidenceID, f.tenant, f.client, f.report, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_target_evidence(tenant_id,client_id,target_id,evidence_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[2], evidenceID)
	eventID := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,created_by_user_id) VALUES($1,$2,$3,'other','Evento','Evento',NOW(),$4)`, eventID, f.tenant, f.client, f.user)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_target_events(tenant_id,client_id,target_id,event_id) VALUES($1,$2,$3,$4)`, f.tenant, f.client, ids[0], eventID)
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListTargetsPage(ctx, f.tenant, f.client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first target page=%#v err=%v", first, err)
	}
	if len(first[0].EvidenceIDs) != 1 || first[0].EvidenceIDs[0] != evidenceID || len(first[0].EventIDs) != 0 || len(first[1].EvidenceIDs) != 0 {
		t.Fatalf("first target links leaked or missing: %#v", first)
	}
	second, err := repo.ListTargetsPage(ctx, f.tenant, f.client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second target page=%#v err=%v", second, err)
	}
	if len(second[0].EventIDs) != 1 || second[0].EventIDs[0] != eventID || len(second[0].EvidenceIDs) != 0 {
		t.Fatalf("second target links leaked or missing: %#v", second)
	}
}
