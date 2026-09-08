package team

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// List returns team members for a tenant honoring the role/sub-org scope and filters.
//
// Notes on scope semantics:
//   - SubOrgScope (set for fso/read_only_fso by RequireRole flow): user must
//     belong to at least one sub-org matching the scope.
//   - SubOrgID (admin UI dropdown): same filter, applied independently.
//
// Sub-orgs for each member are aggregated as a JSON array so the list query
// stays a single round trip.
func (s *Store) List(ctx context.Context, f ListFilters) ([]Member, int, error) {
	where := "u.tenant_id = $1 AND i.id IS NOT NULL"
	args := []any{f.TenantID}
	n := 2

	// Sub-org gating — applied via EXISTS so the GROUP BY stays clean.
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_suborganizations us
			WHERE us.user_id = u.id AND us.suborganization_id = $%d
		)`, n)
		args = append(args, *f.SubOrgScope)
		n++
	}
	if f.SubOrgID != nil {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_suborganizations us
			WHERE us.user_id = u.id AND us.suborganization_id = $%d
		)`, n)
		args = append(args, *f.SubOrgID)
		n++
	}

	// Clearance filter.
	switch f.Clearance {
	case "":
		// no filter
	case "none_or_missing":
		where += " AND (cr.id IS NULL OR cr.clearance = 'none')"
	default:
		where += fmt.Sprintf(" AND cr.clearance = $%d", n)
		args = append(args, f.Clearance)
		n++
	}

	if f.DueWithinDays != nil {
		// make_interval(days => $N) takes an int directly — avoids the text-cast
		// encoding pgx refuses to do for parameterized intervals.
		where += fmt.Sprintf(" AND cr.next_investigation_date IS NOT NULL AND cr.next_investigation_date <= CURRENT_DATE + make_interval(days => $%d)", n)
		args = append(args, *f.DueWithinDays)
		n++
	}

	if f.Search != "" {
		where += fmt.Sprintf(" AND (i.name ILIKE '%%' || $%d || '%%' OR i.email ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, f.Search)
		n++
	}

	// Total before pagination so the UI can render page counts.
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := s.db.QueryRow(ctx, "team.List.count",
		fmt.Sprintf(`SELECT COUNT(*) FROM users u
			JOIN identities i ON i.id = u.identity_id
			LEFT JOIN user_clearance_records cr ON cr.id = u.current_clearance_id
			WHERE %s`, where),
		countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("team list count: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit, f.Offset)

	query := fmt.Sprintf(`
		SELECT
			u.id, i.id, i.name, i.email, u.role,
			(i.suspended_at IS NOT NULL) AS suspended,
			COALESCE(
				(SELECT json_agg(json_build_object('id', s.id, 'name', s.name) ORDER BY s.name)
				 FROM user_suborganizations us
				 JOIN suborganizations s ON s.id = us.suborganization_id
				 WHERE us.user_id = u.id),
				'[]'::json
			) AS sub_orgs,
			COALESCE(cr.clearance::text, '') AS clearance,
			cr.investigation_type::text,
			cr.eligibility_date, cr.last_investigation_date, cr.next_investigation_date,
			cr.recorded_at
		FROM users u
		JOIN identities i ON i.id = u.identity_id
		LEFT JOIN user_clearance_records cr ON cr.id = u.current_clearance_id
		WHERE %s
		ORDER BY i.name
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	rows, err := s.db.Query(ctx, "team.List", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("team list: %w", err)
	}
	defer rows.Close()

	var result []Member
	for rows.Next() {
		var m Member
		var subOrgsJSON []byte
		var invType *string
		if err := rows.Scan(
			&m.UserID, &m.IdentityID, &m.Name, &m.Email, &m.Role, &m.Suspended,
			&subOrgsJSON,
			&m.Clearance, &invType,
			&m.EligibilityDate, &m.LastInvestigationDate, &m.NextInvestigationDate,
			&m.ClearanceRecordedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("team list scan: %w", err)
		}
		m.InvestigationType = invType
		if err := unmarshalSubOrgs(subOrgsJSON, &m.SubOrgs); err != nil {
			return nil, 0, fmt.Errorf("team list sub_orgs decode: %w", err)
		}
		if m.SubOrgs == nil {
			m.SubOrgs = []SubOrgRef{}
		}
		result = append(result, m)
	}
	return result, total, nil
}

// Stats returns the summary counters that drive the Team page header tiles.
// Scope semantics match List.
func (s *Store) Stats(ctx context.Context, tenantID uuid.UUID, scope, subOrgID *uuid.UUID) (*SummaryStats, error) {
	where := "u.tenant_id = $1"
	args := []any{tenantID}
	n := 2
	if scope != nil {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_suborganizations us
			WHERE us.user_id = u.id AND us.suborganization_id = $%d
		)`, n)
		args = append(args, *scope)
		n++
	}
	if subOrgID != nil {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_suborganizations us
			WHERE us.user_id = u.id AND us.suborganization_id = $%d
		)`, n)
		args = append(args, *subOrgID)
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) AS members,
			COUNT(*) FILTER (
				WHERE cr.next_investigation_date IS NOT NULL
				  AND cr.next_investigation_date >= CURRENT_DATE
				  AND cr.next_investigation_date <= CURRENT_DATE + INTERVAL '90 days'
			) AS due_within_90,
			COUNT(*) FILTER (
				WHERE cr.next_investigation_date IS NOT NULL
				  AND cr.next_investigation_date < CURRENT_DATE
			) AS overdue
		FROM users u
		LEFT JOIN user_clearance_records cr ON cr.id = u.current_clearance_id
		WHERE %s`, where)

	var stats SummaryStats
	err := s.db.QueryRow(ctx, "team.Stats", query, args...).Scan(
		&stats.Members, &stats.DueWithin90, &stats.Overdue,
	)
	if err != nil {
		return nil, fmt.Errorf("team stats: %w", err)
	}
	return &stats, nil
}

