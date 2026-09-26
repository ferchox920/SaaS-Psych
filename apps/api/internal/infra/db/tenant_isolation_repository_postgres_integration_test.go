package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
)

func TestSessionNoteRepositoryTenantIsolationPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration repository tenant isolation test")
	}

	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	clientA := uuid.New()
	clientB := uuid.New()
	appointmentA := uuid.New()
	appointmentB := uuid.New()

	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`,
		tenantA, fmt.Sprintf("tenant-a-%s", tenantA), tenantB, fmt.Sprintf("tenant-b-%s", tenantB),
	)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES
			($1, $2, $3, 'hash-a'),
			($4, $5, $6, 'hash-b')
	`,
		userA, tenantA, fmt.Sprintf("owner-a-%s@example.local", userA),
		userB, tenantB, fmt.Sprintf("owner-b-%s@example.local", userB),
	)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public)
		VALUES
			($1, $2, 'Client A', '', ''),
			($3, $4, 'Client B', '', '')
	`,
		clientA, tenantA, clientB, tenantB,
	)

	startA := time.Now().UTC().Add(2 * time.Hour)
	startB := time.Now().UTC().Add(4 * time.Hour)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location)
		VALUES
			($1, $2, $3, $4, $5, 'scheduled', 'Room A'),
			($6, $7, $8, $9, $10, 'scheduled', 'Room B')
	`,
		appointmentA, tenantA, clientA, startA, startA.Add(time.Hour),
		appointmentB, tenantB, clientB, startB, startB.Add(time.Hour),
	)

	repo := NewSessionNoteRepository(pool).WithTransactionalAudit()

	noteA, err := repo.Create(ctx, domainsessionnote.Entity{
		ID:            uuid.New(),
		TenantID:      tenantA,
		AppointmentID: appointmentA,
		AuthorUserID:  userA,
		Body:          "tenant-a-note",
		IsPrivate:     true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create tenant A note: %v", err)
	}

	if _, err := repo.Create(ctx, domainsessionnote.Entity{
		ID:            uuid.New(),
		TenantID:      tenantB,
		AppointmentID: appointmentB,
		AuthorUserID:  userB,
		Body:          "tenant-b-note",
		IsPrivate:     false,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create tenant B note: %v", err)
	}

	exists, err := repo.AppointmentExists(ctx, tenantA, appointmentA)
	if err != nil {
		t.Fatalf("appointment exists same tenant: %v", err)
	}
	if !exists {
		t.Fatalf("expected tenant A appointment to exist for tenant A")
	}

	exists, err = repo.AppointmentExists(ctx, tenantA, appointmentB)
	if err != nil {
		t.Fatalf("appointment exists cross tenant: %v", err)
	}
	if exists {
		t.Fatalf("expected tenant B appointment to be hidden from tenant A")
	}

	if _, err := repo.GetByID(ctx, tenantB, noteA.ID); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("expected cross-tenant GetByID to return ErrNotFound, got %v", err)
	}

	listA, err := repo.ListByAppointment(ctx, tenantA, appointmentA)
	if err != nil {
		t.Fatalf("list notes same tenant: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != noteA.ID {
		t.Fatalf("expected tenant A list to return only tenant A note, got %+v", listA)
	}

	listCrossTenant, err := repo.ListByAppointment(ctx, tenantB, appointmentA)
	if err != nil {
		t.Fatalf("list notes cross tenant: %v", err)
	}
	if len(listCrossTenant) != 0 {
		t.Fatalf("expected cross-tenant list to be empty, got %+v", listCrossTenant)
	}

	_, err = repo.Update(ctx, domainsessionnote.Entity{
		ID:             noteA.ID,
		TenantID:       tenantB,
		AppointmentID:  noteA.AppointmentID,
		AuthorUserID:   noteA.AuthorUserID,
		Body:           "cross-tenant-update",
		IsPrivate:      false,
		Status:         noteA.Status,
		CurrentVersion: noteA.CurrentVersion,
		CreatedAt:      noteA.CreatedAt,
		UpdatedAt:      time.Now().UTC(),
	})
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("expected cross-tenant update to return ErrNotFound, got %v", err)
	}

	stored, err := repo.GetByID(ctx, tenantA, noteA.ID)
	if err != nil {
		t.Fatalf("reload tenant A note: %v", err)
	}
	if stored.Body != noteA.Body || stored.IsPrivate != noteA.IsPrivate {
		t.Fatalf("cross-tenant update should not mutate note, got %+v", stored)
	}

	stored.Body = "tenant-a-draft-version-two"
	stored.UpdatedAt = time.Now().UTC()
	updated, err := repo.Update(ctx, stored)
	if err != nil {
		t.Fatalf("update same-tenant draft: %v", err)
	}
	if updated.CurrentVersion != 2 {
		t.Fatalf("expected draft version 2, got %d", updated.CurrentVersion)
	}

	signedAt := time.Now().UTC().Add(time.Second)
	signed, err := repo.Sign(ctx, tenantA, noteA.ID, updated.CurrentVersion, signedAt)
	if err != nil {
		t.Fatalf("sign note: %v", err)
	}
	if signed.Status != "signed" || signed.SignedAt == nil {
		t.Fatalf("expected signed lifecycle state, got %+v", signed)
	}

	signed.Body = "forbidden overwrite"
	signed.UpdatedAt = signedAt.Add(time.Second)
	if _, err := repo.Update(ctx, signed); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("expected signed overwrite conflict, got %v", err)
	}

	signed.Body = "signed content with documented clarification"
	signed.UpdatedAt = signedAt.Add(2 * time.Second)
	withAddendum, err := repo.Addendum(ctx, signed, userA, "documented clarification")
	if err != nil {
		t.Fatalf("add addendum: %v", err)
	}
	if withAddendum.CurrentVersion != 3 || withAddendum.Status != "signed" {
		t.Fatalf("unexpected addendum lifecycle state: %+v", withAddendum)
	}
	versions, err := repo.ListVersions(ctx, tenantA, noteA.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 3 || versions[0].Body != "tenant-a-note" || versions[2].ChangeKind != "addendum" || versions[2].ChangeReason != "documented clarification" {
		t.Fatalf("unexpected immutable version history: %+v", versions)
	}
	var writeAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE tenant_id = $1 AND entity_id = $2
		  AND action IN ('session_note.create', 'session_note.update', 'session_note.sign', 'session_note.addendum')
	`, tenantA, noteA.ID).Scan(&writeAuditCount); err != nil {
		t.Fatalf("count transactional write audits: %v", err)
	}
	if writeAuditCount != 4 {
		t.Fatalf("expected one transactional audit per clinical write, got %d", writeAuditCount)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM clients WHERE tenant_id = $1 AND id = $2`, tenantA, clientA); err == nil {
		t.Fatal("expected physical deletion of a client with clinical history to be blocked")
	}
}

func TestAuditRepositoryTenantIsolationPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration repository tenant isolation test")
	}

	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`,
		tenantA, fmt.Sprintf("tenant-a-%s", tenantA), tenantB, fmt.Sprintf("tenant-b-%s", tenantB),
	)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES
			($1, $2, $3, 'hash-a'),
			($4, $5, $6, 'hash-b')
	`,
		userA, tenantA, fmt.Sprintf("owner-a-%s@example.local", userA),
		userB, tenantB, fmt.Sprintf("owner-b-%s@example.local", userB),
	)

	repo := NewAuditRepository(pool)

	if err := repo.RecordDomainEvent(ctx, tenantA, userA, "clients.created", "client", nil, map[string]any{"scope": "a-1"}); err != nil {
		t.Fatalf("record tenant A event 1: %v", err)
	}
	if err := repo.RecordDomainEvent(ctx, tenantA, userA, "clients.updated", "client", nil, map[string]any{"scope": "a-2"}); err != nil {
		t.Fatalf("record tenant A event 2: %v", err)
	}
	if err := repo.RecordDomainEvent(ctx, tenantB, userB, "clients.created", "client", nil, map[string]any{"scope": "b-1"}); err != nil {
		t.Fatalf("record tenant B event: %v", err)
	}

	filterA := auditusecase.ListFilter{
		TenantID: tenantA,
		Entity:   "client",
		Limit:    10,
		Offset:   0,
		Order:    "desc",
	}

	entriesA, err := repo.ListAuditLogs(ctx, filterA)
	if err != nil {
		t.Fatalf("list tenant A audit logs: %v", err)
	}
	if len(entriesA) != 2 {
		t.Fatalf("expected 2 tenant A audit logs, got %d", len(entriesA))
	}
	for _, entry := range entriesA {
		if entry.TenantID != tenantA {
			t.Fatalf("expected only tenant A audit logs, got tenant %s", entry.TenantID)
		}
	}

	countA, err := repo.CountAuditLogs(ctx, filterA)
	if err != nil {
		t.Fatalf("count tenant A audit logs: %v", err)
	}
	if countA != 2 {
		t.Fatalf("expected tenant A audit count 2, got %d", countA)
	}

	entriesB, err := repo.ListAuditLogs(ctx, auditusecase.ListFilter{
		TenantID: tenantB,
		Entity:   "client",
		Limit:    10,
		Offset:   0,
		Order:    "desc",
	})
	if err != nil {
		t.Fatalf("list tenant B audit logs: %v", err)
	}
	if len(entriesB) != 1 {
		t.Fatalf("expected 1 tenant B audit log, got %d", len(entriesB))
	}
	if entriesB[0].TenantID != tenantB {
		t.Fatalf("expected tenant B audit log, got tenant %s", entriesB[0].TenantID)
	}
}

func postgresIntegrationDatabaseURL() string {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return "postgres://sessionflow:sessionflow@127.0.0.1:5433/sessionflow?sslmode=disable"
	}
	return databaseURL
}

func mustExecIntegrationSQL(t *testing.T, pool *pgxpool.Pool, ctx context.Context, query string, args ...any) {
	t.Helper()

	if _, err := pool.Exec(ctx, query, args...); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
