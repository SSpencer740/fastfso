package telemetry

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

// MetricsDB wraps a database.DB and records query duration metrics.
type MetricsDB struct {
	db database.DB
}

// WithMetrics returns a DB wrapper that records query duration metrics.
func WithMetrics(db database.DB) *MetricsDB {
	return &MetricsDB{db: db}
}

func (m *MetricsDB) Exec(ctx context.Context, name string, sql string, arguments ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := m.db.Exec(ctx, name, sql, arguments...)
	recordQueryDuration(ctx, name, time.Since(start), err)
	return tag, err
}

func (m *MetricsDB) Query(ctx context.Context, name string, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := m.db.Query(ctx, name, sql, args...)
	recordQueryDuration(ctx, name, time.Since(start), err)
	return rows, err
}

func (m *MetricsDB) QueryRow(ctx context.Context, name string, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := m.db.QueryRow(ctx, name, sql, args...)
	recordQueryDuration(ctx, name, time.Since(start), nil)
	return row
}
