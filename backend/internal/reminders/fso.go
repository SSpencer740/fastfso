package reminders

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/email"
)

// FSO-facing reminders. Distinct from the IC self-reminders in reminders.go:
// these tell an FSO that personnel they're responsible for (cleared users,
// DD254 forms) have items coming due. Aggregated counts per FSO per pass,
// not per-record emails.
//
// Cadence is enforced by last_*_reminder_at columns:
//   - 90d bucket: send once, gate for 60 days
//   - 30d bucket: send once, gate for 20 days
//   - overdue:   send daily (gate on calendar-day change)
//
// The gate windows are sized so that a record can't bounce back and forth
// between buckets (a 91→90 transition fires once, not every day until 30d).

// ClearanceReminderRecipient is one FSO who needs a clearance-due nudge.
// Counts are aggregated across every cleared user whose sub-org points at
// this FSO as primary. RecordIDs is the list of clearance records that
// triggered the bucket — used to update last_due_reminder_at after send.
type ClearanceReminderRecipient struct {
	FSOUserID uuid.UUID
	Email     string
	Name      string
	Counts    email.ClearanceReminderCounts
	RecordIDs []uuid.UUID
}

// Dd254ReminderRecipient is one FSO who needs a DD254-expiration nudge.
type Dd254ReminderRecipient struct {
	FSOUserID uuid.UUID
	Email     string
	Name      string
	Counts    email.Dd254ReminderCounts
	FormIDs   []uuid.UUID
}

