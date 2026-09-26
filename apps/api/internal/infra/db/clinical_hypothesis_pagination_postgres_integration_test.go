package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLongitudinalHypothesisPagesPostgresIntegration(t *testing.T) {
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
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,statement,confidence_level,created_by_user_id,updated_at) VALUES($1,$2,$3,'Hipótesis ficticia','yellow',$4,$5)`, id, f.tenant, f.client, f.user, base.Add(time.Duration(i)*time.Minute))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypotheses(tenant_id,client_id,statement,confidence_level,created_by_user_id) VALUES($1,$2,'Otra hipótesis','yellow',$3)`, f.tenant, otherClient, f.user)
	evidenceIDs := []uuid.UUID{uuid.New(), uuid.New()}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,created_by_user_id) VALUES($1,$2,$3,'session_report',$4,1,'fact-001','patient_report','La persona informó una mejora sostenida.',$5),($6,$2,$3,'session_report',$4,1,'fact-002','patient_report','La persona describió temor al rechazo.',$5)`, evidenceIDs[0], f.tenant, f.client, f.report, f.user, evidenceIDs[1])
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,'supporting'),($1,$2,$5,$6,'contradicting')`, f.tenant, f.client, ids[2], evidenceIDs[0], ids[0], evidenceIDs[1])
	repo := NewClinicalLongitudinalRepository(pool)
	first, err := repo.ListHypothesesPage(ctx, f.tenant, f.client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first hypothesis page=%#v err=%v", first, err)
	}
	if len(first[0].SupportingEvidence) != 1 || first[0].SupportingEvidence[0].ID != evidenceIDs[0] || len(first[0].ContradictingEvidence) != 0 || len(first[1].SupportingEvidence) != 0 {
		t.Fatalf("first hypothesis page evidence=%#v", first)
	}
	second, err := repo.ListHypothesesPage(ctx, f.tenant, f.client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second hypothesis page=%#v err=%v", second, err)
	}
	if len(second[0].ContradictingEvidence) != 1 || second[0].ContradictingEvidence[0].ID != evidenceIDs[1] || len(second[0].SupportingEvidence) != 0 {
		t.Fatalf("second hypothesis page evidence=%#v", second)
	}
}
