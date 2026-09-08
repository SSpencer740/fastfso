package visitrequest

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("visit request not found")

// VisitRequest is the core entity — a flat record matching all 3 wizard steps.
type VisitRequest struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	CreatedByUserID  uuid.UUID  `json:"created_by_user_id"`
	SubOrgID         *uuid.UUID `json:"sub_org_id,omitempty"`
	DestinationName  string     `json:"destination_name"`
	DissSmoCode      string     `json:"diss_smo_code"`
	VisitAddress     string     `json:"visit_address"`
	VisitStartDate   time.Time  `json:"visit_start_date"`
	VisitEndDate     time.Time  `json:"visit_end_date"`
	AccessLevel      string     `json:"access_level"`
	VisitDescription string     `json:"visit_description"`
	PocName          string     `json:"poc_name"`
	PocEmail         string     `json:"poc_email"`
	PocPhone         string     `json:"poc_phone"`
	SecurityPocName  string     `json:"security_poc_name"`
	SecurityPocEmail string     `json:"security_poc_email"`
	SecurityPocPhone string     `json:"security_poc_phone"`
	Status           string     `json:"status"`
	ClonedFromID     *uuid.UUID `json:"cloned_from_id"`
	DD254ID          *uuid.UUID `json:"dd_254_id,omitempty"`
	ReviewedBy       *uuid.UUID `json:"reviewed_by"`
	ReviewedAt       *time.Time `json:"reviewed_at"`
	ReviewerNotes    string     `json:"reviewer_notes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// VisitRequestRow is the list view with submitter name.
type VisitRequestRow struct {
	ID              uuid.UUID `json:"id"`
	DestinationName string    `json:"destination_name"`
	VisitStartDate  time.Time `json:"visit_start_date"`
	VisitEndDate    time.Time `json:"visit_end_date"`
	AccessLevel     string    `json:"access_level"`
	Status          string    `json:"status"`
	SubmitterName   string    `json:"submitter_name"`
	CreatedAt       time.Time `json:"created_at"`
}

// CreateParams input for creating a visit request.
type CreateParams struct {
	TenantID         uuid.UUID  `json:"-"`
	CreatedByUserID  uuid.UUID  `json:"-"`
	SubOrgID         *uuid.UUID `json:"-"`
	DestinationName  string     `json:"destination_name"`
	DissSmoCode      string     `json:"diss_smo_code"`
	VisitAddress     string     `json:"visit_address"`
	VisitStartDate   string     `json:"visit_start_date"`
	VisitEndDate     string     `json:"visit_end_date"`
	AccessLevel      string     `json:"access_level"`
	VisitDescription string     `json:"visit_description"`
	PocName          string     `json:"poc_name"`
	PocEmail         string     `json:"poc_email"`
	PocPhone         string     `json:"poc_phone"`
	SecurityPocName  string     `json:"security_poc_name"`
	SecurityPocEmail string     `json:"security_poc_email"`
	SecurityPocPhone string     `json:"security_poc_phone"`
	ClonedFromID     *uuid.UUID `json:"cloned_from_id"`
	// DD254ID is the contract the IC is declaring this visit is made under.
	// Optional — visits not tied to a specific contract (e.g., general
	// orientation) leave it nil. Server-side check confirms the user is
	// actually read-on; bad values are rejected as 400.
	DD254ID *uuid.UUID `json:"dd_254_id,omitempty"`
}

// ListFilters for filtering visit requests.
type ListFilters struct {
	TenantID    uuid.UUID
	UserID      *uuid.UUID // nil = all (admin), non-nil = specific user (IC)
	Status      string
	Search      string
	SubOrgScope *uuid.UUID // nil = no filter; non-nil = this sub-org + tenant-wide
	Limit       int
	Offset      int
}

// ExportFilters drives the audit-export query. Date range is review date —
// auditors think in "decisions made between X and Y", not "visits scheduled
// for X and Y". From/To are inclusive; an empty Statuses slice means all.
type ExportFilters struct {
	TenantID    uuid.UUID
	From        time.Time
	To          time.Time
	Statuses    []string   // empty = all
	SubOrgScope *uuid.UUID // FSO scope from auth.SubOrgScope
	SubOrgID    *uuid.UUID // admin dropdown filter
	Detail      string     // "full" (default) or "summary"
}

// ExportRow is one flattened row in the audit CSV. Time-sensitive fields
// (visitor clearance, DD254 authorization-at-review) are snapshotted to
// reviewed_at — what was true when the FSO decided, not what's true now.
// See Store.Export for the snapshot SQL.
type ExportRow struct {
	// Visit
	VisitID          uuid.UUID
	DestinationName  string
	DissSmoCode      string
	VisitAddress     string
	VisitStartDate   time.Time
	VisitEndDate     time.Time
	AccessLevel      string
	VisitDescription string
	SubmittedAt      time.Time
	Status           string

	// Submitter
	SubmitterName  string
	SubmitterEmail string

	// Submitter clearance as-of reviewed_at (nil if no clearance recorded
	// at that point, or if the visit was never reviewed).
	ClearanceAtReview            *string
	InvestigationTypeAtReview    *string
	NextInvestigationAtReview    *time.Time

	// Linked DD254 (current row data; see store-level note about non-historized fields).
	DD254ContractNumber  *string
	DD254PrimeContractor *string
	DD254Classification  *string
	DD254PeriodStart     *time.Time
	DD254PeriodEnd       *time.Time

	// Whether the linked DD254 actually authorized the visit at reviewed_at —
	// read-on status (briefed/debriefed temporal), classification cap, and
	// period of performance all evaluated against reviewed_at + visit dates.
	AuthorizationConfirmedAtReview bool

	// Decision
	ReviewerName  *string
	ReviewerEmail *string
	ReviewedAt    *time.Time
	ReviewerNotes string
}

// SummaryStats for dashboard.
type SummaryStats struct {
	Total       int `json:"total"`
	Submitted   int `json:"submitted"`
	UnderReview int `json:"under_review"`
	Approved    int `json:"approved"`
	Rejected    int `json:"rejected"`
	Cancelled   int `json:"cancelled"`
}
