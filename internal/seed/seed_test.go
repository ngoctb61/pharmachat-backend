package seed_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ngoctb61/pharmachat-backend/internal/auth"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/crypto"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/dbtest"
	"github.com/ngoctb61/pharmachat-backend/internal/seed"
)

var env *dbtest.Env

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m, &env)) }

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func sealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	s, err := crypto.NewSealer(make([]byte, 32))
	require.NoError(t, err)
	return s
}

func TestRun_IsIdempotentAndEncryptsKeys(t *testing.T) {
	dbtest.Require(t, env)
	ctx := context.Background()
	opts := seed.Options{TenantSlug: "seed-idem", TenantName: "Seed", AnthropicKey: "sk-ant-test", OpenAIKey: "sk-openai-test", Sealer: sealer(t)}

	first, err := seed.Run(ctx, env.Owner, opts, quiet)
	require.NoError(t, err)
	assert.Equal(t, "created", first.LLMConfig)
	assert.Equal(t, "created", first.EmbeddingConfig)

	second, err := seed.Run(ctx, env.Owner, opts, quiet)
	require.NoError(t, err)
	assert.Equal(t, first.TenantID, second.TenantID)
	assert.Equal(t, "exists", second.LLMConfig)
	assert.Equal(t, "exists", second.EmbeddingConfig)

	var roles, adminPerms, salePerms int
	require.NoError(t, env.Owner.QueryRow(ctx, "SELECT count(*) FROM roles WHERE tenant_id = $1", first.TenantID).Scan(&roles))
	require.NoError(t, env.Owner.QueryRow(ctx, `SELECT count(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
		WHERE r.tenant_id = $1 AND r.name = 'Admin'`, first.TenantID).Scan(&adminPerms))
	require.NoError(t, env.Owner.QueryRow(ctx, `SELECT count(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
		WHERE r.tenant_id = $1 AND r.name = 'Sale'`, first.TenantID).Scan(&salePerms))
	assert.Equal(t, 2, roles)
	assert.Equal(t, 11, adminPerms)
	assert.Equal(t, 4, salePerms)

	var model, sealed string
	require.NoError(t, env.Owner.QueryRow(ctx,
		"SELECT model_id, api_key_encrypted FROM llm_configs WHERE tenant_id = $1 AND is_active", first.TenantID).Scan(&model, &sealed))
	assert.Equal(t, "claude-sonnet-4-6", model)
	assert.NotContains(t, sealed, "sk-ant-test", "API key must be stored encrypted")
	plain, err := opts.Sealer.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-test", plain)
}

func TestRun_SkipsAIConfigWithoutKeys(t *testing.T) {
	dbtest.Require(t, env)
	res, err := seed.Run(context.Background(), env.Owner, seed.Options{TenantSlug: "seed-nokeys", TenantName: "No keys"}, quiet)
	require.NoError(t, err)
	assert.Equal(t, "skipped: ANTHROPIC_API_KEY not set", res.LLMConfig)
	assert.Equal(t, "skipped: OPENAI_API_KEY not set", res.EmbeddingConfig)
}

func TestCreateAdmin(t *testing.T) {
	dbtest.Require(t, env)
	ctx := context.Background()
	_, err := seed.Run(ctx, env.Owner, seed.Options{TenantSlug: "seed-admin", TenantName: "Admin test"}, quiet)
	require.NoError(t, err)

	in := seed.AdminInput{TenantSlug: "seed-admin", Email: " Admin@Example.com ", DisplayName: "Admin", Password: "a-long-enough-password"}
	id, err := seed.CreateAdmin(ctx, env.Owner, in)
	require.NoError(t, err)

	var email, hash, role string
	require.NoError(t, env.Owner.QueryRow(ctx, `SELECT u.email, u.password_hash, r.name FROM users u
		JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id WHERE u.id = $1`, id).Scan(&email, &hash, &role))
	assert.Equal(t, "admin@example.com", email, "email is normalised")
	assert.Equal(t, "Admin", role)
	ok, err := auth.VerifyPassword("a-long-enough-password", hash)
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = seed.CreateAdmin(ctx, env.Owner, in)
	assert.ErrorIs(t, err, seed.ErrUserExists)
}

func TestCreateAdmin_Validation(t *testing.T) {
	dbtest.Require(t, env)
	ctx := context.Background()
	cases := map[string]seed.AdminInput{
		"short password": {TenantSlug: "seed-admin", Email: "a@b.vn", DisplayName: "A", Password: "short"},
		"bad email":      {TenantSlug: "seed-admin", Email: "not-an-email", DisplayName: "A", Password: "a-long-enough-password"},
		"missing name":   {TenantSlug: "seed-admin", Email: "a@b.vn", DisplayName: " ", Password: "a-long-enough-password"},
		"unknown tenant": {TenantSlug: "nope", Email: "a@b.vn", DisplayName: "A", Password: "a-long-enough-password"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := seed.CreateAdmin(ctx, env.Owner, in)
			assert.Error(t, err)
		})
	}
}