// Get returns a single member with full clearance history, scoped by tenant + role.
func (s *Store) Get(ctx context.Context, tenantID, userID uuid.UUID, scope *uuid.UUID) (*MemberDetail, error) {
	// Sub-org gating up front so we never expose history outside scope.
	if scope != nil {
		var allowed bool
		err := s.db.QueryRow(ctx, "team.Get.scopeCheck",
			`SELECT EXISTS (
				SELECT 1 FROM user_suborganizations us
				JOIN users u ON u.id = us.user_id
				WHERE u.id = $1 AND u.tenant_id = $2 AND us.suborganization_id = $3
			)`,
			userID, tenantID, *scope,
		).Scan(&allowed)
		if err != nil {
			return nil, fmt.Errorf("team get scope check: %w", err)
		}
		if !allowed {
			return nil, ErrNotFound
		}
	}

	var m Member
	var subOrgsJSON []byte
	var invType *string
	err := s.db.QueryRow(ctx, "team.Get",
		`SELECT
			u.id, i.id, i.name, i.email, u.role,
			(i.suspended_at IS NOT NULL) AS suspended,
			COALESCE(
				(SELECT json_agg(json_build_object('id', s.id, 'name', s.name) ORDER BY s.name)
				 FROM user_suborganizations us
				 JOIN suborganizations s ON s.id = us.suborganization_id
				 WHERE us.user_id = u.id),
				'[]'::json
			) AS sub_orgs,
			COALESCE(cr.clearance::text, '') AS clearance,
			cr.investigation_type::text,
			cr.eligibility_date, cr.last_investigation_date, cr.next_investigation_date,
			cr.recorded_at
		FROM users u
		JOIN identities i ON i.id = u.identity_id
		LEFT JOIN user_clearance_records cr ON cr.id = u.current_clearance_id
		WHERE u.id = $1 AND u.tenant_id = $2`,
		userID, tenantID,
	).Scan(
		&m.UserID, &m.IdentityID, &m.Name, &m.Email, &m.Role, &m.Suspended,
		&subOrgsJSON,
		&m.Clearance, &invType,
		&m.EligibilityDate, &m.LastInvestigationDate, &m.NextInvestigationDate,
		&m.ClearanceRecordedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("team get: %w", err)
	}
	m.InvestigationType = invType
	if err := unmarshalSubOrgs(subOrgsJSON, &m.SubOrgs); err != nil {
		return nil, fmt.Errorf("team get sub_orgs decode: %w", err)
	}

	history, err := s.history(ctx, userID)
	if err != nil {
		return nil, err
	}
	if history == nil {
		history = []ClearanceRecord{}
	}
	if m.SubOrgs == nil {
		m.SubOrgs = []SubOrgRef{}
	}
	return &MemberDetail{Member: m, History: history}, nil
}

