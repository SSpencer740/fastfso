// Package digest builds and sends the daily summary email for users who have
// opted into the daily_summary notification frequency.
//
// Privacy posture: digest emails contain counts only — no titles, no subject
// names, no tenant identifiers. Users must log in to fastFSO to see what the
// items are.
package digest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/email"
)

// Recipient is one user who is due for a digest.
type Recipient struct {
	UserID uuid.UUID
	Email  string
	Name   string
	Role   string
	// Since is the cutoff timestamp for the window of "new" events. NULL on the
	// first-ever run is handled by the SQL — see eligibleSinceFallback.
}

// Counts mirrors email.DigestCounts.
type Counts struct {
	// New since last digest.
	NewActionItems   int
	NewTasksAssigned int
	NewTaskReviews   int
	NewDebriefs      int
	// Standing buckets — open work the recipient is sitting on regardless of
	// whether anything's "new". Without these, a digest user who's drifted
	// away from the app gets no emails on quiet days and never gets nudged
	// back in.
	OpenTasks    int
	OpenDebriefs int
	OverdueTasks int
}

// Sender abstracts the email service for testing.
type Sender interface {
	SendDailySummary(ctx context.Context, to, name string, counts email.DigestCounts) error
}

// Store reads digest-related data and updates last_digest_sent_at.
type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store { return &Store{db: db} }

// ListEligibleRecipients returns users whose notification_frequency is
// daily_summary AND whose last_digest_sent_at is NULL or older than ~23h.
// (We use 23h instead of 24h to give the cron a small buffer against drift.)
// Deactivated identities are excluded — disabled accounts shouldn't receive
// any new email even if they were previously opted in.
func (s *Store) ListEligibleRecipients(ctx context.Context) ([]Recipient, error) {
	rows, err := s.db.Query(ctx, "digest.ListEligibleRecipients",
		`SELECT u.id, i.email, i.name, u.role
		 FROM user_settings us
		 JOIN users u ON u.id = us.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE us.notification_frequency = 'daily_summary'
		   AND (us.last_digest_sent_at IS NULL
		        OR us.last_digest_sent_at < NOW() - INTERVAL '23 hours')
		   AND i.activated = TRUE
		   AND i.email IS NOT NULL
		   AND i.email <> ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("digest list recipients: %w", err)
	}
	defer rows.Close()

	var out []Recipient
	for rows.Next() {
		var r Recipient
		if err := rows.Scan(&r.UserID, &r.Email, &r.Name, &r.Role); err != nil {
			return nil, fmt.Errorf("scan recipient: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// CountForUser tallies events for one recipient since their last digest. The
// counts are role-aware: FSOs get pending action items assigned to them; ICs
// get newly-assigned tasks plus reviews of their submissions.
func (s *Store) CountForUser(ctx context.Context, r Recipient) (Counts, error) {
	var c Counts
	// Coalesce last_digest_sent_at to now() - 24h on the first run so the user
	// receives a sensible first digest rather than every event since their
	// account was created.
	switch r.Role {
	case "fso", "read_only_fso":
		// FSOs and read-only FSOs only see action items explicitly assigned
		// to them. Unassigned items fall back to admins (see administrator
		// branch and ResolveNotifyTargets in the actionitem store).
		err := s.db.QueryRow(ctx, "digest.CountForFSO",
			`SELECT COUNT(*)
			 FROM action_items ai
			 JOIN user_settings us ON us.user_id = $1
			 WHERE ai.assigned_to = $1
			   AND ai.status = 'pending'
			   AND ai.created_at > COALESCE(us.last_digest_sent_at, NOW() - INTERVAL '24 hours')`,
			r.UserID,
		).Scan(&c.NewActionItems)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count fso action items: %w", err)
		}
	case "administrator":
		// Administrators see action items assigned to them plus any
		// unassigned items in their tenant — mirroring the immediate-email
		// fallback in actionitem.Store.ResolveNotifyTargets.
		err := s.db.QueryRow(ctx, "digest.CountForAdmin",
			`SELECT COUNT(*)
			 FROM action_items ai
			 JOIN user_settings us ON us.user_id = $1
			 JOIN users u ON u.id = $1
			 WHERE (ai.assigned_to = $1 OR (ai.assigned_to IS NULL AND ai.tenant_id = u.tenant_id))
			   AND ai.status = 'pending'
			   AND ai.created_at > COALESCE(us.last_digest_sent_at, NOW() - INTERVAL '24 hours')`,
			r.UserID,
		).Scan(&c.NewActionItems)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count admin action items: %w", err)
		}
	case "individual_contributor":
		err := s.db.QueryRow(ctx, "digest.CountForIC",
			`SELECT COUNT(DISTINCT t.id)
			 FROM tasks t
			 JOIN task_assigned_users tau ON tau.task_id = t.id
			 JOIN user_settings us ON us.user_id = tau.user_id
			 WHERE tau.user_id = $1
			   AND t.status = 'active'
			   AND t.created_at > COALESCE(us.last_digest_sent_at, NOW() - INTERVAL '24 hours')`,
			r.UserID,
		).Scan(&c.NewTasksAssigned)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic tasks: %w", err)
		}
		err = s.db.QueryRow(ctx, "digest.CountForICReviews",
			`SELECT COUNT(*)
			 FROM task_completions tc
			 JOIN user_settings us ON us.user_id = $1
			 WHERE tc.user_id = $1
			   AND tc.reviewed_at IS NOT NULL
			   AND tc.reviewed_at > COALESCE(us.last_digest_sent_at, NOW() - INTERVAL '24 hours')`,
			r.UserID,
		).Scan(&c.NewTaskReviews)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic reviews: %w", err)
		}
		// Newly-assigned debriefs since last digest.
		err = s.db.QueryRow(ctx, "digest.CountForICNewDebriefs",
			`SELECT COUNT(*)
			 FROM travel_debriefs td
			 JOIN user_settings us ON us.user_id = $1
			 WHERE td.user_id = $1
			   AND td.status = 'pending'
			   AND td.created_at > COALESCE(us.last_digest_sent_at, NOW() - INTERVAL '24 hours')`,
			r.UserID,
		).Scan(&c.NewDebriefs)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic new debriefs: %w", err)
		}
		// Open tasks (any age) — surfacing these prevents silent drift.
		err = s.db.QueryRow(ctx, "digest.CountForICOpenTasks",
			`SELECT COUNT(*)
			 FROM task_user_status tus
			 JOIN tasks t ON t.id = tus.task_id
			 WHERE tus.user_id = $1
			   AND t.status = 'active'
			   AND tus.status IN ('to_do', 'in_progress', 'rejected')`,
			r.UserID,
		).Scan(&c.OpenTasks)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic open tasks: %w", err)
		}
		// Open debriefs (any age).
		err = s.db.QueryRow(ctx, "digest.CountForICOpenDebriefs",
			`SELECT COUNT(*)
			 FROM travel_debriefs
			 WHERE user_id = $1 AND status = 'pending'`,
			r.UserID,
		).Scan(&c.OpenDebriefs)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic open debriefs: %w", err)
		}
		// Overdue tasks — past due_date, still incomplete.
		err = s.db.QueryRow(ctx, "digest.CountForICOverdueTasks",
			`SELECT COUNT(*)
			 FROM task_user_status tus
			 JOIN tasks t ON t.id = tus.task_id
			 WHERE tus.user_id = $1
			   AND t.status = 'active'
			   AND tus.status IN ('to_do', 'in_progress', 'rejected')
			   AND t.due_date IS NOT NULL
			   AND t.due_date < CURRENT_DATE`,
			r.UserID,
		).Scan(&c.OverdueTasks)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, fmt.Errorf("count ic overdue tasks: %w", err)
		}
	}
	return c, nil
}

// MarkSent updates last_digest_sent_at to NOW() so the next cron run uses a
// fresh window. Called for every eligible user — including those who had no
// new events to bundle — so the window stays bounded to the cron cadence.
func (s *Store) MarkSent(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "digest.MarkSent",
		`UPDATE user_settings SET last_digest_sent_at = NOW(), updated_at = NOW()
		 WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mark digest sent: %w", err)
	}
	return nil
}

