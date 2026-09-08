package dd254

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

// Create inserts a new DD254 row. Markings are validated up front (handler also
// validates) so the DB CHECK is never the first failure mode.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Form, error) {
	if p.Markings != RequiredMarkings {
		return nil, ErrInvalidMarkings
	}
	if !ValidClassification(p.ClassificationMax) {
		return nil, ErrInvalidClass
	}

	// Confirm the uploader belongs to the tenant — defensive check before FK violation.
	var ok bool
	err := s.db.QueryRow(ctx, "dd254.Create.checkUser",
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND tenant_id = $2)`,
		p.UploadedBy, p.TenantID,
	).Scan(&ok)
	if err != nil {
		return nil, fmt.Errorf("dd254 create check user: %w", err)
	}
	if !ok {
		return nil, ErrUserNotInTenant
	}

	id := p.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	var f Form
	err = s.db.QueryRow(ctx, "dd254.Create",
		`INSERT INTO dd254_forms
		 (id, tenant_id, sub_org_id, contract_number, prime_contractor, classification_max,
		  period_start, period_end, storage_key, filename, content_type, size_bytes,
		  markings, cui_attestation_by, uploaded_by, supersedes_id)
		 VALUES ($1, $2, $3, $4, $5, $6::clearance_level, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		 RETURNING id, tenant_id, sub_org_id, contract_number, prime_contractor,
		           classification_max::text, period_start, period_end,
		           storage_key, filename, content_type, size_bytes, markings,
		           status::text, supersedes_id, uploaded_by, created_at, updated_at`,
		id, p.TenantID, p.SubOrgID, p.ContractNumber, p.PrimeContractor, p.ClassificationMax,
		p.PeriodStart, p.PeriodEnd, p.StorageKey, p.Filename, p.ContentType, p.SizeBytes,
		p.Markings, p.UploadedBy, p.UploadedBy, p.SupersedesID,
	).Scan(
		&f.ID, &f.TenantID, &f.SubOrgID, &f.ContractNumber, &f.PrimeContractor,
		&f.ClassificationMax, &f.PeriodStart, &f.PeriodEnd,
		&f.StorageKey, &f.Filename, &f.ContentType, &f.SizeBytes, &f.Markings,
		&f.Status, &f.SupersedesID, &f.UploadedBy, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("dd254 create: %w", err)
	}
	return &f, nil
}

// List returns DD254 rows for the tenant honoring filters and role/sub-org scope.
func (s *Store) List(ctx context.Context, f ListFilters) ([]Form, error) {
	where := "d.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.SubOrgScope != nil {
		// FSO sees: their sub-org + tenant-wide (sub_org_id IS NULL).
		where += fmt.Sprintf(" AND (d.sub_org_id IS NULL OR d.sub_org_id = $%d)", n)
		args = append(args, *f.SubOrgScope)
		n++
	}
	if f.SubOrgID != nil {
		where += fmt.Sprintf(" AND d.sub_org_id = $%d", n)
		args = append(args, *f.SubOrgID)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND d.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Class != "" {
		where += fmt.Sprintf(" AND d.classification_max = $%d::clearance_level", n)
		args = append(args, f.Class)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND (d.contract_number ILIKE '%%' || $%d || '%%' OR d.prime_contractor ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, f.Search)
	}

	query := fmt.Sprintf(`
		SELECT d.id, d.tenant_id, d.sub_org_id, s.name AS sub_org_name,
		       d.contract_number, d.prime_contractor,
		       d.classification_max::text, d.period_start, d.period_end,
		       d.storage_key, d.filename, d.content_type, d.size_bytes, d.markings,
		       d.status::text, d.supersedes_id,
		       d.uploaded_by, COALESCE(ui.name, '(unknown)') AS uploaded_by_name,
		       COALESCE(rc.read_on_count, 0) AS read_on_count,
		       d.created_at, d.updated_at
		FROM dd254_forms d
		LEFT JOIN suborganizations s ON s.id = d.sub_org_id
		LEFT JOIN users uu ON uu.id = d.uploaded_by
		LEFT JOIN identities ui ON ui.id = uu.identity_id
		LEFT JOIN (
			SELECT dd254_id, COUNT(*) AS read_on_count
			FROM dd254_user_access
			WHERE debriefed_at IS NULL
			GROUP BY dd254_id
		) rc ON rc.dd254_id = d.id
		WHERE %s
		ORDER BY d.created_at DESC`, where)

	rows, err := s.db.Query(ctx, "dd254.List", query, args...)
	if err != nil {
		return nil, fmt.Errorf("dd254 list: %w", err)
	}
	defer rows.Close()

	var out []Form
	for rows.Next() {
		var f Form
		if err := rows.Scan(&f.ID, &f.TenantID, &f.SubOrgID, &f.SubOrgName,
			&f.ContractNumber, &f.PrimeContractor,
			&f.ClassificationMax, &f.PeriodStart, &f.PeriodEnd,
			&f.StorageKey, &f.Filename, &f.ContentType, &f.SizeBytes, &f.Markings,
			&f.Status, &f.SupersedesID,
			&f.UploadedBy, &f.UploadedByName,
			&f.ReadOnCount,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("dd254 list scan: %w", err)
		}
		out = append(out, f)
	}
	return out, nil
}

// Stats returns summary counts for the DD254 page header.
func (s *Store) Stats(ctx context.Context, tenantID uuid.UUID, scope, subOrgID *uuid.UUID) (*SummaryStats, error) {
	where := "tenant_id = $1"
	args := []any{tenantID}
	n := 2
	if scope != nil {
		where += fmt.Sprintf(" AND (sub_org_id IS NULL OR sub_org_id = $%d)", n)
		args = append(args, *scope)
		n++
	}
	if subOrgID != nil {
		where += fmt.Sprintf(" AND sub_org_id = $%d", n)
		args = append(args, *subOrgID)
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'active') AS active,
			COUNT(*) FILTER (
				WHERE status = 'active'
				  AND period_end IS NOT NULL
				  AND period_end >= CURRENT_DATE
				  AND period_end <= CURRENT_DATE + INTERVAL '90 days'
			) AS expiring_soon,
			COUNT(*) FILTER (WHERE status = 'expired') AS expired,
			COUNT(*) FILTER (
				WHERE status = 'active' AND NOT EXISTS (
					SELECT 1 FROM dd254_user_access a
					WHERE a.dd254_id = dd254_forms.id AND a.debriefed_at IS NULL
				)
			) AS no_read_on
		FROM dd254_forms
		WHERE %s`, where)

	var stats SummaryStats
	err := s.db.QueryRow(ctx, "dd254.Stats", query, args...).Scan(
		&stats.Active, &stats.ExpiringSoon, &stats.Expired, &stats.NoReadOn,
	)
	if err != nil {
		return nil, fmt.Errorf("dd254 stats: %w", err)
	}
	return &stats, nil
}

