package auth

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

// ErrMemberNotFound indicates the requested user/tenant pair doesn't exist.
// Distinct from generic DB errors so the handler can return a proper 404
// instead of masking constraint violations as "not found".
var ErrMemberNotFound = errors.New("member not found")

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

type UserWithTenant struct {
	UserID     uuid.UUID
	UserName   string
	UserRole   string
	TenantName string
	SubOrgID   *uuid.UUID
}

type TenantMembership struct {
	TenantID  uuid.UUID
	Name      string
	Role      string
	Suspended bool
}

type AdminSessionRow struct {
	ID           uuid.UUID  `json:"id"`
	IdentityID   uuid.UUID  `json:"identity_id"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	State        string     `json:"state"`
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	AuthMethod   string     `json:"auth_method"`
	SecondFactor *string    `json:"second_factor,omitempty"`
	IsSuperAdmin bool       `json:"is_super_admin"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
}

type AdminTenantRow struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UserCount   int        `json:"user_count"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
}

type AdminTenantUserRow struct {
	UserID     uuid.UUID  `json:"user_id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	IdentityID *uuid.UUID `json:"identity_id,omitempty"`
}

type AdminTenantDetail struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	SuspendedAt *time.Time           `json:"suspended_at,omitempty"`
	Users       []AdminTenantUserRow `json:"users"`
}

type AdminIdentityRow struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	IsSuperAdmin bool       `json:"is_super_admin"`
	Activated    bool       `json:"activated"`
	SuspendedAt  *time.Time `json:"suspended_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	TenantCount  int        `json:"tenant_count"`
}

type AdminIdentityDetail struct {
	ID           uuid.UUID          `json:"id"`
	Email        string             `json:"email"`
	Name         string             `json:"name"`
	IsSuperAdmin bool               `json:"is_super_admin"`
	Activated    bool               `json:"activated"`
	SuspendedAt  *time.Time         `json:"suspended_at,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	Tenants      []TenantMembership `json:"tenants"`
}

// SSOOption represents an available SSO provider for login identification.
type SSOOption struct {
	TenantName string `json:"tenant_name"`
	SSOURL     string `json:"sso_url"`
}

func (s *Store) ListSSOByIdentity(ctx context.Context, identityID uuid.UUID) ([]SSOOption, error) {
	rows, err := s.db.Query(ctx, "auth.ListSSOByIdentity",
		`SELECT t.name
		 FROM sso_configurations sc
		 JOIN tenants t ON t.id = sc.tenant_id
		 JOIN users u ON u.tenant_id = sc.tenant_id
		 WHERE u.identity_id = $1 AND sc.enabled = true
		   AND t.deleted_at IS NULL AND t.suspended_at IS NULL`,
		identityID,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list sso by identity: %w", err)
	}
	defer rows.Close()

	var opts []SSOOption
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("auth store list sso by identity scan: %w", err)
		}
		opts = append(opts, SSOOption{
			TenantName: name,
			SSOURL:     "/api/auth/sso/" + name,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list sso by identity rows: %w", err)
	}
	return opts, nil
}

// HasEnabledSSODomain reports whether the given tenant has an enabled SSO
// configuration with the given email domain registered. Used by the admin
// invite path to decide whether to send an SSO invite (no password setup)
// or the standard setup-token email.
func (s *Store) HasEnabledSSODomain(ctx context.Context, tenantID uuid.UUID, domain string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, "auth.HasEnabledSSODomain",
		`SELECT EXISTS (
		    SELECT 1 FROM sso_email_domains sed
		    JOIN sso_configurations sc ON sc.id = sed.sso_configuration_id
		    WHERE sc.tenant_id = $1
		      AND sc.enabled = TRUE
		      AND lower(sed.domain) = lower($2)
		 )`,
		tenantID, domain,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("auth has enabled sso domain: %w", err)
	}
	return exists, nil
}

