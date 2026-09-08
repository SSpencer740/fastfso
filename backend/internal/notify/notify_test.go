package notify

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/email"
)

type recordingSender struct {
	mu       sync.Mutex
	messages []email.Message
}

func (r *recordingSender) Send(_ context.Context, msg email.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, msg)
	return nil
}

func (r *recordingSender) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.messages)
}

type stubFrequency struct {
	freq map[uuid.UUID]string
	err  error
}

func (s stubFrequency) NotificationFrequency(_ context.Context, userID uuid.UUID) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if f, ok := s.freq[userID]; ok {
		return f, nil
	}
	return auth.FrequencyEveryTask, nil
}

// waitFor waits up to d for fn to return true, polling every 5ms. Notifier
// dispatches its email send inside a goroutine, so the test needs to give it a
// moment to land.
func waitFor(d time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

func TestNotifierGatesByFrequency(t *testing.T) {
	immediate := uuid.New()
	digestOnly := uuid.New()
	settings := stubFrequency{freq: map[uuid.UUID]string{
		immediate:  auth.FrequencyEveryTask,
		digestOnly: auth.FrequencyDailySummary,
	}}

	sender := &recordingSender{}
	svc := email.New(sender, "https://app.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	n := New(svc, settings, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Daily-summary user must be silently skipped.
	n.NotifyActionItemAssigned(context.Background(), digestOnly, "fso@x.com", "FSO", "travel_report")
	n.NotifyTaskAssigned(context.Background(), digestOnly, "ic@x.com", "IC")
	n.NotifyTaskReviewed(context.Background(), digestOnly, "ic@x.com", "IC", "approved")

	// Every-task user receives all three.
	n.NotifyActionItemAssigned(context.Background(), immediate, "fso@x.com", "FSO", "travel_report")
	n.NotifyTaskAssigned(context.Background(), immediate, "ic@x.com", "IC")
	n.NotifyTaskReviewed(context.Background(), immediate, "ic@x.com", "IC", "approved")

	require.True(t, waitFor(time.Second, func() bool { return sender.Count() >= 3 }))
	assert.Equal(t, 3, sender.Count(), "only the every_task user should have received emails")
}

func TestNotifierSendsOnFrequencyLookupFailure(t *testing.T) {
	// On lookup error we err on the side of delivering the email — silently
	// dropping notifications is worse than a stray duplicate.
	settings := stubFrequency{err: errors.New("db down")}
	sender := &recordingSender{}
	svc := email.New(sender, "https://app.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	n := New(svc, settings, slog.New(slog.NewTextHandler(io.Discard, nil)))

	n.NotifyActionItemAssigned(context.Background(), uuid.New(), "fso@x.com", "FSO", "travel_report")
	require.True(t, waitFor(time.Second, func() bool { return sender.Count() >= 1 }))
	assert.Equal(t, 1, sender.Count())
}

func TestNotifierSendsOnNilUserID(t *testing.T) {
	// Admin-fallback paths sometimes don't have a specific user — those should
	// still go out.
	sender := &recordingSender{}
	svc := email.New(sender, "https://app.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	n := New(svc, stubFrequency{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	n.NotifyActionItemAssigned(context.Background(), uuid.Nil, "fso@x.com", "FSO", "travel_report")
	require.True(t, waitFor(time.Second, func() bool { return sender.Count() >= 1 }))
	assert.Equal(t, 1, sender.Count())
}
