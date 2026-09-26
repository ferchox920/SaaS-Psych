package db

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"os"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"testing"
	"time"
)

func TestIngestionJobClaimRecoveryCancellationFencePostgres(t *testing.T) {
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
	cr := NewClinicalConsentRepository(pool)
	cs := consent.NewService(cr, longitudinalAcceptanceAccess{allowed: true})
	grant, err := cs.Grant(ctx, f.tenant, f.client, f.user, consent.Audio, 1)
	if err != nil {
		t.Fatal(err)
	}
	artifact := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_session_artifacts(id,tenant_id,client_id,session_id,media_type,storage_key,encryption_key_id,consent_grant_id,created_by_user_id,retention_until) VALUES($1,$2,$3,$4,'wav',$5,'test',$6,$7,NOW()+INTERVAL '1 day')`, artifact, f.tenant, f.client, f.session, f.tenant.String()+"/"+f.client.String()+"/"+f.session.String()+"/"+artifact.String(), grant.ID, f.user)
	id := uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_ingestion_jobs(id,tenant_id,client_id,session_id,job_type,artifact_id,idempotency_key,configuration_hash,created_by_user_id) VALUES($1,$2,$3,$4,'transcribe_session_audio',$5,$6,'config-v1',$7)`, id, f.tenant, f.client, f.session, artifact, id.String(), f.user)
	repo := NewClinicalIngestionRepository(pool)
	job, err := repo.Claim(ctx, f.tenant, time.Minute)
	if err != nil || job.Attempt != 1 {
		t.Fatalf("claim: %v", err)
	}
	if _, err = repo.Claim(ctx, f.tenant, time.Minute); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("concurrent duplicate claim")
	}
	mustExecIntegrationSQL(t, pool, ctx, `UPDATE clinical_ingestion_jobs SET lease_until=NOW()-INTERVAL '1 second' WHERE tenant_id=$1 AND id=$2`, f.tenant, id)
	if err = repo.FailAttempt(ctx, job, "timeout"); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatal("expired worker retained write authority")
	}
	n, err := repo.RecoverExpired(ctx, f.tenant, 10)
	if err != nil || n != 1 {
		t.Fatalf("recovery: %v", err)
	}
	retry, err := repo.Claim(ctx, f.tenant, time.Minute)
	if err != nil || retry.Attempt != 2 || *retry.LeaseToken == *job.LeaseToken {
		t.Fatal("retry did not fence prior worker")
	}
	if err = repo.FailAttempt(ctx, job, "timeout"); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatal("stale worker changed retry")
	}
	if _, err = repo.CancelJob(ctx, f.tenant, id, f.user); err != nil {
		t.Fatal(err)
	}
	if err = repo.FailAttempt(ctx, retry, "timeout"); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatal("cancelled worker changed state")
	}
	current, err := repo.GetJob(ctx, f.tenant, id)
	if err != nil || current.Status != "cancelled" {
		t.Fatal("cancellation lost")
	}
	if _, err = repo.GetJob(ctx, uuid.New(), id); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("foreign job exposed")
	}
	var attempts int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_ingestion_job_attempts WHERE tenant_id=$1 AND job_id=$2`, f.tenant, id).Scan(&attempts)
	if err != nil || attempts != 2 {
		t.Fatal("attempt history lost")
	}
}
