package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

func (r *TenantRepository) Exists(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM tenants
			WHERE id = $1
		)
	`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, tenantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query tenant existence: %w", err)
	}

	return exists, nil
}
