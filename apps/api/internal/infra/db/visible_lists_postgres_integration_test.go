package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVisibleClientAndAppointmentListsPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, otherTenant, viewer, otherViewer := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	assigned, exception, hidden, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'visible tenant'),($2,'foreign tenant')`, tenant, otherTenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash'),($4,$5,$6,'hash')`, viewer, tenant, viewer.String()+"@example.test", otherViewer, otherTenant, otherViewer.String()+"@example.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Assigned'),($3,$2,'Exception'),($4,$2,'Hidden'),($5,$6,'Foreign')`, assigned, tenant, exception, hidden, foreign, otherTenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO client_clinical_assignments(tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,'treating',$3,$4)`, tenant, assigned, viewer, now.Add(-time.Hour))
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_access_exceptions(tenant_id,client_id,user_id,granted_by_user_id,reason,purpose,starts_at,expires_at) VALUES($1,$2,$3,$3,'synthetic coverage','read',$4,$5)`, tenant, exception, viewer, now.Add(-time.Hour), now.Add(time.Hour))
	for _, client := range []uuid.UUID{assigned, exception, hidden} {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,$4,'scheduled')`, tenant, client, now.Add(time.Duration(client[0])*time.Minute), now.Add(time.Duration(client[0])*time.Minute+time.Hour))
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,$4,'scheduled')`, otherTenant, foreign, now, now.Add(time.Hour))
	clients, err := NewClientRepository(pool).ListVisible(ctx, tenant, viewer, false)
	if err != nil || len(clients) != 2 {
		t.Fatalf("visible clients=%#v err=%v", clients, err)
	}
	appointments, err := NewAppointmentRepository(pool).ListVisibleByRange(ctx, tenant, viewer, now.Add(-time.Minute), now.Add(6*time.Hour))
	if err != nil || len(appointments) != 2 {
		t.Fatalf("visible appointments=%#v err=%v", appointments, err)
	}
	for _, item := range appointments {
		if item.TenantID != tenant || item.ClientID == hidden || item.ClientID == foreign {
			t.Fatalf("unauthorized appointment leaked: %#v", item)
		}
	}
	foreignView, err := NewClientRepository(pool).ListVisible(ctx, otherTenant, viewer, false)
	if err != nil || len(foreignView) != 0 {
		t.Fatalf("cross-tenant client visible: %#v err=%v", foreignView, err)
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_access_exceptions SET revoked_at=$3,revoked_by_user_id=$4,revoke_reason='synthetic expiry' WHERE tenant_id=$1 AND client_id=$2`, tenant, exception, now, viewer)
	clients, err = NewClientRepository(pool).ListVisible(ctx, tenant, viewer, false)
	if err != nil || len(clients) != 1 || clients[0].ID != assigned {
		t.Fatalf("revoked exception still visible: %#v err=%v", clients, err)
	}
	appointments, err = NewAppointmentRepository(pool).ListVisibleByRange(ctx, tenant, viewer, now.Add(-time.Minute), now.Add(6*time.Hour))
	if err != nil || len(appointments) != 1 || appointments[0].ClientID != assigned {
		t.Fatalf("revoked exception appointment still visible: %#v err=%v", appointments, err)
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clients SET archived_at=$3,archived_by_user_id=$4,archive_reason='synthetic archive' WHERE tenant_id=$1 AND id=$2`, tenant, assigned, now, viewer)
	active, err := NewClientRepository(pool).ListVisible(ctx, tenant, viewer, false)
	if err != nil || len(active) != 0 {
		t.Fatalf("archived client in active list: %#v err=%v", active, err)
	}
	archived, err := NewClientRepository(pool).ListVisible(ctx, tenant, viewer, true)
	if err != nil || len(archived) != 1 || archived[0].ID != assigned {
		t.Fatalf("archived client missing: %#v err=%v", archived, err)
	}
}

func TestVisibleListOrderingIndexesPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, name := range []string{"clients_active_tenant_created", "clients_archived_tenant_created", "appointments_tenant_starts_id"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1)`, name).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("visible-list ordering index %s is missing", name)
		}
	}
}

func TestVisibleListsPageAfterFilteringPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, viewer := uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'page tenant')`, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, viewer, tenant, viewer.String()+"@example.test")
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname,created_at) VALUES($1,$2,$3,$4)`, id, tenant, id.String(), now.Add(time.Duration(i)*time.Minute))
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,$4,'scheduled')`, tenant, id, now.Add(time.Duration(i)*time.Hour), now.Add(time.Duration(i+1)*time.Hour))
		if i != 1 {
			mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO client_clinical_assignments(tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,'treating',$3,$4)`, tenant, id, viewer, now.Add(-time.Hour))
		}
	}
	clientsRepo := NewClientRepository(pool)
	first, err := clientsRepo.ListVisiblePage(ctx, tenant, viewer, false, 2, 0)
	if err != nil || len(first) != 2 || first[0].ID != ids[3] || first[1].ID != ids[2] {
		t.Fatalf("first client page=%v err=%v", first, err)
	}
	second, err := clientsRepo.ListVisiblePage(ctx, tenant, viewer, false, 2, 2)
	if err != nil || len(second) != 1 || second[0].ID != ids[0] {
		t.Fatalf("second client page=%v err=%v", second, err)
	}
	appointmentsRepo := NewAppointmentRepository(pool)
	aFirst, err := appointmentsRepo.ListVisiblePage(ctx, tenant, viewer, now.Add(-time.Minute), now.Add(5*time.Hour), 2, 0)
	if err != nil || len(aFirst) != 2 || aFirst[0].ClientID != ids[0] || aFirst[1].ClientID != ids[2] {
		t.Fatalf("first appointment page=%v err=%v", aFirst, err)
	}
	aSecond, err := appointmentsRepo.ListVisiblePage(ctx, tenant, viewer, now.Add(-time.Minute), now.Add(5*time.Hour), 2, 2)
	if err != nil || len(aSecond) != 1 || aSecond[0].ClientID != ids[3] {
		t.Fatalf("second appointment page=%v err=%v", aSecond, err)
	}
}
