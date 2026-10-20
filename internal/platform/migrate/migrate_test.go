package migrate_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/dbtest"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/migrate"
)

func TestUpDownUp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: needs Docker, skipped with -short")
	}
	ctx := context.Background()
	env, err := dbtest.Start(ctx, false)
	require.NoError(t, err)
	defer env.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	require.NoError(t, migrate.Up(ctx, env.Owner, log))
	require.NoError(t, migrate.Up(ctx, env.Owner, log), "second up must be a no-op")
	assert.True(t, tableExists(t, env, "kb_chunks"))
	assert.True(t, tableExists(t, env, "river_job"))

	var dims int
	require.NoError(t, env.Owner.QueryRow(ctx, `
		SELECT atttypmod FROM pg_attribute
		WHERE attrelid = 'kb_chunks'::regclass AND attname = 'embedding'`).Scan(&dims))
	assert.Equal(t, 3072, dims, "kb_chunks.embedding must be halfvec(3072)")

	assert.True(t, loginCandidateExists(t, env))
	assert.True(t, tableExists(t, env, "ux_users_email_active"))

	// Down rolls back only the newest migration.
	require.NoError(t, migrate.Down(ctx, env.Owner, log))
	assert.False(t, loginCandidateExists(t, env))
	assert.False(t, tableExists(t, env, "ux_users_email_active"))
	assert.True(t, tableExists(t, env, "kb_chunks"))

	require.NoError(t, migrate.Up(ctx, env.Owner, log))
	assert.True(t, loginCandidateExists(t, env))
	assert.True(t, tableExists(t, env, "ux_users_email_active"))
}

func loginCandidateExists(t *testing.T, env *dbtest.Env) bool {
	t.Helper()
	var ok bool
	require.NoError(t, env.Owner.QueryRow(context.Background(),
		"SELECT to_regprocedure('public.auth_login_candidate(text)') IS NOT NULL").Scan(&ok))
	return ok
}

func tableExists(t *testing.T, env *dbtest.Env, name string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, env.Owner.QueryRow(context.Background(),
		"SELECT to_regclass('public.' || $1) IS NOT NULL", name).Scan(&ok))
	return ok
}
