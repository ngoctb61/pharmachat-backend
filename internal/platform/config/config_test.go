package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "a-signing-key-of-at-least-32-bytes")
	t.Setenv("DATABASE_URL", "")
	_, err := Load[API]()
	assert.Error(t, err)

	t.Setenv("DATABASE_URL", "postgres://x")
	cfg, err := Load[API]()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoadRequiresJWTSigningKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SIGNING_KEY", "")
	_, err := Load[API]()
	assert.ErrorContains(t, err, "JWT_SIGNING_KEY")

	t.Setenv("JWT_SIGNING_KEY", "a-signing-key-of-at-least-32-bytes")
	cfg, err := Load[API]()
	require.NoError(t, err)
	assert.Equal(t, "a-signing-key-of-at-least-32-bytes", cfg.JWTSigningKey)
}

func TestDecodeKey(t *testing.T) {
	_, err := DecodeKey(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)

	_, err = DecodeKey(base64.StdEncoding.EncodeToString(make([]byte, 16)))
	assert.ErrorContains(t, err, "32 bytes")

	_, err = DecodeKey("%%%")
	assert.ErrorContains(t, err, "base64")
}
