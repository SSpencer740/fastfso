package email

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSendGridSender(t *testing.T, handler http.HandlerFunc) (*SendGridSender, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// Override the global DefaultClient to use our test server
	origClient := sendgrid.DefaultClient
	t.Cleanup(func() { sendgrid.DefaultClient = origClient })
	sendgrid.DefaultClient = &rest.Client{HTTPClient: server.Client()}

	// Create a sender whose client points at the test server
	sender := NewSendGridSender("test-api-key", "noreply@fastfso.com", slog.Default())
	sender.client.BaseURL = server.URL + "/v3/mail/send"

	return sender, server
}

func TestSendGridSender_Success(t *testing.T) {
	var capturedBody string
	sender, _ := newTestSendGridSender(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	})

	msg := Message{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Hello</p>",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	require.NoError(t, err)

	assert.Contains(t, capturedBody, "user@example.com")
	assert.Contains(t, capturedBody, "Test Subject")
	assert.Contains(t, capturedBody, "noreply@fastfso.com")
	assert.Contains(t, capturedBody, "Hello")
}

func TestSendGridSender_TransientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"rate_limited", http.StatusTooManyRequests},
		{"server_error", http.StatusInternalServerError},
		{"bad_gateway", http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, _ := newTestSendGridSender(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error": "try again"}`))
			})

			err := sender.Send(context.Background(), Message{
				To:      "user@example.com",
				Subject: "Test",
				HTML:    "<p>Test</p>",
				Text:    "Test",
			})

			require.Error(t, err)
			var sgErr *SendGridError
			require.ErrorAs(t, err, &sgErr)
			assert.Equal(t, tt.statusCode, sgErr.StatusCode)
			assert.True(t, sgErr.IsTransient())
		})
	}
}

func TestSendGridSender_PermanentError(t *testing.T) {
	sender, _ := newTestSendGridSender(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors": [{"message": "bad request"}]}`))
	})

	err := sender.Send(context.Background(), Message{
		To:      "user@example.com",
		Subject: "Test",
		HTML:    "<p>Test</p>",
		Text:    "Test",
	})

	require.Error(t, err)
	var sgErr *SendGridError
	require.ErrorAs(t, err, &sgErr)
	assert.Equal(t, http.StatusBadRequest, sgErr.StatusCode)
	assert.False(t, sgErr.IsTransient())
}

func TestSendGridError_IsTransient(t *testing.T) {
	tests := []struct {
		code      int
		transient bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		e := &SendGridError{StatusCode: tt.code}
		assert.Equal(t, tt.transient, e.IsTransient(), "status %d", tt.code)
	}
}
