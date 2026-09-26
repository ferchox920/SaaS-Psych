package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func lockProjectHead(ctx context.Context, tx pgx.Tx, t, c uuid.UUID, expected int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text||':'||$2::text,0))`, t, c); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, t, c); err != nil {
		return err
	}
	var version int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2 FOR UPDATE`, t, c).Scan(&version); err != nil {
		return err
	}
	if version != expected {
		return domainerrors.ErrConflict
	}
	return nil
}
func (r *ClinicalLongitudinalRepository) SaveProjectExport(ctx context.Context, e longitudinal.ProjectExport) (longitudinal.ProjectExport, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return e, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockProjectHead(ctx, tx, e.TenantID, e.ClientID, e.StateVersion); err != nil {
		return e, err
	}
	artifact, err := json.Marshal(e.Artifact)
	if err != nil {
		return e, err
	}
	snapshot, err := json.Marshal(e.Snapshot)
	if err != nil {
		return e, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_project_exports(id,tenant_id,client_id,generated_by_user_id,state_version,schema_version,artifact_json,local_snapshot_json,artifact_markdown,content_hash,generated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, e.ID, e.TenantID, e.ClientID, e.GeneratedBy, e.StateVersion, longitudinal.ProjectExportVersion, artifact, snapshot, e.Markdown, e.ContentHash, e.GeneratedAt)
	if err != nil {
		return e, err
	}
	for _, source := range e.Sources {
		_, err = tx.Exec(ctx, `INSERT INTO clinical_project_export_sources(tenant_id,client_id,export_id,export_ref,entity_type,entity_id,entity_version) VALUES($1,$2,$3,$4,$5,$6,$7)`, e.TenantID, e.ClientID, e.ID, source.Ref, source.EntityType, source.EntityID, source.Version)
		if err != nil {
			return e, err
		}
	}
	if err := insertAuditEvent(ctx, tx, e.TenantID, e.GeneratedBy, "clinical_project_export.generated", "clinical_project_export", e.ID, map[string]any{"state_version": e.StateVersion, "source_count": len(e.Sources), "content_hash": e.ContentHash}); err != nil {
		return e, err
	}
	return e, tx.Commit(ctx)
}
func (r *ClinicalLongitudinalRepository) GetProjectExport(ctx context.Context, t, c, id uuid.UUID) (longitudinal.ProjectExport, error) {
	e := longitudinal.ProjectExport{TenantID: t, ClientID: c, ID: id, Lifecycle: "generated", Sources: []longitudinal.ProjectSource{}}
	var artifact, snapshot []byte
	err := r.pool.QueryRow(ctx, `SELECT generated_by_user_id,generated_at,state_version,artifact_json,local_snapshot_json,artifact_markdown,content_hash FROM clinical_project_exports WHERE tenant_id=$1 AND client_id=$2 AND id=$3`, t, c, id).Scan(&e.GeneratedBy, &e.GeneratedAt, &e.StateVersion, &artifact, &snapshot, &e.Markdown, &e.ContentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, domainerrors.ErrNotFound
	}
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(artifact, &e.Artifact); err != nil {
		return e, err
	}
	if err := json.Unmarshal(snapshot, &e.Snapshot); err != nil {
		return e, err
	}
	rows, err := r.pool.Query(ctx, `SELECT export_ref,entity_type,entity_id,entity_version FROM clinical_project_export_sources WHERE tenant_id=$1 AND client_id=$2 AND export_id=$3 ORDER BY export_ref`, t, c, id)
	if err != nil {
		return e, err
	}
	defer rows.Close()
	for rows.Next() {
		var s longitudinal.ProjectSource
		if err := rows.Scan(&s.Ref, &s.EntityType, &s.EntityID, &s.Version); err != nil {
			return e, err
		}
		e.Sources = append(e.Sources, s)
	}
	return e, rows.Err()
}
func (r *ClinicalLongitudinalRepository) ImportProjectProposal(ctx context.Context, p longitudinal.ProjectImportRecord, in longitudinal.CreateDiffInput) (longitudinal.Diff, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if p.ID != in.ExternalProposalID || p.TenantID != in.TenantID || p.ClientID != in.ClientID || p.ImportedBy != in.ActorID || in.RunID != uuid.Nil {
		return longitudinal.Diff{}, domainerrors.NewValidation("inconsistent manual provenance")
	}
	if err := lockProjectHead(ctx, tx, p.TenantID, p.ClientID, in.BaseStateVersion); err != nil {
		return longitudinal.Diff{}, err
	}
	var base int64
	var hash string
	err = tx.QueryRow(ctx, `SELECT state_version,content_hash FROM clinical_project_exports WHERE tenant_id=$1 AND client_id=$2 AND id=$3 FOR SHARE`, p.TenantID, p.ClientID, p.ExportID).Scan(&base, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.Diff{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if base != in.BaseStateVersion || hash != p.Proposal.SourceExportHash {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	raw, err := json.Marshal(p.Proposal)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	meta := p.Proposal.Provenance
	_, err = tx.Exec(ctx, `INSERT INTO clinical_external_proposals(id,tenant_id,client_id,source_export_id,provider_name,model_name,surface,project_context_version,content_hash,imported_by_user_id,proposal_json) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, p.ID, p.TenantID, p.ClientID, p.ExportID, meta.ProviderName, meta.ModelName, meta.Surface, meta.ProjectContextVersion, p.ContentHash, p.ImportedBy, raw)
	if isUniqueViolation(err) {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	id, err := createLongitudinalDiffTx(ctx, tx, in)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if err := insertAuditEvent(ctx, tx, p.TenantID, p.ImportedBy, "clinical_external_proposal.imported", "clinical_external_proposal", p.ID, map[string]any{"source_export_id": p.ExportID, "diff_id": id, "content_hash": p.ContentHash}); err != nil {
		return longitudinal.Diff{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, p.TenantID, id)
}
func (r *ClinicalLongitudinalRepository) GetProjectProposal(ctx context.Context, t, c, id uuid.UUID) (longitudinal.ProjectImportRecord, error) {
	p := longitudinal.ProjectImportRecord{ID: id, TenantID: t, ClientID: c}
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT source_export_id,imported_by_user_id,content_hash,proposal_json FROM clinical_external_proposals WHERE tenant_id=$1 AND client_id=$2 AND id=$3`, t, c, id).Scan(&p.ExportID, &p.ImportedBy, &p.ContentHash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domainerrors.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(raw, &p.Proposal)
}
