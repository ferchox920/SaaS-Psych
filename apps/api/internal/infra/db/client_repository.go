package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type ClientRepository struct {
	pool *pgxpool.Pool
}

func NewClientRepository(pool *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{pool: pool}
}

func (r *ClientRepository) Create(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	const query = `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, fullname, contact, notes_public, created_at, updated_at
	`

	out, err := r.scanClient(
		r.pool.QueryRow(ctx, query, in.ID, in.TenantID, in.FullName, in.Contact, in.NotesPublic, in.CreatedAt, in.UpdatedAt),
	)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("insert client: %w", err)
	}

	return out, nil
}

func (r *ClientRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	const query = `
		SELECT id, tenant_id, fullname, contact, notes_public, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1
		ORDER BY created_at DESC, id DESC
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query clients: %w", err)
	}
	defer rows.Close()

	items := make([]domainclient.Entity, 0)
	for rows.Next() {
		var (
			item      domainclient.Entity
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.FullName,
			&item.Contact,
			&item.NotesPublic,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		item.CreatedAt = createdAt.UTC()
		item.UpdatedAt = updatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client rows: %w", err)
	}

	return items, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error) {
	const query = `
		SELECT id, tenant_id, fullname, contact, notes_public, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1 AND id = $2
		LIMIT 1
	`

	out, err := r.scanClient(r.pool.QueryRow(ctx, query, tenantID, clientID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainclient.Entity{}, domainerrors.ErrNotFound
		}
		return domainclient.Entity{}, fmt.Errorf("query client by id: %w", err)
	}

	return out, nil
}

func (r *ClientRepository) Update(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	const query = `
		UPDATE clients
		SET fullname = $3, contact = $4, notes_public = $5, updated_at = $6
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, fullname, contact, notes_public, created_at, updated_at
	`

	out, err := r.scanClient(r.pool.QueryRow(ctx, query, in.TenantID, in.ID, in.FullName, in.Contact, in.NotesPublic, in.UpdatedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainclient.Entity{}, domainerrors.ErrNotFound
		}
		return domainclient.Entity{}, fmt.Errorf("update client: %w", err)
	}

	return out, nil
}

func (r *ClientRepository) Delete(ctx context.Context, tenantID, clientID uuid.UUID) error {
	const query = `
		DELETE FROM clients
		WHERE tenant_id = $1 AND id = $2
	`

	tag, err := r.pool.Exec(ctx, query, tenantID, clientID)
	if err != nil {
		return fmt.Errorf("delete client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}

	return nil
}

func (r *ClientRepository) scanClient(row pgx.Row) (domainclient.Entity, error) {
	var (
		item      domainclient.Entity
		createdAt time.Time
		updatedAt time.Time
	)

	if err := row.Scan(
		&item.ID,
		&item.TenantID,
		&item.FullName,
		&item.Contact,
		&item.NotesPublic,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domainclient.Entity{}, err
	}
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()

	return item, nil
}