// ListClearanceReminders groups eligible clearance records by their primary
// FSO. A user in N sub-orgs with N primary FSOs produces N rows (one per
// FSO); the FSO sees only the users in sub-orgs they cover.
func (s *Store) ListClearanceReminders(ctx context.Context) ([]ClearanceReminderRecipient, error) {
	rows, err := s.db.Query(ctx, "reminders.ListClearance",
		`WITH user_buckets AS (
			SELECT
				cr.id AS record_id,
				cr.user_id,
				CASE
					WHEN cr.next_investigation_date < CURRENT_DATE
					  AND (cr.last_due_reminder_at IS NULL
					       OR cr.last_due_reminder_at::date < CURRENT_DATE)
						THEN 'overdue'
					WHEN cr.next_investigation_date <= CURRENT_DATE + INTERVAL '30 days'
					  AND cr.next_investigation_date >= CURRENT_DATE
					  AND (cr.last_due_reminder_at IS NULL
					       OR cr.last_due_reminder_at < now() - INTERVAL '20 days')
						THEN 'd30'
					WHEN cr.next_investigation_date <= CURRENT_DATE + INTERVAL '90 days'
					  AND cr.next_investigation_date > CURRENT_DATE + INTERVAL '30 days'
					  AND (cr.last_due_reminder_at IS NULL
					       OR cr.last_due_reminder_at < now() - INTERVAL '60 days')
						THEN 'd90'
					ELSE NULL
				END AS bucket
			FROM user_clearance_records cr
			JOIN users u ON u.id = cr.user_id
			JOIN identities i ON i.id = u.identity_id
			WHERE cr.superseded_at IS NULL
			  AND cr.next_investigation_date IS NOT NULL
			  AND i.suspended_at IS NULL
		)
		SELECT
			fso_u.id,
			fso_i.email,
			fso_i.name,
			COUNT(*) FILTER (WHERE ub.bucket = 'd90')     AS due_90,
			COUNT(*) FILTER (WHERE ub.bucket = 'd30')     AS due_30,
			COUNT(*) FILTER (WHERE ub.bucket = 'overdue') AS overdue,
			array_agg(ub.record_id) AS record_ids
		FROM user_buckets ub
		JOIN user_suborganizations us ON us.user_id = ub.user_id
		JOIN suborganizations so ON so.id = us.suborganization_id
		JOIN users fso_u ON fso_u.id = so.primary_fso_user_id
		JOIN identities fso_i ON fso_i.id = fso_u.identity_id
		WHERE ub.bucket IS NOT NULL
		  AND fso_i.suspended_at IS NULL
		  AND fso_i.email <> ''
		GROUP BY fso_u.id, fso_i.email, fso_i.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("reminders list clearance: %w", err)
	}
	defer rows.Close()

	var out []ClearanceReminderRecipient
	for rows.Next() {
		var r ClearanceReminderRecipient
		if err := rows.Scan(&r.FSOUserID, &r.Email, &r.Name,
			&r.Counts.Due90Days, &r.Counts.Due30Days, &r.Counts.Overdue, &r.RecordIDs,
		); err != nil {
			return nil, fmt.Errorf("scan clearance reminder: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// MarkClearanceReminded bumps last_due_reminder_at for the given records to now.
// Called after a successful email send.
func (s *Store) MarkClearanceReminded(ctx context.Context, recordIDs []uuid.UUID) error {
	if len(recordIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, "reminders.MarkClearanceReminded",
		`UPDATE user_clearance_records SET last_due_reminder_at = now()
		 WHERE id = ANY($1)`,
		recordIDs,
	)
	if err != nil {
		return fmt.Errorf("mark clearance reminded: %w", err)
	}
	return nil
}

// ListDd254Reminders groups eligible DD254 forms by their primary FSO. Forms
// in sub-orgs with no primary FSO are skipped (FSO assignment is the trigger
// for ownership). Tenant-wide DD254s (sub_org_id NULL) are skipped — they
// should be either reassigned to a sub-org with a primary FSO, or surfaced
// through a separate admin-fallback path we can add later if needed.
func (s *Store) ListDd254Reminders(ctx context.Context) ([]Dd254ReminderRecipient, error) {
	rows, err := s.db.Query(ctx, "reminders.ListDd254",
		`WITH form_buckets AS (
			SELECT
				d.id AS form_id,
				d.sub_org_id,
				CASE
					WHEN d.period_end < CURRENT_DATE
					  AND (d.last_expiration_reminder_at IS NULL
					       OR d.last_expiration_reminder_at::date < CURRENT_DATE)
						THEN 'overdue'
					WHEN d.period_end <= CURRENT_DATE + INTERVAL '30 days'
					  AND d.period_end >= CURRENT_DATE
					  AND (d.last_expiration_reminder_at IS NULL
					       OR d.last_expiration_reminder_at < now() - INTERVAL '20 days')
						THEN 'd30'
					WHEN d.period_end <= CURRENT_DATE + INTERVAL '90 days'
					  AND d.period_end > CURRENT_DATE + INTERVAL '30 days'
					  AND (d.last_expiration_reminder_at IS NULL
					       OR d.last_expiration_reminder_at < now() - INTERVAL '60 days')
						THEN 'd90'
					ELSE NULL
				END AS bucket
			FROM dd254_forms d
			WHERE d.status = 'active'
			  AND d.period_end IS NOT NULL
			  AND d.sub_org_id IS NOT NULL
		)
		SELECT
			fso_u.id,
			fso_i.email,
			fso_i.name,
			COUNT(*) FILTER (WHERE fb.bucket = 'd90')     AS due_90,
			COUNT(*) FILTER (WHERE fb.bucket = 'd30')     AS due_30,
			COUNT(*) FILTER (WHERE fb.bucket = 'overdue') AS overdue,
			array_agg(fb.form_id) AS form_ids
		FROM form_buckets fb
		JOIN suborganizations so ON so.id = fb.sub_org_id
		JOIN users fso_u ON fso_u.id = so.primary_fso_user_id
		JOIN identities fso_i ON fso_i.id = fso_u.identity_id
		WHERE fb.bucket IS NOT NULL
		  AND fso_i.suspended_at IS NULL
		  AND fso_i.email <> ''
		GROUP BY fso_u.id, fso_i.email, fso_i.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("reminders list dd254: %w", err)
	}
	defer rows.Close()

	var out []Dd254ReminderRecipient
	for rows.Next() {
		var r Dd254ReminderRecipient
		if err := rows.Scan(&r.FSOUserID, &r.Email, &r.Name,
			&r.Counts.Due90Days, &r.Counts.Due30Days, &r.Counts.Overdue, &r.FormIDs,
		); err != nil {
			return nil, fmt.Errorf("scan dd254 reminder: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// MarkDd254Reminded bumps last_expiration_reminder_at on the given forms.
func (s *Store) MarkDd254Reminded(ctx context.Context, formIDs []uuid.UUID) error {
	if len(formIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, "reminders.MarkDd254Reminded",
		`UPDATE dd254_forms SET last_expiration_reminder_at = now()
		 WHERE id = ANY($1)`,
		formIDs,
	)
	if err != nil {
		return fmt.Errorf("mark dd254 reminded: %w", err)
	}
	return nil
}
