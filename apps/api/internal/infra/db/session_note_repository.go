package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
)

type SessionNoteRepository struct {
	pool               *pgxpool.Pool
	transactionalAudit bool
}

func (r *SessionNoteRepository) WithTransactionalAudit() *SessionNoteRepository {
	r.transactionalAudit = true
	return r
}

func (r *SessionNoteRepository) WritesAreTransactionallyAudited() bool {
	return r.transactionalAudit
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
	in = normalizeSessionNoteLifecycle(in)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("begin create session note: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		INSERT INTO session_notes (
			id, tenant_id, appointment_id, author_user_id, body, is_private,
			status, current_version, signed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private,
		          status, current_version, signed_at, created_at, updated_at
	`
	out, err := r.scanSessionNote(tx.QueryRow(ctx, query,
		in.ID, in.TenantID, in.AppointmentID, in.AuthorUserID, in.Body, in.IsPrivate,
		in.Status, in.CurrentVersion, in.SignedAt, in.CreatedAt, in.UpdatedAt,
	))
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("insert session note: %w", err)
	}
	if err := insertSessionNoteVersion(ctx, tx, out, "draft", "", out.AuthorUserID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := r.insertWriteAudit(ctx, tx, out.TenantID, out.AuthorUserID, "session_note.create", out.ID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("commit create session note: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) Update(ctx context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	if in.CurrentVersion < 1 {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("current_version is required")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("begin update session note: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		UPDATE session_notes
		SET body = $3, is_private = $4, updated_at = $5, current_version = current_version + 1
		WHERE tenant_id = $1 AND id = $2 AND current_version = $6 AND status = 'draft'
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private,
		          status, current_version, signed_at, created_at, updated_at
	`
	out, err := r.scanSessionNote(tx.QueryRow(ctx, query,
		in.TenantID, in.ID, in.Body, in.IsPrivate, in.UpdatedAt, in.CurrentVersion,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if existsErr := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM session_notes WHERE tenant_id = $1 AND id = $2)`,
				in.TenantID, in.ID,
			).Scan(&exists); existsErr != nil {
				return domainsessionnote.Entity{}, fmt.Errorf("check session note update conflict: %w", existsErr)
			}
			if !exists {
				return domainsessionnote.Entity{}, domainerrors.ErrNotFound
			}
			return domainsessionnote.Entity{}, domainerrors.ErrConflict
		}
		return domainsessionnote.Entity{}, fmt.Errorf("update session note: %w", err)
	}
	if err := insertSessionNoteVersion(ctx, tx, out, "draft", "", in.AuthorUserID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := r.insertWriteAudit(ctx, tx, out.TenantID, in.AuthorUserID, "session_note.update", out.ID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("commit update session note: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) Sign(
	ctx context.Context,
	tenantID, noteID uuid.UUID,
	expectedVersion int,
	signedAt time.Time,
) (domainsessionnote.Entity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("begin sign session note: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		UPDATE session_notes
		SET status = 'signed', signed_at = $4, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND current_version = $3 AND status = 'draft'
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private,
		          status, current_version, signed_at, created_at, updated_at
	`
	out, err := r.scanSessionNote(tx.QueryRow(ctx, query, tenantID, noteID, expectedVersion, signedAt))
	if err != nil {
		return domainsessionnote.Entity{}, r.lifecycleWriteErrorWithExecutor(ctx, tx, tenantID, noteID, err, "sign")
	}
	if err := r.insertWriteAudit(ctx, tx, tenantID, out.AuthorUserID, "session_note.sign", out.ID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("commit sign session note: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) Addendum(
	ctx context.Context,
	in domainsessionnote.Entity,
	actorUserID uuid.UUID,
	reason string,
) (domainsessionnote.Entity, error) {
	if in.CurrentVersion < 1 {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("current_version is required")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("begin addendum: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		UPDATE session_notes
		SET body = $3, is_private = $4, updated_at = $5, current_version = current_version + 1
		WHERE tenant_id = $1 AND id = $2 AND current_version = $6 AND status = 'signed'
		RETURNING id, tenant_id, appointment_id, author_user_id, body, is_private,
		          status, current_version, signed_at, created_at, updated_at
	`
	out, err := r.scanSessionNote(tx.QueryRow(ctx, query,
		in.TenantID, in.ID, in.Body, in.IsPrivate, in.UpdatedAt, in.CurrentVersion,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainsessionnote.Entity{}, r.lifecycleWriteErrorWithExecutor(ctx, tx, in.TenantID, in.ID, err, "add addendum")
		}
		return domainsessionnote.Entity{}, fmt.Errorf("add session note addendum: %w", err)
	}
	if err := insertSessionNoteVersion(ctx, tx, out, "addendum", reason, actorUserID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := r.insertWriteAudit(ctx, tx, out.TenantID, actorUserID, "session_note.addendum", out.ID); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("commit addendum: %w", err)
	}
	return out, nil
}

func (r *SessionNoteRepository) ListVersions(
	ctx context.Context,
	tenantID, noteID uuid.UUID,
) ([]domainsessionnote.Version, error) {
	const query = `
		SELECT id, tenant_id, note_id, version, body, is_private,
		       change_kind, change_reason, actor_user_id, created_at
		FROM session_note_versions
		WHERE tenant_id = $1 AND note_id = $2
		ORDER BY version ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, noteID)
	if err != nil {
		return nil, fmt.Errorf("query session note versions: %w", err)
	}
	defer rows.Close()

	versions := make([]domainsessionnote.Version, 0)
	for rows.Next() {
		var version domainsessionnote.Version
		if err := rows.Scan(
			&version.ID, &version.TenantID, &version.NoteID, &version.Version,
			&version.Body, &version.IsPrivate, &version.ChangeKind, &version.ChangeReason,
			&version.ActorUserID, &version.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session note version: %w", err)
		}
		version.CreatedAt = version.CreatedAt.UTC()
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session note versions: %w", err)
	}
	return versions, nil
}

func (r *SessionNoteRepository) lifecycleWriteError(
	ctx context.Context,
	tenantID, noteID uuid.UUID,
	err error,
	action string,
) error {
	return r.lifecycleWriteErrorWithExecutor(ctx, r.pool, tenantID, noteID, err, action)
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *SessionNoteRepository) lifecycleWriteErrorWithExecutor(
	ctx context.Context,
	executor rowQuerier,
	tenantID, noteID uuid.UUID,
	err error,
	action string,
) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s session note: %w", action, err)
	}
	var exists bool
	if existsErr := executor.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM session_notes WHERE tenant_id = $1 AND id = $2)`,
		tenantID, noteID,
	).Scan(&exists); existsErr != nil {
		return fmt.Errorf("check session note lifecycle conflict: %w", existsErr)
	}
	if !exists {
		return domainerrors.ErrNotFound
	}
	return domainerrors.ErrConflict
}

func (r *SessionNoteRepository) GetByID(ctx context.Context, tenantID, noteID uuid.UUID) (domainsessionnote.Entity, error) {
	const query = `
		SELECT id, tenant_id, appointment_id, author_user_id, body, is_private,
		       status, current_version, signed_at, created_at, updated_at
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
		SELECT id, tenant_id, appointment_id, author_user_id, body, is_private,
		       status, current_version, signed_at, created_at, updated_at
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
		if err := rows.Scan(
			&note.ID, &note.TenantID, &note.AppointmentID, &note.AuthorUserID,
			&note.Body, &note.IsPrivate, &note.Status, &note.CurrentVersion,
			&note.SignedAt, &createdAt, &updatedAt,
		); err != nil {
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
	if err := row.Scan(
		&note.ID, &note.TenantID, &note.AppointmentID, &note.AuthorUserID,
		&note.Body, &note.IsPrivate, &note.Status, &note.CurrentVersion,
		&note.SignedAt, &createdAt, &updatedAt,
	); err != nil {
		return domainsessionnote.Entity{}, err
	}
	note.CreatedAt = createdAt.UTC()
	note.UpdatedAt = updatedAt.UTC()
	return note, nil
}

type sessionNoteVersionExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertSessionNoteVersion(
	ctx context.Context,
	executor sessionNoteVersionExecutor,
	note domainsessionnote.Entity,
	changeKind, changeReason string,
	actorUserID uuid.UUID,
) error {
	const query = `
		INSERT INTO session_note_versions (
			tenant_id, note_id, version, body, is_private,
			change_kind, change_reason, actor_user_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := executor.Exec(ctx, query,
		note.TenantID, note.ID, note.CurrentVersion, note.Body, note.IsPrivate,
		changeKind, changeReason, actorUserID, note.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert session note version: %w", err)
	}
	return nil
}

func normalizeSessionNoteLifecycle(note domainsessionnote.Entity) domainsessionnote.Entity {
	if note.Status == "" {
		note.Status = "draft"
	}
	if note.CurrentVersion < 1 {
		note.CurrentVersion = 1
	}
	return note
}

func (r *SessionNoteRepository) insertWriteAudit(
	ctx context.Context,
	executor sessionNoteVersionExecutor,
	tenantID, actorUserID uuid.UUID,
	action string,
	noteID uuid.UUID,
) error {
	if !r.transactionalAudit {
		return nil
	}
	return insertAuditEvent(ctx, executor, tenantID, actorUserID, action, "session_note", noteID, nil)
}
