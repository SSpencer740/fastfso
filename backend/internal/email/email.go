package email

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Message is a fully-rendered email ready for delivery.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender is the transport interface. Implementations: SMTP, SendGrid, cloud queue, etc.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Service owns templates and a Sender. Handlers call typed methods.
type Service struct {
	sender Sender
	tmpl   *templateSet
	appURL string
	logger *slog.Logger
}

// New creates an email Service with the given sender, frontend app URL, and
// logger. The appURL is rendered into templates as the destination for CTA
// buttons and "manage preferences" links.
func New(sender Sender, appURL string, logger *slog.Logger) *Service {
	tmpl, err := loadTemplates()
	if err != nil {
		// Templates are embedded at compile time; failure is a programming error.
		panic(fmt.Sprintf("email: load templates: %v", err))
	}
	return &Service{
		sender: sender,
		tmpl:   tmpl,
		appURL: appURL,
		logger: logger,
	}
}

// SendEmailCode sends a 2FA verification code email.
func (s *Service) SendEmailCode(ctx context.Context, to, code string) error {
	html, text, err := s.tmpl.render("email_code", map[string]string{
		"Code": code,
	})
	if err != nil {
		return fmt.Errorf("render email_code template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "Your fastFSO verification code",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending email code", "to", to)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send email code: %w", err)
	}
	return nil
}

// SendInviteSSO notifies a user that they've been added to a tenant whose
// authentication is handled by their company's SSO. They don't need a
// password — the CTA goes straight to /login where the email-domain match
// will route them to the IdP. The pre-existing identity (already created
// during the admin invite) gets matched by the SSO callback on return.
func (s *Service) SendInviteSSO(ctx context.Context, to, tenantName string) error {
	html, text, err := s.tmpl.render("invite_sso", map[string]string{
		"TenantName": tenantName,
		"AppURL":     s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render invite_sso template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "You've been added to " + tenantName + " on fastFSO",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending sso invite", "to", to, "tenant", tenantName)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send sso invite: %w", err)
	}
	return nil
}

