package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
)

type ClinicalAccessRepository struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func (r *ClinicalAccessRepository) CanAccessClient(
	ctx context.Context,
	tenantID, userID, clientID uuid.UUID,
	relationships ...string,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM client_clinical_assignments
			WHERE tenant_id = $1 AND client_id = $2 AND user_id = $3
			  AND relationship = ANY($4)
			  AND starts_at <= $5
			  AND (ends_at IS NULL OR ends_at > $5)
		) OR ($6 AND EXISTS (
			SELECT 1 FROM clinical_access_exceptions cae
			WHERE cae.tenant_id = $1 AND cae.client_id = $2 AND cae.user_id = $3
			  AND cae.starts_at <= $5 AND cae.expires_at > $5 AND cae.revoked_at IS NULL
		))
	`
	now := r.now()
	var allowed bool
	if err := r.pool.QueryRow(ctx, query, tenantID, clientID, userID, relationships, now, allowsExceptionalRead(relationships)).Scan(&allowed); err != nil {
		return false, fmt.Errorf("query clinical client access: %w", err)
	}
	return allowed, nil
}

func (r *ClinicalAccessRepository) Grant(ctx context.Context, assignment clinicalaccessusecase.Assignment) (clinicalaccessusecase.Assignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalaccessusecase.Assignment{}, fmt.Errorf("begin clinical assignment grant: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		INSERT INTO client_clinical_assignments
			(id, tenant_id, client_id, user_id, relationship, granted_by_user_id, starts_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, client_id, user_id, relationship, granted_by_user_id,
		          starts_at, ends_at, ended_by_user_id, end_reason, created_at
	`
	out, err := scanClinicalAssignment(tx.QueryRow(ctx, query,
		assignment.ID, assignment.TenantID, assignment.ClientID, assignment.UserID,
		assignment.Relationship, assignment.GrantedByUserID, assignment.StartsAt, assignment.CreatedAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return clinicalaccessusecase.Assignment{}, domainerrors.ErrConflict
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return clinicalaccessusecase.Assignment{}, domainerrors.ErrForbidden
		}
		return clinicalaccessusecase.Assignment{}, fmt.Errorf("insert clinical assignment: %w", err)
	}
	if err := insertClinicalAccessAudit(ctx, tx, out.TenantID, out.GrantedByUserID, "clinical_assignment.grant", out.ID); err != nil {
		return clinicalaccessusecase.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalaccessusecase.Assignment{}, fmt.Errorf("commit clinical assignment grant: %w", err)
	}
	return out, nil
}

