package report

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

// Store wraps database.DB for report operations.
type Store struct {
	db database.DB
}

// NewStore creates a new report Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// GetCreatorName returns the full name of the user (via their identity) for use in action item titles.
func (s *Store) GetCreatorName(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	var name string
	err := s.db.QueryRow(ctx, "report.GetCreatorName",
		`SELECT i.name FROM identities i
		 JOIN users u ON u.identity_id = i.id
		 WHERE u.id = $1 AND u.tenant_id = $2`,
		userID, tenantID,
	).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get creator name: %w", err)
	}
	return name, nil
}

// Create inserts a new report and returns it.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, "report.Create",
		`INSERT INTO reports (tenant_id, created_by_user_id, reporting_for, subject_name, report_type, details, sub_org_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, tenant_id, created_by_user_id, sub_org_id, reporting_for, subject_name, report_type,
		           details, status, reviewed_by, reviewed_at, created_at, updated_at`,
		p.TenantID, p.CreatedByUserID, p.ReportingFor, p.SubjectName, p.ReportType, p.Details, p.SubOrgID,
	).Scan(
		&r.ID, &r.TenantID, &r.CreatedByUserID, &r.SubOrgID, &r.ReportingFor, &r.SubjectName, &r.ReportType,
		&r.Details, &r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}
	return &r, nil
}

// Get retrieves a single report by ID scoped to the tenant, including creator info.
func (s *Store) Get(ctx context.Context, tenantID, id uuid.UUID) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, "report.Get",
		`SELECT r.id, r.tenant_id, r.created_by_user_id, i.name, i.email,
		        r.reporting_for, r.subject_name, r.report_type,
		        r.details, r.status, r.reviewed_by, r.reviewed_at, r.created_at, r.updated_at
		 FROM reports r
		 JOIN users u ON u.id = r.created_by_user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE r.id = $1 AND r.tenant_id = $2`,
		id, tenantID,
	).Scan(
		&r.ID, &r.TenantID, &r.CreatedByUserID, &r.CreatorName, &r.CreatorEmail,
		&r.ReportingFor, &r.SubjectName, &r.ReportType,
		&r.Details, &r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get report: %w", err)
	}
	return &r, nil
}

// List returns reports matching the given filters with total count.
func (s *Store) List(ctx context.Context, f ListFilters) ([]ReportRow, int, error) {
	where := "r.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.UserID != nil {
		where += fmt.Sprintf(" AND r.created_by_user_id = $%d", n)
		args = append(args, *f.UserID)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND r.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND (r.report_type ILIKE '%%' || $%d || '%%' OR i.name ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, f.Search)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (r.sub_org_id IS NULL OR r.sub_org_id = $%d)", n)
		args = append(args, f.SubOrgScope)
		n++
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRow(ctx, "report.List.count",
		fmt.Sprintf(`SELECT COUNT(*) FROM reports r
		             JOIN users u ON u.id = r.created_by_user_id
		             JOIN identities i ON i.id = u.identity_id
		             WHERE %s`, where),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, f.Offset)

	query := fmt.Sprintf(`
		SELECT r.id, r.reporting_for, r.subject_name, r.report_type, r.status, i.name, r.created_at
		FROM reports r
		JOIN users u ON u.id = r.created_by_user_id
		JOIN identities i ON i.id = u.identity_id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	rows, err := s.db.Query(ctx, "report.List", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var result []ReportRow
	for rows.Next() {
		var row ReportRow
		if err := rows.Scan(&row.ID, &row.ReportingFor, &row.SubjectName, &row.ReportType, &row.Status, &row.CreatorName, &row.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan report row: %w", err)
		}
		result = append(result, row)
	}
	return result, total, nil
}

// UpdateStatus changes the status of a report.
func (s *Store) UpdateStatus(ctx context.Context, tenantID, id, reviewerID uuid.UUID, status string) error {
	tag, err := s.db.Exec(ctx, "report.UpdateStatus",
		`UPDATE reports SET status = $3, reviewed_by = $4, reviewed_at = now(), updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, status, reviewerID,
	)
	if err != nil {
		return fmt.Errorf("update report status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SummaryStats returns aggregate counts by status.
// If userID is nil, counts all for the tenant.
func (s *Store) SummaryStats(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, subOrgScope *uuid.UUID) (*SummaryStats, error) {
	where := "tenant_id = $1"
	args := []any{tenantID}
	n := 2
	if userID != nil {
		where += fmt.Sprintf(" AND created_by_user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	if subOrgScope != nil {
		where += fmt.Sprintf(" AND (sub_org_id IS NULL OR sub_org_id = $%d)", n)
		args = append(args, subOrgScope)
	}

	var stats SummaryStats
	err := s.db.QueryRow(ctx, "report.SummaryStats",
		fmt.Sprintf(`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'unreviewed'),
		    COUNT(*) FILTER (WHERE status = 'under_review'),
		    COUNT(*) FILTER (WHERE status = 'processed')
		 FROM reports WHERE %s`, where),
		args...,
	).Scan(&stats.Total, &stats.Unreviewed, &stats.UnderReview, &stats.Processed)
	if err != nil {
		return nil, fmt.Errorf("report summary stats: %w", err)
	}
	return &stats, nil
}
