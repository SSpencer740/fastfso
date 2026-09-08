package wiki

import (
	"time"

	"github.com/google/uuid"
)

// Post is a single wiki article belonging to a tenant.
type Post struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	CreatedByUserID uuid.UUID  `json:"created_by_user_id"`
	SubOrgID        *uuid.UUID `json:"sub_org_id"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	Published       bool       `json:"published"`
	CreatorName     string     `json:"creator_name"`
	Files           []PostFile `json:"files"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateParams holds the fields needed to create a new wiki post.
type CreateParams struct {
	TenantID        uuid.UUID
	CreatedByUserID uuid.UUID
	SubOrgID        *uuid.UUID
	Title           string
	Content         string
	Published       bool
}

// UpdateParams holds the fields that can be updated on an existing wiki post.
// SubOrgID is optional: it is applied to the row only when UpdateSubOrgID is
// true, allowing callers (FSOs) to leave the sub-org untouched without having
// to re-fetch the existing value.
type UpdateParams struct {
	TenantID       uuid.UUID
	ID             uuid.UUID
	Title          string
	Content        string
	Published      bool
	SubOrgID       *uuid.UUID
	UpdateSubOrgID bool
}

// PostFile represents a PDF attached to a wiki post.
type PostFile struct {
	ID        uuid.UUID `json:"id"`
	PostID    uuid.UUID `json:"post_id"`
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// FSOInfo holds the name and email of an FSO for the "Your FSO" tile.
type FSOInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