func (s *Store) ListSSOByEmailDomain(ctx context.Context, domain string) ([]SSOOption, error) {
	rows, err := s.db.Query(ctx, "auth.ListSSOByEmailDomain",
		`SELECT t.name
		 FROM sso_email_domains sed
		 JOIN sso_configurations sc ON sc.id = sed.sso_configuration_id
		 JOIN tenants t ON t.id = sc.tenant_id
		 WHERE sed.domain = $1 AND sc.enabled = true
		   AND t.deleted_at IS NULL AND t.suspended_at IS NULL`,
		domain,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list sso by email domain: %w", err)
	}
	defer rows.Close()

	var opts []SSOOption
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("auth store list sso by email domain scan: %w", err)
		}
		opts = append(opts, SSOOption{
			TenantName: name,
			SSOURL:     "/api/auth/sso/" + name,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list sso by email domain rows: %w", err)
	}
	return opts, nil
}

func (s *Store) GetUserWithTenant(ctx context.Context, userID uuid.UUID) (UserWithTenant, error) {
	var u UserWithTenant
	err := s.db.QueryRow(ctx, "auth.GetUserWithTenant",
		`SELECT u.id, i.name, u.role, t.name
		 FROM users u
		 JOIN tenants t ON t.id = u.tenant_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.id = $1`, userID,
	).Scan(&u.UserID, &u.UserName, &u.UserRole, &u.TenantName)
	if err != nil {
		return u, fmt.Errorf("auth store get user with tenant: %w", err)
	}
	return u, nil
}

func (s *Store) ListTenantsByIdentity(ctx context.Context, identityID uuid.UUID) ([]TenantMembership, error) {
	rows, err := s.db.Query(ctx, "auth.ListTenantsByIdentity",
		`SELECT t.id, t.name, u.role, t.suspended_at IS NOT NULL as suspended
		 FROM users u JOIN tenants t ON t.id = u.tenant_id
		 WHERE u.identity_id = $1 AND t.deleted_at IS NULL
		 ORDER BY t.name`, identityID,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []TenantMembership
	for rows.Next() {
		var t TenantMembership
		if err := rows.Scan(&t.TenantID, &t.Name, &t.Role, &t.Suspended); err != nil {
			return nil, fmt.Errorf("auth store list tenants scan: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list tenants rows: %w", err)
	}
	return tenants, nil
}

func (s *Store) GetUserInTenant(ctx context.Context, identityID, tenantID uuid.UUID) (UserWithTenant, error) {
	var u UserWithTenant
	err := s.db.QueryRow(ctx, "auth.GetUserInTenant",
		`SELECT u.id, i.name, u.role, t.name,
		        CASE WHEN u.role = 'administrator'
		             THEN NULL
		             ELSE (SELECT us.suborganization_id FROM user_suborganizations us WHERE us.user_id = u.id LIMIT 1)
		        END
		 FROM users u
		 JOIN tenants t ON t.id = u.tenant_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.identity_id = $1 AND u.tenant_id = $2`,
		identityID, tenantID,
	).Scan(&u.UserID, &u.UserName, &u.UserRole, &u.TenantName, &u.SubOrgID)
	if err != nil {
		return u, fmt.Errorf("auth store get user in tenant: %w", err)
	}
	return u, nil
}

func (s *Store) GetUserRole(ctx context.Context, userID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, "auth.GetUserRole",
		`SELECT role FROM users WHERE id = $1`, userID,
	).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("auth store get user role: %w", err)
	}
	return role, nil
}

