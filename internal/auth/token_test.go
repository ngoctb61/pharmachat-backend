package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKey = []byte(strings.Repeat("k", 32))

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestTokenIssuer_IssueAndParse(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	ti, err := NewTokenIssuer(testKey, fixedClock(now))
	require.NoError(t, err)
	userID, tenantID := uuid.New(), uuid.New()

	token, err := ti.Issue(userID, tenantID, []string{"Admin", "Sale"})
	require.NoError(t, err)

	claims, err := ti.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, tenantID, claims.TenantID)
	assert.Equal(t, []string{"Admin", "Sale"}, claims.Roles)
	assert.Equal(t, Issuer, claims.Issuer)
	assert.True(t, now.Equal(claims.IssuedAt.Time), "iat = %v", claims.IssuedAt.Time)
	assert.Equal(t, 15*time.Minute, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
}

func TestNewTokenIssuer_RejectsShortKey(t *testing.T) {
	_, err := NewTokenIssuer(testKey[:31], time.Now)
	assert.ErrorContains(t, err, "at least 32 bytes")
}

func TestTokenIssuer_ParseRejects(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	ti, err := NewTokenIssuer(testKey, fixedClock(now))
	require.NoError(t, err)

	valid := func() Claims {
		return Claims{
			UserID: uuid.New(), TenantID: uuid.New(), Roles: []string{"Admin"},
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    Issuer,
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			},
		}
	}
	sign := func(t *testing.T, method jwt.SigningMethod, key any, c Claims) string {
		t.Helper()
		s, err := jwt.NewWithClaims(method, c).SignedString(key)
		require.NoError(t, err)
		return s
	}

	cases := map[string]func(t *testing.T) string{
		"expired": func(t *testing.T) string {
			c := valid()
			c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Second))
			return sign(t, jwt.SigningMethodHS256, testKey, c)
		},
		"other key": func(t *testing.T) string {
			return sign(t, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), valid())
		},
		"alg none": func(t *testing.T) string {
			return sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid())
		},
		"HS512": func(t *testing.T) string {
			return sign(t, jwt.SigningMethodHS512, testKey, valid())
		},
		"missing exp": func(t *testing.T) string {
			c := valid()
			c.ExpiresAt = nil
			return sign(t, jwt.SigningMethodHS256, testKey, c)
		},
		"other issuer": func(t *testing.T) string {
			c := valid()
			c.Issuer = "someone-else"
			return sign(t, jwt.SigningMethodHS256, testKey, c)
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ti.Parse(mk(t))
			assert.Error(t, err)
		})
	}
}
