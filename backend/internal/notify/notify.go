package notify

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/email"
)

// FrequencyChecker returns a user's notification preference.
type FrequencyChecker interface {
	NotificationFrequency(ctx context.Context, userID uuid.UUID) (string, error)
}

// Notifier sends fire-and-forget transactional notification emails. Recipients
// who have opted into a daily summary are skipped here — the daily-digest cron
// picks those events up from the underlying tables.
type Notifier struct {
	email    *email.Service
	settings FrequencyChecker
	logger   *slog.Logger
}

func New(svc *email.Service, settings FrequencyChecker, logger *slog.Logger) *Notifier {
	return &Notifier{email: svc, settings: settings, logger: logger}
}

// shouldSendNow returns true when the user wants per-event emails, false when
// they've opted into the daily summary. An unknown user (uuid.Nil) defaults
// to immediate — admin fallback paths fire to whoever inherits the work.
func (n *Notifier) shouldSendNow(ctx context.Context, userID uuid.UUID) bool {
	if userID == uuid.Nil || n.settings == nil {
		return true
	}
	freq, err := n.settings.NotificationFrequency(ctx, userID)
	if err != nil {
		// On lookup failure, send rather than silently drop the email.
		n.logger.WarnContext(ctx, "lookup notification frequency", "error", err, "user_id", userID)
		return true
	}
	return freq != auth.FrequencyDailySummary
}

// NotifyActionItemAssigned sends an email to an FSO when they are assigned an
// action item. Body identifies only the source type ("visit_request",
// "travel_report", etc.) — never the title or any subject info.
func (n *Notifier) NotifyActionItemAssigned(ctx context.Context, userID uuid.UUID, to, name, sourceType string) {
	if to == "" {
		return
	}
	if !n.shouldSendNow(ctx, userID) {
		return
	}
	go func() {
		if err := n.email.SendActionItemAssigned(context.Background(), to, name, sourceType); err != nil {
			n.logger.WarnContext(ctx, "send action item assigned notification", "error", err, "to", to)
		}
	}()
}

// NotifyTaskAssigned sends an email to an IC when a new task is assigned to
// them. Body says only "a new task has been assigned to you" — no title.
func (n *Notifier) NotifyTaskAssigned(ctx context.Context, userID uuid.UUID, to, name string) {
	if to == "" {
		return
	}
	if !n.shouldSendNow(ctx, userID) {
		return
	}
	go func() {
		if err := n.email.SendTaskAssigned(context.Background(), to, name); err != nil {
			n.logger.WarnContext(ctx, "send task assigned notification", "error", err, "to", to)
		}
	}()
}

// NotifyDebriefAssigned sends an email to an IC when the debrief cron has
// auto-created a post-travel debrief for them. Body says only that they have
// a debrief to complete — no trip name or destination. The debrief ID is
// passed through to the email template so the CTA can deep-link to the
// specific debrief.
func (n *Notifier) NotifyDebriefAssigned(ctx context.Context, userID uuid.UUID, to, name, debriefID string) {
	if to == "" {
		return
	}
	if !n.shouldSendNow(ctx, userID) {
		return
	}
	go func() {
		if err := n.email.SendDebriefAssigned(context.Background(), to, name, debriefID); err != nil {
			n.logger.WarnContext(ctx, "send debrief assigned notification", "error", err, "to", to)
		}
	}()
}

// NotifyTaskReminder sends a periodic nudge to an IC who has open and/or
// overdue tasks (and/or debriefs). Body is counts-only — no titles. Same
// frequency gate as the other notifiers; digest users skip this and get the
// counts in their daily summary instead.
func (n *Notifier) NotifyTaskReminder(ctx context.Context, userID uuid.UUID, to, name string, counts email.ReminderCounts) {
	if to == "" {
		return
	}
	if !counts.HasContent() {
		return
	}
	if !n.shouldSendNow(ctx, userID) {
		return
	}
	go func() {
		if err := n.email.SendTaskReminder(context.Background(), to, name, counts); err != nil {
			n.logger.WarnContext(ctx, "send task reminder", "error", err, "to", to)
		}
	}()
}

// NotifyClearanceDue sends a periodic nudge to an FSO when cleared
// personnel they oversee have investigations coming due. Aggregated counts
// only — no names. Same frequency gate as the other notifiers.
func (n *Notifier) NotifyClearanceDue(ctx context.Context, fsoUserID uuid.UUID, to, name string, counts email.ClearanceReminderCounts) {
	if to == "" {
		return
	}
	if !counts.HasContent() {
		return
	}
	if !n.shouldSendNow(ctx, fsoUserID) {
		return
	}
	go func() {
		if err := n.email.SendClearanceReminder(context.Background(), to, name, counts); err != nil {
			n.logger.WarnContext(ctx, "send clearance reminder", "error", err, "to", to)
		}
	}()
}

// NotifyDd254Expiring sends a periodic nudge to an FSO when DD254 forms in
// their sub-orgs are approaching the end of their period of performance.
func (n *Notifier) NotifyDd254Expiring(ctx context.Context, fsoUserID uuid.UUID, to, name string, counts email.Dd254ReminderCounts) {
	if to == "" {
		return
	}
	if !counts.HasContent() {
		return
	}
	if !n.shouldSendNow(ctx, fsoUserID) {
		return
	}
	go func() {
		if err := n.email.SendDd254Reminder(context.Background(), to, name, counts); err != nil {
			n.logger.WarnContext(ctx, "send dd254 reminder", "error", err, "to", to)
		}
	}()
}

// NotifyTaskReviewed sends an email to an IC when their task submission is
// approved or rejected. Body identifies the decision only — no task title.
func (n *Notifier) NotifyTaskReviewed(ctx context.Context, userID uuid.UUID, to, name, decision string) {
	if to == "" {
		return
	}
	if !n.shouldSendNow(ctx, userID) {
		return
	}
	go func() {
		if err := n.email.SendTaskSubmissionReviewed(context.Background(), to, name, decision); err != nil {
			n.logger.WarnContext(ctx, "send task submission reviewed notification", "error", err, "to", to)
		}
	}()
}
