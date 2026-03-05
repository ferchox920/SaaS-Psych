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

func TestAppointmentsRejectCrossTenantClientReferencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration fk tenant/client test")
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
	clientA := uuid.New()
	clientB := uuid.New()

	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`, tenantA, fmt.Sprintf("tenant-a-%s", tenantA.String()), tenantB, fmt.Sprintf("tenant-b-%s", tenantB.String())); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public)
		VALUES ($1, $2, 'Client A', '', ''), ($3, $4, 'Client B', '', '')
	`, clientA, tenantA, clientB, tenantB); err != nil {
		t.Fatalf("insert clients: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location)
		VALUES ($1, $2, $3, $4, $5, 'scheduled', '')
	`, uuid.New(), tenantA, clientB, time.Now().UTC().Add(2*time.Hour), time.Now().UTC().Add(3*time.Hour))
	if err == nil {
		t.Fatalf("expected FK violation for cross-tenant client reference")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected pg error, got %T: %v", err, err)
	}
	if pgErr.Code != "23503" {
		t.Fatalf("expected foreign_key_violation (23503), got %s (%s)", pgErr.Code, pgErr.Message)
	}
}
