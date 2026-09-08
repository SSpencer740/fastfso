// Package reminders sends due-date and overdue notifications to users who
// have opted into per-event emails (notification_frequency = every_task).
//
// Users on the daily summary already get overdue and open counts surfaced in
// their digest, so this package skips them — otherwise they'd get two emails
// a day saying largely the same thing.
//
// Privacy posture mirrors the digest: counts only, no titles or other
// tenant-specific content.
package reminders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
	"github.com/fastfso/fastfso/backend/internal/email"
)

// DueSoonWindowDays is how many days ahead counts as "due soon." Anything
// past CURRENT_DATE counts as overdue regardless.
const DueSoonWindowDays = 3

// Recipient is one IC who is candidate for a reminder.
type Recipient struct {
	UserID uuid.UUID
	Email  string
	Name   string
}

// Notifier sends the actual email. Matches notify.Notifier's gating contract.
type Notifier interface {
	NotifyTaskReminder(ctx context.Context, userID uuid.UUID, to, name string, counts email.ReminderCounts)
	NotifyClearanceDue(ctx context.Context, fsoUserID uuid.UUID, to, name string, counts email.ClearanceReminderCounts)
	NotifyDd254Expiring(ctx context.Context, fsoUserID uuid.UUID, to, name string, counts email.Dd254ReminderCounts)
}

// Store reads the data needed to populate ReminderCounts.
type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store { return &Store{db: db} }

