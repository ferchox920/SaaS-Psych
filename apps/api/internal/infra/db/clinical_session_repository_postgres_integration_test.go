package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	domainclinicalsession "sessionflow/apps/api/internal/domain/clinicalsession"
)

func TestClinicalSessionTenantAndLifecyclePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, otherTenant, user, client, appointment := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'session tenant'),($2,'other')`, tenant, otherTenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'client')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','scheduled')`, appointment, tenant, client)
	repo := NewClinicalSessionRepository(pool)
	now := time.Now().UTC()
	item, _ := domainclinicalsession.New(tenant, client, &appointment, user, now.Add(-time.Minute), now)
	created, err := repo.Create(ctx, item, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, otherTenant, created.ID); err == nil {
		t.Fatal("cross-tenant read must fail")
	}
	if err := created.Complete(now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Transition(ctx, created, user); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM clinical_sessions WHERE tenant_id=$1 AND id=$2`, tenant, created.ID); err == nil {
		t.Fatal("hard delete with audit/report-ready history must be blocked")
	}
}