func (s *Store) ListAllActiveSessions(ctx context.Context, limit, offset int) ([]AdminSessionRow, error) {
	rows, err := s.db.Query(ctx, "auth.ListAllActiveSessions",
		`SELECT s.id, s.identity_id, s.tenant_id, s.user_id, s.state,
		        s.ip_address::text, s.user_agent, s.auth_method, s.second_factor,
		        s.is_super_admin, s.created_at, s.last_active_at, s.expires_at,
		        i.email, i.name
		 FROM sessions s
		 JOIN identities i ON i.id = s.identity_id
		 WHERE s.revoked_at IS NULL AND s.expires_at > now()
		 ORDER BY s.last_active_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []AdminSessionRow
	for rows.Next() {
		var s AdminSessionRow
		if err := rows.Scan(
			&s.ID, &s.IdentityID, &s.TenantID, &s.UserID, &s.State,
			&s.IPAddress, &s.UserAgent, &s.AuthMethod, &s.SecondFactor,
			&s.IsSuperAdmin, &s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt,
			&s.Email, &s.Name,
		); err != nil {
			return nil, fmt.Errorf("auth store list active sessions scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list active sessions rows: %w", err)
	}
	return sessions, nil
}

func (s *Store) ListAllTenants(ctx context.Context) ([]AdminTenantRow, error) {
	rows, err := s.db.Query(ctx, "auth.ListAllTenants",
		`SELECT t.id, t.name, t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM users u WHERE u.tenant_id = t.id) as user_count,
		        t.suspended_at
		 FROM tenants t WHERE t.deleted_at IS NULL ORDER BY t.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []AdminTenantRow
	for rows.Next() {
		var t AdminTenantRow
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt, &t.UserCount, &t.SuspendedAt); err != nil {
			return nil, fmt.Errorf("auth store list tenants scan: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list tenants rows: %w", err)
	}
	return tenants, nil
}

func (s *Store) CreateTenant(ctx context.Context, name string) (AdminTenantRow, error) {
	var t AdminTenantRow
	err := s.db.QueryRow(ctx, "auth.CreateTenant",
		`INSERT INTO tenants (name) VALUES ($1)
		 RETURNING id, name, created_at, updated_at`, name,
	).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, fmt.Errorf("auth store create tenant: %w", err)
	}
	t.UserCount = 0
	return t, nil
}

func (s *Store) GetTenantDetail(ctx context.Context, tenantID uuid.UUID) (AdminTenantDetail, error) {
	var d AdminTenantDetail
	err := s.db.QueryRow(ctx, "auth.GetTenantDetail",
		`SELECT id, name, created_at, updated_at, suspended_at FROM tenants WHERE id = $1 AND deleted_at IS NULL`, tenantID,
	).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.UpdatedAt, &d.SuspendedAt)
	if err != nil {
		return d, fmt.Errorf("auth store get tenant detail: %w", err)
	}

	rows, err := s.db.Query(ctx, "auth.GetTenantDetail.users",
		`SELECT u.id, i.email, i.name, u.role, u.identity_id
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.tenant_id = $1 ORDER BY i.name`, tenantID,
	)
	if err != nil {
		return d, fmt.Errorf("auth store get tenant users: %w", err)
	}
	defer rows.Close()

	d.Users = []AdminTenantUserRow{}
	for rows.Next() {
		var u AdminTenantUserRow
		if err := rows.Scan(&u.UserID, &u.Email, &u.Name, &u.Role, &u.IdentityID); err != nil {
			return d, fmt.Errorf("auth store get tenant users scan: %w", err)
		}
		d.Users = append(d.Users, u)
	}
	if err := rows.Err(); err != nil {
		return d, fmt.Errorf("auth store get tenant users rows: %w", err)
	}
	return d, nil
}

func (s *Store) UpdateTenant(ctx context.Context, tenantID uuid.UUID, name string) error {
	tag, err := s.db.Exec(ctx, "auth.UpdateTenant",
		`UPDATE tenants SET name = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`, name, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth store update tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auth store update tenant: not found")
	}
	return nil
}

