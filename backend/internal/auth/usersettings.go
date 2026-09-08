package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/gin-gonic/gin"
)

// UserSettings holds per-user preferences stored server-side.
type UserSettings struct {
	NotificationFrequency string `json:"notification_frequency"`
}

// NotificationFrequency values.
const (
	FrequencyEveryTask    = "every_task"
	FrequencyDailySummary = "daily_summary"
)

// UserSettingsStore handles reading and writing user_settings rows.
type UserSettingsStore struct {
	db database.DB
}

// NewUserSettingsStore creates a UserSettingsStore.
func NewUserSettingsStore(db database.DB) *UserSettingsStore {
	return &UserSettingsStore{db: db}
}

// Get returns settings for a user, returning defaults if no row exists yet.
func (s *UserSettingsStore) Get(ctx context.Context, userID uuid.UUID) (UserSettings, error) {
	var us UserSettings
	err := s.db.QueryRow(ctx, "usersettings.Get",
		`SELECT notification_frequency FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&us.NotificationFrequency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserSettings{NotificationFrequency: "every_task"}, nil
		}
		return UserSettings{}, fmt.Errorf("usersettings get: %w", err)
	}
	return us, nil
}

// NotificationFrequency returns just the frequency for a user, defaulting to
// every_task when no row exists yet. Used by the notifier to decide whether to
// send an immediate email or hold the event for the daily digest.
func (s *UserSettingsStore) NotificationFrequency(ctx context.Context, userID uuid.UUID) (string, error) {
	us, err := s.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	return us.NotificationFrequency, nil
}

// Upsert creates or updates the settings row for a user.
func (s *UserSettingsStore) Upsert(ctx context.Context, userID uuid.UUID, us UserSettings) error {
	_, err := s.db.Exec(ctx, "usersettings.Upsert",
		`INSERT INTO user_settings (user_id, notification_frequency)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE
		   SET notification_frequency = EXCLUDED.notification_frequency,
		       updated_at = now()`,
		userID, us.NotificationFrequency,
	)
	if err != nil {
		return fmt.Errorf("usersettings upsert: %w", err)
	}
	return nil
}

// --- HTTP handlers (attached to auth.Handler via a separate store field) ---

// GetUserSettings handles GET /api/v1/settings.
func (h *Handler) GetUserSettings(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	us, err := h.userSettings.Get(c.Request.Context(), *sess.UserID)
	if err != nil {
		h.logger.Error("get user settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, us)
}

// UpdateUserSettings handles PUT /api/v1/settings.
func (h *Handler) UpdateUserSettings(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var body UserSettings
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	allowed := map[string]bool{"every_task": true, "daily_summary": true}
	if !allowed[body.NotificationFrequency] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification_frequency"})
		return
	}

	if err := h.userSettings.Upsert(c.Request.Context(), *sess.UserID, body); err != nil {
		h.logger.Error("update user settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, body)
}
