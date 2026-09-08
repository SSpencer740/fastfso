package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("session not found")

const (
	// sessionDuration is the absolute hard cap on session lifetime.
	sessionDuration = 7 * 24 * time.Hour
	// preAuthSessionDuration applies to half-authenticated states
	// (pre_auth, setup_2fa) where the user has identified themselves with
	// a password but has not completed their second factor. Inheriting the
	// full 7-day TTL there means a stolen pre_auth cookie keeps a usable
	// "waiting for 2FA" window open for a week; tightening it to 15 min
	// matches typical industry practice and bounds the hijack window to
	// the time a real user would plausibly take to enter a TOTP code.
	// Once the session transitions to pre_tenant (post-2FA) or
	// authenticated, the full sessionDuration applies.
	preAuthSessionDuration = 15 * time.Minute
	// IdleTimeout forces re-auth after this much inactivity. Enforced by middleware.
	IdleTimeout = 30 * time.Minute
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

type CreateParams struct {
	IdentityID uuid.UUID
	State      string
	IPAddress  string
	UserAgent  string
	AuthMethod string
	CSRFToken  string
}

func (s *Store) Create(ctx context.Context, p CreateParams) (*Session, error) {
	sess := &Session{}
	now := time.Now()
	duration := sessionDuration
	if p.State == StatePre_Auth || p.State == StateSetup2FA {
		duration = preAuthSessionDuration
	}
	err := s.db.QueryRow(ctx, "session.Create",
		`INSERT INTO sessions (identity_id, state, ip_address, user_agent, auth_method, csrf_token, expires_at)
		 VALUES ($1, $2, $3::inet, $4, $5, $6, $7)
		 RETURNING id, identity_id, tenant_id, user_id, sub_org_id, user_role, state, ip_address::text, user_agent,
		           auth_method, second_factor, is_super_admin, csrf_token,
		           created_at, last_active_at, expires_at, revoked_at, revoked_by, revoke_reason`,
		p.IdentityID, p.State, p.IPAddress, p.UserAgent, p.AuthMethod, p.CSRFToken,
		now.Add(duration),
	).Scan(
		&sess.ID, &sess.IdentityID, &sess.TenantID, &sess.UserID, &sess.SubOrgID, &sess.UserRole, &sess.State,
		&sess.IPAddress, &sess.UserAgent, &sess.AuthMethod, &sess.SecondFactor,
		&sess.IsSuperAdmin, &sess.CSRFToken,
		&sess.CreatedAt, &sess.LastActiveAt, &sess.ExpiresAt,
		&sess.RevokedAt, &sess.RevokedBy, &sess.RevokeReason,
	)
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return sess, nil
}

func (s *Store) GetValid(ctx context.Context, id uuid.UUID) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRow(ctx, "session.GetValid",
		`SELECT id, identity_id, tenant_id, user_id, sub_org_id, user_role, state, ip_address::text, user_agent,
		        auth_method, second_factor, is_super_admin, csrf_token,
		        created_at, last_active_at, expires_at, revoked_at, revoked_by, revoke_reason
		 FROM sessions
		 WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`,
		id,
	).Scan(
		&sess.ID, &sess.IdentityID, &sess.TenantID, &sess.UserID, &sess.SubOrgID, &sess.UserRole, &sess.State,
		&sess.IPAddress, &sess.UserAgent, &sess.AuthMethod, &sess.SecondFactor,
		&sess.IsSuperAdmin, &sess.CSRFToken,
		&sess.CreatedAt, &sess.LastActiveAt, &sess.ExpiresAt,
		&sess.RevokedAt, &sess.RevokedBy, &sess.RevokeReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session get: %w", err)
	}
	return sess, nil
}

