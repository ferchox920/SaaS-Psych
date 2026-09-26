package db

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

func (r *ClinicalIngestionRepository) MaintenanceArtifacts(ctx context.Context, t, after uuid.UUID, limit int) ([]ingestion.Artifact, error) {
	if t == uuid.Nil || limit < 1 || limit > 100 {
		return nil, domainerrors.NewValidation("invalid maintenance batch")
	}
	rows, err := r.pool.Query(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id>$2 AND status<>'deleted' ORDER BY id LIMIT $3`, t, after, limit)
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
func (r *ClinicalIngestionRepository) MarkArtifactMissing(ctx context.Context, t, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	v, err := scanArtifact(tx.QueryRow(ctx, `SELECT `+artifactColumns+` FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2`, t, id))
	if err != nil {
		return err
	}
	if err := lockConsentClient(ctx, tx, t, v.ClientID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE clinical_session_artifacts SET status='missing' WHERE tenant_id=$1 AND id=$2 AND status='available'`, t, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status='cancelled',error_code='artifact_deleted',lease_token=NULL,lease_until=NULL,cancelled_at=clock_timestamp(),completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND artifact_id=$2 AND status IN('queued','running')`, t, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='cancelled',error_code='artifact_deleted',completed_at=clock_timestamp() WHERE tenant_id=$1 AND outcome='running' AND job_id IN(SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND artifact_id=$2 AND status='cancelled')`, t, id)
	if err != nil {
		return err
	}
	if err := insertAuditEvent(ctx, tx, t, v.ActorID, "clinical_audio.missing", "clinical_artifact", id, map[string]any{"status": "missing"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
