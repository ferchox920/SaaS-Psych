package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func (r *ClientRepository) GetProfile(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Profile, error) {
	var data []byte
	err := r.pool.QueryRow(ctx, `SELECT p.profile FROM client_profiles p JOIN clients c ON c.tenant_id=p.tenant_id AND c.id=p.client_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND c.archived_at IS NULL`, tenantID, clientID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := r.GetByID(ctx, tenantID, clientID); err != nil {
			return domainclient.Profile{}, err
		}
		return domainclient.DefaultProfile(), nil
	}
	if err != nil {
		return domainclient.Profile{}, fmt.Errorf("read client profile: %w", err)
	}
	var profile domainclient.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return profile, err
	}
	return profile, nil
}

func (r *ClientRepository) SaveProfile(ctx context.Context, tenantID, clientID, actorID uuid.UUID, profile domainclient.Profile) (domainclient.Profile, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainclient.Profile{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize with archive and other profile writes; keep every lookup tenant-scoped.
	var id uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM clients WHERE tenant_id=$1 AND id=$2 AND archived_at IS NULL FOR UPDATE`, tenantID, clientID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainclient.Profile{}, domainerrors.ErrNotFound
		}
		return domainclient.Profile{}, err
	}
	previous := profile.Revision
	for _, reference := range profile.ChatGPTProjects {
		// Ordered project locks prevent concurrent assignment of one case to two patients.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, tenantID.String()+"/chatgpt/"+reference.ProjectID); err != nil {
			return domainclient.Profile{}, err
		}
		var assigned bool
		refJSON, _ := json.Marshal([]map[string]string{{"project_id": reference.ProjectID}})
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM client_profiles WHERE tenant_id=$1 AND client_id<>$2 AND profile->'chatgpt_projects' @> $3::jsonb)`, tenantID, clientID, refJSON).Scan(&assigned); err != nil {
			return domainclient.Profile{}, err
		}
		if assigned {
			return domainclient.Profile{}, domainerrors.NewValidation("ChatGPT project is already associated with another patient")
		}
	}
	profile.Revision++
	data, err := json.Marshal(profile)
	if err != nil {
		return domainclient.Profile{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO client_profiles (tenant_id,client_id,profile,revision) SELECT $1,$2,$3,1 WHERE $4=0
 ON CONFLICT (tenant_id,client_id) DO UPDATE SET profile=EXCLUDED.profile,revision=client_profiles.revision+1,updated_at=now() WHERE client_profiles.revision=$4`, tenantID, clientID, data, previous)
	// Existing profiles use an explicit compare-and-swap, since INSERT above is only for revision zero.
	if err == nil && tag.RowsAffected() == 0 && previous > 0 {
		tag, err = tx.Exec(ctx, `UPDATE client_profiles SET profile=$3,revision=revision+1,updated_at=now() WHERE tenant_id=$1 AND client_id=$2 AND revision=$4`, tenantID, clientID, data, previous)
	}
	if err != nil {
		return domainclient.Profile{}, err
	}
	if tag.RowsAffected() != 1 {
		return domainclient.Profile{}, domainerrors.ErrConflict
	}
	if err = insertAuditEvent(ctx, tx, tenantID, actorID, "client.profile.update", "client", clientID, nil); err != nil {
		return domainclient.Profile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domainclient.Profile{}, err
	}
	return profile, nil
}
