package http

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each HTTP integration case owns its credential fixture. Production
// migrations intentionally invalidate the historical known demo password.
func createAuthPostgresOwner(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, string, string, func()) {
	t.Helper()
	ctx := context.Background()
	tenantID, userID, roleID := uuid.New(), uuid.New(), uuid.New()
	email := "owner-" + userID.String() + "@example.test"
	password := "TestOnly-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic auth integration tenant')`, tenantID); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID); err != nil {
			t.Errorf("clean up synthetic auth tenant: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,crypt($4,gen_salt('bf')))`, userID, tenantID, email, password); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(id,tenant_id,user_id,role) VALUES($1,$2,$3,'owner')`, roleID, tenantID, userID); err != nil {
		t.Fatal(err)
	}
	return tenantID, email, password, cleanup
}
