package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
)

type SessionNoteRepository struct {
	pool *pgxpool.Pool
}

func NewSessionNoteRepository(pool *pgxpool.Pool) *SessionNoteRepository {
	return &SessionNoteRepository{pool: pool}
}

func (r *SessionNoteRepository) AppointmentExists(ctx context.Context, tenantID, appointmentID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM appointments WHERE tenant_id = $1 AND id = $2
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, tenantID, appointmentID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query appointment exists: %w", err)
	}
	return exists, nil
}

func (r *SessionNoteRepository) Create(ctx context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	const query = `
		INSERT INTO session_notes (id, tenant_id, appointment_id, author_user_id, body, is_private, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private, created_at, updated_at
	`
	out, err := r.scanSessionNote(r.pool.QueryRow(ctx, query,
		in.ID, in.TenantID, in.AppointmentID, in.AuthorUserID, in.Body, in.IsPrivate, in.CreatedAt, in.UpdatedAt,
	))
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("insert session note: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) Update(ctx context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	const query = `
		UPDATE session_notes
		SET body = $3, is_private = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private, created_at, updated_at
	`
	out, err := r.scanSessionNote(r.pool.QueryRow(ctx, query,
		in.TenantID, in.ID, in.Body, in.IsPrivate, in.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainsessionnote.Entity{}, domainerrors.ErrNotFound
		}
		return domainsessionnote.Entity{}, fmt.Errorf("update session note: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) GetByID(ctx context.Context, tenantID, noteID uuid.UUID) (domainsessionnote.Entity, error) {
	const query = `
		SELECT id, tenant_id, appointment_id, author_user_id, body, is_private, created_at, updated_at
		FROM session_notes
		WHERE tenant_id = $1 AND id = $2
		LIMIT 1
	`
	out, err := r.scanSessionNote(r.pool.QueryRow(ctx, query, tenantID, noteID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainsessionnote.Entity{}, domainerrors.ErrNotFound
		}
		return domainsessionnote.Entity{}, fmt.Errorf("query session note by id: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) ListByAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID) ([]domainsessionnote.Entity, error) {
	const query = `
		SELECT id, tenant_id, appointment_id, author_user_id, body, is_private, created_at, updated_at
		FROM session_notes
		WHERE tenant_id = $1 AND appointment_id = $2
		ORDER BY created_at DESC, id DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, appointmentID)
	if err != nil {
		return nil, fmt.Errorf("query session notes: %w", err)
	}
	defer rows.Close()

	notes := make([]domainsessionnote.Entity, 0)
	for rows.Next() {
		var (
			note      domainsessionnote.Entity
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(&note.ID, &note.TenantID, &note.AppointmentID, &note.AuthorUserID, &note.Body, &note.IsPrivate, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session note row: %w", err)
		}
		note.CreatedAt = createdAt.UTC()
		note.UpdatedAt = updatedAt.UTC()
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session note rows: %w", err)
	}
	return notes, nil
}

func (r *SessionNoteRepository) scanSessionNote(row pgx.Row) (domainsessionnote.Entity, error) {
	var (
		note      domainsessionnote.Entity
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(&note.ID, &note.TenantID, &note.AppointmentID, &note.AuthorUserID, &note.Body, &note.IsPrivate, &createdAt, &updatedAt); err != nil {
		return domainsessionnote.Entity{}, err
	}
	note.CreatedAt = createdAt.UTC()
	note.UpdatedAt = updatedAt.UTC()
	return note, nil
}
