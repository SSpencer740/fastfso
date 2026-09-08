package email

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSender struct {
	messages []Message
}

func (m *mockSender) Send(_ context.Context, msg Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func TestSendEmailCode(t *testing.T) {
	sender := &mockSender{}
	svc := New(sender, "https://app.example.com", slog.Default())

	err := svc.SendEmailCode(context.Background(), "test@example.com", "123456")
	require.NoError(t, err)

	require.Len(t, sender.messages, 1)
	msg := sender.messages[0]
	assert.Equal(t, "test@example.com", msg.To)
	assert.Equal(t, "Your fastFSO verification code", msg.Subject)
	assert.Contains(t, msg.HTML, "123456")
	assert.Contains(t, msg.Text, "123456")
}

// TestNotificationEmailsHaveNoSensitiveContent asserts the privacy posture:
// titles, subject names, trip identifiers, and any other tenant-specific
// content must never appear in notification emails. Anyone who intercepts a
// notification should learn at most "user X has fastFSO activity" — never the
// *what* or *who*.
func TestNotificationEmailsHaveNoSensitiveContent(t *testing.T) {
	sender := &mockSender{}
	svc := New(sender, "https://app.example.com", slog.Default())

	// Strings that must NOT appear in any notification email under any
	// circumstance — they are the kinds of values upstream callers had been
	// passing through prior to this work.
	leakers := []string{
		"John Smith",
		"jane.doe@example.com",
		"Berlin Conference 2026",
		"Travel Report: J. Smith — Iran",
		"Q3 SF-86 update",
		"Iran",
	}

	require.NoError(t, svc.SendActionItemAssigned(context.Background(), "fso@x.com", "FSO Name", "travel_report"))
	require.NoError(t, svc.SendTaskAssigned(context.Background(), "ic@x.com", "IC Name"))
	require.NoError(t, svc.SendTaskSubmissionReviewed(context.Background(), "ic@x.com", "IC Name", "approved"))
	require.NoError(t, svc.SendDebriefAssigned(context.Background(), "ic@x.com", "IC Name", "00000000-0000-0000-0000-000000000001"))
	require.NoError(t, svc.SendTaskReminder(context.Background(), "ic@x.com", "IC Name", ReminderCounts{
		OverdueTasks: 2, DueSoonTasks: 1, OverdueDebriefs: 1, DueSoonDebriefs: 1,
	}))
	require.NoError(t, svc.SendDailySummary(context.Background(), "user@x.com", "User Name", DigestCounts{
		NewActionItems: 3, NewTasksAssigned: 1, NewTaskReviews: 2, NewDebriefs: 1,
		OpenTasks: 4, OpenDebriefs: 1, OverdueTasks: 1,
	}))

	require.Len(t, sender.messages, 6)
	for i, msg := range sender.messages {
		for _, leaker := range leakers {
			assert.NotContains(t, msg.Subject, leaker, "msg[%d] subject must not include %q", i, leaker)
			assert.NotContains(t, msg.HTML, leaker, "msg[%d] html must not include %q", i, leaker)
			assert.NotContains(t, msg.Text, leaker, "msg[%d] text must not include %q", i, leaker)
		}
	}
}

// TestNotificationEmailsRenderCTALinks asserts every notification email
// includes a clickable link back to the app — without one, the user has to
// know the URL by heart.
func TestNotificationEmailsRenderCTALinks(t *testing.T) {
	sender := &mockSender{}
	const appURL = "https://app.example.com"
	svc := New(sender, appURL, slog.Default())

	require.NoError(t, svc.SendActionItemAssigned(context.Background(), "fso@x.com", "FSO", "travel_report"))
	require.NoError(t, svc.SendTaskAssigned(context.Background(), "ic@x.com", "IC"))
	require.NoError(t, svc.SendTaskSubmissionReviewed(context.Background(), "ic@x.com", "IC", "approved"))
	require.NoError(t, svc.SendDebriefAssigned(context.Background(), "ic@x.com", "IC", "00000000-0000-0000-0000-000000000001"))
	require.NoError(t, svc.SendTaskReminder(context.Background(), "ic@x.com", "IC", ReminderCounts{OverdueTasks: 1}))
	require.NoError(t, svc.SendDailySummary(context.Background(), "user@x.com", "User", DigestCounts{NewActionItems: 1}))

	require.Len(t, sender.messages, 6)
	// Every notification email has at least one link back to the app —
	// either the generic /login landing or a deep-link into a specific
	// resource. We check appURL is referenced rather than a specific path
	// so the assertion isn't brittle when individual templates evolve.
	for i, msg := range sender.messages {
		assert.Contains(t, msg.HTML, "href=\""+appURL, "msg[%d] missing CTA link to app", i)
	}
	// Debrief assigned (msg[3]) deep-links to the specific debrief.
	assert.Contains(t, sender.messages[3].HTML, "/app/travel?debrief=00000000-0000-0000-0000-000000000001", "debrief email should deep-link to the debrief")
	// Reminder (msg[4]) and daily summary (msg[5]) both expose the
	// manage-preferences link.
	assert.Contains(t, sender.messages[4].HTML, appURL+"/settings", "task reminder missing settings link")
	assert.Contains(t, sender.messages[5].HTML, appURL+"/settings", "daily summary missing settings link")
}

func TestSourceTypeLabelIsGeneric(t *testing.T) {
	for _, st := range []string{"visit_request", "travel_report", "travel_debrief", "task_submission", "report"} {
		label := SourceTypeLabel(st)
		assert.NotEmpty(t, label)
		assert.NotContains(t, label, "_") // human-readable, not a raw enum
	}
	// Unknown source types still get a non-empty fallback so emails never
	// render with a blank label.
	assert.NotEmpty(t, SourceTypeLabel("totally_made_up"))
}

func TestTemplateRendering(t *testing.T) {
	tmpl, err := loadTemplates()
	require.NoError(t, err)

	html, text, err := tmpl.render("email_code", map[string]string{
		"Code": "654321",
	})
	require.NoError(t, err)
	assert.Contains(t, html, "654321")
	assert.Contains(t, html, "fastFSO")
	assert.Contains(t, text, "654321")
	assert.NotContains(t, text, "<") // No HTML tags in plain text
}

func TestTemplateNotFound(t *testing.T) {
	tmpl, err := loadTemplates()
	require.NoError(t, err)

	_, _, err = tmpl.render("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHTMLToPlainText(t *testing.T) {
	html := "<p>Hello <strong>world</strong></p><br><p>Line two</p>"
	text := htmlToPlainText(html)
	assert.NotContains(t, text, "<")
	assert.Contains(t, text, "Hello")
	assert.Contains(t, text, "world")
	assert.Contains(t, text, "Line two")
}

func TestLogSender(t *testing.T) {
	sender := NewLogSender(slog.Default())
	err := sender.Send(context.Background(), Message{
		To:      "test@example.com",
		Subject: "Test",
		HTML:    "<p>Test</p>",
		Text:    "Test",
	})
	assert.NoError(t, err)
}
