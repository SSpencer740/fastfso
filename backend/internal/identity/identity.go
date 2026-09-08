package identity

import (
	"time"

	"github.com/google/uuid"
)

const (
	MaxFailedLogins = 5
	LockDuration    = 30 * time.Minute
)

type Identity struct {
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	PasswordHash        *string    `json:"-"`
	IsSuperAdmin        bool       `json:"is_super_admin"`
	Activated           bool       `json:"activated"`
	SuspendedAt         *time.Time `json:"suspended_at,omitempty"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// IsLocked returns true if the identity is currently locked out.
func (i *Identity) IsLocked() bool {
	return i.LockedUntil != nil && time.Now().Before(*i.LockedUntil)
}

// IsSuspended returns true if the identity is currently suspended.
func (i *Identity) IsSuspended() bool {
	return i.SuspendedAt != nil
}

// HasPassword returns true if the identity has a password set.
func (i *Identity) HasPassword() bool {
	return i.PasswordHash != nil && *i.PasswordHash != ""
}
