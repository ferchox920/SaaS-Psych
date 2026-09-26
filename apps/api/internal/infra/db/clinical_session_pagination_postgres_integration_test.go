package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClinicalSessionPagesPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, user, client, otherClient := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'session page tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Ficticio'),($3,$2,'Otro')`, client, tenant, otherClient)
	base := time.Now().UTC().Truncate(time.Second)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(id,tenant_id,client_id,therapist_user_id,status,started_at) VALUES($1,$2,$3,$4,'in_progress',$5)`, id, tenant, client, user, base.Add(time.Duration(i)*time.Minute))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_sessions(tenant_id,client_id,therapist_user_id,status,started_at) VALUES($1,$2,$3,'in_progress',$4)`, tenant, otherClient, user, base)
	repo := NewClinicalSessionRepository(pool)
	first, err := repo.ListByClientPage(ctx, tenant, client, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first session page=%#v err=%v", first, err)
	}
	second, err := repo.ListByClientPage(ctx, tenant, client, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second session page=%#v err=%v", second, err)
	}
}
