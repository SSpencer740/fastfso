package travel

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("travel report not found")

// Report is the core travel report entity.
type Report struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	UserID             uuid.UUID  `json:"user_id"`
	SubOrgID           *uuid.UUID `json:"sub_org_id,omitempty"`
	TripName           string     `json:"trip_name"`
	MultiCountry       bool       `json:"multi_country"`
	PassportNumber     string     `json:"passport_number"`
	Status             string     `json:"status"`
	EmergencyFirstName string     `json:"emergency_first_name"`
	EmergencyLastName  string     `json:"emergency_last_name"`
	EmergencyPhone     string     `json:"emergency_phone"`
	AdditionalComments string     `json:"additional_comments"`
	SubmittedAt        *time.Time `json:"submitted_at"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	ReviewedBy         *uuid.UUID `json:"reviewed_by"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Country represents a travel destination country.
type Country struct {
	ID                 uuid.UUID  `json:"id"`
	ReportID           uuid.UUID  `json:"report_id"`
	CountryName        string     `json:"country_name"`
	SortOrder          int        `json:"sort_order"`
	StartDate          *time.Time `json:"start_date"`
	EndDate            *time.Time `json:"end_date"`
	Reason             string     `json:"reason"`
	Transportation     []string   `json:"transportation"`
	HasCompanions      bool       `json:"has_companions"`
	CompanionsDetail   string     `json:"companions_detail"`
	HasForeignContacts bool       `json:"has_foreign_contacts"`
	ContactsDetail     string     `json:"contacts_detail"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Upload stores file upload metadata for travel reports.
type Upload struct {
	ID          uuid.UUID `json:"id"`
	ReportID    uuid.UUID `json:"report_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
	StorageKey  string    `json:"storage_key"`
	CreatedAt   time.Time `json:"created_at"`
}

// ReportRow is used in list queries (admin + IC).
type ReportRow struct {
	ID           uuid.UUID  `json:"id"`
	TripName     string     `json:"trip_name"`
	Status       string     `json:"status"`
	Countries    string     `json:"countries"`
	EarliestDate *time.Time `json:"earliest_date"`
	LatestDate   *time.Time `json:"latest_date"`
	CreatorName  string     `json:"creator_name"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ReportDetail is the full detail view.
type ReportDetail struct {
	Report      Report    `json:"report"`
	Countries   []Country `json:"countries"`
	Uploads     []Upload  `json:"uploads"`
	CreatorName string    `json:"creator_name"`
}

// CreateCountryParams is the input for creating a country entry.
type CreateCountryParams struct {
	CountryName        string   `json:"country_name"`
	SortOrder          int      `json:"sort_order"`
	StartDate          *string  `json:"start_date"`
	EndDate            *string  `json:"end_date"`
	Reason             string   `json:"reason"`
	Transportation     []string `json:"transportation"`
	HasCompanions      bool     `json:"has_companions"`
	CompanionsDetail   string   `json:"companions_detail"`
	HasForeignContacts bool     `json:"has_foreign_contacts"`
	ContactsDetail     string   `json:"contacts_detail"`
}

// CreateParams is the input for creating a travel report.
type CreateParams struct {
	TenantID           uuid.UUID             `json:"-"`
	UserID             uuid.UUID             `json:"-"`
	SubOrgID           *uuid.UUID            `json:"-"`
	TripName           string                `json:"trip_name"`
	MultiCountry       bool                  `json:"multi_country"`
	PassportNumber     string                `json:"passport_number"`
	EmergencyFirstName string                `json:"emergency_first_name"`
	EmergencyLastName  string                `json:"emergency_last_name"`
	EmergencyPhone     string                `json:"emergency_phone"`
	AdditionalComments string                `json:"additional_comments"`
	Countries          []CreateCountryParams `json:"countries"`
}

// UpdateParams is for editing a draft report.
type UpdateParams struct {
	TripName           string                `json:"trip_name"`
	MultiCountry       bool                  `json:"multi_country"`
	PassportNumber     string                `json:"passport_number"`
	EmergencyFirstName string                `json:"emergency_first_name"`
	EmergencyLastName  string                `json:"emergency_last_name"`
	EmergencyPhone     string                `json:"emergency_phone"`
	AdditionalComments string                `json:"additional_comments"`
	Countries          []CreateCountryParams `json:"countries"`
}

// ListFilters for filtering travel reports.
type ListFilters struct {
	TenantID    uuid.UUID
	UserID      *uuid.UUID // nil = all users (admin), non-nil = specific user (IC)
	Status      string
	Search      string
	SubOrgScope *uuid.UUID // nil = no filter; non-nil = this sub-org + tenant-wide
	Limit       int
	Offset      int
}

// SummaryStats for dashboard.
type SummaryStats struct {
	Total       int `json:"total"`
	Draft       int `json:"draft"`
	Submitted   int `json:"submitted"`
	UnderReview int `json:"under_review"`
	Approved    int `json:"approved"`
	Rejected    int `json:"rejected"`
}
