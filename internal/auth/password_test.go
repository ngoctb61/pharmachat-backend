package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$"))

	ok, err := VerifyPassword("correct horse battery", h)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = VerifyPassword("wrong", h)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHashIsSalted(t *testing.T) {
	a, err := HashPassword("same password")
	require.NoError(t, err)
	b, err := HashPassword("same password")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestVerifyRejectsForeignFormats(t *testing.T) {
	for _, h := range []string{
		"",
		"plain",
		// The unsalted SHA-256 base64 format the .NET backend wrote (AuthService.cs:91).
		"XohImNooBHFR0OVvjcYpJ3NgPQ1qq73WKhHvch0VQtg=",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=x,t=2,p=1$c2FsdA$aGFzaA",
	} {
		_, err := VerifyPassword("x", h)
		assert.ErrorIs(t, err, ErrInvalidHash, "hash %q", h)
	}
}
