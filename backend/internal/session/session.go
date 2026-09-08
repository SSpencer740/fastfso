package session

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatePre_Auth      = "pre_auth"
	StateSetup2FA      = "setup_2fa"
	StatePreTenant     = "pre_tenant"
	StateAuthenticated = "authenticated"
)

type Session struct {
	ID           uuid.UUID  `json:"id"`
	IdentityID   uuid.UUID  `json:"identity_id"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	SubOrgID     *uuid.UUID `json:"sub_org_id,omitempty"`
	UserRole     *string    `json:"user_role,omitempty"`
	State        string     `json:"state"`
	IPAddress    string     `json:"ip_address,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	AuthMethod   string     `json:"auth_method"`
	SecondFactor *string    `json:"second_factor,omitempty"`
	IsSuperAdmin bool       `json:"is_super_admin"`
	CSRFToken    string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    *uuid.UUID `json:"revoked_by,omitempty"`
	RevokeReason *string    `json:"revoke_reason,omitempty"`
}

// IsValid returns true if the session is not revoked and not expired.
func (s *Session) IsValid() bool {
	return s.RevokedAt == nil && time.Now().Before(s.ExpiresAt)
}
