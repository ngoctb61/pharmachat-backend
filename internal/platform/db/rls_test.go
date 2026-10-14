package db_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/dbtest"
)

var env *dbtest.Env

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m, &env)) }

// newTenant creates a tenant with one contact, as the owner.
func newTenant(t *testing.T, slug string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO tenants (slug, display_name) VALUES ($1, $1) RETURNING id", slug).Scan(&id))
	_, err := env.Owner.Exec(ctx,
		"INSERT INTO contacts (tenant_id, display_name) VALUES ($1, $2)", id, "contact of "+slug)
	require.NoError(t, err)
	return id
}

func countContacts(t *testing.T, tenant uuid.UUID) (n int, names []string) {
	t.Helper()
	err := db.WithTenant(context.Background(), env.App, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), "SELECT display_name FROM contacts")
		if err != nil {
			return err
		}
		names, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	require.NoError(t, err)
	return len(names), names
}

func TestRLS_TenantSeesOnlyItsOwnRows(t *testing.T) {
	dbtest.Require(t, env)
	a, b := newTenant(t, "rls-a"), newTenant(t, "rls-b")

	n, names := countContacts(t, a)
	assert.Equal(t, 1, n)
	assert.Equal(t, []string{"contact of rls-a"}, names)

	n, names = countContacts(t, b)
	assert.Equal(t, 1, n)
	assert.Equal(t, []string{"contact of rls-b"}, names)
}

func TestRLS_NoTenantScopeSeesNothing(t *testing.T) {
	dbtest.Require(t, env)
	newTenant(t, "rls-unscoped")
	ctx := context.Background()

	var contacts, tenants int
	require.NoError(t, env.App.QueryRow(ctx, "SELECT count(*) FROM contacts").Scan(&contacts))
	require.NoError(t, env.App.QueryRow(ctx, "SELECT count(*) FROM tenants").Scan(&tenants))
	assert.Zero(t, contacts, "app role without app.tenant_id must not see contacts")
	assert.Zero(t, tenants, "app role without app.tenant_id must not see tenants")
}

func TestRLS_CannotWriteIntoAnotherTenant(t *testing.T) {
	dbtest.Require(t, env)
	a, b := newTenant(t, "rls-write-a"), newTenant(t, "rls-write-b")

	err := db.WithTenant(context.Background(), env.App, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"INSERT INTO contacts (tenant_id, display_name) VALUES ($1, 'intruder')", b)
		return err
	})
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected a postgres error, got %v", err)
	assert.Equal(t, "42501", pgErr.Code, "insufficient_privilege from the RLS WITH CHECK")

	n, _ := countContacts(t, b)
	assert.Equal(t, 1, n)
}

func TestRLS_ScopeDoesNotLeakAcrossPooledConnections(t *testing.T) {
	dbtest.Require(t, env)
	a := newTenant(t, "rls-leak")
	ctx := context.Background()

	// One connection, so the second query reuses the connection the tenant tx used.
	cfg, err := pgxpool.ParseConfig(env.AppURL)
	require.NoError(t, err)
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, db.WithTenant(ctx, pool, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "SELECT 1")
		return err
	}))
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM contacts").Scan(&n))
	assert.Zero(t, n)
}

func TestRLS_EveryTenantTableHasRowLevelSecurity(t *testing.T) {
	dbtest.Require(t, env)
	rows, err := env.Owner.Query(context.Background(), `
		SELECT c.relname FROM pg_class c
		JOIN pg_namespace ns ON ns.oid = c.relnamespace
		WHERE ns.nspname = 'public' AND c.relkind = 'r' AND NOT c.relrowsecurity
		  AND EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped)`)
	require.NoError(t, err)
	missing, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	assert.Empty(t, missing, "tables with tenant_id but no RLS")
}

func TestGrants_AppendOnlyTables(t *testing.T) {
	dbtest.Require(t, env)
	tenant := newTenant(t, "grants")
	ctx := context.Background()

	var convID, msgID, sessionID uuid.UUID
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO conversations (tenant_id, platform, external_thread_id) VALUES ($1, 'messenger', 't1') RETURNING id",
		tenant).Scan(&convID))
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO messages (tenant_id, conversation_id, direction, sender_type, content) VALUES ($1, $2, 'in', 'contact', 'xin chao') RETURNING id",
		tenant, convID).Scan(&msgID))
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO agent_sessions (tenant_id, agent_code) VALUES ($1, 'chat') RETURNING id", tenant).Scan(&sessionID))
	_, err := env.Owner.Exec(ctx, "INSERT INTO agent_traces (tenant_id, session_id, phase) VALUES ($1, $2, 'llm')", tenant, sessionID)
	require.NoError(t, err)

	denied := map[string]string{
		"update message content": "UPDATE messages SET content = 'edited'",
		"delete message":         "DELETE FROM messages",
		"update agent trace":     "UPDATE agent_traces SET phase = 'x'",
		"delete agent trace":     "DELETE FROM agent_traces",
		"write permissions":      "INSERT INTO permissions (code) VALUES ('evil')",
	}
	for name, stmt := range denied {
		t.Run(name, func(t *testing.T) {
			err := db.WithTenant(ctx, env.App, tenant, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, stmt)
				return err
			})
			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "expected permission error, got %v", err)
			assert.Equal(t, "42501", pgErr.Code)
		})
	}

	t.Run("update message delivery metadata", func(t *testing.T) {
		require.NoError(t, db.WithTenant(ctx, env.App, tenant, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "UPDATE messages SET external_message_id = 'm.1', metadata_json = '{\"sent\":true}' WHERE id = $1", msgID)
			return err
		}))
	})
}

func TestGrants_AppCanUseRiverTables(t *testing.T) {
	dbtest.Require(t, env)
	var n int
	require.NoError(t, env.App.QueryRow(context.Background(), "SELECT count(*) FROM river_job").Scan(&n))
}

func TestWithTenant_RejectsNilTenant(t *testing.T) {
	dbtest.Require(t, env)
	err := db.WithTenant(context.Background(), env.App, uuid.Nil, func(pgx.Tx) error { return nil })
	require.Error(t, err)
}
