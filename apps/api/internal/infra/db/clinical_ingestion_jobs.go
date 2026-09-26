package db

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"time"
)

type ClinicalIngestionRepository struct{ pool *pgxpool.Pool }

func NewClinicalIngestionRepository(p *pgxpool.Pool) *ClinicalIngestionRepository {
	return &ClinicalIngestionRepository{p}
}

const ingestionJobColumns = `id,tenant_id,client_id,session_id,job_type,artifact_id,transcript_version_id,status,attempt,max_attempts,lease_token,lease_until,error_code,created_by_user_id,configuration_hash,language`

func scanIngestionJob(row pgx.Row) (j ingestion.Job, err error) {
	err = row.Scan(&j.ID, &j.TenantID, &j.ClientID, &j.SessionID, &j.Type, &j.ArtifactID, &j.TranscriptID, &j.Status, &j.Attempt, &j.MaxAttempts, &j.LeaseToken, &j.LeaseUntil, &j.ErrorCode, &j.ActorID, &j.ConfigurationHash, &j.Language)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domainerrors.ErrNotFound
	}
	return
}
func (r *ClinicalIngestionRepository) GetJob(ctx context.Context, t, id uuid.UUID) (ingestion.Job, error) {
	return scanIngestionJob(r.pool.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2`, t, id))
}

// Claim is an internal worker operation scoped to one tenant. A lease token
// fences every later attempt write; expired workers cannot publish results.
func (r *ClinicalIngestionRepository) Claim(ctx context.Context, t uuid.UUID, lease time.Duration) (ingestion.Job, error) {
	if t == uuid.Nil || lease < time.Second || lease > 30*time.Minute {
		return ingestion.Job{}, domainerrors.NewValidation("invalid tenant or lease")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ingestion.Job{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	j, err := scanIngestionJob(tx.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND status='queued' AND available_at<=NOW() AND attempt<max_attempts ORDER BY available_at,created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, t))
	if err != nil {
		return j, err
	}
	token := uuid.New()
	j, err = scanIngestionJob(tx.QueryRow(ctx, `UPDATE clinical_ingestion_jobs SET status='running',attempt=attempt+1,lease_token=$3,lease_until=NOW()+$4*INTERVAL '1 second',started_at=NOW(),updated_at=NOW(),error_code=NULL WHERE tenant_id=$1 AND id=$2 RETURNING `+ingestionJobColumns, t, j.ID, token, lease.Seconds()))
	if err != nil {
		return j, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_ingestion_job_attempts(tenant_id,client_id,session_id,job_id,attempt,lease_token) VALUES($1,$2,$3,$4,$5,$6)`, t, j.ClientID, j.SessionID, j.ID, j.Attempt, token)
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) FailAttempt(ctx context.Context, j ingestion.Job, code string) error {
	if j.LeaseToken == nil {
		return domainerrors.ErrConflict
	}
	if !ingestion.Retryable(code) && code != "validation" && code != "consent_revoked" && code != "artifact_deleted" && code != "cancelled" {
		code = "validation"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	status := "failed"
	// Use current locked state and a post-lock lease check, never caller attempt
	// fields or transaction-start NOW() after a potentially long lock wait.
	j, err = lockIngestionAttemptTx(ctx, tx, j)
	if err != nil {
		return err
	}
	if ingestion.Retryable(code) && j.Attempt < j.MaxAttempts {
		status = "queued"
	}
	tag, err := tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status=$4,error_code=$5,lease_token=NULL,lease_until=NULL,available_at=clock_timestamp()+$6*INTERVAL '1 second',completed_at=CASE WHEN $4='failed' THEN clock_timestamp() ELSE NULL END,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND lease_token=$3 AND status='running' AND lease_until>clock_timestamp()`, j.TenantID, j.ID, *j.LeaseToken, status, code, 5*j.Attempt*j.Attempt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domainerrors.ErrConflict
	}
	if err := terminateIngestionRunsTx(ctx, tx, j.TenantID, j.ID, code); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='failed',error_code=$4,completed_at=NOW() WHERE tenant_id=$1 AND job_id=$2 AND lease_token=$3 AND outcome='running'`, j.TenantID, j.ID, *j.LeaseToken, code)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) CancelJob(ctx context.Context, t, id, a uuid.UUID) (ingestion.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ingestion.Job{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	j, err := scanIngestionJob(tx.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, t, id))
	if err != nil {
		return j, err
	}
	if j.Status == "cancelled" {
		return j, tx.Commit(ctx)
	}
	if j.Status == "succeeded" {
		return j, domainerrors.ErrConflict
	}
	if err := terminateIngestionRunsTx(ctx, tx, t, id, "cancelled"); err != nil {
		return j, err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='cancelled',error_code='cancelled',completed_at=NOW() WHERE tenant_id=$1 AND job_id=$2 AND outcome='running'`, t, id)
	if err != nil {
		return j, err
	}
	j, err = scanIngestionJob(tx.QueryRow(ctx, `UPDATE clinical_ingestion_jobs SET status='cancelled',error_code='cancelled',cancelled_at=NOW(),completed_at=NOW(),lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE tenant_id=$1 AND id=$2 RETURNING `+ingestionJobColumns, t, id))
	if err != nil {
		return j, err
	}
	if err := insertAuditEvent(ctx, tx, t, a, "clinical_job.cancelled", "clinical_job", id, map[string]any{"job_type": j.Type, "attempt": j.Attempt}); err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) RecoverExpired(ctx context.Context, t uuid.UUID, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, domainerrors.NewValidation("recovery batch must be 1..100")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND status='running' AND lease_until<=NOW() ORDER BY lease_until,id FOR UPDATE SKIP LOCKED LIMIT $2`, t, limit)
	if err != nil {
		return 0, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := terminateIngestionRunsTx(ctx, tx, t, id, "worker_lost"); err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='failed',error_code='worker_lost',completed_at=NOW() WHERE tenant_id=$1 AND job_id=$2 AND outcome='running'`, t, id)
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status=CASE WHEN attempt<max_attempts THEN 'queued' ELSE 'failed' END,error_code='worker_lost',lease_token=NULL,lease_until=NULL,available_at=NOW(),updated_at=NOW(),completed_at=CASE WHEN attempt>=max_attempts THEN NOW() ELSE NULL END WHERE tenant_id=$1 AND id=$2`, t, id)
		if err != nil {
			return 0, err
		}
	}
	return len(ids), tx.Commit(ctx)
}
