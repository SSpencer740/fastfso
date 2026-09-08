package visitrequest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

// Store wraps database.DB for visit request operations.
type Store struct {
	db database.DB
}

// NewStore creates a new visit request Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new visit request and returns it with all database-generated fields.
func (s *Store) Create(ctx context.Context, p CreateParams) (*VisitRequest, error) {
	startDate, err := time.Parse("2006-01-02", p.VisitStartDate)
	if err != nil {
		return nil, fmt.Errorf("parse visit_start_date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", p.VisitEndDate)
	if err != nil {
		return nil, fmt.Errorf("parse visit_end_date: %w", err)
	}

	var vr VisitRequest
	err = s.db.QueryRow(ctx, "visitrequest.Create",
		`INSERT INTO visit_requests (
			tenant_id, created_by_user_id, destination_name, diss_smo_code, visit_address,
			visit_start_date, visit_end_date, access_level, visit_description,
			poc_name, poc_email, poc_phone,
			security_poc_name, security_poc_email, security_poc_phone,
			cloned_from_id, sub_org_id, dd_254_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, tenant_id, created_by_user_id, sub_org_id, destination_name, diss_smo_code, visit_address,
		          visit_start_date, visit_end_date, access_level, visit_description,
		          poc_name, poc_email, poc_phone,
		          security_poc_name, security_poc_email, security_poc_phone,
		          status, cloned_from_id, dd_254_id, reviewed_by, reviewed_at, reviewer_notes,
		          created_at, updated_at`,
		p.TenantID, p.CreatedByUserID, p.DestinationName, p.DissSmoCode, p.VisitAddress,
		startDate, endDate, p.AccessLevel, p.VisitDescription,
		p.PocName, p.PocEmail, p.PocPhone,
		p.SecurityPocName, p.SecurityPocEmail, p.SecurityPocPhone,
		p.ClonedFromID, p.SubOrgID, p.DD254ID,
	).Scan(
		&vr.ID, &vr.TenantID, &vr.CreatedByUserID, &vr.SubOrgID, &vr.DestinationName, &vr.DissSmoCode, &vr.VisitAddress,
		&vr.VisitStartDate, &vr.VisitEndDate, &vr.AccessLevel, &vr.VisitDescription,
		&vr.PocName, &vr.PocEmail, &vr.PocPhone,
		&vr.SecurityPocName, &vr.SecurityPocEmail, &vr.SecurityPocPhone,
		&vr.Status, &vr.ClonedFromID, &vr.DD254ID, &vr.ReviewedBy, &vr.ReviewedAt, &vr.ReviewerNotes,
		&vr.CreatedAt, &vr.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert visit request: %w", err)
	}
	return &vr, nil
}

// Get retrieves a single visit request by ID, scoped to the given tenant.
func (s *Store) Get(ctx context.Context, tenantID, id uuid.UUID) (*VisitRequest, error) {
	var vr VisitRequest
	err := s.db.QueryRow(ctx, "visitrequest.Get",
		`SELECT id, tenant_id, created_by_user_id, destination_name, diss_smo_code, visit_address,
		        visit_start_date, visit_end_date, access_level, visit_description,
		        poc_name, poc_email, poc_phone,
		        security_poc_name, security_poc_email, security_poc_phone,
		        status, cloned_from_id, dd_254_id, reviewed_by, reviewed_at, reviewer_notes,
		        created_at, updated_at
		 FROM visit_requests WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(
		&vr.ID, &vr.TenantID, &vr.CreatedByUserID, &vr.DestinationName, &vr.DissSmoCode, &vr.VisitAddress,
		&vr.VisitStartDate, &vr.VisitEndDate, &vr.AccessLevel, &vr.VisitDescription,
		&vr.PocName, &vr.PocEmail, &vr.PocPhone,
		&vr.SecurityPocName, &vr.SecurityPocEmail, &vr.SecurityPocPhone,
		&vr.Status, &vr.ClonedFromID, &vr.DD254ID, &vr.ReviewedBy, &vr.ReviewedAt, &vr.ReviewerNotes,
		&vr.CreatedAt, &vr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get visit request: %w", err)
	}
	return &vr, nil
}

// List returns visit requests matching the given filters, along with the total count for pagination.
func (s *Store) List(ctx context.Context, f ListFilters) ([]VisitRequestRow, int, error) {
	where := "vr.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.UserID != nil {
		where += fmt.Sprintf(" AND vr.created_by_user_id = $%d", n)
		args = append(args, *f.UserID)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND vr.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND vr.destination_name ILIKE '%%' || $%d || '%%'", n)
		args = append(args, f.Search)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (vr.sub_org_id IS NULL OR vr.sub_org_id = $%d)", n)
		args = append(args, f.SubOrgScope)
		n++
	}

	// Count
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRow(ctx, "visitrequest.List.count",
		fmt.Sprintf("SELECT COUNT(*) FROM visit_requests vr WHERE %s", where),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count visit requests: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, f.Offset)

	query := fmt.Sprintf(`
		SELECT vr.id, vr.destination_name, vr.visit_start_date, vr.visit_end_date,
		       vr.access_level, vr.status, i.name AS submitter_name, vr.created_at
		FROM visit_requests vr
		JOIN users u ON u.id = vr.created_by_user_id
		JOIN identities i ON i.id = u.identity_id
		WHERE %s
		ORDER BY vr.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	rows, err := s.db.Query(ctx, "visitrequest.List", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list visit requests: %w", err)
	}
	defer rows.Close()

	var result []VisitRequestRow
	for rows.Next() {
		var r VisitRequestRow
		if err := rows.Scan(&r.ID, &r.DestinationName, &r.VisitStartDate, &r.VisitEndDate, &r.AccessLevel, &r.Status, &r.SubmitterName, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan visit request row: %w", err)
		}
		result = append(result, r)
	}

	return result, total, nil
}

// Cancel sets a visit request to cancelled, scoped to the owning user.
// Only submitted requests can be cancelled (not ones already under review or decided).
func (s *Store) Cancel(ctx context.Context, tenantID, id, userID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "visitrequest.Cancel",
		`UPDATE visit_requests SET status = 'cancelled', updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND created_by_user_id = $3
		   AND status = 'submitted'`,
		id, tenantID, userID,
	)
	if err != nil {
		return fmt.Errorf("cancel visit request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus changes the status, reviewer notes, and reviewer of a visit request.
func (s *Store) UpdateStatus(ctx context.Context, tenantID, id, reviewerID uuid.UUID, status, notes string) error {
	tag, err := s.db.Exec(ctx, "visitrequest.UpdateStatus",
		`UPDATE visit_requests SET status = $3, reviewer_notes = $4, reviewed_by = $5, reviewed_at = now(), updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, status, notes, reviewerID,
	)
	if err != nil {
		return fmt.Errorf("update visit request status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SummaryStats returns aggregate visit request counts by status.
// If userID is nil, counts all for the tenant. If non-nil, counts only for that user.
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
	err := s.db.QueryRow(ctx, "visitrequest.SummaryStats",
		fmt.Sprintf(`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'submitted'),
		    COUNT(*) FILTER (WHERE status = 'under_review'),
		    COUNT(*) FILTER (WHERE status = 'approved'),
		    COUNT(*) FILTER (WHERE status = 'rejected'),
		    COUNT(*) FILTER (WHERE status = 'cancelled')
		 FROM visit_requests WHERE %s`, where),
		args...,
	).Scan(&stats.Total, &stats.Submitted, &stats.UnderReview, &stats.Approved, &stats.Rejected, &stats.Cancelled)
	if err != nil {
		return nil, fmt.Errorf("visit request summary stats: %w", err)
	}
	return &stats, nil
}

// ListCloneable returns completed/approved visit requests by a user, for the clone picker.
func (s *Store) ListCloneable(ctx context.Context, tenantID, userID uuid.UUID) ([]VisitRequestRow, error) {
	rows, err := s.db.Query(ctx, "visitrequest.ListCloneable",
		`SELECT vr.id, vr.destination_name, vr.visit_start_date, vr.visit_end_date,
		        vr.access_level, vr.status, i.name AS submitter_name, vr.created_at
		 FROM visit_requests vr
		 JOIN users u ON u.id = vr.created_by_user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE vr.tenant_id = $1
		   AND vr.created_by_user_id = $2
		   AND vr.status IN ('submitted', 'under_review', 'approved')
		 ORDER BY vr.created_at DESC
		 LIMIT 50`,
		tenantID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cloneable visit requests: %w", err)
	}
	defer rows.Close()

	var result []VisitRequestRow
	for rows.Next() {
		var r VisitRequestRow
		if err := rows.Scan(&r.ID, &r.DestinationName, &r.VisitStartDate, &r.VisitEndDate, &r.AccessLevel, &r.Status, &r.SubmitterName, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cloneable visit request row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}
