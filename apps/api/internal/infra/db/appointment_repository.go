package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type AppointmentRepository struct {
	pool               *pgxpool.Pool
	transactionalAudit bool
}

func (r *AppointmentRepository) WithTransactionalAudit() *AppointmentRepository {
	r.transactionalAudit = true
	return r
}

func (r *AppointmentRepository) WritesAreTransactionallyAudited() bool { return r.transactionalAudit }

func NewAppointmentRepository(pool *pgxpool.Pool) *AppointmentRepository {
	return &AppointmentRepository{pool: pool}
}

func (r *AppointmentRepository) Create(ctx context.Context, in domainappointment.Entity, actorUserID uuid.UUID) (domainappointment.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("begin create appointment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAppointmentTenant(ctx, tx, in.TenantID); err != nil {
		return domainappointment.Entity{}, err
	}
	if in.Status != domainappointment.StatusCanceled {
		overlaps, err := appointmentOverlap(ctx, tx, in.TenantID, in.StartsAt, in.EndsAt, nil)
		if err != nil {
			return domainappointment.Entity{}, err
		}
		if overlaps {
			return domainappointment.Entity{}, fmt.Errorf("appointment overlaps existing slot: %w", domainerrors.ErrConflict)
		}
	}
	const query = `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at, revision
	`
	out, err := r.scanAppointment(tx.QueryRow(ctx, query,
		in.ID, in.TenantID, in.ClientID, in.StartsAt, in.EndsAt, in.Status, in.Location, in.CreatedAt, in.UpdatedAt,
	))
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("insert appointment: %w", err)
	}
	if err := r.insertAudit(ctx, tx, in.TenantID, actorUserID, "appointment.create", in.ID); err != nil {
		return domainappointment.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainappointment.Entity{}, fmt.Errorf("commit create appointment: %w", err)
	}
	return out, nil
}

func (r *AppointmentRepository) ListByRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error) {
	return r.listByRange(ctx, tenantID, nil, from, to, 0, 0)
}

func (r *AppointmentRepository) ListVisibleByRange(ctx context.Context, tenantID, viewerID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error) {
	return r.listByRange(ctx, tenantID, &viewerID, from, to, 0, 0)
}

func (r *AppointmentRepository) ListVisiblePage(ctx context.Context, tenantID, viewerID uuid.UUID, from, to time.Time, limit, offset int) ([]domainappointment.Entity, error) {
	return r.listByRange(ctx, tenantID, &viewerID, from, to, limit, offset)
}

