package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

type Store struct {
	db     database.DB
	logger *slog.Logger
}

func NewStore(db database.DB, logger *slog.Logger) *Store {
	return &Store{db: db, logger: logger}
}

type LogParams struct {
	IdentityID *uuid.UUID
	SessionID  *uuid.UUID
	TenantID   *uuid.UUID
	Action     string
	IPAddress  string
	UserAgent  string
	Metadata   map[string]any
}

func (s *Store) Log(ctx context.Context, p LogParams) {
	meta, err := json.Marshal(p.Metadata)
	if err != nil {
		meta = []byte("{}")
	}

	_, err = s.db.Exec(ctx, "audit.Log",
		`INSERT INTO auth_audit_log (identity_id, session_id, tenant_id, action, ip_address, user_agent, metadata)
		 VALUES ($1, $2, $3, $4, $5::inet, $6, $7)`,
		p.IdentityID, p.SessionID, p.TenantID, p.Action, nullIfEmpty(p.IPAddress), p.UserAgent, meta,
	)
	if err != nil {
		s.logger.Error("failed to write audit log", "action", p.Action, "error", err)
	}
}

func (s *Store) Query(ctx context.Context, identityID *uuid.UUID, limit, offset int) ([]Entry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, identity_id, session_id, tenant_id, action, ip_address::text, user_agent, metadata, created_at
	          FROM auth_audit_log`
	args := []any{}
	argIdx := 1

	if identityID != nil {
		query += fmt.Sprintf(" WHERE identity_id = $%d", argIdx)
		args = append(args, identityID)
		argIdx++
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, "audit.Query", query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit query: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var meta []byte
		if err := rows.Scan(
			&e.ID, &e.IdentityID, &e.SessionID, &e.TenantID,
			&e.Action, &e.IPAddress, &e.UserAgent, &meta, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("audit query scan: %w", err)
		}
		if err := json.Unmarshal(meta, &e.Metadata); err != nil {
			e.Metadata = map[string]any{}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

type TenantEntry struct {
	Entry
	IdentityName  string `json:"identity_name,omitempty"`
	IdentityEmail string `json:"identity_email,omitempty"`
}

// QueryByTenant returns audit entries scoped to a tenant with optional action and search filters.
func (s *Store) QueryByTenant(ctx context.Context, tenantID uuid.UUID, action, search string, limit, offset int) ([]TenantEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}

	args := []any{tenantID}
	// Include rows that are explicitly tenant-scoped, plus rows that are
	// tenant-less (e.g. login_attempt / login_success — these are written
	// before tenant selection happens) where the identity belongs to this
	// tenant. Without the second clause, login events never surface in the
	// tenant audit log even though the user belongs to that tenant.
	where := `WHERE (a.tenant_id = $1
	             OR (a.tenant_id IS NULL
	                 AND a.identity_id IS NOT NULL
	                 AND EXISTS (SELECT 1 FROM users u WHERE u.identity_id = a.identity_id AND u.tenant_id = $1)))`
	argIdx := 2

	if action != "" {
		where += fmt.Sprintf(" AND a.action = $%d", argIdx)
		args = append(args, action)
		argIdx++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (a.ip_address::text ILIKE $%d OR i.email ILIKE $%d OR i.name ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM auth_audit_log a LEFT JOIN identities i ON i.id = a.identity_id ` + where
	if err := s.db.QueryRow(ctx, "audit.QueryByTenant.count", countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit tenant count: %w", err)
	}

	query := `SELECT a.id, a.identity_id, a.session_id, a.tenant_id, a.action,
	                 COALESCE(a.ip_address::text, ''), a.user_agent, a.metadata, a.created_at,
	                 COALESCE(i.name, ''), COALESCE(i.email, '')
	          FROM auth_audit_log a
	          LEFT JOIN identities i ON i.id = a.identity_id
	          ` + where + fmt.Sprintf(" ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, "audit.QueryByTenant", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit tenant query: %w", err)
	}
	defer rows.Close()

	var entries []TenantEntry
	for rows.Next() {
		var e TenantEntry
		var meta []byte
		if err := rows.Scan(
			&e.ID, &e.IdentityID, &e.SessionID, &e.TenantID,
			&e.Action, &e.IPAddress, &e.UserAgent, &meta, &e.CreatedAt,
			&e.IdentityName, &e.IdentityEmail,
		); err != nil {
			return nil, 0, fmt.Errorf("audit tenant scan: %w", err)
		}
		if err := json.Unmarshal(meta, &e.Metadata); err != nil {
			e.Metadata = map[string]any{}
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// CountRecentByIP counts audit log entries matching the given actions for an
// IP address within the specified window. Used by the rate limiter.
func (s *Store) CountRecentByIP(ctx context.Context, ip string, actions []string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, "audit.CountRecentByIP",
		`SELECT COUNT(*) FROM auth_audit_log
		 WHERE ip_address = $1::inet
		 AND action = ANY($2)
		 AND created_at > $3`,
		ip, actions, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("audit count recent by ip: %w", err)
	}
	return count, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