// Service runs the daily digest pass.
type Service struct {
	store  *Store
	sender Sender
	logger *slog.Logger
}

// New constructs a digest Service.
func New(store *Store, sender Sender, logger *slog.Logger) *Service {
	return &Service{store: store, sender: sender, logger: logger}
}

// Run executes one digest pass: finds eligible recipients, counts events, sends
// emails for those with content, and advances last_digest_sent_at for all.
// Errors on individual recipients are logged but do not abort the run.
func (s *Service) Run(ctx context.Context) error {
	recipients, err := s.store.ListEligibleRecipients(ctx)
	if err != nil {
		return err
	}

	var sent, skipped, failed int
	for _, r := range recipients {
		counts, err := s.store.CountForUser(ctx, r)
		if err != nil {
			failed++
			s.logger.WarnContext(ctx, "digest count failed", "user_id", r.UserID, "error", err)
			continue
		}
		ec := email.DigestCounts{
			NewActionItems:   counts.NewActionItems,
			NewTasksAssigned: counts.NewTasksAssigned,
			NewTaskReviews:   counts.NewTaskReviews,
			NewDebriefs:      counts.NewDebriefs,
			OpenTasks:        counts.OpenTasks,
			OpenDebriefs:     counts.OpenDebriefs,
			OverdueTasks:     counts.OverdueTasks,
		}
		if ec.HasContent() {
			if err := s.sender.SendDailySummary(ctx, r.Email, r.Name, ec); err != nil {
				failed++
				s.logger.WarnContext(ctx, "digest send failed", "user_id", r.UserID, "error", err)
				continue
			}
			sent++
		} else {
			skipped++
		}
		if err := s.store.MarkSent(ctx, r.UserID); err != nil {
			s.logger.WarnContext(ctx, "digest mark-sent failed", "user_id", r.UserID, "error", err)
		}
	}

	s.logger.InfoContext(ctx, "digest run complete",
		"recipients", len(recipients),
		"sent", sent,
		"skipped_empty", skipped,
		"failed", failed,
	)
	return nil
}
