package email

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQueueSender(t *testing.T) {
	// Verify the constructor sets fields correctly.
	// We pass nil for the client since we're only checking field wiring.
	qs := NewQueueSender(nil, "projects/p/locations/l/queues/q", "https://app.example.com", "sa@proj.iam.gserviceaccount.com", slog.Default())

	assert.Equal(t, "projects/p/locations/l/queues/q", qs.queue)
	assert.Equal(t, "https://app.example.com/api/tasks/send-email", qs.targetURL)
	assert.Equal(t, "https://app.example.com", qs.audience)
	assert.Equal(t, "sa@proj.iam.gserviceaccount.com", qs.saEmail)
}