func (s *Store) TouchLastActive(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "session.TouchLastActive",
		`UPDATE sessions SET last_active_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("session touch: %w", err)
	}
	return nil
}

func (s *Store) UpdateState(ctx context.Context, id uuid.UUID, state string) error {
	_, err := s.db.Exec(ctx, "session.UpdateState",
		`UPDATE sessions SET state = $1 WHERE id = $2`,
		state, id,
	)
	if err != nil {
		return fmt.Errorf("session update state: %w", err)
	}
	return nil
}

func (s *Store) SetTenant(ctx context.Context, id, tenantID, userID uuid.UUID, subOrgID *uuid.UUID, userRole string) error {
	_, err := s.db.Exec(ctx, "session.SetTenant",
		`UPDATE sessions SET tenant_id = $1, user_id = $2, is_super_admin = FALSE, state = $3,
		        sub_org_id = $4, user_role = $5, expires_at = $6
		 WHERE id = $7`,
		tenantID, userID, StateAuthenticated, subOrgID, userRole,
		time.Now().Add(sessionDuration), id,
	)
	if err != nil {
		return fmt.Errorf("session set tenant: %w", err)
	}
	return nil
}

func (s *Store) SetSuperAdmin(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "session.SetSuperAdmin",
		`UPDATE sessions SET is_super_admin = TRUE, state = $1, expires_at = $2 WHERE id = $3`,
		StateAuthenticated, time.Now().Add(sessionDuration), id,
	)
	if err != nil {
		return fmt.Errorf("session set super admin: %w", err)
	}
	return nil
}

// SetSecondFactor promotes a pre_auth session to pre_tenant after 2FA
// verification and resets the expiry to the full sessionDuration. Without
// the expiry reset, the short pre_auth TTL would still apply and could
// kick the user out shortly after a successful 2FA step — the rest of
// the auth flow (tenant selection) needs the full window.
func (s *Store) SetSecondFactor(ctx context.Context, id uuid.UUID, factor string) error {
	_, err := s.db.Exec(ctx, "session.SetSecondFactor",
		`UPDATE sessions SET second_factor = $1, state = $2, expires_at = $3 WHERE id = $4`,
		factor, StatePreTenant, time.Now().Add(sessionDuration), id,
	)
	if err != nil {
		return fmt.Errorf("session set second factor: %w", err)
	}
	return nil
}

func (s *Store) Revoke(ctx context.Context, id uuid.UUID, revokedBy *uuid.UUID, reason string) error {
	tag, err := s.db.Exec(ctx, "session.Revoke",
		`UPDATE sessions SET revoked_at = now(), revoked_by = $1, revoke_reason = $2
		 WHERE id = $3 AND revoked_at IS NULL`,
		revokedBy, reason, id,
	)
	if err != nil {
		return fmt.Errorf("session revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]Session, error) {
	rows, err := s.db.Query(ctx, "session.ListByIdentity",
		`SELECT id, identity_id, tenant_id, user_id, sub_org_id, user_role, state, ip_address::text, user_agent,
		        auth_method, second_factor, is_super_admin, csrf_token,
		        created_at, last_active_at, expires_at, revoked_at, revoked_by, revoke_reason
		 FROM sessions
		 WHERE identity_id = $1 AND revoked_at IS NULL AND expires_at > now()
		 ORDER BY last_active_at DESC`,
		identityID,
	)
	if err != nil {
		return nil, fmt.Errorf("session list: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(
			&sess.ID, &sess.IdentityID, &sess.TenantID, &sess.UserID, &sess.SubOrgID, &sess.UserRole, &sess.State,
			&sess.IPAddress, &sess.UserAgent, &sess.AuthMethod, &sess.SecondFactor,
			&sess.IsSuperAdmin, &sess.CSRFToken,
			&sess.CreatedAt, &sess.LastActiveAt, &sess.ExpiresAt,
			&sess.RevokedAt, &sess.RevokedBy, &sess.RevokeReason,
		); err != nil {
			return nil, fmt.Errorf("session list scan: %w", err)
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// RevokeByTenant bulk-revokes all active sessions for a tenant.
func (s *Store) RevokeByTenant(ctx context.Context, tenantID uuid.UUID, reason string) (int64, error) {
	tag, err := s.db.Exec(ctx, "session.RevokeByTenant",
		`UPDATE sessions SET revoked_at = now(), revoke_reason = $1
		 WHERE tenant_id = $2 AND revoked_at IS NULL AND expires_at > now()`,
		reason, tenantID,
	)
	if err != nil {
		return 0, fmt.Errorf("session revoke by tenant: %w", err)
	}
	return tag.RowsAffected(), nil
}

// UpdateSubOrg updates the sub_org_id on all active sessions for a given user.
// Call this when an admin reassigns a user to a different sub-org so the cached
// scope takes effect immediately without requiring a re-login.
func (s *Store) UpdateSubOrg(ctx context.Context, userID uuid.UUID, subOrgID *uuid.UUID) error {
	_, err := s.db.Exec(ctx, "session.UpdateSubOrg",
		`UPDATE sessions SET sub_org_id = $1
		 WHERE user_id = $2 AND revoked_at IS NULL AND expires_at > now()`,
		subOrgID, userID,
	)
	if err != nil {
		return fmt.Errorf("session update sub org: %w", err)
	}
	return nil
}

// RevokeByIdentity bulk-revokes all active sessions for an identity.
func (s *Store) RevokeByIdentity(ctx context.Context, identityID uuid.UUID, reason string) (int64, error) {
	tag, err := s.db.Exec(ctx, "session.RevokeByIdentity",
		`UPDATE sessions SET revoked_at = now(), revoke_reason = $1
		 WHERE identity_id = $2 AND revoked_at IS NULL AND expires_at > now()`,
		reason, identityID,
	)
	if err != nil {
		return 0, fmt.Errorf("session revoke by identity: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RevokeByIdentityExcept bulk-revokes every active session for the identity
// other than the one identified by exceptID. Used by the user-facing
// "sign out everywhere else" flow so the user does not log themselves out
// of the very session they are using to issue the request.
func (s *Store) RevokeByIdentityExcept(ctx context.Context, identityID, exceptID uuid.UUID, reason string) (int64, error) {
	tag, err := s.db.Exec(ctx, "session.RevokeByIdentityExcept",
		`UPDATE sessions SET revoked_at = now(), revoked_by = $2, revoke_reason = $1
		 WHERE identity_id = $2 AND id <> $3 AND revoked_at IS NULL AND expires_at > now()`,
		reason, identityID, exceptID,
	)
	if err != nil {
		return 0, fmt.Errorf("session revoke by identity except: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ResetContext clears tenant/admin context and returns the session to pre_tenant state.
func (s *Store) ResetContext(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "session.ResetContext",
		`UPDATE sessions
		 SET tenant_id = NULL, user_id = NULL, is_super_admin = FALSE, state = $1
		 WHERE id = $2`,
		StatePreTenant, id,
	)
	if err != nil {
		return fmt.Errorf("session reset context: %w", err)
	}
	return nil
}

// CleanupExpired marks expired sessions as revoked and deletes sessions older than 90 days.
func (s *Store) CleanupExpired(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, "session.CleanupExpired.revoke",
		`UPDATE sessions SET revoked_at = now(), revoke_reason = 'expired'
		 WHERE revoked_at IS NULL AND expires_at < now()`,
	)
	if err != nil {
		return 0, fmt.Errorf("session cleanup revoke: %w", err)
	}
	revoked := tag.RowsAffected()

	_, err = s.db.Exec(ctx, "session.CleanupExpired.delete",
		`DELETE FROM sessions WHERE created_at < now() - interval '90 days'`,
	)
	if err != nil {
		return revoked, fmt.Errorf("session cleanup delete: %w", err)
	}
	return revoked, nil
}
