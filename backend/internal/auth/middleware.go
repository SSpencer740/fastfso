package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/session"
)

const (
	sessionCookieName  = "fastfso_session"
	sessionContextKey  = "session"
	userRoleContextKey = "user_role"
	touchDebounce      = time.Minute
)

// GetSession retrieves the session from the Gin context.
func GetSession(c *gin.Context) (*session.Session, bool) {
	val, exists := c.Get(sessionContextKey)
	if !exists {
		return nil, false
	}
	sess, ok := val.(*session.Session)
	return sess, ok
}

type sessionValidator interface {
	GetValid(ctx context.Context, id uuid.UUID) (*session.Session, error)
	TouchLastActive(ctx context.Context, id uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID, revokedBy *uuid.UUID, reason string) error
}

// RequireSession validates the session cookie and injects the session into context.
func RequireSession(store sessionValidator, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no session"})
			return
		}

		sessionID, err := uuid.Parse(cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		sess, err := store.GetValid(c.Request.Context(), sessionID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or revoked"})
			return
		}

		// Enforce idle timeout. GetValid already checks the absolute expiry; this
		// catches sessions that are still within the hard cap but have been dormant
		// past the idle threshold. Revoke lazily so the DB state matches.
		if time.Since(sess.LastActiveAt) > session.IdleTimeout {
			if err := store.Revoke(c.Request.Context(), sess.ID, nil, "idle_timeout"); err != nil {
				logger.Warn("failed to revoke idle session", "session_id", sess.ID, "error", err)
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or revoked"})
			return
		}

		// Debounced last_active_at update
		if time.Since(sess.LastActiveAt) > touchDebounce {
			if err := store.TouchLastActive(c.Request.Context(), sess.ID); err != nil {
				logger.Warn("failed to touch session", "session_id", sess.ID, "error", err)
			}
		}

		c.Set(sessionContextKey, sess)
		c.Next()
	}
}

// RequireState checks that the session state matches one of the allowed states.
func RequireState(states ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(states))
	for _, s := range states {
		allowed[s] = true
	}
	return func(c *gin.Context) {
		sess, ok := GetSession(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no session"})
			return
		}
		if !allowed[sess.State] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid session state", "state": sess.State})
			return
		}
		c.Next()
	}
}

// RequireTenant is shorthand for RequireState("authenticated").
func RequireTenant() gin.HandlerFunc {
	return RequireState(session.StateAuthenticated)
}

// RequireSuperAdmin checks that the session has super admin access.
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := GetSession(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no session"})
			return
		}
		if !sess.IsSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "super admin required"})
			return
		}
		c.Next()
	}
}

// RequireRole loads the user role and checks against allowed roles.
func RequireRole(store UserRoleGetter, roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		sess, ok := GetSession(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no session"})
			return
		}
		if sess.UserID == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no tenant selected"})
			return
		}
		role, err := store.GetUserRole(c.Request.Context(), *sess.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to get user role"})
			return
		}
		if !allowed[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Set(userRoleContextKey, role)
		c.Next()
	}
}

// UserRoleGetter retrieves a user's role by user ID.
type UserRoleGetter interface {
	GetUserRole(ctx context.Context, userID uuid.UUID) (string, error)
}

// SubOrgScope returns the sub-org ID to use for filtering data queries.
//
// For FSO and read_only_fso roles: returns the sub-org stored in the session
// (mandatory scope — they can only see their sub-org + tenant-wide records).
//
// For administrators: returns the sub_org_id query param if provided, else nil
// (no scope = see everything).
//
// Call this only on routes that use RequireRole, so the role is in context.
func SubOrgScope(c *gin.Context) *uuid.UUID {
	role, _ := c.Get(userRoleContextKey)
	r, _ := role.(string)
	if r == "fso" || r == "read_only_fso" {
		sess, ok := GetSession(c)
		if ok {
			return sess.SubOrgID
		}
		return nil
	}
	// administrator: honour optional query param for UI filtering
	if s := c.Query("sub_org_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			return &id
		}
	}
	return nil
}

// RequireCloudScheduler rejects requests that don't originate from Cloud Scheduler.
// On Cloud Run, GCP sets the X-CloudScheduler header to "true" for authenticated
// scheduler invocations. In non-production environments, requests are allowed through.
func RequireCloudScheduler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !env.IsCloud() {
			c.Next()
			return
		}
		if c.GetHeader("X-CloudScheduler") != "true" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// RequireCloudTasks validates the OIDC token attached by Cloud Tasks.
// In non-cloud environments, requests are allowed through without validation.
func RequireCloudTasks() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !env.IsCloud() {
			c.Next()
			return
		}

		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		payload, err := idtoken.Validate(c.Request.Context(), token, env.BackendURL())
		if err != nil {
			slog.Error("cloud tasks token validation failed", "error", err)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		email, _ := payload.Claims["email"].(string)
		if email != env.BackendServiceAccount() {
			slog.Error("cloud tasks token SA mismatch", "expected", env.BackendServiceAccount(), "got", email)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
