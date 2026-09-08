package email

import (
	"context"
	"log/slog"
)

// LogSender logs emails instead of sending them. Used in development.
type LogSender struct {
	logger *slog.Logger
}

// NewLogSender creates a LogSender.
func NewLogSender(logger *slog.Logger) *LogSender {
	return &LogSender{logger: logger}
}

// Send logs the email details.
func (s *LogSender) Send(ctx context.Context, msg Message) error {
	s.logger.WarnContext(ctx, "email not sent (log-only sender)",
		"to", msg.To,
		"subject", msg.Subject,
	)
	s.logger.DebugContext(ctx, "email body", "html", msg.HTML, "text", msg.Text)
	return nil
}