func (r *AppointmentRepository) listByRange(ctx context.Context, tenantID uuid.UUID, viewerID *uuid.UUID, from, to time.Time, limit, offset int) ([]domainappointment.Entity, error) {
	query := `
		SELECT a.id, a.tenant_id, a.client_id, a.starts_at, a.ends_at, a.status, a.location, a.created_at, a.updated_at, a.revision
		FROM appointments a
		WHERE a.tenant_id = $1
		  AND a.starts_at >= $2
		  AND a.starts_at < $3
	`
	args := []any{tenantID, from, to}
	if viewerID != nil {
		query += ` AND (
			EXISTS (SELECT 1 FROM client_clinical_assignments cca
				WHERE cca.tenant_id=a.tenant_id AND cca.client_id=a.client_id AND cca.user_id=$4
				AND cca.relationship IN ('treating','supervisor')
				AND cca.starts_at<=$5 AND (cca.ends_at IS NULL OR cca.ends_at>$5))
			OR EXISTS (SELECT 1 FROM clinical_access_exceptions cae
				WHERE cae.tenant_id=a.tenant_id AND cae.client_id=a.client_id AND cae.user_id=$4
				AND cae.starts_at<=$5 AND cae.expires_at>$5 AND cae.revoked_at IS NULL)
		)`
		args = append(args, *viewerID, time.Now().UTC())
	}
	query += ` ORDER BY a.starts_at ASC, a.id ASC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query appointments by range: %w", err)
	}
	defer rows.Close()

	items := make([]domainappointment.Entity, 0)
	for rows.Next() {
		var (
			item      domainappointment.Entity
			startsAt  time.Time
			endsAt    time.Time
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ClientID, &startsAt, &endsAt, &item.Status, &item.Location, &createdAt, &updatedAt, &item.Revision); err != nil {
			return nil, fmt.Errorf("scan appointment row: %w", err)
		}
		item.StartsAt = startsAt.UTC()
		item.EndsAt = endsAt.UTC()
		item.CreatedAt = createdAt.UTC()
		item.UpdatedAt = updatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate appointment rows: %w", err)
	}
	return items, nil
}

func (r *AppointmentRepository) GetByID(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error) {
	const query = `
		SELECT id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at, revision
		FROM appointments
		WHERE tenant_id = $1 AND id = $2
		LIMIT 1
	`

	out, err := r.scanAppointment(r.pool.QueryRow(ctx, query, tenantID, appointmentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainappointment.Entity{}, domainerrors.ErrNotFound
		}
		return domainappointment.Entity{}, fmt.Errorf("query appointment by id: %w", err)
	}
	return out, nil
}

func (r *AppointmentRepository) Update(ctx context.Context, in domainappointment.Entity, actorUserID uuid.UUID, action string) (domainappointment.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("begin update appointment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAppointmentTenant(ctx, tx, in.TenantID); err != nil {
		return domainappointment.Entity{}, err
	}
	if in.Status != domainappointment.StatusCanceled {
		overlaps, err := appointmentOverlap(ctx, tx, in.TenantID, in.StartsAt, in.EndsAt, &in.ID)
		if err != nil {
			return domainappointment.Entity{}, err
		}
		if overlaps {
			return domainappointment.Entity{}, fmt.Errorf("appointment overlaps existing slot: %w", domainerrors.ErrConflict)
		}
	}
	const query = `
		UPDATE appointments
		SET starts_at = $3, ends_at = $4, status = $5, location = $6, updated_at = $7, revision = revision + 1
		WHERE tenant_id = $1 AND id = $2 AND revision = $8
		RETURNING id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at, revision
	`
	out, err := r.scanAppointment(tx.QueryRow(ctx, query,
		in.TenantID, in.ID, in.StartsAt, in.EndsAt, in.Status, in.Location, in.UpdatedAt, in.Revision,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if lookupErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM appointments WHERE tenant_id=$1 AND id=$2)`, in.TenantID, in.ID).Scan(&exists); lookupErr != nil {
				return domainappointment.Entity{}, fmt.Errorf("check appointment after revision conflict: %w", lookupErr)
			}
			if exists {
				return domainappointment.Entity{}, domainerrors.ErrConflict
			}
			return domainappointment.Entity{}, domainerrors.ErrNotFound
		}
		return domainappointment.Entity{}, fmt.Errorf("update appointment: %w", err)
	}
	if err := r.insertAudit(ctx, tx, in.TenantID, actorUserID, action, in.ID); err != nil {
		return domainappointment.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainappointment.Entity{}, fmt.Errorf("commit update appointment: %w", err)
	}
	return out, nil
}

func (r *AppointmentRepository) insertAudit(ctx context.Context, executor auditExecutor, tenantID, actorUserID uuid.UUID, action string, appointmentID uuid.UUID) error {
	if !r.transactionalAudit {
		return nil
	}
	return insertAuditEvent(ctx, executor, tenantID, actorUserID, action, "appointment", appointmentID, nil)
}

func (r *AppointmentRepository) ExistsOverlap(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
	return appointmentOverlap(ctx, r.pool, tenantID, startsAt, endsAt, excludeID)
}

type appointmentOverlapQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func appointmentOverlap(ctx context.Context, querier appointmentOverlapQuerier, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM appointments
			WHERE tenant_id = $1
			  AND status <> 'canceled'
			  AND starts_at < $2
			  AND ends_at > $3
	`
	args := []any{tenantID, endsAt, startsAt}
	if excludeID != nil {
		query += " AND id <> $4"
		args = append(args, *excludeID)
	}
	query += ")"

	var exists bool
	if err := querier.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("query overlap: %w", err)
	}
	return exists, nil
}

func lockAppointmentTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	// All writes for one tenant serialize before checking overlap. The lock is
	// scoped to this transaction and releases on commit or rollback.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, tenantID); err != nil {
		return fmt.Errorf("lock tenant appointment schedule: %w", err)
	}
	return nil
}

func (r *AppointmentRepository) ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM clients WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL
		)
	`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, tenantID, clientID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query client exists: %w", err)
	}

	return exists, nil
}

func (r *AppointmentRepository) scanAppointment(row pgx.Row) (domainappointment.Entity, error) {
	var (
		item      domainappointment.Entity
		startsAt  time.Time
		endsAt    time.Time
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(&item.ID, &item.TenantID, &item.ClientID, &startsAt, &endsAt, &item.Status, &item.Location, &createdAt, &updatedAt, &item.Revision); err != nil {
		return domainappointment.Entity{}, err
	}
	item.StartsAt = startsAt.UTC()
	item.EndsAt = endsAt.UTC()
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()
	return item, nil
}
