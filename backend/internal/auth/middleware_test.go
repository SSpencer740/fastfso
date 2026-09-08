package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireCloudTasks_LocalMode(t *testing.T) {
	// In local mode (DEPLOY_ENV not "cloud"), requests pass through without validation.
	t.Setenv("DEPLOY_ENV", "local")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/tasks/send-email", nil)

	handler := RequireCloudTasks()
	handler(c)

	assert.False(t, c.IsAborted())
}

func TestRequireCloudTasks_CloudMode_NoToken(t *testing.T) {
	t.Setenv("DEPLOY_ENV", "cloud")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/tasks/send-email", nil)

	handler := RequireCloudTasks()
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireCloudTasks_CloudMode_InvalidToken(t *testing.T) {
	t.Setenv("DEPLOY_ENV", "cloud")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/send-email", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	c.Request = req

	handler := RequireCloudTasks()
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireCloudScheduler_LocalMode(t *testing.T) {
	t.Setenv("DEPLOY_ENV", "local")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/cron/session-cleanup", nil)

	handler := RequireCloudScheduler()
	handler(c)

	assert.False(t, c.IsAborted())
}

func TestRequireCloudScheduler_CloudMode_NoHeader(t *testing.T) {
	t.Setenv("DEPLOY_ENV", "cloud")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/cron/session-cleanup", nil)

	handler := RequireCloudScheduler()
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}
