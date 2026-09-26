package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLongitudinalEventPagesPostgresIntegration(t *testing.T) {
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
	tenant, user, client, otherClient := f.tenant, f.user, f.client, uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Otro')`, otherClient, tenant)
	base := time.Now().UTC().Truncate(time.Second)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,$3,'other','Ficticio','Ficticio',$4,'proposed',$5)`, id, tenant, client, base.Add(time.Duration(i)*time.Minute), user)
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_events(tenant_id,client_id,event_type,title,description,observed_at,approval_status,created_by_user_id) VALUES($1,$2,'other','Otro','Otro',$3,'proposed',$4)`, tenant, otherClient, base, user)
	evidenceIDs := []uuid.UUID{uuid.New(), uuid.New()}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5),($6,$2,$3,'session_report',$4,1,'fact-002','patient_report','La persona describió temor al rechazo.',$5)`, evidenceIDs[0], tenant, client, f.report, user, evidenceIDs[1])
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4),($1,$2,$5,$6)`, tenant, client, ids[2], evidenceIDs[0], ids[0], evidenceIDs[1])
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListEventsPage(ctx, tenant, client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first event page=%#v err=%v", first, err)
	}
	if len(first[0].Evidence) != 1 || first[0].Evidence[0].ID != evidenceIDs[0] || len(first[1].Evidence) != 0 {
		t.Fatalf("first page evidence leaked or missing: %#v", first)
	}
	second, err := repo.ListEventsPage(ctx, tenant, client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second event page=%#v err=%v", second, err)
	}
	if len(second[0].Evidence) != 1 || second[0].Evidence[0].ID != evidenceIDs[1] {
		t.Fatalf("second page evidence leaked or missing: %#v", second)
	}
}
