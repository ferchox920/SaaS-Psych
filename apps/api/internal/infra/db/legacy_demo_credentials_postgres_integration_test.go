package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLegacyDemoCredentialMigrationRotatesOnlyKnownPasswordsPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ownerTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	owner := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	memberTenant := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	member := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=crypt('ChangeMe123!',gen_salt('bf')) WHERE tenant_id=$1 AND id=$2`, ownerTenant, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=crypt('operator-changed',gen_salt('bf')) WHERE tenant_id=$1 AND id=$2`, memberTenant, member); err != nil {
		t.Fatal(err)
	}
	tokenHash := "legacy-migration-" + uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO refresh_tokens(tenant_id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, ownerTenant, owner, tokenHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000029_disable_legacy_demo_credentials.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var ownerKnown, memberChanged, tokenRevoked bool
	if err := tx.QueryRow(ctx, `SELECT password_hash=crypt('ChangeMe123!',password_hash) FROM users WHERE tenant_id=$1 AND id=$2`, ownerTenant, owner).Scan(&ownerKnown); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT password_hash=crypt('operator-changed',password_hash) FROM users WHERE tenant_id=$1 AND id=$2`, memberTenant, member).Scan(&memberChanged); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE tenant_id=$1 AND token_hash=$2`, ownerTenant, tokenHash).Scan(&tokenRevoked); err != nil {
		t.Fatal(err)
	}
	if ownerKnown || !memberChanged || !tokenRevoked {
		t.Fatalf("credential migration contract failed: owner_known=%t member_changed_preserved=%t token_revoked=%t", ownerKnown, memberChanged, tokenRevoked)
	}
}
