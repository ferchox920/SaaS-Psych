package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
)

func TestClientProfileIsolationAndRevisionPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, actor, unassigned := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic profile integration test')`, tenant)
	for _, id := range []uuid.UUID{actor, unassigned} {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'disabled-test-hash')`, id, tenant, id.String()+"@example.invalid")
	}
	repo := NewClientRepository(pool).WithTransactionalAudit()
	entity, err := domainclient.NewEntity(tenant, "Synthetic profile patient", "", "", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	entity, err = repo.Create(ctx, entity, actor)
	if err != nil {
		t.Fatal(err)
	}
	service := clientusecase.NewService(repo, NewAuditRepository(pool)).WithClinicalAccess(NewClinicalAccessRepository(pool))
	profile, err := service.GetProfile(ctx, tenant, entity.ID, actor)
	if err != nil || profile.Revision != 0 || profile.ReminderLeadHours != 24 {
		t.Fatalf("default profile: %#v err=%v", profile, err)
	}
	profile.Email = "synthetic@example.invalid"
	profile.ReminderChannel = "email"
	profile.ReminderConsent = true
	profile.ChatGPTProjects = []domainclient.ChatGPTProjectReference{{ProjectID: "g-p-11111111111111111111111111111111", ProjectName: "Synthetic case"}}
	saved, err := service.SaveProfile(ctx, tenant, entity.ID, actor, profile)
	if err != nil || saved.Revision != 1 {
		t.Fatalf("first save: %#v err=%v", saved, err)
	}
	if _, err = service.SaveProfile(ctx, tenant, entity.ID, actor, profile); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale insert: %v", err)
	}
	saved.Phone = "+5491112345678"
	updated, err := service.SaveProfile(ctx, tenant, entity.ID, actor, saved)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("second save: %#v err=%v", updated, err)
	}
	if _, err = service.SaveProfile(ctx, tenant, entity.ID, actor, saved); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale update: %v", err)
	}
	read, err := service.GetProfile(ctx, tenant, entity.ID, actor)
	if err != nil || read.Email != updated.Email || read.Phone != updated.Phone || read.Revision != 2 {
		t.Fatalf("round trip: %#v err=%v", read, err)
	}
	if len(read.ChatGPTProjects) != 1 || read.ChatGPTProjects[0] != profile.ChatGPTProjects[0] {
		t.Fatal("project association was not preserved")
	}
	otherEntity, err := domainclient.NewEntity(tenant, "Second synthetic patient", "", "", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	otherEntity, err = repo.Create(ctx, otherEntity, actor)
	if err != nil {
		t.Fatal(err)
	}
	otherProfile := domainclient.DefaultProfile()
	otherProfile.ChatGPTProjects = profile.ChatGPTProjects
	if _, err = service.SaveProfile(ctx, tenant, otherEntity.ID, actor, otherProfile); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("project assigned to two patients: %v", err)
	}
	if _, err = service.GetProfile(ctx, uuid.New(), entity.ID, actor); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
	if _, err = service.SaveProfile(ctx, uuid.New(), entity.ID, actor, updated); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("cross-tenant write: %v", err)
	}
	if _, err = service.GetProfile(ctx, tenant, entity.ID, unassigned); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("unassigned read: %v", err)
	}
	if _, err = service.SaveProfile(ctx, tenant, entity.ID, unassigned, updated); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("unassigned write: %v", err)
	}
	var auditCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND entity_id=$2 AND action='client.profile.update'`, tenant, entity.ID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("transactional audit count=%d err=%v", auditCount, err)
	}
	calendarRepo := NewGoogleCalendarRepository(pool)
	connection, err := calendarRepo.UpsertConnection(ctx, domaincalendar.Connection{ID: uuid.New(), TenantID: tenant, UserID: actor, CalendarID: "primary", EncryptedRefreshToken: "synthetic-disabled-token", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	appointmentID := uuid.New()
	start := time.Now().UTC().Add(24 * time.Hour)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,$4,$5,'scheduled')`, appointmentID, tenant, entity.ID, start, start.Add(time.Hour))
	if err = calendarRepo.CreateLink(ctx, domaincalendar.Link{ID: uuid.New(), TenantID: tenant, AppointmentID: appointmentID, ConnectionID: connection.ID, GoogleEventID: "synthetic-calendar-event", SyncStatus: "synced", LastSyncedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	linked, err := calendarRepo.ListLinkedEvents(ctx, tenant, actor, start.Add(-time.Hour), start.Add(time.Hour), 100, 0)
	if err != nil || len(linked) != 1 || linked[0].ClientID != entity.ID.String() {
		t.Fatalf("same-tenant linked events: %#v err=%v", linked, err)
	}
	for _, scope := range []struct{ tenant, user uuid.UUID }{{uuid.New(), actor}, {tenant, unassigned}} {
		linked, err = calendarRepo.ListLinkedEvents(ctx, scope.tenant, scope.user, start.Add(-time.Hour), start.Add(time.Hour), 100, 0)
		if err != nil || len(linked) != 0 {
			t.Fatalf("linked events leaked: %#v err=%v", linked, err)
		}
	}
}
