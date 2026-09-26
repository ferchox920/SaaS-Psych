package db

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

func TestRefreshRotationIsAtomicAndSingleUsePostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run PostgreSQL refresh rotation test")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	tenantID, otherTenantID, userID := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic rotation tenant'),($2,'Other synthetic tenant')`, tenantID, otherTenantID)
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id IN ($1,$2)`, tenantID, otherTenantID)
	}()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic')`, userID, tenantID, "rotation-"+userID.String()+"@example.test")
	repo := NewAuthRepository(pool)
	now := time.Now().UTC()
	write := func(hash string, expires time.Time) authusecase.RefreshTokenWrite {
		return authusecase.RefreshTokenWrite{TenantID: tenantID, UserID: userID, TokenHash: hash, ExpiresAt: expires}
	}
	for _, hash := range []string{"old-rollback", "existing-replacement", "old-race", "old-expired", "old-revoked"} {
		if err := repo.CreateRefreshToken(ctx, write(hash, now.Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
	}

	// A uniqueness failure in the replacement INSERT must roll back the revocation.
	if err := repo.RotateRefreshToken(ctx, tenantID, "old-rollback", write("existing-replacement", now.Add(time.Hour)), now); err == nil {
		t.Fatal("expected replacement INSERT to fail")
	}
	old, err := repo.GetRefreshToken(ctx, tenantID, "old-rollback")
	if err != nil || old.RevokedAt != nil {
		t.Fatalf("failed rotation consumed old token: token=%+v err=%v", old, err)
	}
	if err := repo.RotateRefreshToken(ctx, tenantID, "old-rollback", write("replacement-after-rollback", now.Add(time.Hour)), now); err != nil {
		t.Fatalf("old token should remain usable after rollback: %v", err)
	}

	if err := repo.RotateRefreshToken(ctx, otherTenantID, "old-race", write("cross-tenant", now.Add(time.Hour)), now); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("cross-tenant token should be hidden, got %v", err)
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE refresh_tokens SET expires_at=$3 WHERE tenant_id=$1 AND token_hash=$2`, tenantID, "old-expired", now.Add(-time.Second))
	if err := repo.RotateRefreshToken(ctx, tenantID, "old-expired", write("expired-replacement", now.Add(time.Hour)), now); !errors.Is(err, domainerrors.ErrUnauthorized) {
		t.Fatalf("expired token accepted: %v", err)
	}
	if err := repo.RevokeRefreshToken(ctx, tenantID, "old-revoked"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RotateRefreshToken(ctx, tenantID, "old-revoked", write("revoked-replacement", now.Add(time.Hour)), now); !errors.Is(err, domainerrors.ErrUnauthorized) {
		t.Fatalf("revoked token accepted: %v", err)
	}

	// Both calls start together. The row lock must let exactly one commit.
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, hash := range []string{"race-replacement-a", "race-replacement-b"} {
		wg.Add(1)
		go func(hash string) {
			defer wg.Done()
			<-start
			results <- repo.RotateRefreshToken(ctx, tenantID, "old-race", write(hash, now.Add(time.Hour)), now)
		}(hash)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, rejected := 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, domainerrors.ErrUnauthorized):
			rejected++
		default:
			t.Fatalf("unexpected concurrent rotation result: %v", result)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected one commit and one rejection, got successes=%d rejected=%d", successes, rejected)
	}
	var replacements int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE tenant_id=$1 AND token_hash IN ('race-replacement-a','race-replacement-b')`, tenantID).Scan(&replacements); err != nil {
		t.Fatal(err)
	}
	if replacements != 1 {
		t.Fatalf("expected exactly one durable replacement, got %d", replacements)
	}
}
