package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSessionIsValid(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "valid session",
			session: Session{
				ExpiresAt: time.Now().Add(time.Hour),
				RevokedAt: nil,
			},
			expected: true,
		},
		{
			name: "expired session",
			session: Session{
				ExpiresAt: time.Now().Add(-time.Hour),
				RevokedAt: nil,
			},
			expected: false,
		},
		{
			name: "revoked session",
			session: Session{
				ExpiresAt: time.Now().Add(time.Hour),
				RevokedAt: ptrTime(time.Now()),
			},
			expected: false,
		},
		{
			name: "revoked and expired session",
			session: Session{
				ExpiresAt: time.Now().Add(-time.Hour),
				RevokedAt: ptrTime(time.Now()),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.session.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSessionConstants(t *testing.T) {
	if StatePre_Auth != "pre_auth" {
		t.Errorf("StatePre_Auth = %q, want %q", StatePre_Auth, "pre_auth")
	}
	if StatePreTenant != "pre_tenant" {
		t.Errorf("StatePreTenant = %q, want %q", StatePreTenant, "pre_tenant")
	}
	if StateAuthenticated != "authenticated" {
		t.Errorf("StateAuthenticated = %q, want %q", StateAuthenticated, "authenticated")
	}
}

func TestSessionFields(t *testing.T) {
	id := uuid.New()
	tenantID := uuid.New()
	sess := Session{
		ID:        id,
		TenantID:  &tenantID,
		State:     StateAuthenticated,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if sess.ID != id {
		t.Error("ID mismatch")
	}
	if sess.TenantID == nil || *sess.TenantID != tenantID {
		t.Error("TenantID mismatch")
	}
	if !sess.IsValid() {
		t.Error("expected session to be valid")
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
