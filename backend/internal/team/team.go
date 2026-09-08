package team

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound          = errors.New("team member not found")
	ErrInvalidClearance  = errors.New("invalid clearance level")
	ErrInvalidInvestType = errors.New("invalid investigation type")
)

// Valid clearance levels (must match the clearance_level enum).
var clearanceLevels = map[string]struct{}{
	"none":         {},
	"confidential": {},
	"secret":       {},
	"top_secret":   {},
	"ts_sci":       {},
}

// Valid investigation types (must match the investigation_type enum).
var investigationTypes = map[string]struct{}{
	"T3":  {},
	"T3R": {},
	"T5":  {},
	"T5R": {},
}

// ValidClearance reports whether s is a permitted clearance_level value.
func ValidClearance(s string) bool {
	_, ok := clearanceLevels[s]
	return ok
}

// ValidInvestigationType reports whether s is a permitted investigation_type value.
func ValidInvestigationType(s string) bool {
	_, ok := investigationTypes[s]
	return ok
}

// Member is one row in the team list — joins user + identity + (optional) current clearance.
type Member struct {
	UserID                uuid.UUID   `json:"user_id"`
	IdentityID            uuid.UUID   `json:"identity_id"`
	Name                  string      `json:"name"`
	Email                 string      `json:"email"`
	Role                  string      `json:"role"`
	Suspended             bool        `json:"suspended"`
	SubOrgs               []SubOrgRef `json:"sub_orgs"`
	Clearance             string      `json:"clearance"` // "" if no record
	InvestigationType     *string     `json:"investigation_type,omitempty"`
	EligibilityDate       *time.Time  `json:"eligibility_date,omitempty"`
	LastInvestigationDate *time.Time  `json:"last_investigation_date,omitempty"`
	NextInvestigationDate *time.Time  `json:"next_investigation_date,omitempty"`
	ClearanceRecordedAt   *time.Time  `json:"clearance_recorded_at,omitempty"`
}

// SubOrgRef is a lightweight sub-org reference embedded in member rows.
type SubOrgRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ClearanceRecord is one row in user_clearance_records.
type ClearanceRecord struct {
	ID                    uuid.UUID  `json:"id"`
	UserID                uuid.UUID  `json:"user_id"`
	Clearance             string     `json:"clearance"`
	InvestigationType     *string    `json:"investigation_type,omitempty"`
	EligibilityDate       *time.Time `json:"eligibility_date,omitempty"`
	LastInvestigationDate *time.Time `json:"last_investigation_date,omitempty"`
	NextInvestigationDate *time.Time `json:"next_investigation_date,omitempty"`
	Notes                 string     `json:"notes"`
	RecordedBy            uuid.UUID  `json:"recorded_by"`
	RecordedByName        string     `json:"recorded_by_name"`
	RecordedAt            time.Time  `json:"recorded_at"`
	SupersededAt          *time.Time `json:"superseded_at,omitempty"`
}

// MemberDetail is a single member with full clearance history.
type MemberDetail struct {
	Member
	History []ClearanceRecord `json:"history"`
}

// ListFilters constrains a team list query.
type ListFilters struct {
	TenantID      uuid.UUID
	SubOrgScope   *uuid.UUID // nil = no scope; non-nil = this sub-org + tenant-wide
	SubOrgID      *uuid.UUID // explicit filter from admin UI
	Clearance     string     // exact match; "none_or_missing" matches users with no record or clearance='none'
	DueWithinDays *int       // include only users whose next_investigation_date is within N days (incl. overdue)
	Search        string
	Limit         int // page size; <= 0 falls back to the store default
	Offset        int
}

// SummaryStats for the Team tab summary cards.
type SummaryStats struct {
	Members     int `json:"members"`
	DueWithin90 int `json:"due_within_90"`
	Overdue     int `json:"overdue"`
}

// SetClearanceParams is the payload for recording a clearance update.
type SetClearanceParams struct {
	Clearance             string  `json:"clearance"`
	InvestigationType     *string `json:"investigation_type,omitempty"`
	EligibilityDate       *string `json:"eligibility_date,omitempty"`        // YYYY-MM-DD
	LastInvestigationDate *string `json:"last_investigation_date,omitempty"` // YYYY-MM-DD
	NextInvestigationDate *string `json:"next_investigation_date,omitempty"` // YYYY-MM-DD
	Notes                 string  `json:"notes"`
}
