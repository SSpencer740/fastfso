package verification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/env"
)

// LocalEnqueuer is a development-only Enqueuer implementation that runs the
// verification worker inline in a goroutine instead of via Cloud Tasks. This
// lets developers exercise the full upload → AI verification → admin-review
// flow on localhost without standing up a Cloud Tasks queue.
//
// SAFETY: LocalEnqueuer refuses to construct itself when DEPLOY_ENV=cloud
// (see NewLocalEnqueuer). The router also gates this on env.IsCloud(), but
// the defense-in-depth check here ensures it never silently runs in a cloud
// environment if someone misconfigures the wiring.
type LocalEnqueuer struct {
	worker *Worker
	logger *slog.Logger
}

// NewLocalEnqueuer returns a LocalEnqueuer for non-cloud environments.
// Returns an error if invoked in cloud mode to prevent silent fallback from
// the real Cloud Tasks enqueuer.
func NewLocalEnqueuer(worker *Worker, logger *slog.Logger) (*LocalEnqueuer, error) {
	if env.IsCloud() {
		return nil, fmt.Errorf("LocalEnqueuer must not be used in cloud environments")
	}
	return &LocalEnqueuer{worker: worker, logger: logger}, nil
}

// Enqueue runs the verification worker in a background goroutine. The
// goroutine uses a fresh context (not the upload request's) so the worker
// keeps running after the upload handler returns. Timeout matches the
// HandleVerifyTask path so behavior is consistent with the Cloud Tasks path.
func (l *LocalEnqueuer) Enqueue(_ context.Context, uploadID uuid.UUID) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := l.worker.Process(ctx, uploadID); err != nil {
			l.logger.ErrorContext(ctx, "local verify process", "upload_id", uploadID, "error", err)
		}
	}()
	l.logger.Info("local verification scheduled", "upload_id", uploadID)
	return nil
}
