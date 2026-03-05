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
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (authusecase.User, error) {
	const query = `
		SELECT id, password_hash
		FROM users
		WHERE tenant_id = $1 AND email = $2
		LIMIT 1
	`

	var userID uuid.UUID
	var passwordHash string
	if err := r.pool.QueryRow(ctx, query, tenantID, email).Scan(&userID, &passwordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authusecase.User{}, domainerrors.ErrNotFound
		}

		return authusecase.User{}, fmt.Errorf("query user by email: %w", err)
	}

	return authusecase.User{
		ID:           userID,
		PasswordHash: passwordHash,
	}, nil
}

func (r *AuthRepository) GetUserRole(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	const query = `
		SELECT role
		FROM user_roles
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY created_at ASC
		LIMIT 1
	`

	var role string
	if err := r.pool.QueryRow(ctx, query, tenantID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domainerrors.ErrNotFound
		}

		return "", fmt.Errorf("query user role: %w", err)
	}

	return role, nil
}

func (r *AuthRepository) CreateRefreshToken(ctx context.Context, token authusecase.RefreshTokenWrite) error {
	const query = `
		INSERT INTO refresh_tokens (tenant_id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	if _, err := r.pool.Exec(ctx, query, token.TenantID, token.UserID, token.TokenHash, token.ExpiresAt); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}

	return nil
}

func (r *AuthRepository) GetRefreshToken(ctx context.Context, tenantID uuid.UUID, tokenHash string) (authusecase.StoredRefreshToken, error) {
	const query = `
		SELECT user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE tenant_id = $1 AND token_hash = $2
		LIMIT 1
	`

	var userID uuid.UUID
	var expiresAt time.Time
	var revokedAt *time.Time
	if err := r.pool.QueryRow(ctx, query, tenantID, tokenHash).Scan(&userID, &expiresAt, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authusecase.StoredRefreshToken{}, domainerrors.ErrNotFound
		}

		return authusecase.StoredRefreshToken{}, fmt.Errorf("query refresh token: %w", err)
	}

	return authusecase.StoredRefreshToken{
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
	}, nil
}

func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, tenantID uuid.UUID, tokenHash string) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE tenant_id = $1 AND token_hash = $2 AND revoked_at IS NULL
	`

	tag, err := r.pool.Exec(ctx, query, tenantID, tokenHash)
	if err != nil {
		return fmt.Errorf("update refresh token revoked_at: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrNotFound
	}

	return nil
}
