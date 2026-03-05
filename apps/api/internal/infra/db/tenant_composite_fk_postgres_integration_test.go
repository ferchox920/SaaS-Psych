package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSessionNotesRejectCrossTenantAppointmentReferencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration session_notes tenant fk test")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("rollback tx: %v", err)
		}
	}()

	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	clientA := uuid.New()
	clientB := uuid.New()
	appointmentA := uuid.New()
	appointmentB := uuid.New()

	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`, tenantA, fmt.Sprintf("tenant-a-%s", tenantA.String()), tenantB, fmt.Sprintf("tenant-b-%s", tenantB.String())); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES ($1, $2, $3, 'hash')
	`, userA, tenantA, fmt.Sprintf("owner-a-%s@example.local", userA.String())); err != nil {
		t.Fatalf("insert user A: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public)
		VALUES ($1, $2, 'Client A', '', ''), ($3, $4, 'Client B', '', '')
	`, clientA, tenantA, clientB, tenantB); err != nil {
		t.Fatalf("insert clients: %v", err)
	}

	startA := time.Now().UTC().Add(2 * time.Hour)
	endA := startA.Add(1 * time.Hour)
	startB := time.Now().UTC().Add(4 * time.Hour)
	endB := startB.Add(1 * time.Hour)
	if _, err := tx.Exec(ctx, `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location)
		VALUES
			($1, $2, $3, $4, $5, 'scheduled', ''),
			($6, $7, $8, $9, $10, 'scheduled', '')
	`, appointmentA, tenantA, clientA, startA, endA, appointmentB, tenantB, clientB, startB, endB); err != nil {
		t.Fatalf("insert appointments: %v", err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT before_cross_tenant_session_note`); err != nil {
		t.Fatalf("create savepoint: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO session_notes (id, tenant_id, appointment_id, author_user_id, body, is_private)
		VALUES ($1, $2, $3, $4, 'cross tenant', true)
	`, uuid.New(), tenantA, appointmentB, userA); err == nil {
		t.Fatalf("expected FK violation for cross-tenant appointment reference in session_notes")
	} else {
		assertForeignKeyViolation(t, err)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT before_cross_tenant_session_note`); rbErr != nil {
			t.Fatalf("rollback to savepoint: %v", rbErr)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO session_notes (id, tenant_id, appointment_id, author_user_id, body, is_private)
		VALUES ($1, $2, $3, $4, 'same tenant', true)
	`, uuid.New(), tenantA, appointmentA, userA); err != nil {
		t.Fatalf("expected same-tenant session note insert to succeed: %v", err)
	}
}

func TestRefreshTokensRejectCrossTenantUserReferencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration refresh_tokens tenant fk test")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("rollback tx: %v", err)
		}
	}()

	tenantA := uuid.New()
	tenantB := uuid.New()
	userB := uuid.New()

	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`, tenantA, fmt.Sprintf("tenant-a-%s", tenantA.String()), tenantB, fmt.Sprintf("tenant-b-%s", tenantB.String())); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES ($1, $2, $3, 'hash')
	`, userB, tenantB, fmt.Sprintf("member-b-%s@example.local", userB.String())); err != nil {
		t.Fatalf("insert user B: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, tenant_id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, NOW() + interval '1 day')
	`, uuid.New(), tenantA, userB, fmt.Sprintf("hash-%s", uuid.NewString()))
	if err == nil {
		t.Fatalf("expected FK violation for cross-tenant user reference in refresh_tokens")
	}

	assertForeignKeyViolation(t, err)
}

func assertForeignKeyViolation(t *testing.T, err error) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected pg error, got %T: %v", err, err)
	}
	if pgErr.Code != "23503" {
		t.Fatalf("expected foreign_key_violation (23503), got %s (%s)", pgErr.Code, pgErr.Message)
	}
}
