package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

type ClinicalMemoryRepository struct{ pool *pgxpool.Pool }

func NewClinicalMemoryRepository(pool *pgxpool.Pool) *ClinicalMemoryRepository {
	return &ClinicalMemoryRepository{pool: pool}
}

func (r *ClinicalMemoryRepository) CreateSuggestion(ctx context.Context, input clinicalmemory.CreateSuggestionInput) (clinicalmemory.Suggestion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		INSERT INTO clinical_ai_suggestions (
			tenant_id, client_id, appointment_id, created_by_user_id, mode, prompt_version, result, ai_run_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, client_id, appointment_id, created_by_user_id, mode, prompt_version,
		          result, disposition, correction_text, decision_reason, decided_by_user_id, decided_at, created_at, ai_run_id
	`
	item, err := scanSuggestion(tx.QueryRow(ctx, query, input.TenantID, input.ClientID, input.AppointmentID, input.ActorUserID, input.Mode, input.PromptVersion, input.Result, input.AIRunID))
	if err != nil {
		return clinicalmemory.Suggestion{}, fmt.Errorf("insert clinical AI suggestion: %w", err)
	}
	if err := insertAuditEvent(ctx, tx, input.TenantID, input.ActorUserID, "clinical_ai.suggestion.create", "clinical_ai_suggestion", item.ID, map[string]any{
		"appointment_id": input.AppointmentID, "mode": input.Mode, "prompt_version": input.PromptVersion,
	}); err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	return item, nil
}

func (r *ClinicalMemoryRepository) GetSuggestion(ctx context.Context, tenantID, suggestionID uuid.UUID) (clinicalmemory.Suggestion, error) {
	const query = `
		SELECT id, tenant_id, client_id, appointment_id, created_by_user_id, mode, prompt_version,
		       result, disposition, correction_text, decision_reason, decided_by_user_id, decided_at, created_at, ai_run_id
		FROM clinical_ai_suggestions WHERE tenant_id = $1 AND id = $2
	`
	item, err := scanSuggestion(r.pool.QueryRow(ctx, query, tenantID, suggestionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return clinicalmemory.Suggestion{}, domainerrors.ErrNotFound
	}
	return item, err
}

func (r *ClinicalMemoryRepository) ClientIDForAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID) (uuid.UUID, error) {
	var clientID uuid.UUID
	if err := r.pool.QueryRow(ctx, `SELECT client_id FROM appointments WHERE tenant_id = $1 AND id = $2`, tenantID, appointmentID).Scan(&clientID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, domainerrors.ErrNotFound
		}
		return uuid.Nil, err
	}
	return clientID, nil
}

func (r *ClinicalMemoryRepository) SuggestionProvenance(ctx context.Context, tenantID, suggestionID uuid.UUID) (uuid.UUID, string, error) {
	var clientID uuid.UUID
	var disposition string
	if err := r.pool.QueryRow(ctx, `SELECT client_id, disposition FROM clinical_ai_suggestions WHERE tenant_id = $1 AND id = $2`, tenantID, suggestionID).Scan(&clientID, &disposition); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", domainerrors.ErrNotFound
		}
		return uuid.Nil, "", err
	}
	return clientID, disposition, nil
}

func (r *ClinicalMemoryRepository) ListSuggestions(ctx context.Context, tenantID, appointmentID uuid.UUID) ([]clinicalmemory.Suggestion, error) {
	const query = `
		SELECT id, tenant_id, client_id, appointment_id, created_by_user_id, mode, prompt_version,
		       result, disposition, correction_text, decision_reason, decided_by_user_id, decided_at, created_at, ai_run_id
		FROM clinical_ai_suggestions
		WHERE tenant_id = $1 AND appointment_id = $2
		ORDER BY created_at DESC, id DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, appointmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]clinicalmemory.Suggestion, 0)
	for rows.Next() {
		item, err := scanSuggestion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ClinicalMemoryRepository) DecideSuggestion(ctx context.Context, tenantID, suggestionID, actorUserID uuid.UUID, disposition, correction, reason string) (clinicalmemory.Suggestion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE clinical_ai_suggestions
		SET disposition = $3, correction_text = $4, decision_reason = $5,
		    decided_by_user_id = $6, decided_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND disposition IN ('pending', 'postponed')
		RETURNING id, tenant_id, client_id, appointment_id, created_by_user_id, mode, prompt_version,
		          result, disposition, correction_text, decision_reason, decided_by_user_id, decided_at, created_at, ai_run_id
	`
	item, err := scanSuggestion(tx.QueryRow(ctx, query, tenantID, suggestionID, disposition, correction, reason, actorUserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return clinicalmemory.Suggestion{}, domainerrors.ErrConflict
	}
	if err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	if err := insertAuditEvent(ctx, tx, tenantID, actorUserID, "clinical_ai.suggestion."+disposition, "clinical_ai_suggestion", suggestionID, map[string]any{
		"disposition": disposition, "has_correction": correction != "", "has_reason": reason != "",
	}); err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalmemory.Suggestion{}, err
	}
	return item, nil
}

func (r *ClinicalMemoryRepository) CreateSnapshot(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, summary string, anchors []clinicalmemory.Anchor) (clinicalmemory.Snapshot, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockClinicalFormulation(ctx, tx, tenantID, clientID); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	const snapshotQuery = `
		INSERT INTO clinical_formulation_snapshots (tenant_id, client_id, version, approved_summary, created_by_user_id)
		SELECT $1, $2, COALESCE(MAX(version), 0) + 1, $3, $4
		FROM clinical_formulation_snapshots WHERE tenant_id = $1 AND client_id = $2
		RETURNING id, tenant_id, client_id, version, approved_summary, status,
		          created_by_user_id, approved_by_user_id, approved_at, created_at
	`
	item, err := scanSnapshot(tx.QueryRow(ctx, snapshotQuery, tenantID, clientID, summary, actorUserID))
	if err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	const anchorQuery = `
		INSERT INTO clinical_formulation_anchors (tenant_id, snapshot_id, source_id, kind, summary, traffic_light, source_suggestion_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, source_id, kind, summary, traffic_light, source_suggestion_id
	`
	item.Anchors = make([]clinicalmemory.Anchor, 0, len(anchors))
	for _, anchor := range anchors {
		created, err := scanAnchor(tx.QueryRow(ctx, anchorQuery, tenantID, item.ID, anchor.SourceID, anchor.Kind, anchor.Summary, anchor.TrafficLight, anchor.SourceSuggestionID))
		if err != nil {
			return clinicalmemory.Snapshot{}, err
		}
		item.Anchors = append(item.Anchors, created)
	}
	if err := insertAuditEvent(ctx, tx, tenantID, actorUserID, "clinical_formulation.create_draft", "clinical_formulation", item.ID, map[string]any{
		"client_id": clientID, "version": item.Version, "anchor_count": len(anchors),
	}); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	return item, nil
}

func (r *ClinicalMemoryRepository) ListSnapshots(ctx context.Context, tenantID, clientID uuid.UUID) ([]clinicalmemory.Snapshot, error) {
	const query = `
		SELECT id, tenant_id, client_id, version, approved_summary, status,
		       created_by_user_id, approved_by_user_id, approved_at, created_at
		FROM clinical_formulation_snapshots
		WHERE tenant_id = $1 AND client_id = $2 ORDER BY version DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]clinicalmemory.Snapshot, 0)
	for rows.Next() {
		item, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		anchors, err := r.listAnchors(ctx, tenantID, item.ID)
		if err != nil {
			return nil, err
		}
		item.Anchors = anchors
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ClinicalMemoryRepository) GetApprovedContext(ctx context.Context, tenantID, clientID uuid.UUID) (clinicalmemory.ApprovedContext, error) {
	const query = `
		SELECT id, version, approved_summary
		FROM clinical_formulation_snapshots
		WHERE tenant_id = $1 AND client_id = $2 AND status = 'approved'
		ORDER BY version DESC, id DESC
		LIMIT 1
	`
	var item clinicalmemory.ApprovedContext
	if err := r.pool.QueryRow(ctx, query, tenantID, clientID).Scan(&item.SnapshotID, &item.Version, &item.ApprovedSummary); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return clinicalmemory.ApprovedContext{}, domainerrors.ErrNotFound
		}
		return clinicalmemory.ApprovedContext{}, err
	}
	anchors, err := r.listAnchors(ctx, tenantID, item.SnapshotID)
	if err != nil {
		return clinicalmemory.ApprovedContext{}, err
	}
	item.Anchors = anchors
	return item, nil
}