func (s *Store) ListAllIdentities(ctx context.Context, limit, offset int) ([]AdminIdentityRow, error) {
	rows, err := s.db.Query(ctx, "auth.ListAllIdentities",
		`SELECT i.id, i.email, i.name, i.is_super_admin, i.activated, i.suspended_at, i.created_at,
		        (SELECT COUNT(*) FROM users u WHERE u.identity_id = i.id) as tenant_count
		 FROM identities i ORDER BY i.email
		 LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("auth store list identities: %w", err)
	}
	defer rows.Close()

	var identities []AdminIdentityRow
	for rows.Next() {
		var r AdminIdentityRow
		if err := rows.Scan(&r.ID, &r.Email, &r.Name, &r.IsSuperAdmin, &r.Activated, &r.SuspendedAt, &r.CreatedAt, &r.TenantCount); err != nil {
			return nil, fmt.Errorf("auth store list identities scan: %w", err)
		}
		identities = append(identities, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth store list identities rows: %w", err)
	}
	return identities, nil
}

func (s *Store) GetIdentityDetail(ctx context.Context, identityID uuid.UUID) (AdminIdentityDetail, error) {
	var d AdminIdentityDetail
	err := s.db.QueryRow(ctx, "auth.GetIdentityDetail",
		`SELECT id, email, name, is_super_admin, activated, suspended_at, created_at
		 FROM identities WHERE id = $1`, identityID,
	).Scan(&d.ID, &d.Email, &d.Name, &d.IsSuperAdmin, &d.Activated, &d.SuspendedAt, &d.CreatedAt)
	if err != nil {
		return d, fmt.Errorf("auth store get identity detail: %w", err)
	}

	rows, err := s.db.Query(ctx, "auth.GetIdentityDetail.tenants",
		`SELECT t.id, t.name, u.role
		 FROM users u JOIN tenants t ON t.id = u.tenant_id
		 WHERE u.identity_id = $1 ORDER BY t.name`, identityID,
	)
	if err != nil {
		return d, fmt.Errorf("auth store get identity tenants: %w", err)
	}
	defer rows.Close()

	d.Tenants = []TenantMembership{}
	for rows.Next() {
		var m TenantMembership
		if err := rows.Scan(&m.TenantID, &m.Name, &m.Role); err != nil {
			return d, fmt.Errorf("auth store get identity tenants scan: %w", err)
		}
		d.Tenants = append(d.Tenants, m)
	}
	if err := rows.Err(); err != nil {
		return d, fmt.Errorf("auth store get identity tenants rows: %w", err)
	}
	return d, nil
}

type TenantMemberRow struct {
	UserID     uuid.UUID  `json:"user_id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	SubOrgID   *uuid.UUID `json:"sub_org_id"`
	SubOrgName *string    `json:"sub_org_name"`
}

func (s *Store) ListTenantMembers(ctx context.Context, tenantID uuid.UUID, search string) ([]TenantMemberRow, error) {
	where := "u.tenant_id = $1"
	args := []any{tenantID}
	if search != "" {
		where += " AND (i.name ILIKE '%' || $2 || '%' OR i.email ILIKE '%' || $2 || '%')"
		args = append(args, search)
	}
	rows, err := s.db.Query(ctx, "auth.ListTenantMembers",
		fmt.Sprintf(`SELECT DISTINCT ON (u.id) u.id, i.name, i.email, u.role,
		        s.id, s.name
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 LEFT JOIN user_suborganizations us ON us.user_id = u.id
		 LEFT JOIN suborganizations s ON s.id = us.suborganization_id
		 WHERE %s
		 ORDER BY u.id, s.name`, where),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("auth list tenant members: %w", err)
	}
	defer rows.Close()
	var result []TenantMemberRow
	for rows.Next() {
		var r TenantMemberRow
		if err := rows.Scan(&r.UserID, &r.Name, &r.Email, &r.Role, &r.SubOrgID, &r.SubOrgName); err != nil {
			return nil, fmt.Errorf("auth list tenant members scan: %w", err)
		}
		result = append(result, r)
	}
	// Re-sort by name after DISTINCT ON reorders by user.id
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (s *Store) UpdateMemberRole(ctx context.Context, userID, tenantID uuid.UUID, role string) error {
	tag, err := s.db.Exec(ctx, "auth.UpdateMemberRole",
		`UPDATE users SET role = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`,
		role, userID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auth update member role: not found")
	}
	return nil
}

// RemoveMember deletes the (user_id, tenant_id) membership row, then —
// if the underlying identity has no other tenant memberships and is not a
// super admin — also hard-deletes the identity. The cascade chain on
// identities (sessions, totp_secrets, passkeys, password_reset_tokens, etc.)
// then wipes the rest of the account, so admins removing a one-tenant user
// fully delete that user instead of leaving a phantom identity that could
// still log in but belong to no tenant.
//
// Identities that still belong to other tenants are preserved (the user is
// just removed from this tenant). Super-admin identities are never deleted
// from this path — that requires the super-admin-only delete flow with
// last-active-super-admin guards.
func (s *Store) RemoveMember(ctx context.Context, userID, tenantID uuid.UUID) error {
	// Capture the identity_id before the row vanishes.
	var identityID uuid.UUID
	err := s.db.QueryRow(ctx, "auth.RemoveMember.lookupIdentity",
		`SELECT identity_id FROM users WHERE id = $1 AND tenant_id = $2`,
		userID, tenantID,
	).Scan(&identityID)
	if err != nil {
		// pgx returns a sentinel error for no-rows; the existing handler
		// path treats any DB failure here as not-found, which matches what
		// users would see today.
		return ErrMemberNotFound
	}

	tag, err := s.db.Exec(ctx, "auth.RemoveMember",
		`DELETE FROM users WHERE id = $1 AND tenant_id = $2`,
		userID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}

	// If this identity is now orphaned (no other tenant memberships) and is
	// not a super admin, delete it outright. Failure here is logged-only —
	// the membership removal already succeeded and we don't want to roll
	// that back if the identity cleanup hits a constraint.
	var remaining int
	var isSuperAdmin bool
	if err := s.db.QueryRow(ctx, "auth.RemoveMember.countRemaining",
		`SELECT COUNT(u.id), bool_or(i.is_super_admin)
		 FROM identities i
		 LEFT JOIN users u ON u.identity_id = i.id
		 WHERE i.id = $1
		 GROUP BY i.id`,
		identityID,
	).Scan(&remaining, &isSuperAdmin); err != nil {
		// Identity already gone (unlikely but possible with concurrent
		// admin actions) — nothing left to do.
		return nil
	}

	if remaining == 0 && !isSuperAdmin {
		if _, err := s.db.Exec(ctx, "auth.RemoveMember.deleteIdentity",
			`DELETE FROM identities WHERE id = $1`,
			identityID,
		); err != nil {
			// Best-effort. Membership is already gone, so the admin's
			// requested action succeeded; surface the cleanup failure but
			// don't fail the whole operation. The handler logs this.
			return fmt.Errorf("auth remove member: orphan identity cleanup: %w", err)
		}
	}
	return nil
}

// IsOrphanCleanupError reports whether err came from the post-removal
// identity cleanup step (the membership delete already succeeded). The
// handler treats these as warnings rather than failures.
func IsOrphanCleanupError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "orphan identity cleanup")
}

