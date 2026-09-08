package database

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDB records calls for assertion.
type mockDB struct {
	execCalled     bool
	queryCalled    bool
	queryRowCalled bool
	err            error
}

func (m *mockDB) Exec(_ context.Context, _ string, _ string, _ ...any) (pgconn.CommandTag, error) {
	m.execCalled = true
	return pgconn.NewCommandTag("SELECT 1"), m.err
}

func (m *mockDB) Query(_ context.Context, _ string, _ string, _ ...any) (pgx.Rows, error) {
	m.queryCalled = true
	return nil, m.err
}

func (m *mockDB) QueryRow(_ context.Context, _ string, _ string, _ ...any) pgx.Row {
	m.queryRowCalled = true
	return nil
}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestLoggingDB_Exec(t *testing.T) {
	var buf bytes.Buffer
	mock := &mockDB{}
	db := WithLogging(mock, newTestLogger(&buf))

	_, err := db.Exec(context.Background(), "test.Exec", "UPDATE users SET name = $1")
	require.NoError(t, err)
	assert.True(t, mock.execCalled)
	assert.Contains(t, buf.String(), "UPDATE users SET name = $1")
	assert.Contains(t, buf.String(), "name=test.Exec")
	assert.Contains(t, buf.String(), "duration=")
}

func TestLoggingDB_Query(t *testing.T) {
	var buf bytes.Buffer
	mock := &mockDB{}
	db := WithLogging(mock, newTestLogger(&buf))

	_, err := db.Query(context.Background(), "test.Query", "SELECT * FROM users")
	require.NoError(t, err)
	assert.True(t, mock.queryCalled)
	assert.Contains(t, buf.String(), "SELECT * FROM users")
}

func TestLoggingDB_QueryRow(t *testing.T) {
	var buf bytes.Buffer
	mock := &mockDB{}
	db := WithLogging(mock, newTestLogger(&buf))

	db.QueryRow(context.Background(), "test.QueryRow", "SELECT id FROM users WHERE id = $1")
	assert.True(t, mock.queryRowCalled)
	assert.Contains(t, buf.String(), "SELECT id FROM users WHERE id = $1")
}

func TestLoggingDB_Error(t *testing.T) {
	var buf bytes.Buffer
	mock := &mockDB{err: errors.New("connection refused")}
	db := WithLogging(mock, newTestLogger(&buf))

	_, err := db.Exec(context.Background(), "test.Error", "INSERT INTO users (name) VALUES ($1)")
	require.Error(t, err)
	assert.Contains(t, buf.String(), "query error")
	assert.Contains(t, buf.String(), "connection refused")
}
