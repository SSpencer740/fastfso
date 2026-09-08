package actionitem

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("action item not found")

// ActionItem is the core action item entity.
type ActionItem struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	SourceType    string     `json:"source_type"`
	SourceID      *uuid.UUID `json:"source_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Priority      string     `json:"priority"`
	Status        string     `json:"status"`
	AssignedTo    *uuid.UUID `json:"assigned_to"`
	AssigneeName  *string    `json:"assignee_name"`
	AssigneeEmail *string    `json:"assignee_email"`
	Notes         string     `json:"notes"`
	DueDate       *time.Time `json:"due_date"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// FSOUser is a lightweight user record for the assignee picker (FSO + admin only).
type FSOUser struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// ActionItemRow is the list view with assignee details.
type ActionItemRow struct {
	ID            uuid.UUID  `json:"id"`
	SourceType    string     `json:"source_type"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Priority      string     `json:"priority"`
	Status        string     `json:"status"`
	AssigneeName  *string    `json:"assignee_name"`
	AssigneeEmail *string    `json:"assignee_email"`
	SubOrgName    *string    `json:"sub_org_name"`
	DueDate       *time.Time `json:"due_date"`
	CreatedAt     time.Time  `json:"created_at"`
}

// CreateParams for creating a new action item.
type CreateParams struct {
	TenantID    uuid.UUID
	SourceType  string     `json:"source_type"`
	SourceID    *uuid.UUID `json:"source_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	AssignedTo  *uuid.UUID `json:"assigned_to"`
	DueDate     *time.Time `json:"due_date"`
	SubOrgID    *uuid.UUID `json:"-"`
}

// ListFilters for filtering action items.
type ListFilters struct {
	TenantID    uuid.UUID
	SourceType  string
	Status      string
	Search      string
	SubOrgScope *uuid.UUID // nil = no filter (admin); non-nil = show this sub-org + tenant-wide
	AssignedTo  *uuid.UUID // non-nil = only items assigned to this user (IC scope)
	Limit       int
	Offset      int
}

// SummaryStats for the action items dashboard.
type SummaryStats struct {
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	UnderReview int `json:"under_review"`
	Processed   int `json:"processed"`
}
