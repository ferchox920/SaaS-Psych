package db

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"os"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"testing"
)

func TestConsentScopesHistoryRevocationAndIsolationPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	repo := NewClinicalConsentRepository(pool)
	service := consent.NewService(repo, longitudinalAcceptanceAccess{allowed: true})
	grant, err := service.Grant(ctx, f.tenant, f.client, f.user, consent.LocalAI, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{consent.Audio, consent.Transcription, consent.ExternalManual} {
		if _, err = repo.Authorize(ctx, f.tenant, f.client, scope, "test", uuid.New()); !errors.Is(err, domainerrors.ErrForbidden) {
			t.Fatalf("local AI implicitly authorized %s: %v", scope, err)
		}
	}
	entity := uuid.New()
	if _, err = repo.Authorize(ctx, f.tenant, f.client, consent.LocalAI, "analysis", entity); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Grant(ctx, f.tenant, f.client, f.user, consent.LocalAI, 1); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatal("duplicate active grant")
	}
	if _, err = service.Revoke(ctx, f.tenant, uuid.New(), f.user, grant.ID); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("foreign client revocation")
	}
	if _, err = service.Revoke(ctx, uuid.New(), f.client, f.user, grant.ID); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("foreign tenant revocation")
	}
	revoked, err := service.Revoke(ctx, f.tenant, f.client, f.user, grant.ID)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err = repo.Authorize(ctx, f.tenant, f.client, consent.LocalAI, "analysis", uuid.New()); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatal("revoked grant authorized processing")
	}
	replacement, err := service.Grant(ctx, f.tenant, f.client, f.user, consent.LocalAI, 1)
	if err != nil || replacement.ID == grant.ID {
		t.Fatal("regrant destroyed history")
	}
	var original uuid.UUID
	var version int
	err = pool.QueryRow(ctx, `SELECT grant_id,definition_version FROM clinical_ingestion_authorizations WHERE tenant_id=$1 AND entity_id=$2`, f.tenant, entity).Scan(&original, &version)
	if err != nil || original != grant.ID || version != 1 {
		t.Fatal("historical authorization changed")
	}
	denied := consent.NewService(repo, longitudinalAcceptanceAccess{allowed: false})
	if _, err = denied.Grant(ctx, f.tenant, f.client, f.user, consent.Audio, 1); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatal("unauthorized grant")
	}
	if _, err = service.Grant(ctx, f.tenant, f.client, f.user, "AUTOMATED_REMOTE_AI", 1); err == nil {
		t.Fatal("remote scope enabled")
	}
}
