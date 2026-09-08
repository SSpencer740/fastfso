package errorreport

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(logger *slog.Logger) *gin.Engine {
	r := gin.New()
	r.POST("/api/v1/errors", Handler(logger))
	return r
}

func TestHandler_ValidRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := setupRouter(logger)

	stack := "Error: test\n    at foo (app.js:1:1)"
	body := reportRequest{
		Message:   "test error",
		Stack:     &stack,
		URL:       "https://app.fastfso.com/dashboard",
		Timestamp: "2025-01-01T00:00:00Z",
		Source:    "window.onerror",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/errors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	logOutput := buf.String()
	if logOutput == "" {
		t.Fatal("expected log output, got empty")
	}

	// Verify the log contains ReportedErrorEvent type
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}
	if logEntry["@type"] != "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent" {
		t.Errorf("unexpected @type: %v", logEntry["@type"])
	}
	if logEntry["error_source"] != "window.onerror" {
		t.Errorf("unexpected error_source: %v", logEntry["error_source"])
	}
}

func TestHandler_ValidRequest_OptionalFieldsOmitted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := setupRouter(logger)

	body := reportRequest{
		Message:   "test error",
		URL:       "https://app.fastfso.com/login",
		Timestamp: "2025-01-01T00:00:00Z",
		Source:    "unhandledrejection",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/errors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MissingRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := setupRouter(logger)

	// Missing message, url, timestamp, source
	body := map[string]any{}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/errors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_InvalidSource(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := setupRouter(logger)

	body := map[string]any{
		"message":   "test",
		"url":       "https://app.fastfso.com",
		"timestamp": "2025-01-01T00:00:00Z",
		"source":    "invalid_source",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/errors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_AllSources(t *testing.T) {
	sources := []string{"window.onerror", "unhandledrejection", "error_boundary", "manual"}
	for _, src := range sources {
		t.Run(src, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			r := setupRouter(logger)

			body := map[string]any{
				"message":   "test",
				"url":       "https://app.fastfso.com",
				"timestamp": "2025-01-01T00:00:00Z",
				"source":    src,
			}
			b, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/errors", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusAccepted {
				t.Errorf("expected status 202 for source %q, got %d", src, w.Code)
			}
		})
	}
}
