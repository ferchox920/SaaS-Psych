package db

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

const artifactColumns = `id,tenant_id,client_id,session_id,status,media_type,storage_key,encryption_key_id,content_hash,size_bytes,retention_until,created_by_user_id,created_at`

func scanArtifact(row pgx.Row) (v ingestion.Artifact, err error) {
	err = row.Scan(&v.ID, &v.TenantID, &v.ClientID, &v.SessionID, &v.Status, &v.MediaType, &v.StorageKey, &v.KeyID, &v.Hash, &v.Size, &v.RetentionUntil, &v.ActorID, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domainerrors.ErrNotFound
	}
	return
}
func (r *ClinicalIngestionRepository) GetArtifact(ctx context.Context, t, id uuid.UUID) (ingestion.Artifact, error) {
	return scanArtifact(r.pool.QueryRow(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2`, t, id))
}
func (r *ClinicalIngestionRepository) ReserveArtifact(ctx context.Context, v ingestion.Artifact) (ingestion.Artifact, error) {
	expected := strings.Join([]string{v.TenantID.String(), v.ClientID.String(), v.SessionID.String(), v.ID.String()}, "/")
	if v.StorageKey != expected || v.ID == uuid.Nil || v.ActorID == uuid.Nil {
		return v, domainerrors.NewValidation("invalid artifact identity")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	g, err := authorizeConsentTx(ctx, tx, v.TenantID, v.ClientID, consent.Audio, "audio_upload", v.ID)
	if err != nil {
		return v, err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM clinical_sessions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 FOR SHARE`, v.TenantID, v.SessionID, v.ClientID).Scan(&status)
	if err != nil {
		return v, err
	}
	if status == "voided" {
		return v, domainerrors.ErrConflict
	}
	v, err = scanArtifact(tx.QueryRow(ctx, `INSERT INTO clinical_session_artifacts(id,tenant_id,client_id,session_id,media_type,storage_key,encryption_key_id,retention_until,created_by_user_id,consent_grant_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+artifactColumns, v.ID, v.TenantID, v.ClientID, v.SessionID, v.MediaType, v.StorageKey, v.KeyID, v.RetentionUntil, v.ActorID, g.ID))
	if err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, v.TenantID, v.ActorID, "clinical_audio.upload_started", "clinical_artifact", v.ID, map[string]any{"session_id": v.SessionID, "consent_id": g.ID}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) FinalizeArtifact(ctx context.Context, v ingestion.Artifact, receipt ingestion.AudioReceipt) (ingestion.Artifact, error) {
	if receipt.Key != v.StorageKey || receipt.KeyID != v.KeyID || receipt.Size < 1 || !ingestion.ValidHash(receipt.Hash) {
		return v, domainerrors.NewValidation("invalid storage receipt")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = authorizeConsentTx(ctx, tx, v.TenantID, v.ClientID, consent.Audio, "audio_finalize", v.ID); err != nil {
		return v, err
	}
	v, err = scanArtifact(tx.QueryRow(ctx, `UPDATE clinical_session_artifacts SET status='available',content_hash=$3,size_bytes=$4 WHERE tenant_id=$1 AND id=$2 AND status='uploading' AND retention_until>NOW() RETURNING `+artifactColumns, v.TenantID, v.ID, receipt.Hash, receipt.Size))
	if errors.Is(err, domainerrors.ErrNotFound) {
		err = domainerrors.ErrConflict
	}
	if err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, v.TenantID, v.ActorID, "clinical_audio.uploaded", "clinical_artifact", v.ID, map[string]any{"content_hash": receipt.Hash, "size_bytes": receipt.Size}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) BeginDeleteArtifact(ctx context.Context, t, id, a uuid.UUID) (ingestion.Artifact, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ingestion.Artifact{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Consent lock precedes artifact/job locks consistently with completion/revoke.
	v, err := scanArtifact(tx.QueryRow(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2`, t, id))
	if err != nil {
		return v, err
	}
	if err := lockConsentClient(ctx, tx, t, v.ClientID); err != nil {
		return v, err
	}
	v, err = scanArtifact(tx.QueryRow(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, t, id))
	if err != nil {
		return v, err
	}
	if v.Status == "deleted" {
		return v, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status='cancelled',error_code='artifact_deleted',lease_token=NULL,lease_until=NULL,cancelled_at=NOW(),completed_at=NOW(),updated_at=NOW() WHERE tenant_id=$1 AND artifact_id=$2 AND job_type='transcribe_session_audio' AND status IN('queued','running')`, t, id)
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='cancelled',error_code='artifact_deleted',completed_at=NOW() WHERE tenant_id=$1 AND outcome='running' AND job_id IN(SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND artifact_id=$2 AND status='cancelled')`, t, id)
	if err != nil {
		return v, err
	}
	v, err = scanArtifact(tx.QueryRow(ctx, `UPDATE clinical_session_artifacts SET status='deleting',deletion_requested_at=COALESCE(deletion_requested_at,NOW()) WHERE tenant_id=$1 AND id=$2 RETURNING `+artifactColumns, t, id))
	if err != nil {
		return v, err
	}
	if err := insertAuditEvent(ctx, tx, t, a, "clinical_audio.deletion_requested", "clinical_artifact", id, map[string]any{"status": "deleting"}); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) FinishDeleteArtifact(ctx context.Context, t, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	v, err := scanArtifact(tx.QueryRow(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, t, id))
	if err != nil {
		return err
	}
	if v.Status == "deleted" {
		return tx.Commit(ctx)
	}
	if v.Status != "deleting" {
		return domainerrors.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE clinical_session_artifacts SET status='deleted',deleted_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='deleting'`, t, id); err != nil {
		return err
	}
	// Completion can be a maintenance recovery, not a new human decision. The
	// requesting actor remains in deletion_requested; this event is system-owned.
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(tenant_id,actor_user_id,action,entity,entity_id,metadata) VALUES($1,NULL,'clinical_audio.deleted','clinical_artifact',$2,'{"status":"deleted"}'::jsonb)`, t, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) Enqueue(ctx context.Context, j ingestion.Job) (ingestion.Job, bool, error) {
	if !ingestion.ValidHash(j.ConfigurationHash) || j.ActorID == uuid.Nil || j.ID == uuid.Nil || j.Language != "es" || j.MaxAttempts < 1 || j.MaxAttempts > 5 {
		return j, false, domainerrors.NewValidation("invalid job")
	}
	scope := consent.Transcription
	var source uuid.UUID
	switch {
	case j.Type == ingestion.Transcribe && j.ArtifactID != nil && j.TranscriptID == nil:
		source = *j.ArtifactID
	case j.Type == ingestion.Analyze && j.TranscriptID != nil && j.ArtifactID == nil:
		source = *j.TranscriptID
		scope = consent.LocalAI
	default:
		return j, false, domainerrors.NewValidation("invalid job source")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return j, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = authorizeConsentTx(ctx, tx, j.TenantID, j.ClientID, scope, "job_enqueue", j.ID); err != nil {
		return j, false, err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM clinical_sessions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 FOR SHARE`, j.TenantID, j.SessionID, j.ClientID).Scan(&status)
	if err != nil {
		return j, false, err
	}
	if status == "voided" || (j.Type == ingestion.Analyze && status != "completed") {
		return j, false, domainerrors.ErrConflict
	}
	var hash string
	if j.Type == ingestion.Transcribe {
		err = tx.QueryRow(ctx, `SELECT content_hash FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND status='available' AND retention_until>NOW() FOR SHARE`, j.TenantID, source, j.ClientID, j.SessionID).Scan(&hash)
	} else {
		err = tx.QueryRow(ctx, `SELECT content_hash FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND status='available' FOR SHARE`, j.TenantID, source, j.ClientID, j.SessionID).Scan(&hash)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return j, false, domainerrors.ErrNotFound
	}
	if err != nil {
		return j, false, err
	}
	key := ingestion.Hash(j.SessionID.String() + "/" + j.Type + "/" + source.String() + "/" + hash + "/" + j.ConfigurationHash + "/" + j.Language)
	existing, e := scanIngestionJob(tx.QueryRow(ctx, `SELECT `+ingestionJobColumns+` FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND client_id=$2 AND idempotency_key=$3`, j.TenantID, j.ClientID, key))
	if e == nil {
		return existing, false, tx.Commit(ctx)
	}
	if !errors.Is(e, domainerrors.ErrNotFound) {
		return j, false, e
	}
	j, err = scanIngestionJob(tx.QueryRow(ctx, `INSERT INTO clinical_ingestion_jobs(id,tenant_id,client_id,session_id,job_type,artifact_id,transcript_version_id,created_by_user_id,configuration_hash,language,max_attempts,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+ingestionJobColumns, j.ID, j.TenantID, j.ClientID, j.SessionID, j.Type, j.ArtifactID, j.TranscriptID, j.ActorID, j.ConfigurationHash, j.Language, j.MaxAttempts, key))
	if err != nil {
		return j, false, err
	}
	if err := insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "clinical_job.queued", "clinical_job", j.ID, map[string]any{"job_type": j.Type, "configuration_hash": j.ConfigurationHash}); err != nil {
		return j, false, err
	}
	return j, true, tx.Commit(ctx)
}
