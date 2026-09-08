package invite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	tok, err := generateToken()
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	// 32 bytes base64url → 43 chars (no padding)
	assert.Len(t, tok, 43)
}

func TestGenerateToken_Unique(t *testing.T) {
	tok1, err := generateToken()
	require.NoError(t, err)
	tok2, err := generateToken()
	require.NoError(t, err)
	assert.NotEqual(t, tok1, tok2)
}

func TestHashToken(t *testing.T) {
	hash := hashToken("test-token")
	assert.Len(t, hash, 64) // SHA-256 hex = 64 chars
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("same-input")
	h2 := hashToken("same-input")
	assert.Equal(t, h1, h2)
}

func TestHashToken_DifferentInputs(t *testing.T) {
	h1 := hashToken("input-a")
	h2 := hashToken("input-b")
	assert.NotEqual(t, h1, h2)
}