// ListRecipients returns ICs eligible for an immediate reminder. Users on
// daily_summary are excluded — their digest already carries overdue counts.
// Suspended identities are excluded so disabled accounts don't keep
// receiving mail.
func (s *Store) ListRecipients(ctx context.Context) ([]Recipient, error) {
	rows, err := s.db.Query(ctx, "reminders.ListRecipients",
		`SELECT u.id, i.email, i.name
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 LEFT JOIN user_settings us ON us.user_id = u.id
		 WHERE u.role = 'individual_contributor'
		   AND i.activated = TRUE
		   AND i.suspended_at IS NULL
		   AND i.email IS NOT NULL AND i.email <> ''
		   AND COALESCE(us.notification_frequency, 'every_task') <> 'daily_summary'`,
	)
	if err != nil {
		return nil, fmt.Errorf("reminders list recipients: %w", err)
	}
	defer rows.Close()

	var out []Recipient
	for rows.Next() {
		var r Recipient
		if err := rows.Scan(&r.UserID, &r.Email, &r.Name); err != nil {
			return nil, fmt.Errorf("scan recipient: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// CountForUser tallies the four reminder buckets for one IC. Errors
// propagate to the caller — the Run loop decides whether to skip or abort.
// $2 needs an explicit ::int cast: pgx binds untyped integer literals as
// `unknown`, and CURRENT_DATE + unknown is ambiguous in postgres.
func (s *Store) CountForUser(ctx context.Context, userID uuid.UUID) (email.ReminderCounts, error) {
	var c email.ReminderCounts
	err := s.db.QueryRow(ctx, "reminders.CountTasks",
		`SELECT
		    (SELECT COUNT(*) FROM task_user_status tus
		      JOIN tasks t ON t.id = tus.task_id
		      WHERE tus.user_id = $1
		        AND t.status = 'active'
		        AND tus.status IN ('to_do', 'in_progress', 'rejected')
		        AND t.due_date < CURRENT_DATE),
		    (SELECT COUNT(*) FROM task_user_status tus
		      JOIN tasks t ON t.id = tus.task_id
		      WHERE tus.user_id = $1
		        AND t.status = 'active'
		        AND tus.status IN ('to_do', 'in_progress', 'rejected')
		        AND t.due_date >= CURRENT_DATE
		        AND t.due_date <= CURRENT_DATE + $2::int)`,
		userID, DueSoonWindowDays,
	).Scan(&c.OverdueTasks, &c.DueSoonTasks)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("count tasks for reminders: %w", err)
	}

	err = s.db.QueryRow(ctx, "reminders.CountDebriefs",
		`SELECT
		    (SELECT COUNT(*) FROM travel_debriefs
		      WHERE user_id = $1 AND status = 'pending' AND due_date < CURRENT_DATE),
		    (SELECT COUNT(*) FROM travel_debriefs
		      WHERE user_id = $1 AND status = 'pending'
		        AND due_date >= CURRENT_DATE AND due_date <= CURRENT_DATE + $2::int)`,
		userID, DueSoonWindowDays,
	).Scan(&c.OverdueDebriefs, &c.DueSoonDebriefs)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("count debriefs for reminders: %w", err)
	}
	return c, nil
}

// Service runs one reminder pass.
type Service struct {
	store    *Store
	notifier Notifier
	logger   *slog.Logger
}

func New(store *Store, notifier Notifier, logger *slog.Logger) *Service {
	return &Service{store: store, notifier: notifier, logger: logger}
}

// Run scans every eligible IC, computes counts, and sends an email when
// there's something to nudge about. The notifier silently no-ops on empty
// counts and on daily_summary users (defense in depth — the SQL filter
// already excludes them). After IC reminders, it also dispatches FSO-facing
// clearance and DD254 reminders.
func (s *Service) Run(ctx context.Context) error {
	recipients, err := s.store.ListRecipients(ctx)
	if err != nil {
		return err
	}

	var sent, skipped, failed int
	for _, r := range recipients {
		counts, err := s.store.CountForUser(ctx, r.UserID)
		if err != nil {
			failed++
			s.logger.WarnContext(ctx, "reminder count failed", "user_id", r.UserID, "error", err)
			continue
		}
		if !counts.HasContent() {
			skipped++
			continue
		}
		s.notifier.NotifyTaskReminder(ctx, r.UserID, r.Email, r.Name, counts)
		sent++
	}

	s.logger.InfoContext(ctx, "reminder run complete",
		"recipients", len(recipients),
		"sent", sent,
		"skipped_empty", skipped,
		"failed", failed,
	)

	if err := s.runClearance(ctx); err != nil {
		s.logger.WarnContext(ctx, "clearance reminder pass failed", "error", err)
	}
	if err := s.runDd254(ctx); err != nil {
		s.logger.WarnContext(ctx, "dd254 reminder pass failed", "error", err)
	}
	return nil
}

// runClearance dispatches per-FSO clearance reminders and bumps
// last_due_reminder_at so the same record doesn't re-fire on the next pass.
func (s *Service) runClearance(ctx context.Context) error {
	recipients, err := s.store.ListClearanceReminders(ctx)
	if err != nil {
		return err
	}
	var sent int
	for _, r := range recipients {
		s.notifier.NotifyClearanceDue(ctx, r.FSOUserID, r.Email, r.Name, r.Counts)
		if err := s.store.MarkClearanceReminded(ctx, r.RecordIDs); err != nil {
			s.logger.WarnContext(ctx, "mark clearance reminded", "error", err, "fso_user_id", r.FSOUserID)
		}
		sent++
	}
	s.logger.InfoContext(ctx, "clearance reminder pass complete", "fsos_notified", sent)
	return nil
}

// runDd254 dispatches per-FSO DD254 expiration reminders.
func (s *Service) runDd254(ctx context.Context) error {
	recipients, err := s.store.ListDd254Reminders(ctx)
	if err != nil {
		return err
	}
	var sent int
	for _, r := range recipients {
		s.notifier.NotifyDd254Expiring(ctx, r.FSOUserID, r.Email, r.Name, r.Counts)
		if err := s.store.MarkDd254Reminded(ctx, r.FormIDs); err != nil {
			s.logger.WarnContext(ctx, "mark dd254 reminded", "error", err, "fso_user_id", r.FSOUserID)
		}
		sent++
	}
	s.logger.InfoContext(ctx, "dd254 reminder pass complete", "fsos_notified", sent)
	return nil
}