func (s *Store) history(ctx context.Context, userID uuid.UUID) ([]ClearanceRecord, error) {
	rows, err := s.db.Query(ctx, "team.Get.history",
		`SELECT cr.id, cr.user_id, cr.clearance::text, cr.investigation_type::text,
		        cr.eligibility_date, cr.last_investigation_date, cr.next_investigation_date,
		        cr.notes, cr.recorded_by, COALESCE(ri.name, '(unknown)'), cr.recorded_at, cr.superseded_at
		 FROM user_clearance_records cr
		 LEFT JOIN users ru ON ru.id = cr.recorded_by
		 LEFT JOIN identities ri ON ri.id = ru.identity_id
		 WHERE cr.user_id = $1
		 ORDER BY cr.recorded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("team history: %w", err)
	}
	defer rows.Close()

	var out []ClearanceRecord
	for rows.Next() {
		var r ClearanceRecord
		var invType *string
		if err := rows.Scan(&r.ID, &r.UserID, &r.Clearance, &invType,
			&r.EligibilityDate, &r.LastInvestigationDate, &r.NextInvestigationDate,
			&r.Notes, &r.RecordedBy, &r.RecordedByName, &r.RecordedAt, &r.SupersededAt,
		); err != nil {
			return nil, fmt.Errorf("team history scan: %w", err)
		}
		r.InvestigationType = invType
		out = append(out, r)
	}
	return out, nil
}

// SetClearance records a new clearance entry in a single tx: insert the new
// row, mark any prior current row superseded, and repoint users.current_clearance_id.
func (s *Store) SetClearance(ctx context.Context, tenantID, userID, recordedBy uuid.UUID, p SetClearanceParams) (*ClearanceRecord, error) {
	if !ValidClearance(p.Clearance) {
		return nil, ErrInvalidClearance
	}
	if p.InvestigationType != nil && *p.InvestigationType != "" && !ValidInvestigationType(*p.InvestigationType) {
		return nil, ErrInvalidInvestType
	}

	eligibility, err := parseOptionalDate(p.EligibilityDate)
	if err != nil {
		return nil, fmt.Errorf("eligibility_date: %w", err)
	}
	last, err := parseOptionalDate(p.LastInvestigationDate)
	if err != nil {
		return nil, fmt.Errorf("last_investigation_date: %w", err)
	}
	next, err := parseOptionalDate(p.NextInvestigationDate)
	if err != nil {
		return nil, fmt.Errorf("next_investigation_date: %w", err)
	}

	// Confirm user exists in tenant first; cleaner errors than relying on FK violation.
	var exists bool
	err = s.db.QueryRow(ctx, "team.SetClearance.exists",
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND tenant_id = $2)`,
		userID, tenantID,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("team set clearance exists: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	// 1. Supersede any current row (if present).
	_, err = s.db.Exec(ctx, "team.SetClearance.supersede",
		`UPDATE user_clearance_records SET superseded_at = now()
		 WHERE user_id = $1 AND superseded_at IS NULL`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("team set clearance supersede: %w", err)
	}

	// 2. Insert the new row.
	var invTypeArg any
	if p.InvestigationType != nil && *p.InvestigationType != "" {
		invTypeArg = *p.InvestigationType
	}
	var r ClearanceRecord
	var invType *string
	err = s.db.QueryRow(ctx, "team.SetClearance.insert",
		`INSERT INTO user_clearance_records
		 (user_id, clearance, investigation_type, eligibility_date, last_investigation_date, next_investigation_date, notes, recorded_by)
		 VALUES ($1, $2::clearance_level, $3::investigation_type, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, clearance::text, investigation_type::text,
		           eligibility_date, last_investigation_date, next_investigation_date,
		           notes, recorded_by, recorded_at, superseded_at`,
		userID, p.Clearance, invTypeArg, eligibility, last, next, p.Notes, recordedBy,
	).Scan(&r.ID, &r.UserID, &r.Clearance, &invType,
		&r.EligibilityDate, &r.LastInvestigationDate, &r.NextInvestigationDate,
		&r.Notes, &r.RecordedBy, &r.RecordedAt, &r.SupersededAt)
	if err != nil {
		return nil, fmt.Errorf("team set clearance insert: %w", err)
	}
	r.InvestigationType = invType

	// 3. Repoint the user.
	_, err = s.db.Exec(ctx, "team.SetClearance.repoint",
		`UPDATE users SET current_clearance_id = $1, updated_at = now() WHERE id = $2`,
		r.ID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("team set clearance repoint: %w", err)
	}

	return &r, nil
}

func parseOptionalDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
