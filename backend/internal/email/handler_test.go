package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSender struct {
	err error
}

func (f *fakeSender) Send(_ context.Context, _ Message) error {
	return f.err
}

func setupHandlerTest(t *testing.T, sender Sender, body []byte, retryCount string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/api/tasks/send-email", HandleSendTask(sender, slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/send-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if retryCount != "" {
		req.Header.Set("X-CloudTasks-TaskRetryCount", retryCount)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

func TestHandleSendTask_Success(t *testing.T) {
	msg := Message{
		To:      "user@example.com",
		Subject: "Hello",
		HTML:    "<p>Hi</p>",
		Text:    "Hi",
	}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	w := setupHandlerTest(t, &fakeSender{}, body, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendTask_InvalidJSON(t *testing.T) {
	w := setupHandlerTest(t, &fakeSender{}, []byte(`{invalid`), "")
	// Returns 200 to prevent Cloud Tasks from retrying bad payloads
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendTask_TransientError_Retry(t *testing.T) {
	msg := Message{To: "user@example.com", Subject: "Test", HTML: "<p>T</p>", Text: "T"}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	sender := &fakeSender{err: &SendGridError{StatusCode: http.StatusTooManyRequests, Body: "rate limited"}}
	w := setupHandlerTest(t, sender, body, "1")

	// Should return 503 to trigger Cloud Tasks retry
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleSendTask_TransientError_FinalRetry(t *testing.T) {
	msg := Message{To: "user@example.com", Subject: "Test", HTML: "<p>T</p>", Text: "T"}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	sender := &fakeSender{err: &SendGridError{StatusCode: http.StatusInternalServerError, Body: "server error"}}
	w := setupHandlerTest(t, sender, body, "4")

	// After 4 retries, should return 200 (dead letter) and stop retrying
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendTask_NonTransientError(t *testing.T) {
	msg := Message{To: "user@example.com", Subject: "Test", HTML: "<p>T</p>", Text: "T"}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	sender := &fakeSender{err: &SendGridError{StatusCode: http.StatusBadRequest, Body: "bad request"}}
	w := setupHandlerTest(t, sender, body, "0")

	// Non-transient error should return 200 (don't retry)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendTask_GenericError(t *testing.T) {
	msg := Message{To: "user@example.com", Subject: "Test", HTML: "<p>T</p>", Text: "T"}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	sender := &fakeSender{err: fmt.Errorf("network failure")}
	w := setupHandlerTest(t, sender, body, "0")

	// Generic errors are non-transient, return 200 (don't retry)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendTask_NonTransientError_FinalRetry(t *testing.T) {
	msg := Message{To: "user@example.com", Subject: "Test", HTML: "<p>T</p>", Text: "T"}
	body, err := json.Marshal(msg)
	require.NoError(t, err)

	sender := &fakeSender{err: &SendGridError{StatusCode: http.StatusBadRequest, Body: "bad"}}
	w := setupHandlerTest(t, sender, body, "4")

	// Final retry with non-transient error: dead letter, return 200
	assert.Equal(t, http.StatusOK, w.Code)
}
