package db

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestUserRolesRejectCrossTenantUserReferencePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration user_roles tenant fk test")
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
	userB := uuid.New()

	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`, tenantA, fmt.Sprintf("tenant-a-%s", tenantA.String()), tenantB, fmt.Sprintf("tenant-b-%s", tenantB.String())); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES
			($1, $2, $3, 'hash-a'),
			($4, $5, $6, 'hash-b')
	`, userA, tenantA, fmt.Sprintf("owner-a-%s@example.local", userA.String()), userB, tenantB, fmt.Sprintf("member-b-%s@example.local", userB.String())); err != nil {
		t.Fatalf("insert users: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (id, tenant_id, user_id, role)
		VALUES ($1, $2, $3, 'member')
	`, uuid.New(), tenantA, userB); err == nil {
		t.Fatalf("expected FK violation for cross-tenant user_roles reference")
	} else {
		assertForeignKeyViolation(t, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (id, tenant_id, user_id, role)
		VALUES ($1, $2, $3, 'owner')
	`, uuid.New(), tenantA, userA); err != nil {
		t.Fatalf("expected same-tenant user_role insert to succeed: %v", err)
	}
}
