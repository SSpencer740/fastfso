package logging

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"

	"github.com/fastfso/fastfso/backend/internal/env"
)

// Setup creates the root logger based on the APP_ENV environment variable.
// In production it writes JSON to stdout with GCP Cloud Logging field names;
// otherwise it writes colored text to stderr for local development.
// The returned logger is also set as the slog default.
func Setup() *slog.Logger {
	var handler slog.Handler
	if env.IsCloud() {
		handler = newGCPHandler()
	} else {
		handler = newDevHandler()
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// Component returns a sub-logger tagged with the given component name.
func Component(root *slog.Logger, name string) *slog.Logger {
	return root.With("component", name)
}

func newDevHandler() slog.Handler {
	return tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	})
}

func newGCPHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
				a.Value = slog.StringValue(gcpSeverity(a.Value.Any().(slog.Level)))
			case slog.MessageKey:
				a.Key = "message"
			}
			return a
		},
	})
}

func gcpSeverity(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARNING"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}
