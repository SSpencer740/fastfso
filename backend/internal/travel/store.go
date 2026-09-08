package travel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// parseDate parses a "2006-01-02" string into *time.Time. Returns nil for nil input.
func parseDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", *s, err)
	}
	return &t, nil
}

// Create inserts a new travel report and its country entries.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, "travel.Create",
		`INSERT INTO travel_reports (tenant_id, user_id, trip_name, multi_country, passport_number,
		    emergency_first_name, emergency_last_name, emergency_phone, additional_comments, status, sub_org_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'draft', $10)
		 RETURNING id, tenant_id, user_id, sub_org_id, trip_name, multi_country, passport_number, status,
		    emergency_first_name, emergency_last_name, emergency_phone, additional_comments,
		    submitted_at, reviewed_at, reviewed_by, created_at, updated_at`,
		p.TenantID, p.UserID, p.TripName, p.MultiCountry, p.PassportNumber,
		p.EmergencyFirstName, p.EmergencyLastName, p.EmergencyPhone, p.AdditionalComments, p.SubOrgID,
	).Scan(
		&r.ID, &r.TenantID, &r.UserID, &r.SubOrgID, &r.TripName, &r.MultiCountry, &r.PassportNumber, &r.Status,
		&r.EmergencyFirstName, &r.EmergencyLastName, &r.EmergencyPhone, &r.AdditionalComments,
		&r.SubmittedAt, &r.ReviewedAt, &r.ReviewedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("travel create report: %w", err)
	}

	for _, cp := range p.Countries {
		startDate, err := parseDate(cp.StartDate)
		if err != nil {
			return nil, fmt.Errorf("travel create country start_date: %w", err)
		}
		endDate, err := parseDate(cp.EndDate)
		if err != nil {
			return nil, fmt.Errorf("travel create country end_date: %w", err)
		}

		_, err = s.db.Exec(ctx, "travel.Create.country",
			`INSERT INTO travel_report_countries (report_id, country_name, sort_order, start_date, end_date,
			    reason, transportation, has_companions, companions_detail, has_foreign_contacts, contacts_detail)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			r.ID, cp.CountryName, cp.SortOrder, startDate, endDate,
			cp.Reason, cp.Transportation, cp.HasCompanions, cp.CompanionsDetail,
			cp.HasForeignContacts, cp.ContactsDetail,
		)
		if err != nil {
			return nil, fmt.Errorf("travel create country: %w", err)
		}
	}

	return &r, nil
}

// Get returns the full detail view for a travel report scoped to the owning user.
func (s *Store) Get(ctx context.Context, tenantID, userID, reportID uuid.UUID) (*ReportDetail, error) {
	var d ReportDetail
	err := s.db.QueryRow(ctx, "travel.Get",
		`SELECT r.id, r.tenant_id, r.user_id, r.trip_name, r.multi_country, r.passport_number,
		        r.status, r.emergency_first_name, r.emergency_last_name, r.emergency_phone,
		        r.additional_comments, r.submitted_at, r.reviewed_at, r.reviewed_by,
		        r.created_at, r.updated_at, i.name
		 FROM travel_reports r
		 JOIN users u ON u.id = r.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE r.id = $1 AND r.tenant_id = $2 AND r.user_id = $3`,
		reportID, tenantID, userID,
	).Scan(
		&d.Report.ID, &d.Report.TenantID, &d.Report.UserID, &d.Report.TripName,
		&d.Report.MultiCountry, &d.Report.PassportNumber, &d.Report.Status,
		&d.Report.EmergencyFirstName, &d.Report.EmergencyLastName, &d.Report.EmergencyPhone,
		&d.Report.AdditionalComments, &d.Report.SubmittedAt, &d.Report.ReviewedAt,
		&d.Report.ReviewedBy, &d.Report.CreatedAt, &d.Report.UpdatedAt, &d.CreatorName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("travel get report: %w", err)
	}
	return s.fetchDetail(ctx, &d, reportID)
}

// AdminGet returns the full detail view for any report in the tenant (no user filter).
func (s *Store) AdminGet(ctx context.Context, tenantID, reportID uuid.UUID) (*ReportDetail, error) {
	var d ReportDetail
	err := s.db.QueryRow(ctx, "travel.AdminGet",
		`SELECT r.id, r.tenant_id, r.user_id, r.sub_org_id, r.trip_name, r.multi_country, r.passport_number,
		        r.status, r.emergency_first_name, r.emergency_last_name, r.emergency_phone,
		        r.additional_comments, r.submitted_at, r.reviewed_at, r.reviewed_by,
		        r.created_at, r.updated_at, i.name
		 FROM travel_reports r
		 JOIN users u ON u.id = r.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE r.id = $1 AND r.tenant_id = $2`,
		reportID, tenantID,
	).Scan(
		&d.Report.ID, &d.Report.TenantID, &d.Report.UserID, &d.Report.SubOrgID, &d.Report.TripName,
		&d.Report.MultiCountry, &d.Report.PassportNumber, &d.Report.Status,
		&d.Report.EmergencyFirstName, &d.Report.EmergencyLastName, &d.Report.EmergencyPhone,
		&d.Report.AdditionalComments, &d.Report.SubmittedAt, &d.Report.ReviewedAt,
		&d.Report.ReviewedBy, &d.Report.CreatedAt, &d.Report.UpdatedAt, &d.CreatorName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("travel admin get report: %w", err)
	}
	return s.fetchDetail(ctx, &d, reportID)
}

// fetchDetail loads countries and uploads into an already-populated ReportDetail.
func (s *Store) fetchDetail(ctx context.Context, d *ReportDetail, reportID uuid.UUID) (*ReportDetail, error) {
	rows, err := s.db.Query(ctx, "travel.Get.countries",
		`SELECT id, report_id, country_name, sort_order, start_date, end_date,
		        reason, transportation, has_companions, companions_detail,
		        has_foreign_contacts, contacts_detail, created_at, updated_at
		 FROM travel_report_countries
		 WHERE report_id = $1
		 ORDER BY sort_order`,
		reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("travel get countries: %w", err)
	}
	defer rows.Close()

	d.Countries = []Country{}
	for rows.Next() {
		var c Country
		if err := rows.Scan(
			&c.ID, &c.ReportID, &c.CountryName, &c.SortOrder, &c.StartDate, &c.EndDate,
			&c.Reason, &c.Transportation, &c.HasCompanions, &c.CompanionsDetail,
			&c.HasForeignContacts, &c.ContactsDetail, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("travel get countries scan: %w", err)
		}
		d.Countries = append(d.Countries, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("travel get countries rows: %w", err)
	}

	uploadRows, err := s.db.Query(ctx, "travel.Get.uploads",
		`SELECT id, report_id, file_name, file_size, content_type, storage_key, created_at
		 FROM travel_report_uploads
		 WHERE report_id = $1
		 ORDER BY created_at`,
		reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("travel get uploads: %w", err)
	}
	defer uploadRows.Close()

	d.Uploads = []Upload{}
	for uploadRows.Next() {
		var u Upload
		if err := uploadRows.Scan(
			&u.ID, &u.ReportID, &u.FileName, &u.FileSize, &u.ContentType, &u.StorageKey, &u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("travel get uploads scan: %w", err)
		}
		d.Uploads = append(d.Uploads, u)
	}
	if err := uploadRows.Err(); err != nil {
		return nil, fmt.Errorf("travel get uploads rows: %w", err)
	}

	return d, nil
}

// List returns paginated travel reports with aggregated country info.
func (s *Store) List(ctx context.Context, f ListFilters) ([]ReportRow, int, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}

	where := "WHERE r.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.UserID != nil {
		where += fmt.Sprintf(" AND r.user_id = $%d", n)
		args = append(args, *f.UserID)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND r.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND r.trip_name ILIKE '%%' || $%d || '%%'", n)
		args = append(args, f.Search)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (r.sub_org_id IS NULL OR r.sub_org_id = $%d)", n)
		args = append(args, f.SubOrgScope)
		n++
	}

	// Count query
	var total int
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM travel_reports r %s`, where)
	err := s.db.QueryRow(ctx, "travel.List.count", countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("travel list count: %w", err)
	}

	// List query
	listSQL := fmt.Sprintf(
		`SELECT r.id, r.trip_name, r.status,
		        COALESCE(c.country_names, '') AS countries,
		        c.earliest_date, c.latest_date,
		        i.name AS creator_name, r.created_at
		 FROM travel_reports r
		 JOIN users u ON u.id = r.user_id
		 JOIN identities i ON i.id = u.identity_id
		 LEFT JOIN LATERAL (
		     SELECT STRING_AGG(tc.country_name, ', ' ORDER BY tc.sort_order) AS country_names,
		            MIN(tc.start_date) AS earliest_date,
		            MAX(tc.end_date) AS latest_date
		     FROM travel_report_countries tc
		     WHERE tc.report_id = r.id
		 ) c ON true
		 %s
		 ORDER BY r.created_at DESC
		 LIMIT $%d OFFSET $%d`,
		where, n, n+1,
	)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(ctx, "travel.List", listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("travel list: %w", err)
	}
	defer rows.Close()

	var reports []ReportRow
	for rows.Next() {
		var r ReportRow
		if err := rows.Scan(
			&r.ID, &r.TripName, &r.Status, &r.Countries,
			&r.EarliestDate, &r.LatestDate, &r.CreatorName, &r.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("travel list scan: %w", err)
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("travel list rows: %w", err)
	}

	return reports, total, nil
}

// Update updates a draft travel report and replaces its countries.
func (s *Store) Update(ctx context.Context, tenantID, userID, reportID uuid.UUID, p UpdateParams) error {
	tag, err := s.db.Exec(ctx, "travel.Update",
		`UPDATE travel_reports
		 SET trip_name = $1, multi_country = $2, passport_number = $3,
		     emergency_first_name = $4, emergency_last_name = $5, emergency_phone = $6,
		     additional_comments = $7, updated_at = now()
		 WHERE id = $8 AND tenant_id = $9 AND user_id = $10 AND status = 'draft'`,
		p.TripName, p.MultiCountry, p.PassportNumber,
		p.EmergencyFirstName, p.EmergencyLastName, p.EmergencyPhone,
		p.AdditionalComments, reportID, tenantID, userID,
	)
	if err != nil {
		return fmt.Errorf("travel update report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Delete existing countries
	_, err = s.db.Exec(ctx, "travel.Update.deleteCountries",
		`DELETE FROM travel_report_countries WHERE report_id = $1`, reportID,
	)
	if err != nil {
		return fmt.Errorf("travel update delete countries: %w", err)
	}

	// Re-insert countries
	for _, cp := range p.Countries {
		startDate, err := parseDate(cp.StartDate)
		if err != nil {
			return fmt.Errorf("travel update country start_date: %w", err)
		}
		endDate, err := parseDate(cp.EndDate)
		if err != nil {
			return fmt.Errorf("travel update country end_date: %w", err)
		}

		_, err = s.db.Exec(ctx, "travel.Update.country",
			`INSERT INTO travel_report_countries (report_id, country_name, sort_order, start_date, end_date,
			    reason, transportation, has_companions, companions_detail, has_foreign_contacts, contacts_detail)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			reportID, cp.CountryName, cp.SortOrder, startDate, endDate,
			cp.Reason, cp.Transportation, cp.HasCompanions, cp.CompanionsDetail,
			cp.HasForeignContacts, cp.ContactsDetail,
		)
		if err != nil {
			return fmt.Errorf("travel update country: %w", err)
		}
	}

	return nil
}

// Submit transitions a draft report to submitted status.
func (s *Store) Submit(ctx context.Context, tenantID, userID, reportID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "travel.Submit",
		`UPDATE travel_reports
		 SET status = 'submitted', submitted_at = now(), updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status = 'draft'`,
		reportID, tenantID, userID,
	)
	if err != nil {
		return fmt.Errorf("travel submit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkUnderReview transitions a submitted report to under_review. No-ops if already past submitted.
func (s *Store) MarkUnderReview(ctx context.Context, tenantID, reportID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "travel.MarkUnderReview",
		`UPDATE travel_reports
		 SET status = 'under_review', updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND status = 'submitted'`,
		reportID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("travel mark under review: %w", err)
	}
	return nil
}

// UpdateStatus sets status, reviewed_at, and reviewed_by (for admin approve/reject).
func (s *Store) UpdateStatus(ctx context.Context, tenantID, reportID uuid.UUID, status string, reviewedBy uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "travel.UpdateStatus",
		`UPDATE travel_reports
		 SET status = $1, reviewed_at = now(), reviewed_by = $2, updated_at = now()
		 WHERE id = $3 AND tenant_id = $4`,
		status, reviewedBy, reportID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("travel update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SummaryStats returns aggregated counts by status for the dashboard.
func (s *Store) SummaryStats(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, subOrgScope *uuid.UUID) (*SummaryStats, error) {
	where := "WHERE tenant_id = $1"
	args := []any{tenantID}
	n := 2

	if userID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	if subOrgScope != nil {
		where += fmt.Sprintf(" AND (sub_org_id IS NULL OR sub_org_id = $%d)", n)
		args = append(args, subOrgScope)
	}

	query := fmt.Sprintf(
		`SELECT COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'draft') AS draft,
		        COUNT(*) FILTER (WHERE status = 'submitted') AS submitted,
		        COUNT(*) FILTER (WHERE status = 'under_review') AS under_review,
		        COUNT(*) FILTER (WHERE status = 'approved') AS approved,
		        COUNT(*) FILTER (WHERE status = 'rejected') AS rejected
		 FROM travel_reports %s`, where,
	)

	var stats SummaryStats
	err := s.db.QueryRow(ctx, "travel.SummaryStats", query, args...).Scan(
		&stats.Total, &stats.Draft, &stats.Submitted,
		&stats.UnderReview, &stats.Approved, &stats.Rejected,
	)
	if err != nil {
		return nil, fmt.Errorf("travel summary stats: %w", err)
	}
	return &stats, nil
}

// CreateUpload inserts a file upload record for a travel report.
func (s *Store) CreateUpload(ctx context.Context, reportID uuid.UUID, fileName string, fileSize int64, contentType, storageKey string) (*Upload, error) {
	var u Upload
	err := s.db.QueryRow(ctx, "travel.CreateUpload",
		`INSERT INTO travel_report_uploads (report_id, file_name, file_size, content_type, storage_key)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, report_id, file_name, file_size, content_type, storage_key, created_at`,
		reportID, fileName, fileSize, contentType, storageKey,
	).Scan(&u.ID, &u.ReportID, &u.FileName, &u.FileSize, &u.ContentType, &u.StorageKey, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("travel create upload: %w", err)
	}
	return &u, nil
}

// ListUploads returns all uploads for a travel report.
func (s *Store) ListUploads(ctx context.Context, reportID uuid.UUID) ([]Upload, error) {
	rows, err := s.db.Query(ctx, "travel.ListUploads",
		`SELECT id, report_id, file_name, file_size, content_type, storage_key, created_at
		 FROM travel_report_uploads
		 WHERE report_id = $1
		 ORDER BY created_at`,
		reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("travel list uploads: %w", err)
	}
	defer rows.Close()

	var uploads []Upload
	for rows.Next() {
		var u Upload
		if err := rows.Scan(
			&u.ID, &u.ReportID, &u.FileName, &u.FileSize, &u.ContentType, &u.StorageKey, &u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("travel list uploads scan: %w", err)
		}
		uploads = append(uploads, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("travel list uploads rows: %w", err)
	}
	return uploads, nil
}

// GetUploadForDownload returns the storage key, file name, and content type for an upload,
// scoped to the given tenant (via report join) for access control.
func (s *Store) GetUploadForDownload(ctx context.Context, tenantID, uploadID uuid.UUID) (key, fileName, contentType string, err error) {
	err = s.db.QueryRow(ctx, "travel.GetUploadForDownload",
		`SELECT u.storage_key, u.file_name, u.content_type
		 FROM travel_report_uploads u
		 JOIN travel_reports r ON r.id = u.report_id
		 WHERE u.id = $1 AND r.tenant_id = $2`,
		uploadID, tenantID,
	).Scan(&key, &fileName, &contentType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrNotFound
		}
	}
	return
}

// DeleteUpload removes an upload record and returns its storage key for cleanup.
// tenantID is verified via the report join to prevent cross-tenant deletion.
func (s *Store) DeleteUpload(ctx context.Context, tenantID, uploadID uuid.UUID) (string, error) {
	var storageKey string
	err := s.db.QueryRow(ctx, "travel.DeleteUpload",
		`DELETE FROM travel_report_uploads tru
		 USING travel_reports tr
		 WHERE tru.id = $1 AND tru.report_id = tr.id AND tr.tenant_id = $2
		 RETURNING tru.storage_key`,
		uploadID, tenantID,
	).Scan(&storageKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("travel delete upload: %w", err)
	}
	return storageKey, nil
}

// GetUserContact returns the email and display name for a tenant user. Used
// by the debrief cron to email the IC when their post-travel questionnaire is
// auto-created. Empty strings (no error) when the user is not found.
func (s *Store) GetUserContact(ctx context.Context, tenantID, userID uuid.UUID) (email, name string, err error) {
	err = s.db.QueryRow(ctx, "travel.GetUserContact",
		`SELECT i.email, i.name
		 FROM users u JOIN identities i ON i.id = u.identity_id
		 WHERE u.id = $1 AND u.tenant_id = $2`,
		userID, tenantID,
	).Scan(&email, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return
}
