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
	pool               *pgxpool.Pool
	transactionalAudit bool
}

func (r *ClientRepository) WithTransactionalAudit() *ClientRepository {
	r.transactionalAudit = true
	return r
}

func (r *ClientRepository) WritesAreTransactionallyAudited() bool { return r.transactionalAudit }

func NewClientRepository(pool *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{pool: pool}
}

func (r *ClientRepository) Create(ctx context.Context, in domainclient.Entity, actorUserID uuid.UUID) (domainclient.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("begin create client: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, fullname, contact, notes_public,
		          archived_at, archived_by_user_id, archive_reason, created_at, updated_at
	`

	out, err := r.scanClient(
		tx.QueryRow(ctx, query, in.ID, in.TenantID, in.FullName, in.Contact, in.NotesPublic, in.CreatedAt, in.UpdatedAt),
	)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("insert client: %w", err)
	}
	assignmentID := uuid.New()
	const assignmentQuery = `
		INSERT INTO client_clinical_assignments
			(id, tenant_id, client_id, user_id, relationship, granted_by_user_id, starts_at, created_at)
		VALUES ($1, $2, $3, $4, 'treating', $4, $5, $5)
	`
	if _, err := tx.Exec(ctx, assignmentQuery, assignmentID, in.TenantID, in.ID, actorUserID, in.CreatedAt); err != nil {
		return domainclient.Entity{}, fmt.Errorf("assign client creator as treating clinician: %w", err)
	}
	if err := r.insertAudit(ctx, tx, in.TenantID, actorUserID, "client.create", "client", in.ID); err != nil {
		return domainclient.Entity{}, err
	}
	if err := r.insertAudit(ctx, tx, in.TenantID, actorUserID, "clinical_assignment.grant", "clinical_assignment", assignmentID); err != nil {
		return domainclient.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainclient.Entity{}, fmt.Errorf("commit create client: %w", err)
	}

	return out, nil
}

func (r *ClientRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	return r.listByArchiveState(ctx, tenantID, false)
}

func (r *ClientRepository) ListArchived(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	return r.listByArchiveState(ctx, tenantID, true)
}

func (r *ClientRepository) listByArchiveState(ctx context.Context, tenantID uuid.UUID, archived bool) ([]domainclient.Entity, error) {
	archivePredicate := "archived_at IS NULL"
	if archived {
		archivePredicate = "archived_at IS NOT NULL"
	}
	query := `
		SELECT id, tenant_id, fullname, contact, notes_public,
		       archived_at, archived_by_user_id, archive_reason, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1 AND ` + archivePredicate + `
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
			&item.ArchivedAt,
			&item.ArchivedByUserID,
			&item.ArchiveReason,
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

func (r *ClientRepository) Restore(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, restoredAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin restore client: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE clients
		SET archived_at = NULL, archived_by_user_id = NULL, archive_reason = '', updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND archived_at IS NOT NULL
	`
	tag, err := tx.Exec(ctx, query, tenantID, clientID, restoredAt)
	if err != nil {
		return fmt.Errorf("restore client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}
	if err := r.insertAudit(ctx, tx, tenantID, actorUserID, "client.restore", "client", clientID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit restore client: %w", err)
	}
	return nil
}

func (r *ClientRepository) GetByID(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error) {
	const query = `
		SELECT id, tenant_id, fullname, contact, notes_public,
		       archived_at, archived_by_user_id, archive_reason, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL
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

func (r *ClientRepository) Update(ctx context.Context, in domainclient.Entity, actorUserID uuid.UUID) (domainclient.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("begin update client: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE clients
		SET fullname = $3, contact = $4, notes_public = $5, updated_at = $6
		WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL
		RETURNING id, tenant_id, fullname, contact, notes_public,
		          archived_at, archived_by_user_id, archive_reason, created_at, updated_at
	`

	out, err := r.scanClient(tx.QueryRow(ctx, query, in.TenantID, in.ID, in.FullName, in.Contact, in.NotesPublic, in.UpdatedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainclient.Entity{}, domainerrors.ErrNotFound
		}
		return domainclient.Entity{}, fmt.Errorf("update client: %w", err)
	}
	if err := r.insertAudit(ctx, tx, in.TenantID, actorUserID, "client.update", "client", in.ID); err != nil {
		return domainclient.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainclient.Entity{}, fmt.Errorf("commit update client: %w", err)
	}

	return out, nil
}

func (r *ClientRepository) Archive(
	ctx context.Context,
	tenantID, clientID, actorUserID uuid.UUID,
	reason string,
	archivedAt time.Time,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin archive client: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE clients
		SET archived_at = $3,
		    archived_by_user_id = $4,
		    archive_reason = $5,
		    updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL
	`

	tag, err := tx.Exec(ctx, query, tenantID, clientID, archivedAt, actorUserID, reason)
	if err != nil {
		return fmt.Errorf("archive client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}
	if err := r.insertAudit(ctx, tx, tenantID, actorUserID, "client.archive", "client", clientID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit archive client: %w", err)
	}

	return nil
}

func (r *ClientRepository) insertAudit(ctx context.Context, executor auditExecutor, tenantID, actorUserID uuid.UUID, action, entity string, entityID uuid.UUID) error {
	if !r.transactionalAudit {
		return nil
	}
	return insertAuditEvent(ctx, executor, tenantID, actorUserID, action, entity, entityID, nil)
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
		&item.ArchivedAt,
		&item.ArchivedByUserID,
		&item.ArchiveReason,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domainclient.Entity{}, err
	}
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()

	return item, nil
}
