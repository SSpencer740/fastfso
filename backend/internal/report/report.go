package report

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("report not found")

// Report is the full report entity.
type Report struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	CreatedByUserID uuid.UUID  `json:"created_by_user_id"`
	SubOrgID        *uuid.UUID `json:"sub_org_id,omitempty"`
	CreatorName     string     `json:"creator_name"`
	CreatorEmail    string     `json:"creator_email"`
	ReportingFor    string     `json:"reporting_for"` // self | other | fcl
	SubjectName     string     `json:"subject_name"`
	ReportType      string     `json:"report_type"`
	Details         string     `json:"details"`
	Status          string     `json:"status"`
	ReviewedBy      *uuid.UUID `json:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ReportRow is the list view with creator name.
type ReportRow struct {
	ID           uuid.UUID `json:"id"`
	ReportingFor string    `json:"reporting_for"`
	SubjectName  string    `json:"subject_name"`
	ReportType   string    `json:"report_type"`
	Status       string    `json:"status"`
	CreatorName  string    `json:"creator_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateParams for submitting a new report.
type CreateParams struct {
	TenantID        uuid.UUID  `json:"-"`
	CreatedByUserID uuid.UUID  `json:"-"`
	SubOrgID        *uuid.UUID `json:"-"`
	ReportingFor    string     `json:"reporting_for"`
	SubjectName     string     `json:"subject_name"`
	ReportType      string     `json:"report_type"`
	Details         string     `json:"details"`
}

// ListFilters for filtering reports.
type ListFilters struct {
	TenantID    uuid.UUID
	UserID      *uuid.UUID // nil = all (admin), non-nil = specific user (IC)
	Status      string
	Search      string
	SubOrgScope *uuid.UUID // nil = no filter; non-nil = this sub-org + tenant-wide
	Limit       int
	Offset      int
}

// SummaryStats for dashboard cards.
type SummaryStats struct {
	Total       int `json:"total"`
	Unreviewed  int `json:"unreviewed"`
	UnderReview int `json:"under_review"`
	Processed   int `json:"processed"`
}

// ReportTypeLabel returns the human-readable label for a report type.
func ReportTypeLabel(t string) string {
	labels := map[string]string{
		"alcohol_drug_treatment":  "Alcohol/Drug Related Treatment",
		"arrests":                 "Arrests",
		"cohabitant":              "Cohabitant",
		"elicitation":             "Elicitation, Exploitation, Blackmail, Coercion, or Enticement",
		"financial":               "Financial",
		"foreign_activities":      "Foreign Activities",
		"foreign_contacts":        "Foreign Contacts",
		"marriage_divorce":        "Marriage/Divorce",
		"media_contact":           "Media Contact",
		"other":                   "Other",
		"fcl_change_of_ownership": "FCL: Change of Ownership",
		"fcl_address_change":      "FCL: Address Change",
		"fcl_unable_to_safeguard": "FCL: Unable to Safeguard Classified Information",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return t
}
