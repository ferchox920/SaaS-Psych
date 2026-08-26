package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainclinicalsession "sessionflow/apps/api/internal/domain/clinicalsession"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type ClinicalSessionRepository struct{ pool *pgxpool.Pool }

func NewClinicalSessionRepository(pool *pgxpool.Pool) *ClinicalSessionRepository {
	return &ClinicalSessionRepository{pool: pool}
}

func (r *ClinicalSessionRepository) ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clients WHERE tenant_id = $1 AND id = $2)`, tenantID, clientID).Scan(&exists)
	return exists, err
}
func (r *ClinicalSessionRepository) AppointmentClient(ctx context.Context, tenantID, appointmentID uuid.UUID) (uuid.UUID, error) {
	var clientID uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT client_id FROM appointments WHERE tenant_id = $1 AND id = $2`, tenantID, appointmentID).Scan(&clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domainerrors.ErrNotFound
	}
	return clientID, err
}
func (r *ClinicalSessionRepository) Create(ctx context.Context, item domainclinicalsession.Entity, actorID uuid.UUID) (domainclinicalsession.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainclinicalsession.Entity{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `INSERT INTO clinical_sessions (id, tenant_id, client_id, appointment_id, therapist_user_id, status, started_at, ended_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, client_id, appointment_id, therapist_user_id, status, started_at, ended_at, created_at, updated_at`
	out, err := scanClinicalSession(tx.QueryRow(ctx, query, item.ID, item.TenantID, item.ClientID, item.AppointmentID, item.TherapistUserID, item.Status, item.StartedAt, item.EndedAt, item.CreatedAt, item.UpdatedAt))
	if err != nil {
		return domainclinicalsession.Entity{}, mapClinicalSessionWriteError(err)
	}
	if err := insertAuditEvent(ctx, tx, item.TenantID, actorID, "clinical_session.created", "clinical_session", item.ID, map[string]any{"status": item.Status, "appointment_linked": item.AppointmentID != nil}); err != nil {
		return domainclinicalsession.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainclinicalsession.Entity{}, err
	}
	return out, nil
}
func (r *ClinicalSessionRepository) GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (domainclinicalsession.Entity, error) {
	const query = `SELECT id, tenant_id, client_id, appointment_id, therapist_user_id, status, started_at, ended_at, created_at, updated_at FROM clinical_sessions WHERE tenant_id = $1 AND id = $2`
	item, err := scanClinicalSession(r.pool.QueryRow(ctx, query, tenantID, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainclinicalsession.Entity{}, domainerrors.ErrNotFound
	}
	return item, err
}
func (r *ClinicalSessionRepository) ListByClient(ctx context.Context, tenantID, clientID uuid.UUID) ([]domainclinicalsession.Entity, error) {
	const query = `SELECT id, tenant_id, client_id, appointment_id, therapist_user_id, status, started_at, ended_at, created_at, updated_at FROM clinical_sessions WHERE tenant_id = $1 AND client_id = $2 ORDER BY started_at DESC, id DESC`
	rows, err := r.pool.Query(ctx, query, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domainclinicalsession.Entity{}
	for rows.Next() {
		item, scanErr := scanClinicalSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *ClinicalSessionRepository) Transition(ctx context.Context, item domainclinicalsession.Entity, actorID uuid.UUID) (domainclinicalsession.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainclinicalsession.Entity{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `UPDATE clinical_sessions SET status=$3, ended_at=$4, updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND status='in_progress' RETURNING id, tenant_id, client_id, appointment_id, therapist_user_id, status, started_at, ended_at, created_at, updated_at`
	out, err := scanClinicalSession(tx.QueryRow(ctx, query, item.TenantID, item.ID, item.Status, item.EndedAt, item.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainclinicalsession.Entity{}, domainerrors.ErrConflict
	}
	if err != nil {
		return domainclinicalsession.Entity{}, err
	}
	if err := insertAuditEvent(ctx, tx, item.TenantID, actorID, "clinical_session."+item.Status, "clinical_session", item.ID, map[string]any{"status": item.Status}); err != nil {
		return domainclinicalsession.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainclinicalsession.Entity{}, err
	}
	return out, nil
}
func scanClinicalSession(row pgx.Row) (domainclinicalsession.Entity, error) {
	var item domainclinicalsession.Entity
	err := row.Scan(&item.ID, &item.TenantID, &item.ClientID, &item.AppointmentID, &item.TherapistUserID, &item.Status, &item.StartedAt, &item.EndedAt, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}
func mapClinicalSessionWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return domainerrors.ErrConflict
		}
		if pgErr.Code == "23503" {
			return domainerrors.ErrForbidden
		}
		if pgErr.Code == "23514" {
			return domainerrors.ErrValidation
		}
	}
	return fmt.Errorf("write clinical session: %w", err)
}