// AddIdentityToTenant inserts the (identity, tenant, role) membership row
// and returns the new user_id so callers (notably the invite flow) can
// assign sub-org membership in the same request.
func (s *Store) AddIdentityToTenant(ctx context.Context, identityID, tenantID uuid.UUID, role string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, "auth.AddIdentityToTenant",
		`INSERT INTO users (identity_id, tenant_id, role)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		identityID, tenantID, role,
	).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth store add identity to tenant: %w", err)
	}
	return userID, nil
}

func (s *Store) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "auth.DeleteTenant",
		`UPDATE tenants SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth store delete tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auth store delete tenant: not found")
	}
	return nil
}

func (s *Store) SuspendTenant(ctx context.Context, tenantID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "auth.SuspendTenant",
		`UPDATE tenants SET suspended_at = now(), updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND suspended_at IS NULL`, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth store suspend tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auth store suspend tenant: not found or already suspended")
	}
	return nil
}

func (s *Store) UnsuspendTenant(ctx context.Context, tenantID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "auth.UnsuspendTenant",
		`UPDATE tenants SET suspended_at = NULL, updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND suspended_at IS NOT NULL`, tenantID,
	)
	if err != nil {
		return fmt.Errorf("auth store unsuspend tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auth store unsuspend tenant: not found or not suspended")
	}
	return nil
}

func (s *Store) IsTenantSuspended(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	var suspended bool
	err := s.db.QueryRow(ctx, "auth.IsTenantSuspended",
		`SELECT suspended_at IS NOT NULL FROM tenants WHERE id = $1 AND deleted_at IS NULL`, tenantID,
	).Scan(&suspended)
	if err != nil {
		return false, fmt.Errorf("auth store is tenant suspended: %w", err)
	}
	return suspended, nil
}