// SendAdminInvite sends an invitation email to a new admin.
func (s *Service) SendAdminInvite(ctx context.Context, to, tenantName, appURL string) error {
	html, text, err := s.tmpl.render("invite_admin", map[string]string{
		"TenantName": tenantName,
		"AppURL":     appURL,
	})
	if err != nil {
		return fmt.Errorf("render invite_admin template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "You've been invited to " + tenantName + " on fastFSO",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending admin invite", "to", to, "tenant", tenantName)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send admin invite: %w", err)
	}
	return nil
}

// SourceTypeLabel maps an action item source_type to a short human-readable
// label safe for inclusion in emails. The label is generic — never includes a
// subject name, trip name, or any tenant-specific content.
func SourceTypeLabel(sourceType string) string {
	switch sourceType {
	case "visit_request":
		return "visit request"
	case "travel_report":
		return "travel report"
	case "travel_debrief":
		return "travel debrief"
	case "task_submission":
		return "task submission"
	case "report":
		return "life event report"
	default:
		return "item"
	}
}

// SendActionItemAssigned notifies an FSO that a new action item has been assigned
// to them. Body identifies only the source-type label — never the action item
// title or any subject info — so a leaked email reveals nothing about whom or
// what the item is about.
func (s *Service) SendActionItemAssigned(ctx context.Context, to, name, sourceType string) error {
	label := SourceTypeLabel(sourceType)
	html, text, err := s.tmpl.render("action_item_assigned", map[string]string{
		"Name":   name,
		"Label":  label,
		"AppURL": s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render action_item_assigned template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "fastFSO: new " + label + " to review",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending action item assigned notification", "to", to, "source_type", sourceType)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send action item assigned: %w", err)
	}
	return nil
}

// SendDebriefAssigned notifies an IC that the debrief cron auto-created a
// post-travel debrief for them. Body and subject contain no trip name or
// destination. The debrief ID is included in the deep-link URL only — the
// raw UUID has no semantic value to anyone without an authenticated session.
func (s *Service) SendDebriefAssigned(ctx context.Context, to, name, debriefID string) error {
	html, text, err := s.tmpl.render("debrief_assigned", map[string]string{
		"Name":      name,
		"AppURL":    s.appURL,
		"DebriefID": debriefID,
	})
	if err != nil {
		return fmt.Errorf("render debrief_assigned template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "fastFSO: post-travel debrief ready for you",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending debrief assigned notification", "to", to)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send debrief assigned: %w", err)
	}
	return nil
}

// SendTaskReminder nudges an IC about overdue and/or due-soon tasks and
// debriefs. Body is counts-only.
func (s *Service) SendTaskReminder(ctx context.Context, to, name string, counts ReminderCounts) error {
	html, text, err := s.tmpl.render("task_reminder", map[string]any{
		"Name":            name,
		"OverdueTasks":    counts.OverdueTasks,
		"DueSoonTasks":    counts.DueSoonTasks,
		"OverdueDebriefs": counts.OverdueDebriefs,
		"DueSoonDebriefs": counts.DueSoonDebriefs,
		"AppURL":          s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render task_reminder template: %w", err)
	}

	subject := "fastFSO: items waiting on you"
	if counts.OverdueTasks > 0 || counts.OverdueDebriefs > 0 {
		subject = "fastFSO: you have overdue items"
	}

	msg := Message{
		To:      to,
		Subject: subject,
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending task reminder",
		"to", to,
		"overdue_tasks", counts.OverdueTasks,
		"due_soon_tasks", counts.DueSoonTasks,
		"overdue_debriefs", counts.OverdueDebriefs,
		"due_soon_debriefs", counts.DueSoonDebriefs,
	)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send task reminder: %w", err)
	}
	return nil
}

// SendClearanceReminder nudges an FSO about cleared personnel whose
// investigations are coming due. Body is counts-only — no names.
func (s *Service) SendClearanceReminder(ctx context.Context, to, name string, counts ClearanceReminderCounts) error {
	html, text, err := s.tmpl.render("clearance_reminder", map[string]any{
		"Name":      name,
		"Due90Days": counts.Due90Days,
		"Due30Days": counts.Due30Days,
		"Overdue":   counts.Overdue,
		"AppURL":    s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render clearance_reminder template: %w", err)
	}

	subject := "fastFSO: clearance investigations coming due"
	if counts.Overdue > 0 {
		subject = "fastFSO: overdue clearance investigations"
	}

	msg := Message{To: to, Subject: subject, HTML: html, Text: text}
	s.logger.InfoContext(ctx, "sending clearance reminder",
		"to", to, "due_90", counts.Due90Days, "due_30", counts.Due30Days, "overdue", counts.Overdue,
	)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send clearance reminder: %w", err)
	}
	return nil
}

// SendDd254Reminder nudges an FSO about DD254 forms whose period of
// performance is ending. Body is counts-only — no contract numbers.
func (s *Service) SendDd254Reminder(ctx context.Context, to, name string, counts Dd254ReminderCounts) error {
	html, text, err := s.tmpl.render("dd254_reminder", map[string]any{
		"Name":      name,
		"Due90Days": counts.Due90Days,
		"Due30Days": counts.Due30Days,
		"Overdue":   counts.Overdue,
		"AppURL":    s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render dd254_reminder template: %w", err)
	}

	subject := "fastFSO: DD254 forms expiring"
	if counts.Overdue > 0 {
		subject = "fastFSO: expired DD254 forms"
	}

	msg := Message{To: to, Subject: subject, HTML: html, Text: text}
	s.logger.InfoContext(ctx, "sending dd254 reminder",
		"to", to, "due_90", counts.Due90Days, "due_30", counts.Due30Days, "overdue", counts.Overdue,
	)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send dd254 reminder: %w", err)
	}
	return nil
}

// SendTaskAssigned notifies an IC that a new task has been assigned to them.
// Body and subject contain no task title or other identifying detail.
func (s *Service) SendTaskAssigned(ctx context.Context, to, name string) error {
	html, text, err := s.tmpl.render("task_assigned", map[string]string{
		"Name":   name,
		"AppURL": s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render task_assigned template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "fastFSO: a new task has been assigned to you",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending task assigned notification", "to", to)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send task assigned: %w", err)
	}
	return nil
}

// SendTaskSubmissionReviewed notifies an IC that their task submission was
// approved or rejected. Body identifies only the decision — no task title.
func (s *Service) SendTaskSubmissionReviewed(ctx context.Context, to, name, decision string) error {
	html, text, err := s.tmpl.render("task_submission_reviewed", map[string]string{
		"Name":     name,
		"Decision": decision,
		"AppURL":   s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render task_submission_reviewed template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "fastFSO: your task submission was " + decision,
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending task submission reviewed notification", "to", to, "decision", decision)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send task submission reviewed: %w", err)
	}
	return nil
}

// DigestCounts is the body content for the daily summary email. All fields are
// counts only — no titles, no names, no tenant identifiers. The recipient must
// log in to fastFSO to see what the items actually are.
type DigestCounts struct {
	// New since last digest
	NewActionItems   int
	NewTasksAssigned int
	NewTaskReviews   int
	NewDebriefs      int
	// Standing buckets — open work the recipient is sitting on regardless of
	// whether anything is "new". Without these, a digest user who's drifted
	// away from the app gets no emails on quiet days and never gets nudged
	// back.
	OpenTasks    int
	OpenDebriefs int
	OverdueTasks int
}

// HasContent reports whether there is anything worth sending.
func (d DigestCounts) HasContent() bool {
	return d.NewActionItems > 0 || d.NewTasksAssigned > 0 || d.NewTaskReviews > 0 ||
		d.NewDebriefs > 0 || d.OpenTasks > 0 || d.OpenDebriefs > 0 || d.OverdueTasks > 0
}

// ReminderCounts is the body content for the standalone task-reminder email
// that goes to every_task users. Same privacy posture as DigestCounts:
// counts only, no titles.
type ReminderCounts struct {
	OverdueTasks    int
	DueSoonTasks    int
	OverdueDebriefs int
	DueSoonDebriefs int
}

// ClearanceReminderCounts is the body content for clearance investigation
// reminders sent to FSOs. Aggregates how many users they oversee have
// investigations coming due within each bucket.
type ClearanceReminderCounts struct {
	Due90Days int
	Due30Days int
	Overdue   int
}

// HasContent reports whether the clearance reminder is worth sending.
func (c ClearanceReminderCounts) HasContent() bool {
	return c.Due90Days > 0 || c.Due30Days > 0 || c.Overdue > 0
}

// Dd254ReminderCounts is the body content for DD254 expiration reminders.
type Dd254ReminderCounts struct {
	Due90Days int
	Due30Days int
	Overdue   int
}

// HasContent reports whether the DD254 reminder is worth sending.
func (c Dd254ReminderCounts) HasContent() bool {
	return c.Due90Days > 0 || c.Due30Days > 0 || c.Overdue > 0
}

// HasContent reports whether the reminder is worth sending.
func (r ReminderCounts) HasContent() bool {
	return r.OverdueTasks > 0 || r.DueSoonTasks > 0 || r.OverdueDebriefs > 0 || r.DueSoonDebriefs > 0
}

// SendDailySummary sends the daily digest email. Body is counts-only — no
// titles, names, or other tenant-specific content. Includes a CTA link to the
// app and a "manage preferences" link to /settings.
func (s *Service) SendDailySummary(ctx context.Context, to, name string, counts DigestCounts) error {
	today := time.Now().UTC().Format("January 2, 2006")
	html, text, err := s.tmpl.render("daily_summary", map[string]any{
		"Name":             name,
		"Date":             today,
		"NewActionItems":   counts.NewActionItems,
		"NewTasksAssigned": counts.NewTasksAssigned,
		"NewTaskReviews":   counts.NewTaskReviews,
		"NewDebriefs":      counts.NewDebriefs,
		"OpenTasks":        counts.OpenTasks,
		"OpenDebriefs":     counts.OpenDebriefs,
		"OverdueTasks":     counts.OverdueTasks,
		"AppURL":           s.appURL,
	})
	if err != nil {
		return fmt.Errorf("render daily_summary template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "fastFSO: your daily summary for " + today,
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending daily summary",
		"to", to,
		"action_items", counts.NewActionItems,
		"tasks_assigned", counts.NewTasksAssigned,
		"task_reviews", counts.NewTaskReviews,
	)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send daily summary: %w", err)
	}
	return nil
}

// SendPasswordReset sends a password reset link to the user.
func (s *Service) SendPasswordReset(ctx context.Context, to, appURL, token string) error {
	resetURL := appURL + "/reset-password?token=" + token
	html, text, err := s.tmpl.render("password_reset", map[string]string{
		"ResetURL": resetURL,
	})
	if err != nil {
		return fmt.Errorf("render password_reset template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "Reset your fastFSO password",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending password reset email", "to", to)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send password reset: %w", err)
	}
	return nil
}

// SendAdminInviteSetup sends a setup email to a new admin who needs to set their password.
func (s *Service) SendAdminInviteSetup(ctx context.Context, to, tenantName, appURL, token string) error {
	setupURL := appURL + "/accept-invite?token=" + token
	html, text, err := s.tmpl.render("invite_setup", map[string]string{
		"TenantName": tenantName,
		"SetupURL":   setupURL,
	})
	if err != nil {
		return fmt.Errorf("render invite_setup template: %w", err)
	}

	msg := Message{
		To:      to,
		Subject: "You've been invited to " + tenantName + " on fastFSO",
		HTML:    html,
		Text:    text,
	}

	s.logger.InfoContext(ctx, "sending admin invite setup", "to", to, "tenant", tenantName, "setup_url", setupURL)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send admin invite setup: %w", err)
	}
	return nil
}