func (r *ClinicalMemoryRepository) ApproveSnapshot(ctx context.Context, tenantID, clientID, snapshotID, actorUserID uuid.UUID) (clinicalmemory.Snapshot, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockClinicalFormulation(ctx, tx, tenantID, clientID); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE clinical_formulation_snapshots SET status = 'superseded' WHERE tenant_id = $1 AND client_id = $2 AND status = 'approved'`, tenantID, clientID); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	const query = `
		UPDATE clinical_formulation_snapshots
		SET status = 'approved', approved_by_user_id = $4, approved_at = NOW()
		WHERE tenant_id = $1 AND client_id = $2 AND id = $3 AND status = 'draft'
		RETURNING id, tenant_id, client_id, version, approved_summary, status,
		          created_by_user_id, approved_by_user_id, approved_at, created_at
	`
	item, err := scanSnapshot(tx.QueryRow(ctx, query, tenantID, clientID, snapshotID, actorUserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return clinicalmemory.Snapshot{}, domainerrors.ErrConflict
	}
	if err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	anchors, err := listAnchorsWith(ctx, tx, tenantID, snapshotID)
	if err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	item.Anchors = anchors
	if err := insertAuditEvent(ctx, tx, tenantID, actorUserID, "clinical_formulation.approve", "clinical_formulation", snapshotID, map[string]any{
		"client_id": clientID, "version": item.Version, "anchor_count": len(anchors),
	}); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalmemory.Snapshot{}, err
	}
	return item, nil
}

func lockClinicalFormulation(ctx context.Context, tx pgx.Tx, tenantID, clientID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text || ':' || $2::text, 0))`, tenantID, clientID); err != nil {
		return fmt.Errorf("lock clinical formulation: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanSuggestion(row rowScanner) (clinicalmemory.Suggestion, error) {
	var item clinicalmemory.Suggestion
	var result []byte
	err := row.Scan(&item.ID, &item.TenantID, &item.ClientID, &item.AppointmentID, &item.CreatedByUserID,
		&item.Mode, &item.PromptVersion, &result, &item.Disposition, &item.CorrectionText, &item.DecisionReason,
		&item.DecidedByUserID, &item.DecidedAt, &item.CreatedAt, &item.AIRunID)
	item.Result = json.RawMessage(result)
	return item, err
}

func scanSnapshot(row rowScanner) (clinicalmemory.Snapshot, error) {
	var item clinicalmemory.Snapshot
	err := row.Scan(&item.ID, &item.TenantID, &item.ClientID, &item.Version, &item.ApprovedSummary, &item.Status,
		&item.CreatedByUserID, &item.ApprovedByUserID, &item.ApprovedAt, &item.CreatedAt)
	return item, err
}

func scanAnchor(row rowScanner) (clinicalmemory.Anchor, error) {
	var item clinicalmemory.Anchor
	err := row.Scan(&item.ID, &item.SourceID, &item.Kind, &item.Summary, &item.TrafficLight, &item.SourceSuggestionID)
	return item, err
}

func (r *ClinicalMemoryRepository) listAnchors(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]clinicalmemory.Anchor, error) {
	return listAnchorsWith(ctx, r.pool, tenantID, snapshotID)
}

type anchorQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listAnchorsWith(ctx context.Context, querier anchorQuerier, tenantID, snapshotID uuid.UUID) ([]clinicalmemory.Anchor, error) {
	const query = `
		SELECT id, source_id, kind, summary, traffic_light, source_suggestion_id
		FROM clinical_formulation_anchors
		WHERE tenant_id = $1 AND snapshot_id = $2 ORDER BY created_at ASC, id ASC
	`
	rows, err := querier.Query(ctx, query, tenantID, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]clinicalmemory.Anchor, 0)
	for rows.Next() {
		item, err := scanAnchor(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
