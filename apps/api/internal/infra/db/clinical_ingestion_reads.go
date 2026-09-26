package db

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

func (r *ClinicalIngestionRepository) ListArtifacts(ctx context.Context, t, sid uuid.UUID) ([]ingestion.Artifact, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND session_id=$2 ORDER BY created_at DESC,id LIMIT 200`, t, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ingestion.Artifact{}
	for rows.Next() {
		v, e := scanArtifact(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ClinicalIngestionRepository) ListJobs(ctx context.Context, t, sid uuid.UUID) ([]ingestion.Job, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND session_id=$2 ORDER BY created_at DESC,id LIMIT 200`, t, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ingestion.Job{}
	for rows.Next() {
		v, e := scanIngestionJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ClinicalIngestionRepository) ListTranscripts(ctx context.Context, t, sid uuid.UUID) ([]ingestion.Transcript, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND session_id=$2 ORDER BY version DESC,id LIMIT 100`, t, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ingestion.Transcript{}
	for rows.Next() {
		v, e := scanTranscript(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ClinicalIngestionRepository) RetryJob(ctx context.Context, t, id, a uuid.UUID) (ingestion.Job, error) {
	j, err := r.GetJob(ctx, t, id)
	if err != nil {
		return j, err
	}
	if a == uuid.Nil {
		return j, domainerrors.ErrForbidden
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return j, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	scope := consent.Transcription
	if j.Type == ingestion.Analyze {
		scope = consent.LocalAI
	} else if j.Type != ingestion.Transcribe {
		return j, domainerrors.ErrConflict
	}
	if _, err = authorizeConsentTx(ctx, tx, t, j.ClientID, scope, "job_retry", j.ID); err != nil {
		return j, err
	}
	j, err = scanIngestionJob(tx.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, t, id))
	if err != nil {
		return j, err
	}
	if j.Status != "queued" || j.ErrorCode == nil || !ingestion.Retryable(*j.ErrorCode) || j.Attempt >= j.MaxAttempts {
		return j, domainerrors.ErrConflict
	}
	// Expedite a scheduled retry only; never reset the attempt budget or reopen a
	// cancelled/permanent-failure job. Execution revalidates its exact source.
	j, err = scanIngestionJob(tx.QueryRow(ctx, `UPDATE clinical_ingestion_jobs SET available_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 RETURNING `+ingestionJobColumns, t, id))
	if err != nil {
		return j, err
	}
	if err := insertAuditEvent(ctx, tx, t, a, "clinical_job.retry_requested", "clinical_job", id, map[string]any{"attempt": j.Attempt, "job_type": j.Type}); err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
