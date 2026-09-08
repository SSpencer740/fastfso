package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// LoggingDB wraps a DB and logs every query's SQL, duration, and any error.
type LoggingDB struct {
	db     DB
	logger *slog.Logger
}

// WithLogging returns a DB that logs queries via the provided logger.
func WithLogging(db DB, logger *slog.Logger) *LoggingDB {
	return &LoggingDB{db: db, logger: logger}
}

func (l *LoggingDB) Exec(ctx context.Context, name string, sql string, arguments ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := l.db.Exec(ctx, name, sql, arguments...)
	l.log(name, sql, time.Since(start), err)
	return tag, err
}

func (l *LoggingDB) Query(ctx context.Context, name string, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := l.db.Query(ctx, name, sql, args...)
	l.log(name, sql, time.Since(start), err)
	return rows, err
}

func (l *LoggingDB) QueryRow(ctx context.Context, name string, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := l.db.QueryRow(ctx, name, sql, args...)
	l.log(name, sql, time.Since(start), nil)
	return row
}

func (l *LoggingDB) log(name string, sql string, duration time.Duration, err error) {
	if err != nil {
		l.logger.Error("query error", "name", name, "sql", sql, "duration", duration, "error", err)
		return
	}
	l.logger.Debug("query", "name", name, "sql", sql, "duration", duration)
}