// Get retrieves a single DD254 with current read-on list.
func (s *Store) Get(ctx context.Context, tenantID, id uuid.UUID, scope *uuid.UUID) (*Detail, error) {
	var f Form
	where := "d.id = $1 AND d.tenant_id = $2"
	args := []any{id, tenantID}
	if scope != nil {
		where += " AND (d.sub_org_id IS NULL OR d.sub_org_id = $3)"
		args = append(args, *scope)
	}

	err := s.db.QueryRow(ctx, "dd254.Get",
		fmt.Sprintf(`SELECT d.id, d.tenant_id, d.sub_org_id, s.name,
		                   d.contract_number, d.prime_contractor,
		                   d.classification_max::text, d.period_start, d.period_end,
		                   d.storage_key, d.filename, d.content_type, d.size_bytes, d.markings,
		                   d.status::text, d.supersedes_id,
		                   d.uploaded_by, COALESCE(ui.name, '(unknown)'),
		                   d.created_at, d.updated_at
		            FROM dd254_forms d
		            LEFT JOIN suborganizations s ON s.id = d.sub_org_id
		            LEFT JOIN users uu ON uu.id = d.uploaded_by
		            LEFT JOIN identities ui ON ui.id = uu.identity_id
		            WHERE %s`, where),
		args...,
	).Scan(&f.ID, &f.TenantID, &f.SubOrgID, &f.SubOrgName,
		&f.ContractNumber, &f.PrimeContractor,
		&f.ClassificationMax, &f.PeriodStart, &f.PeriodEnd,
		&f.StorageKey, &f.Filename, &f.ContentType, &f.SizeBytes, &f.Markings,
		&f.Status, &f.SupersedesID,
		&f.UploadedBy, &f.UploadedByName,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("dd254 get: %w", err)
	}

	grants, err := s.listAccess(ctx, id)
	if err != nil {
		return nil, err
	}
	if grants == nil {
		grants = []AccessGrant{}
	}
	f.ReadOnCount = 0
	for _, g := range grants {
		if g.DebriefedAt == nil {
			f.ReadOnCount++
		}
	}
	return &Detail{Form: f, AccessGrants: grants}, nil
}

