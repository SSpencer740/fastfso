package auth

import "testing"

func TestEmailDomain(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"alice@acme.com", "acme.com"},
		{"  Alice@ACME.com ", "acme.com"},
		{"weird+plus@sub.acme.com", "sub.acme.com"},
		{"no-at-sign", ""},
		{"trailing@", ""},
		{"", ""},
		{"@nouser.com", "nouser.com"},
	}
	for _, c := range cases {
		if got := emailDomain(c.in); got != c.want {
			t.Errorf("emailDomain(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