func (r *ClinicalAccessRepository) List(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]clinicalaccessusecase.Assignment, error) {
	const query = `
		SELECT id, tenant_id, client_id, user_id, relationship, granted_by_user_id,
		       starts_at, ends_at, ended_by_user_id, end_reason, created_at
		FROM client_clinical_assignments
		WHERE tenant_id = $1 AND client_id = $2
		ORDER BY starts_at DESC, id DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, clientID)
	if err != nil {
		return nil, fmt.Errorf("query clinical assignments: %w", err)
	}
	defer rows.Close()
	items := make([]clinicalaccessusecase.Assignment, 0)
	for rows.Next() {
		item, err := scanClinicalAssignment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan clinical assignment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clinical assignments: %w", err)
	}
	if err := insertClinicalAccessAudit(ctx, r.pool, tenantID, actorUserID, "clinical_assignment.list", clientID); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ClinicalAccessRepository) End(ctx context.Context, tenantID, clientID, assignmentID, actorUserID uuid.UUID, reason string, endedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin clinical assignment end: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE client_clinical_assignments
		SET ends_at = $5, ended_by_user_id = $4, end_reason = $6
		WHERE tenant_id = $1 AND client_id = $2 AND id = $3 AND ends_at IS NULL
	`
	tag, err := tx.Exec(ctx, query, tenantID, clientID, assignmentID, actorUserID, endedAt, strings.TrimSpace(reason))
	if err != nil {
		return fmt.Errorf("end clinical assignment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}
	if err := insertClinicalAccessAudit(ctx, tx, tenantID, actorUserID, "clinical_assignment.end", assignmentID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit clinical assignment end: %w", err)
	}
	return nil
}

func (r *ClinicalAccessRepository) GrantException(ctx context.Context, exception clinicalaccessusecase.AccessException) (clinicalaccessusecase.AccessException, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalaccessusecase.AccessException{}, fmt.Errorf("begin clinical access exception: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		INSERT INTO clinical_access_exceptions
			(id, tenant_id, client_id, user_id, granted_by_user_id, reason, purpose, starts_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, client_id, user_id, granted_by_user_id, reason, purpose,
		          starts_at, expires_at, revoked_at, revoked_by_user_id, revoke_reason, created_at
	`
	out, err := scanClinicalAccessException(tx.QueryRow(ctx, query,
		exception.ID, exception.TenantID, exception.ClientID, exception.UserID,
		exception.GrantedByUserID, exception.Reason, exception.Purpose,
		exception.StartsAt, exception.ExpiresAt, exception.CreatedAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return clinicalaccessusecase.AccessException{}, domainerrors.ErrForbidden
		}
		return clinicalaccessusecase.AccessException{}, fmt.Errorf("insert clinical access exception: %w", err)
	}
	if err := insertClinicalAccessAudit(ctx, tx, out.TenantID, out.GrantedByUserID, "clinical_access_exception.grant", out.ID); err != nil {
		return clinicalaccessusecase.AccessException{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalaccessusecase.AccessException{}, fmt.Errorf("commit clinical access exception: %w", err)
	}
	return out, nil
}

func (r *ClinicalAccessRepository) ListExceptions(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]clinicalaccessusecase.AccessException, error) {
	const query = `
		SELECT id, tenant_id, client_id, user_id, granted_by_user_id, reason, purpose,
		       starts_at, expires_at, revoked_at, revoked_by_user_id, revoke_reason, created_at
		FROM clinical_access_exceptions
		WHERE tenant_id = $1 AND client_id = $2
		ORDER BY created_at DESC, id DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, clientID)
	if err != nil {
		return nil, fmt.Errorf("query clinical access exceptions: %w", err)
	}
	defer rows.Close()
	items := make([]clinicalaccessusecase.AccessException, 0)
	for rows.Next() {
		item, err := scanClinicalAccessException(rows)
		if err != nil {
			return nil, fmt.Errorf("scan clinical access exception: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clinical access exceptions: %w", err)
	}
	if err := insertClinicalAccessAudit(ctx, r.pool, tenantID, actorUserID, "clinical_access_exception.list", clientID); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ClinicalAccessRepository) RevokeException(ctx context.Context, tenantID, clientID, exceptionID, actorUserID uuid.UUID, reason string, revokedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revoke clinical access exception: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		UPDATE clinical_access_exceptions
		SET revoked_at = $5, revoked_by_user_id = $4, revoke_reason = $6
		WHERE tenant_id = $1 AND client_id = $2 AND id = $3 AND revoked_at IS NULL
	`
	tag, err := tx.Exec(ctx, query, tenantID, clientID, exceptionID, actorUserID, revokedAt, strings.TrimSpace(reason))
	if err != nil {
		return fmt.Errorf("revoke clinical access exception: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}
	if err := insertClinicalAccessAudit(ctx, tx, tenantID, actorUserID, "clinical_access_exception.revoke", exceptionID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit revoke clinical access exception: %w", err)
	}
	return nil
}

func scanClinicalAccessException(row pgx.Row) (clinicalaccessusecase.AccessException, error) {
	var item clinicalaccessusecase.AccessException
	if err := row.Scan(
		&item.ID, &item.TenantID, &item.ClientID, &item.UserID, &item.GrantedByUserID,
		&item.Reason, &item.Purpose, &item.StartsAt, &item.ExpiresAt,
		&item.RevokedAt, &item.RevokedByUserID, &item.RevokeReason, &item.CreatedAt,
	); err != nil {
		return clinicalaccessusecase.AccessException{}, err
	}
	item.StartsAt = item.StartsAt.UTC()
	item.ExpiresAt = item.ExpiresAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	return item, nil
}

func scanClinicalAssignment(row pgx.Row) (clinicalaccessusecase.Assignment, error) {
	var item clinicalaccessusecase.Assignment
	if err := row.Scan(
		&item.ID, &item.TenantID, &item.ClientID, &item.UserID, &item.Relationship,
		&item.GrantedByUserID, &item.StartsAt, &item.EndsAt, &item.EndedByUserID,
		&item.EndReason, &item.CreatedAt,
	); err != nil {
		return clinicalaccessusecase.Assignment{}, err
	}
	item.StartsAt = item.StartsAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	return item, nil
}

func insertClinicalAccessAudit(ctx context.Context, executor auditExecutor, tenantID, actorUserID uuid.UUID, action string, entityID uuid.UUID) error {
	entity := "clinical_assignment"
	if strings.HasPrefix(action, "clinical_access_exception.") {
		entity = "clinical_access_exception"
	}
	return insertAuditEvent(ctx, executor, tenantID, actorUserID, action, entity, entityID, nil)
}

func NewClinicalAccessRepository(pool *pgxpool.Pool) *ClinicalAccessRepository {
	return &ClinicalAccessRepository{pool: pool, now: func() time.Time { return time.Now().UTC() }}
}

func (r *ClinicalAccessRepository) CanAccessAppointment(
	ctx context.Context,
	tenantID, userID, appointmentID uuid.UUID,
	relationships ...string,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM appointments a
			JOIN client_clinical_assignments cca
			  ON cca.tenant_id = a.tenant_id
			 AND cca.client_id = a.client_id
			WHERE a.tenant_id = $1
			  AND a.id = $2
			  AND cca.user_id = $3
			  AND cca.relationship = ANY($4)
			  AND cca.starts_at <= $5
			  AND (cca.ends_at IS NULL OR cca.ends_at > $5)
		) OR ($6 AND EXISTS (
			SELECT 1
			FROM appointments a
			JOIN clinical_access_exceptions cae
			  ON cae.tenant_id = a.tenant_id AND cae.client_id = a.client_id
			WHERE a.tenant_id = $1 AND a.id = $2 AND cae.user_id = $3
			  AND cae.starts_at <= $5 AND cae.expires_at > $5 AND cae.revoked_at IS NULL
		))
	`
	now := r.now()
	var allowed bool
	if err := r.pool.QueryRow(ctx, query, tenantID, appointmentID, userID, relationships, now, allowsExceptionalRead(relationships)).Scan(&allowed); err != nil {
		return false, fmt.Errorf("query clinical appointment access: %w", err)
	}
	return allowed, nil
}

func allowsExceptionalRead(relationships []string) bool {
	for _, relationship := range relationships {
		if relationship == clinicalaccessusecase.RelationshipSupervisor {
			return true
		}
	}
	return false
}
