package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/SSpencer740/fastfso/backend/internal/env"
)

const (
	csrfCookieName = "fastfso_csrf"
	csrfHeaderName = "X-CSRF-Token"
)

func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(env.SessionSecret()))
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func setCSRFCookie(c *gin.Context, token string) {
	c.SetCookie(csrfCookieName, token, 0, "/", "", env.IsCloud(), false)
}

// SetCSRFCookiePublic is exported for use by other packages (e.g. passkey).
func SetCSRFCookiePublic(c *gin.Context, token string) {
	setCSRFCookie(c, token)
}

func clearCSRFCookie(c *gin.Context) {
	c.SetCookie(csrfCookieName, "", -1, "/", "", env.IsCloud(), false)
}

// CSRFMiddleware validates the double-submit cookie pattern on non-GET requests.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		sess, exists := GetSession(c)
		if !exists {
			c.Next()
			return
		}

		headerToken := c.GetHeader(csrfHeaderName)
		if headerToken == "" || headerToken != sess.CSRFToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}

		c.Next()
	}
}
