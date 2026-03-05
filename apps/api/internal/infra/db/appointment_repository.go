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
	pool *pgxpool.Pool
}

func NewAppointmentRepository(pool *pgxpool.Pool) *AppointmentRepository {
	return &AppointmentRepository{pool: pool}
}

func (r *AppointmentRepository) Create(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	const query = `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at
	`
	out, err := r.scanAppointment(r.pool.QueryRow(ctx, query,
		in.ID, in.TenantID, in.ClientID, in.StartsAt, in.EndsAt, in.Status, in.Location, in.CreatedAt, in.UpdatedAt,
	))
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("insert appointment: %w", err)
	}
	return out, nil
}

func (r *AppointmentRepository) ListByRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error) {
	const query = `
		SELECT id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at
		FROM appointments
		WHERE tenant_id = $1
		  AND starts_at >= $2
		  AND starts_at < $3
		ORDER BY starts_at ASC, id ASC
	`

	rows, err := r.pool.Query(ctx, query, tenantID, from, to)
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
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ClientID, &startsAt, &endsAt, &item.Status, &item.Location, &createdAt, &updatedAt); err != nil {
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
		SELECT id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at
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

func (r *AppointmentRepository) Update(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	const query = `
		UPDATE appointments
		SET starts_at = $3, ends_at = $4, status = $5, location = $6, updated_at = $7
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, client_id, starts_at, ends_at, status, location, created_at, updated_at
	`
	out, err := r.scanAppointment(r.pool.QueryRow(ctx, query,
		in.TenantID, in.ID, in.StartsAt, in.EndsAt, in.Status, in.Location, in.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainappointment.Entity{}, domainerrors.ErrNotFound
		}
		return domainappointment.Entity{}, fmt.Errorf("update appointment: %w", err)
	}
	return out, nil
}

func (r *AppointmentRepository) ExistsOverlap(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
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
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("query overlap: %w", err)
	}
	return exists, nil
}

func (r *AppointmentRepository) ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM clients WHERE tenant_id = $1 AND id = $2
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
	if err := row.Scan(&item.ID, &item.TenantID, &item.ClientID, &startsAt, &endsAt, &item.Status, &item.Location, &createdAt, &updatedAt); err != nil {
		return domainappointment.Entity{}, err
	}
	item.StartsAt = startsAt.UTC()
	item.EndsAt = endsAt.UTC()
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()
	return item, nil
}
