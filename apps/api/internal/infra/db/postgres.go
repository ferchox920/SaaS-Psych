package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPostgresPoolWithTracing(ctx, databaseURL, PoolTracingConfig{})
}

func NewPostgresPoolWithTracing(ctx context.Context, databaseURL string, tracingCfg PoolTracingConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pgx pool config: %w", err)
	}
	if tracingCfg.Tracer != nil {
		cfg.ConnConfig.Tracer = newQueryTracer(databaseURL, tracingCfg)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
