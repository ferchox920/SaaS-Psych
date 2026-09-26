package db

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func (r *ClinicalLongitudinalRepository) RunProvenance(ctx context.Context, t, c, id uuid.UUID) (out longitudinal.RunProvenance, err error) {
	err = r.pool.QueryRow(ctx, `SELECT r.id,r.provider,r.model,r.operation,r.prompt_name,r.prompt_version,r.status,r.started_at,r.completed_at,(SELECT a.transcript_version_id FROM clinical_ingestion_run_attempts a WHERE a.tenant_id=r.tenant_id AND a.client_id=r.client_id AND a.ai_run_id=r.id LIMIT 1) FROM clinical_ai_runs r WHERE r.tenant_id=$1 AND r.client_id=$2 AND r.id=$3`, t, c, id).Scan(&out.ID, &out.Provider, &out.Model, &out.Operation, &out.PromptName, &out.PromptVersion, &out.Status, &out.StartedAt, &out.CompletedAt, &out.TranscriptVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domainerrors.ErrNotFound
	}
	return
}
func (r *ClinicalLongitudinalRepository) EvidencePage(ctx context.Context, t, c uuid.UUID, offset int) (out longitudinal.EvidencePage, err error) {
	out = longitudinal.EvidencePage{Items: []longitudinal.Evidence{}, Offset: offset}
	rows, err := r.pool.Query(ctx, evidenceSelect+` WHERE e.tenant_id=$1 AND e.client_id=$2 ORDER BY e.created_at DESC,e.id LIMIT 26 OFFSET $3`, t, c, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanEvidence(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	if len(out.Items) > 25 {
		out.HasMore = true
		out.Items = out.Items[:25]
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) HistoryPage(ctx context.Context, t, c, id uuid.UUID, kind string, offset int) (out longitudinal.HistoryPage, err error) {
	out = longitudinal.HistoryPage{Items: []longitudinal.HistoryTransition{}, Offset: offset}
	tables := map[string]string{"process": "clinical_processes", "hypothesis": "clinical_hypotheses", "goal": "clinical_goals", "gira": "giras"}
	table, ok := tables[kind]
	if !ok {
		return out, domainerrors.ErrValidation
	}
	var exists bool
	err = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE tenant_id=$1 AND client_id=$2 AND id=$3)`, t, c, id).Scan(&exists)
	if err != nil {
		return out, err
	}
	if !exists {
		return out, domainerrors.ErrNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT tr.id,tr.diff_id,tr.operation_id,tr.entity_type,tr.entity_id,tr.action,tr.from_version,tr.to_version,tr.from_status,tr.to_status,tr.actor_user_id,d.merged_by_user_id,d.merged_at,o.original_proposal,o.human_modification,tr.created_at FROM clinical_longitudinal_transitions tr JOIN clinical_diffs d ON d.tenant_id=tr.tenant_id AND d.id=tr.diff_id JOIN clinical_diff_operations o ON o.tenant_id=tr.tenant_id AND o.id=tr.operation_id WHERE tr.tenant_id=$1 AND tr.client_id=$2 AND tr.entity_type=$3 AND tr.entity_id=$4 ORDER BY tr.created_at DESC,tr.id LIMIT 26 OFFSET $5`, t, c, kind, id, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x longitudinal.HistoryTransition
		if err := rows.Scan(&x.ID, &x.DiffID, &x.OperationID, &x.EntityType, &x.EntityID, &x.Action, &x.FromVersion, &x.ToVersion, &x.FromStatus, &x.ToStatus, &x.ActorUserID, &x.MergedByUserID, &x.MergedAt, &x.OriginalProposal, &x.HumanModification, &x.CreatedAt); err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	if len(out.Items) > 25 {
		out.HasMore = true
		out.Items = out.Items[:25]
	}
	return out, rows.Err()
}
