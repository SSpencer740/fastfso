package email

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// SendGridSender sends emails via the SendGrid v3 Mail Send API.
type SendGridSender struct {
	client *sendgrid.Client
	from   string
	logger *slog.Logger
}

// NewSendGridSender creates a SendGridSender.
func NewSendGridSender(apiKey, from string, logger *slog.Logger) *SendGridSender {
	return &SendGridSender{
		client: sendgrid.NewSendClient(apiKey),
		from:   from,
		logger: logger,
	}
}

// Send delivers the message via SendGrid.
func (s *SendGridSender) Send(ctx context.Context, msg Message) error {
	from := mail.NewEmail("fastFSO", s.from)
	to := mail.NewEmail("", msg.To)
	m := mail.NewV3MailInit(from, msg.Subject, to)
	m.AddContent(mail.NewContent("text/plain", msg.Text))
	m.AddContent(mail.NewContent("text/html", msg.HTML))

	resp, err := s.client.Send(m)
	if err != nil {
		return fmt.Errorf("sendgrid request failed: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return &SendGridError{
			StatusCode: resp.StatusCode,
			Body:       resp.Body,
		}
	}

	s.logger.InfoContext(ctx, "email sent via sendgrid", "to", msg.To, "subject", msg.Subject, "status", resp.StatusCode)
	return nil
}

// SendGridError represents a non-success response from SendGrid.
type SendGridError struct {
	StatusCode int
	Body       string
}

func (e *SendGridError) Error() string {
	return fmt.Sprintf("sendgrid returned status %d: %s", e.StatusCode, e.Body)
}

// IsTransient returns true for rate-limiting or server errors that may succeed on retry.
func (e *SendGridError) IsTransient() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}