func (s *Store) listAccess(ctx context.Context, dd254ID uuid.UUID) ([]AccessGrant, error) {
	rows, err := s.db.Query(ctx, "dd254.listAccess",
		`SELECT a.user_id, COALESCE(i.name, '(unknown)'), COALESCE(i.email, ''),
		        a.briefed_at, a.debriefed_at, a.added_by, a.added_at
		 FROM dd254_user_access a
		 LEFT JOIN users u ON u.id = a.user_id
		 LEFT JOIN identities i ON i.id = u.identity_id
		 WHERE a.dd254_id = $1
		 ORDER BY a.added_at DESC`,
		dd254ID,
	)
	if err != nil {
		return nil, fmt.Errorf("dd254 list access: %w", err)
	}
	defer rows.Close()

	var out []AccessGrant
	for rows.Next() {
		var g AccessGrant
		if err := rows.Scan(&g.UserID, &g.Name, &g.Email, &g.BriefedAt, &g.DebriefedAt, &g.AddedBy, &g.AddedAt); err != nil {
			return nil, fmt.Errorf("dd254 access scan: %w", err)
		}
		out = append(out, g)
	}
	return out, nil
}

// GrantAccess adds a user to a DD254. Idempotent — re-inserts replace briefed/debriefed dates.
func (s *Store) GrantAccess(ctx context.Context, tenantID, dd254ID, userID, addedBy uuid.UUID, briefedAt, debriefedAt *time.Time) error {
	// Verify both DD254 and user belong to the tenant.
	var dd254Exists bool
	err := s.db.QueryRow(ctx, "dd254.GrantAccess.checkDD254",
		`SELECT EXISTS(SELECT 1 FROM dd254_forms WHERE id = $1 AND tenant_id = $2)`,
		dd254ID, tenantID,
	).Scan(&dd254Exists)
	if err != nil {
		return fmt.Errorf("dd254 grant check dd254: %w", err)
	}
	if !dd254Exists {
		return ErrNotFound
	}

	var userExists bool
	err = s.db.QueryRow(ctx, "dd254.GrantAccess.checkUser",
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND tenant_id = $2)`,
		userID, tenantID,
	).Scan(&userExists)
	if err != nil {
		return fmt.Errorf("dd254 grant check user: %w", err)
	}
	if !userExists {
		return ErrUserNotInTenant
	}

	_, err = s.db.Exec(ctx, "dd254.GrantAccess",
		`INSERT INTO dd254_user_access (dd254_id, user_id, briefed_at, debriefed_at, added_by)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (dd254_id, user_id) DO UPDATE
		 SET briefed_at = EXCLUDED.briefed_at,
		     debriefed_at = EXCLUDED.debriefed_at,
		     added_by = EXCLUDED.added_by,
		     added_at = now()`,
		dd254ID, userID, briefedAt, debriefedAt, addedBy,
	)
	if err != nil {
		return fmt.Errorf("dd254 grant access: %w", err)
	}
	return nil
}

// RevokeAccess removes a user from a DD254.
func (s *Store) RevokeAccess(ctx context.Context, tenantID, dd254ID, userID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "dd254.RevokeAccess",
		`DELETE FROM dd254_user_access
		 WHERE dd254_id = $1 AND user_id = $2
		   AND dd254_id IN (SELECT id FROM dd254_forms WHERE tenant_id = $3)`,
		dd254ID, userID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("dd254 revoke access: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete soft-deletes by marking the form superseded. Storage cleanup is the caller's
// responsibility — we keep the row so visit_request linkage history stays intact.
func (s *Store) Delete(ctx context.Context, tenantID, id uuid.UUID) (string, error) {
	var key string
	err := s.db.QueryRow(ctx, "dd254.Delete",
		`UPDATE dd254_forms SET status = 'superseded', updated_at = now()
		 WHERE id = $1 AND tenant_id = $2
		 RETURNING storage_key`,
		id, tenantID,
	).Scan(&key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("dd254 delete: %w", err)
	}
	return key, nil
}

// SuggestForVisit returns DD254s likely to authorize the given visit. The visit's
// creator must have an active (non-debriefed) read-on access entry, the DD254 must
// be active, and the classification_max must meet or exceed the visit's access_level.
// classOrder is used to rank "exact" matches above "higher than needed" matches.
var classOrder = map[string]int{
	"none":         0,
	"confidential": 1,
	"secret":       2,
	"top_secret":   3,
	"ts_sci":       4,
}

func (s *Store) SuggestForVisit(ctx context.Context, tenantID, visitRequestID uuid.UUID) ([]Suggestion, error) {
	// Resolve the visit's creator + access level + dates.
	var createdBy uuid.UUID
	var accessLevel string
	var startDate, endDate time.Time
	err := s.db.QueryRow(ctx, "dd254.SuggestForVisit.visit",
		`SELECT created_by_user_id, access_level, visit_start_date, visit_end_date
		 FROM visit_requests WHERE id = $1 AND tenant_id = $2`,
		visitRequestID, tenantID,
	).Scan(&createdBy, &accessLevel, &startDate, &endDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("dd254 suggest visit: %w", err)
	}

	// Normalize access_level into the clearance_level enum domain. Visit-request
	// access_level uses values like "secret", "top_secret"; if a value falls
	// outside our map, we treat it as "none" (no clearance gating).
	requiredRank, knownLevel := classOrder[accessLevel]
	if !knownLevel {
		requiredRank = 0
	}

	rows, err := s.db.Query(ctx, "dd254.SuggestForVisit",
		`SELECT d.id, d.tenant_id, d.sub_org_id, s.name,
		        d.contract_number, d.prime_contractor,
		        d.classification_max::text, d.period_start, d.period_end,
		        d.storage_key, d.filename, d.content_type, d.size_bytes, d.markings,
		        d.status::text, d.supersedes_id,
		        d.uploaded_by, COALESCE(ui.name, '(unknown)'),
		        d.created_at, d.updated_at
		 FROM dd254_forms d
		 JOIN dd254_user_access a ON a.dd254_id = d.id AND a.user_id = $1 AND a.debriefed_at IS NULL
		 LEFT JOIN suborganizations s ON s.id = d.sub_org_id
		 LEFT JOIN users uu ON uu.id = d.uploaded_by
		 LEFT JOIN identities ui ON ui.id = uu.identity_id
		 WHERE d.tenant_id = $2 AND d.status = 'active'
		   AND (d.period_start IS NULL OR d.period_start <= $3)
		   AND (d.period_end IS NULL OR d.period_end >= $4)`,
		createdBy, tenantID, startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("dd254 suggest query: %w", err)
	}
	defer rows.Close()

	var out []Suggestion
	for rows.Next() {
		var s Suggestion
		if err := rows.Scan(&s.ID, &s.TenantID, &s.SubOrgID, &s.SubOrgName,
			&s.ContractNumber, &s.PrimeContractor,
			&s.ClassificationMax, &s.PeriodStart, &s.PeriodEnd,
			&s.StorageKey, &s.Filename, &s.ContentType, &s.SizeBytes, &s.Markings,
			&s.Status, &s.SupersedesID,
			&s.UploadedBy, &s.UploadedByName,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("dd254 suggest scan: %w", err)
		}
		rank := classOrder[s.ClassificationMax]
		switch {
		case rank < requiredRank:
			continue // doesn't cover the visit's access level
		case rank == requiredRank:
			s.MatchReason = "exact_class"
		default:
			s.MatchReason = "above_class"
		}
		out = append(out, s)
	}
	return out, nil
}

// SetVisitRequestDD254 links a DD254 to a visit request (or clears the link with nil).
func (s *Store) SetVisitRequestDD254(ctx context.Context, tenantID, visitRequestID uuid.UUID, dd254ID *uuid.UUID) error {
	if dd254ID != nil {
		// Confirm the DD254 belongs to the same tenant.
		var ok bool
		err := s.db.QueryRow(ctx, "dd254.SetVisitRequestDD254.check",
			`SELECT EXISTS(SELECT 1 FROM dd254_forms WHERE id = $1 AND tenant_id = $2)`,
			*dd254ID, tenantID,
		).Scan(&ok)
		if err != nil {
			return fmt.Errorf("dd254 link check: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
	}
	tag, err := s.db.Exec(ctx, "dd254.SetVisitRequestDD254",
		`UPDATE visit_requests SET dd_254_id = $3, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		visitRequestID, tenantID, dd254ID,
	)
	if err != nil {
		return fmt.Errorf("dd254 link visit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MyAuthorizations returns active DD254s the given user is currently
// read-on to (no debrief date). Powers the IC-facing visit-submit wizard:
// the IC picks which contract their visit is being made under.
func (s *Store) MyAuthorizations(ctx context.Context, tenantID, userID uuid.UUID) ([]Authorization, error) {
	rows, err := s.db.Query(ctx, "dd254.MyAuthorizations",
		`SELECT d.id, d.contract_number, d.prime_contractor,
		        d.classification_max::text, d.period_start, d.period_end
		 FROM dd254_forms d
		 JOIN dd254_user_access a ON a.dd254_id = d.id AND a.user_id = $1 AND a.debriefed_at IS NULL
		 WHERE d.tenant_id = $2 AND d.status = 'active'
		 ORDER BY d.contract_number`,
		userID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("dd254 my authorizations: %w", err)
	}
	defer rows.Close()

	out := []Authorization{}
	for rows.Next() {
		var a Authorization
		if err := rows.Scan(&a.ID, &a.ContractNumber, &a.PrimeContractor,
			&a.ClassificationMax, &a.PeriodStart, &a.PeriodEnd); err != nil {
			return nil, fmt.Errorf("dd254 my authorizations scan: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// UserHasAccess checks whether a user is currently read-on (not debriefed)
// to a specific DD254. Used to validate the IC's declared contract on visit
// submission — server-side defense in case the wizard is bypassed.
func (s *Store) UserHasAccess(ctx context.Context, tenantID, userID, dd254ID uuid.UUID) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, "dd254.UserHasAccess",
		`SELECT EXISTS (
			SELECT 1 FROM dd254_user_access a
			JOIN dd254_forms d ON d.id = a.dd254_id
			WHERE a.dd254_id = $1 AND a.user_id = $2 AND a.debriefed_at IS NULL
			  AND d.tenant_id = $3 AND d.status = 'active'
		)`,
		dd254ID, userID, tenantID,
	).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("dd254 user has access: %w", err)
	}
	return ok, nil
}

// GetStorageKey looks up a DD254's storage_key+filename, scope-checked.
func (s *Store) GetStorageKey(ctx context.Context, tenantID, id uuid.UUID, scope *uuid.UUID) (key, filename, contentType string, err error) {
	where := "id = $1 AND tenant_id = $2"
	args := []any{id, tenantID}
	if scope != nil {
		where += " AND (sub_org_id IS NULL OR sub_org_id = $3)"
		args = append(args, *scope)
	}
	err = s.db.QueryRow(ctx, "dd254.GetStorageKey",
		fmt.Sprintf("SELECT storage_key, filename, content_type FROM dd254_forms WHERE %s", where),
		args...,
	).Scan(&key, &filename, &contentType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrNotFound
		}
		return "", "", "", fmt.Errorf("dd254 get key: %w", err)
	}
	return key, filename, contentType, nil
}
