package sso

import "testing"

func TestEmailDomainAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		allowed []EmailDomain
		want    bool
	}{
		{
			name:    "empty allowlist permits any email",
			email:   "anyone@example.com",
			allowed: nil,
			want:    true,
		},
		{
			name:    "exact domain match",
			email:   "alice@acme.com",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    true,
		},
		{
			name:    "case-insensitive domain match",
			email:   "Alice@ACME.com",
			allowed: []EmailDomain{{Domain: "Acme.COM"}},
			want:    true,
		},
		{
			name:    "domain mismatch rejected",
			email:   "attacker@evil.com",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    false,
		},
		{
			name:    "personal account at same provider rejected",
			email:   "alice@gmail.com",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    false,
		},
		{
			name:    "subdomain not allowed by parent domain",
			email:   "alice@sub.acme.com",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    false,
		},
		{
			name:    "whitespace tolerated in stored domain",
			email:   "alice@acme.com",
			allowed: []EmailDomain{{Domain: "  acme.com  "}},
			want:    true,
		},
		{
			name:    "multiple domains, second matches",
			email:   "bob@beta.io",
			allowed: []EmailDomain{{Domain: "acme.com"}, {Domain: "beta.io"}},
			want:    true,
		},
		{
			name:    "malformed email rejected when allowlist non-empty",
			email:   "no-at-sign",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    false,
		},
		{
			name:    "trailing @ rejected",
			email:   "alice@",
			allowed: []EmailDomain{{Domain: "acme.com"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := emailDomainAllowed(tt.email, tt.allowed)
			if got != tt.want {
				t.Errorf("emailDomainAllowed(%q, %+v) = %v, want %v", tt.email, tt.allowed, got, tt.want)
			}
		})
	}
}
