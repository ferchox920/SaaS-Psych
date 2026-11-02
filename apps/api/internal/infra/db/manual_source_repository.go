package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	material "sessionflow/apps/api/internal/domain/manualsource"
)

type ManualSourceRepository struct{ pool *pgxpool.Pool }

func NewManualSourceRepository(p *pgxpool.Pool) *ManualSourceRepository {
	return &ManualSourceRepository{p}
}

func (r *ManualSourceRepository) Import(ctx context.Context, t, c, a uuid.UUID, v material.Import, raw []byte, hash string) (material.Material, bool, error) {
	var empty material.Material
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var client uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM clients WHERE tenant_id=$1 AND id=$2 AND archived_at IS NULL FOR UPDATE`, t, c).Scan(&client); err != nil {
		return empty, false, sourceError(err)
	}
	// Lock an active treating assignment so revocation cannot race publication.
	var assignment uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM client_clinical_assignments WHERE tenant_id=$1 AND client_id=$2 AND user_id=$3 AND relationship='treating' AND starts_at<=clock_timestamp() AND (ends_at IS NULL OR ends_at>clock_timestamp()) ORDER BY id LIMIT 1 FOR SHARE`, t, c, a).Scan(&assignment); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = domainerrors.ErrForbidden
		}
		return empty, false, err
	}
	var associated bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM client_profiles WHERE tenant_id=$1 AND client_id=$2 AND profile->'chatgpt_projects' @> $3::jsonb)`, t, c, []byte(`[{"project_id":"`+v.ProjectID+`"}]`)).Scan(&associated); err != nil {
		return empty, false, err
	}
	if !associated {
		return empty, false, domainerrors.NewValidation("project is not associated with this patient")
	}
	if v.SessionID != nil {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_sessions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND status<>'voided')`, t, *v.SessionID, c).Scan(&valid); err != nil {
			return empty, false, err
		}
		if !valid {
			return empty, false, domainerrors.ErrNotFound
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_manual_sources(tenant_id,client_id,source_kind,project_id,source_id,source_created_at,source_updated_at,candidate_session_date,session_date_basis,source_timezone,clinical_session_id,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8::date,$9,$10,$11,$12) ON CONFLICT(tenant_id,source_kind,project_id,source_id) DO NOTHING`, t, c, v.Kind, v.ProjectID, v.SourceID, v.SourceCreatedAt, v.SourceUpdatedAt, v.CandidateDate, v.DateBasis, v.Timezone, v.SessionID, a)
	if err != nil {
		return empty, false, err
	}
	var id, owner uuid.UUID
	var current *uuid.UUID
	var previousUpdate *time.Time
	err = tx.QueryRow(ctx, `SELECT id,client_id,current_revision_id,source_updated_at FROM clinical_manual_sources WHERE tenant_id=$1 AND source_kind=$2 AND project_id=$3 AND source_id=$4 FOR UPDATE`, t, v.Kind, v.ProjectID, v.SourceID).Scan(&id, &owner, &current, &previousUpdate)
	if err != nil {
		return empty, false, err
	}
	if owner != c {
		return empty, false, domainerrors.ErrConflict
	}
	var revision uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM clinical_manual_source_revisions WHERE tenant_id=$1 AND source_id=$2 AND hash_scheme=$3 AND content_hash=$4`, t, id, material.HashScheme, hash).Scan(&revision)
	created := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !created {
		return empty, false, err
	}
	if created {
		if current != nil && previousUpdate != nil && v.SourceUpdatedAt != nil && v.SourceUpdatedAt.Before(*previousUpdate) {
			return empty, false, domainerrors.ErrConflict
		}
		err = tx.QueryRow(ctx, `INSERT INTO clinical_manual_source_revisions(tenant_id,client_id,source_id,content_hash,hash_scheme,created_by_user_id,created_at) VALUES($1,$2,$3,$4,$5,$6,clock_timestamp()) RETURNING id`, t, c, id, hash, material.HashScheme, a).Scan(&revision)
		if err != nil {
			return empty, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO clinical_manual_source_contents(tenant_id,client_id,revision_id,payload) VALUES($1,$2,$3,$4)`, t, c, revision, raw); err != nil {
			return empty, false, err
		}
		if _, err = tx.Exec(ctx, `UPDATE clinical_manual_sources SET current_revision_id=$3,source_updated_at=COALESCE($4,source_updated_at) WHERE tenant_id=$1 AND id=$2 AND client_id=$5`, t, id, revision, v.SourceUpdatedAt, c); err != nil {
			return empty, false, err
		}
		if err = insertAuditEvent(ctx, tx, t, a, "manual_source.imported", "manual_source", id, map[string]any{"revision_id": revision, "content_hash": hash, "source_kind": v.Kind}); err != nil {
			return empty, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, false, err
	}
	out, err := r.Get(ctx, t, c, id, &revision)
	return out, created, err
}

const sourceColumns = `s.id,s.client_id,s.source_kind,s.project_id,s.source_id,s.source_created_at,s.source_updated_at,s.candidate_session_date::text,s.session_date_basis,COALESCE(s.source_timezone,''),s.clinical_session_id,r.id,r.content_hash,r.review_status,r.created_at,(SELECT count(*) FROM clinical_manual_source_revisions v WHERE v.tenant_id=s.tenant_id AND v.source_id=s.id)`
const sourceJoins = ` FROM clinical_manual_sources s JOIN clients cl ON cl.tenant_id=s.tenant_id AND cl.id=s.client_id AND cl.archived_at IS NULL JOIN clinical_manual_source_revisions r ON r.tenant_id=s.tenant_id AND r.source_id=s.id AND r.client_id=s.client_id `

func scanMaterial(row pgx.Row, full bool) (v material.Material, err error) {
	var data []byte
	err = row.Scan(&v.ID, &v.ClientID, &v.Kind, &v.ProjectID, &v.SourceID, &v.SourceCreatedAt, &v.SourceUpdatedAt, &v.CandidateDate, &v.DateBasis, &v.Timezone, &v.SessionID, &v.RevisionID, &v.Hash, &v.ReviewStatus, &v.ImportedAt, &v.VersionCount, &data)
	if err != nil {
		return v, sourceError(err)
	}
	if full {
		err = json.Unmarshal(data, &v.Content)
	} else {
		v.Content.Title = string(data)
	}
	return
}
func (r *ManualSourceRepository) List(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]material.Material, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+sourceColumns+`,convert_to(p.payload->>'title','UTF8')`+sourceJoins+`JOIN clinical_manual_source_contents p ON p.tenant_id=r.tenant_id AND p.revision_id=r.id AND p.client_id=s.client_id WHERE s.tenant_id=$1 AND s.client_id=$2 AND r.id=s.current_revision_id ORDER BY COALESCE(s.candidate_session_date,s.source_created_at::date) DESC NULLS LAST,s.id LIMIT $3 OFFSET $4`, t, c, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []material.Material{}
	for rows.Next() {
		v, e := scanMaterial(rows, false)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ManualSourceRepository) Get(ctx context.Context, t, c, id uuid.UUID, revision *uuid.UUID) (material.Material, error) {
	return scanMaterial(r.pool.QueryRow(ctx, `SELECT `+sourceColumns+`,p.payload`+sourceJoins+`JOIN clinical_manual_source_contents p ON p.tenant_id=r.tenant_id AND p.revision_id=r.id AND p.client_id=s.client_id WHERE s.tenant_id=$1 AND s.client_id=$2 AND s.id=$3 AND r.id=COALESCE($4::uuid,s.current_revision_id)`, t, c, id, revision), true)
}
func sourceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domainerrors.ErrNotFound
	}
	return err
}

func (r *ManualSourceRepository) Versions(ctx context.Context, t, c, id uuid.UUID, limit, offset int) ([]material.Material, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+sourceColumns+`,convert_to(p.payload->>'title','UTF8')`+sourceJoins+`JOIN clinical_manual_source_contents p ON p.tenant_id=r.tenant_id AND p.revision_id=r.id AND p.client_id=s.client_id WHERE s.tenant_id=$1 AND s.client_id=$2 AND s.id=$3 ORDER BY r.created_at DESC,r.id DESC LIMIT $4 OFFSET $5`, t, c, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []material.Material{}
	for rows.Next() {
		v, e := scanMaterial(rows, false)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
