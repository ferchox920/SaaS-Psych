package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

const transcriptColumns = `id,tenant_id,client_id,session_id,version,parent_version_id,source_artifact_id,source_artifact_hash,source_artifact_version,origin,status,content_hash,created_at,COALESCE(transcript_text,''),COALESCE(segments_json,'[]'::jsonb),language,engine,model,engine_version,configuration_hash,COALESCE(duration_seconds,0),COALESCE(processing_seconds,0)`

func scanTranscript(row pgx.Row) (v ingestion.Transcript, err error) {
	var segments []byte
	err = row.Scan(&v.ID, &v.TenantID, &v.ClientID, &v.SessionID, &v.Version, &v.ParentID, &v.ArtifactID, &v.ArtifactHash, &v.ArtifactVersion, &v.Origin, &v.Status, &v.ContentHash, &v.CreatedAt, &v.Text, &segments, &v.Language, &v.Engine, &v.Model, &v.EngineVersion, &v.ConfigurationHash, &v.DurationSeconds, &v.ProcessingSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domainerrors.ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(segments, &v.Segments)
	}
	return
}
func (r *ClinicalIngestionRepository) GetTranscript(ctx context.Context, t, id uuid.UUID) (ingestion.Transcript, error) {
	return scanTranscript(r.pool.QueryRow(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2`, t, id))
}
func (r *ClinicalIngestionRepository) DeleteTranscript(ctx context.Context, t, id, a uuid.UUID) (ingestion.Transcript, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ingestion.Transcript{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	v, err := scanTranscript(tx.QueryRow(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2`, t, id))
	if err != nil {
		return v, err
	}
	if a == uuid.Nil {
		return v, domainerrors.ErrForbidden
	}
	if err := lockConsentClient(ctx, tx, t, v.ClientID); err != nil {
		return v, err
	}
	v, err = scanTranscript(tx.QueryRow(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, t, id))
	if err != nil {
		return v, err
	}
	if v.Status == "deleted" {
		return v, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status='cancelled',error_code='artifact_deleted',lease_token=NULL,lease_until=NULL,cancelled_at=clock_timestamp(),completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND transcript_version_id=$2 AND status IN('queued','running')`, t, id)
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ai_runs SET status='cancelled',error_code='artifact_deleted',completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND status='running' AND id IN(SELECT ai_run_id FROM clinical_ingestion_run_attempts WHERE tenant_id=$1 AND transcript_version_id=$2)`, t, id)
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='cancelled',error_code='artifact_deleted',completed_at=clock_timestamp() WHERE tenant_id=$1 AND outcome='running' AND job_id IN(SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND transcript_version_id=$2 AND status='cancelled')`, t, id)
	if err != nil {
		return v, err
	}
	v, err = scanTranscript(tx.QueryRow(ctx, `UPDATE clinical_transcript_versions SET status='deleted',transcript_text=NULL,segments_json=NULL,deleted_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 RETURNING `+transcriptColumns, t, id))
	if err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, t, a, "clinical_transcript.deleted", "clinical_transcript", id, map[string]any{"version": v.Version, "content_hash": v.ContentHash}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func insertTranscriptTx(ctx context.Context, tx pgx.Tx, v ingestion.Transcript, actor uuid.UUID) (ingestion.Transcript, error) {
	if err := v.Validate(); err != nil {
		return v, err
	}
	// Serialize version allocation per clinical session, including human correction.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('transcript:'||$1::text||':'||$2::text,0))`, v.TenantID, v.SessionID); err != nil {
		return v, err
	}
	segments, err := json.Marshal(v.Segments)
	if err != nil {
		return v, err
	}
	canonical, err := json.Marshal(v.TranscriptContent)
	if err != nil {
		return v, err
	}
	v.ContentHash = ingestion.Hash(string(canonical))
	return scanTranscript(tx.QueryRow(ctx, `INSERT INTO clinical_transcript_versions(id,tenant_id,client_id,session_id,version,parent_version_id,source_artifact_id,source_artifact_hash,source_artifact_version,origin,transcript_text,segments_json,language,engine,model,engine_version,configuration_hash,content_hash,created_by_user_id,duration_seconds,processing_seconds) SELECT $1,$2,$3,$4,COALESCE(MAX(version),0)+1,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20 FROM clinical_transcript_versions WHERE tenant_id=$2 AND session_id=$4 RETURNING `+transcriptColumns, v.ID, v.TenantID, v.ClientID, v.SessionID, v.ParentID, v.ArtifactID, v.ArtifactHash, v.ArtifactVersion, v.Origin, v.Text, segments, v.Language, v.Engine, v.Model, v.EngineVersion, v.ConfigurationHash, v.ContentHash, actor, v.DurationSeconds, v.ProcessingSeconds))
}
func (r *ClinicalIngestionRepository) CorrectTranscript(ctx context.Context, v ingestion.Transcript, a uuid.UUID) (ingestion.Transcript, error) {
	if v.ParentID == nil || v.Origin != "human_asr_correction" || a == uuid.Nil {
		return v, domainerrors.NewValidation("ASR correction requires a parent and human actor")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockConsentClient(ctx, tx, v.TenantID, v.ClientID); err != nil {
		return v, err
	}
	parent, err := scanTranscript(tx.QueryRow(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND status='available' FOR SHARE`, v.TenantID, *v.ParentID, v.ClientID, v.SessionID))
	if err != nil {
		return v, err
	}
	// A human corrects ASR text, not machine provenance or clinical facts.
	v.ArtifactID = parent.ArtifactID
	v.ArtifactHash = parent.ArtifactHash
	v.ArtifactVersion = parent.ArtifactVersion
	v.Engine = parent.Engine
	v.Model = parent.Model
	v.EngineVersion = parent.EngineVersion
	v.ConfigurationHash = parent.ConfigurationHash
	v.Language = parent.Language
	v.DurationSeconds = parent.DurationSeconds
	v.ProcessingSeconds = parent.ProcessingSeconds
	v, err = insertTranscriptTx(ctx, tx, v, a)
	if err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, v.TenantID, a, "clinical_transcript.asr_corrected", "clinical_transcript", v.ID, map[string]any{"version": v.Version, "parent_version_id": v.ParentID, "content_hash": v.ContentHash}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// lockIngestionAttemptTx must follow the consent-client lock. Database wall time
// (not transaction start time) fences a lease even after a long lock wait.
func lockIngestionAttemptTx(ctx context.Context, tx pgx.Tx, j ingestion.Job) (ingestion.Job, error) {
	if j.LeaseToken == nil {
		return ingestion.Job{}, domainerrors.ErrConflict
	}
	current, err := scanIngestionJob(tx.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND lease_token=$5 AND status='running' AND lease_until>clock_timestamp() FOR UPDATE`, j.TenantID, j.ID, j.ClientID, j.SessionID, j.LeaseToken))
	if errors.Is(err, domainerrors.ErrNotFound) {
		err = domainerrors.ErrConflict
	}
	if err != nil {
		return current, err
	}
	// A WHERE clause can be evaluated before FOR UPDATE waits on an unchanged
	// row. Re-read DB wall time only after ownership of that row lock is acquired.
	var valid bool
	err = tx.QueryRow(ctx, `SELECT lease_until>clock_timestamp() FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2`, j.TenantID, j.ID).Scan(&valid)
	if err == nil && !valid {
		err = domainerrors.ErrConflict
	}
	return current, err
}
func completeIngestionAttemptTx(ctx context.Context, tx pgx.Tx, j ingestion.Job, transcript, run, report *uuid.UUID) error {
	tag, err := tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status='succeeded',lease_token=NULL,lease_until=NULL,error_code=NULL,completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='running' AND lease_token=$3 AND lease_until>clock_timestamp()`, j.TenantID, j.ID, j.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domainerrors.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='succeeded',completed_at=clock_timestamp() WHERE tenant_id=$1 AND job_id=$2 AND lease_token=$3 AND outcome='running'`, j.TenantID, j.ID, j.LeaseToken); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO clinical_ingestion_results(tenant_id,client_id,session_id,job_id,transcript_version_id,ai_run_id,report_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, j.TenantID, j.ClientID, j.SessionID, j.ID, transcript, run, report); err != nil {
		return err
	}
	return insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "clinical_job.succeeded", "clinical_job", j.ID, map[string]any{"job_type": j.Type, "attempt": j.Attempt})
}
func (r *ClinicalIngestionRepository) CompleteTranscription(ctx context.Context, j ingestion.Job, content ingestion.TranscriptContent) (ingestion.Transcript, error) {
	if err := content.Validate(); err != nil {
		return ingestion.Transcript{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ingestion.Transcript{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = authorizeConsentTx(ctx, tx, j.TenantID, j.ClientID, consent.Transcription, "transcription_complete", j.ID); err != nil {
		return ingestion.Transcript{}, err
	}
	j, err = lockIngestionAttemptTx(ctx, tx, j)
	if err != nil {
		return ingestion.Transcript{}, err
	}
	if j.Type != ingestion.Transcribe || j.ArtifactID == nil || content.ConfigurationHash != j.ConfigurationHash {
		return ingestion.Transcript{}, domainerrors.ErrConflict
	}
	var hash string
	err = tx.QueryRow(ctx, `SELECT content_hash FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND status='available' AND retention_until>clock_timestamp() FOR SHARE`, j.TenantID, j.ArtifactID, j.ClientID, j.SessionID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ingestion.Transcript{}, domainerrors.ErrConflict
	}
	if err != nil {
		return ingestion.Transcript{}, err
	}
	v, err := insertTranscriptTx(ctx, tx, ingestion.Transcript{ID: uuid.New(), TenantID: j.TenantID, ClientID: j.ClientID, SessionID: j.SessionID, ArtifactID: *j.ArtifactID, ArtifactHash: hash, ArtifactVersion: 1, Origin: "machine", TranscriptContent: content}, j.ActorID)
	if err != nil {
		return v, err
	}
	if err := completeIngestionAttemptTx(ctx, tx, j, &v.ID, nil, nil); err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "clinical_transcript.created", "clinical_transcript", v.ID, map[string]any{"version": v.Version, "content_hash": v.ContentHash, "source_artifact_id": v.ArtifactID}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
