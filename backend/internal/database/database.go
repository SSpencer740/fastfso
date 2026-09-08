package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fastfso/fastfso/backend/internal/env"
)

// DB is the interface for executing queries against PostgreSQL.
// The name parameter identifies each query for metrics and logging (e.g. "identity.GetByEmail").
type DB interface {
	Exec(ctx context.Context, name string, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, name string, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, name string, sql string, args ...any) pgx.Row
}

// PoolAdapter wraps *pgxpool.Pool to satisfy the DB interface.
type PoolAdapter struct{ Pool *pgxpool.Pool }

func (a *PoolAdapter) Exec(ctx context.Context, _ string, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.Pool.Exec(ctx, sql, args...)
}

func (a *PoolAdapter) Query(ctx context.Context, _ string, sql string, args ...any) (pgx.Rows, error) {
	return a.Pool.Query(ctx, sql, args...)
}

func (a *PoolAdapter) QueryRow(ctx context.Context, _ string, sql string, args ...any) pgx.Row {
	return a.Pool.QueryRow(ctx, sql, args...)
}

// DSN returns a PostgreSQL connection string from environment variables.
// It prefers DATABASE_URL if set, otherwise constructs a DSN from individual
// DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, and DB_SSLMODE vars.
func DSN() string {
	if url := env.DatabaseURL(); url != "" {
		return url
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		env.DBUser(),
		env.DBPassword(),
		env.DBHost(),
		env.DBPort(),
		env.DBName(),
		env.DBSSLMode(),
	)
}

// Connect creates a new pgx connection pool using environment variables.
// Pool sizing is set explicitly via DB_MAX_CONNS / DB_MIN_CONNS rather than
// pgx's NumCPU-based defaults so that total connections at peak
// (`cloud_run_max_instances * DB_MAX_CONNS`) stays predictable as we scale
// horizontally — see env.DBMaxConns for the rationale.
func Connect(ctx context.Context, logger *slog.Logger) (*pgxpool.Pool, error) {
	logger.Info("connecting to database")

	cfg, err := pgxpool.ParseConfig(DSN())
	if err != nil {
		return nil, fmt.Errorf("database parse config: %w", err)
	}
	cfg.MaxConns = int32(env.DBMaxConns())
	cfg.MinConns = int32(env.DBMinConns())

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	logger.Info("database connected",
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns,
	)
	return pool, nil
}
