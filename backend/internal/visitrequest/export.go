package visitrequest

import (
	"context"
	"fmt"
)

// Export returns a flattened audit row per visit matching the filters.
//
// Snapshot semantics — the audit needs to answer "what authorized this visit
// when it was approved", not "what's true today":
//   - Visitor clearance: latest user_clearance_records row that was current
//     on visit.reviewed_at (recorded_at <= reviewed_at AND not yet
//     superseded then). Pulled via a lateral join.
//   - DD254 authorization-at-review: classification cap, period of
//     performance, and read-on status (briefed_at/debriefed_at temporal)
//     all evaluated against reviewed_at + visit dates.
//
// What's *not* snapshotted: DD254 form fields themselves (contract_number,
// classification_max, period dates). DD254s use a supersedes chain rather
// than per-field history; routine edits are rare and major changes produce
// a new superseded row. The export uses current dd254_forms data with that
// caveat. If audits start asking "what was the POP on the day you approved",
// we add a forms-history table.
//
// Unreviewed visits (status=submitted) show NULL for snapshot fields —
// nothing happened at reviewed_at because reviewed_at is NULL.
func (s *Store) Export(ctx context.Context, f ExportFilters) ([]ExportRow, error) {
	var out []ExportRow
	_, err := s.ExportEach(ctx, f, func(r ExportRow) error {
		out = append(out, r)
		return nil
	})
	return out, err
}

// ExportEach streams matching rows to fn one at a time and returns the count.
// The handler uses this to write CSV without buffering the whole result set in
// memory (a wide date range on a large tenant could be tens of thousands of
// rows). Export above is a thin slice-collecting wrapper kept for tests.
func (s *Store) ExportEach(ctx context.Context, f ExportFilters, fn func(ExportRow) error) (int, error) {
	where := "v.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	// Filter by review date if reviewed; for unreviewed visits, fall back to
	// submit date so the auditor can still see "what was pending in this window".
	where += fmt.Sprintf(" AND COALESCE(v.reviewed_at, v.created_at) >= $%d", n)
	args = append(args, f.From)
	n++
	where += fmt.Sprintf(" AND COALESCE(v.reviewed_at, v.created_at) <= $%d", n)
	args = append(args, f.To)
	n++

	if len(f.Statuses) > 0 {
		where += fmt.Sprintf(" AND v.status = ANY($%d)", n)
		args = append(args, f.Statuses)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (v.sub_org_id IS NULL OR v.sub_org_id = $%d)", n)
		args = append(args, *f.SubOrgScope)
		n++
	}
	if f.SubOrgID != nil {
		where += fmt.Sprintf(" AND v.sub_org_id = $%d", n)
		args = append(args, *f.SubOrgID)
	}

	query := fmt.Sprintf(`
		WITH class_rank(level, rank) AS (
			VALUES ('none', 0), ('confidential', 1), ('secret', 2),
			       ('top_secret', 3), ('ts_sci', 4), ('top_secret_sci', 3)
		)
		SELECT
			v.id, v.destination_name, v.diss_smo_code, v.visit_address,
			v.visit_start_date, v.visit_end_date, v.access_level,
			v.visit_description, v.created_at, v.status,
			sub_i.name, sub_i.email,
			clr_snap.clearance, clr_snap.investigation_type, clr_snap.next_investigation_date,
			d.contract_number, d.prime_contractor, d.classification_max::text,
			d.period_start, d.period_end,
			CASE
				WHEN v.dd_254_id IS NULL OR v.reviewed_at IS NULL THEN FALSE
				WHEN d.id IS NULL THEN FALSE
				WHEN COALESCE(d_class.rank, 0) < COALESCE(v_class.rank, 0) THEN FALSE
				WHEN d.period_start IS NOT NULL AND d.period_start > v.visit_start_date THEN FALSE
				WHEN d.period_end IS NOT NULL AND d.period_end < v.visit_end_date THEN FALSE
				WHEN NOT EXISTS (
					SELECT 1 FROM dd254_user_access a
					WHERE a.dd254_id = v.dd_254_id
					  AND a.user_id = v.created_by_user_id
					  AND (a.briefed_at IS NULL OR a.briefed_at <= v.reviewed_at::date)
					  AND (a.debriefed_at IS NULL OR a.debriefed_at > v.reviewed_at::date)
				) THEN FALSE
				ELSE TRUE
			END AS auth_confirmed,
			rev_i.name, rev_i.email, v.reviewed_at, v.reviewer_notes
		FROM visit_requests v
		JOIN users sub_u ON sub_u.id = v.created_by_user_id
		JOIN identities sub_i ON sub_i.id = sub_u.identity_id
		LEFT JOIN LATERAL (
			SELECT cr.clearance::text AS clearance,
			       cr.investigation_type::text AS investigation_type,
			       cr.next_investigation_date
			FROM user_clearance_records cr
			WHERE cr.user_id = v.created_by_user_id
			  AND (v.reviewed_at IS NULL OR cr.recorded_at <= v.reviewed_at)
			  AND (cr.superseded_at IS NULL OR (v.reviewed_at IS NOT NULL AND cr.superseded_at > v.reviewed_at))
			ORDER BY cr.recorded_at DESC
			LIMIT 1
		) clr_snap ON v.reviewed_at IS NOT NULL
		LEFT JOIN dd254_forms d ON d.id = v.dd_254_id
		LEFT JOIN class_rank d_class ON d_class.level = d.classification_max::text
		LEFT JOIN class_rank v_class ON v_class.level = v.access_level
		LEFT JOIN users rev_u ON rev_u.id = v.reviewed_by
		LEFT JOIN identities rev_i ON rev_i.id = rev_u.identity_id
		WHERE %s
		ORDER BY COALESCE(v.reviewed_at, v.created_at) ASC`, where)

	rows, err := s.db.Query(ctx, "visitrequest.Export", query, args...)
	if err != nil {
		return 0, fmt.Errorf("visit export query: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var r ExportRow
		if err := rows.Scan(
			&r.VisitID, &r.DestinationName, &r.DissSmoCode, &r.VisitAddress,
			&r.VisitStartDate, &r.VisitEndDate, &r.AccessLevel,
			&r.VisitDescription, &r.SubmittedAt, &r.Status,
			&r.SubmitterName, &r.SubmitterEmail,
			&r.ClearanceAtReview, &r.InvestigationTypeAtReview, &r.NextInvestigationAtReview,
			&r.DD254ContractNumber, &r.DD254PrimeContractor, &r.DD254Classification,
			&r.DD254PeriodStart, &r.DD254PeriodEnd,
			&r.AuthorizationConfirmedAtReview,
			&r.ReviewerName, &r.ReviewerEmail, &r.ReviewedAt, &r.ReviewerNotes,
		); err != nil {
			return count, fmt.Errorf("visit export scan: %w", err)
		}
		if err := fn(r); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

