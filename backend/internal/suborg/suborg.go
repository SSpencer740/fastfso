package suborg

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("sub-organization not found")
	ErrDefaultLocked = errors.New("the Default sub-organization cannot be deleted")
	ErrNameTaken     = errors.New("a sub-organization with that name already exists")
)

// SubOrg is a sub-organization within a tenant.
type SubOrg struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	Name             string     `json:"name"`
	MemberCount      int        `json:"member_count"`
	PrimaryFSOUserID *uuid.UUID `json:"primary_fso_user_id"`
	PrimaryFSOName   *string    `json:"primary_fso_name"`
	PrimaryFSOEmail  *string    `json:"primary_fso_email"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// FSOContact is the primary FSO details returned to an IC.
type FSOContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SubOrgMember is a user listed under a sub-org.
type SubOrgMember struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
}
