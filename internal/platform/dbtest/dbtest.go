// Package dbtest starts a throwaway pgvector Postgres for integration tests, applies
// the real migrations, and exposes two pools: the schema owner (bypasses RLS, like
// pcctl) and a login role in pharmachat_app (subject to RLS, like the services).
package dbtest

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/migrate"
)

// Image matches deploy/docker-compose.yml.
const Image = "pgvector/pgvector:pg17"

const (
	ownerUser   = "pharmachat_owner"
	appUser     = "pharmachat_api"
	appPassword = "test-app-password"
)

// Env is a migrated database.
type Env struct {
	Owner     *pgxpool.Pool
	App       *pgxpool.Pool
	OwnerURL  string
	AppURL    string
	container testcontainers.Container
}

// Start launches the container. Set migrated to false to get an empty database.
func Start(ctx context.Context, migrated bool) (*Env, error) {
	c, err := postgres.Run(ctx, Image,
		postgres.WithDatabase("pharmachat"),
		postgres.WithUsername(ownerUser),
		postgres.WithPassword("test-owner-password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}
	env := &Env{container: c}
	if env.OwnerURL, err = c.ConnectionString(ctx, "sslmode=disable"); err != nil {
		env.Close()
		return nil, fmt.Errorf("container connection string: %w", err)
	}
	if env.Owner, err = db.Open(ctx, env.OwnerURL); err != nil {
		env.Close()
		return nil, err
	}
	if !migrated {
		return env, nil
	}
	if err := env.Migrate(ctx); err != nil {
		env.Close()
		return nil, err
	}
	return env, nil
}

// Migrate applies migrations and opens the RLS-bound App pool.
func (e *Env) Migrate(ctx context.Context) error {
	if err := migrate.Up(ctx, e.Owner, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		return err
	}
	if _, err := e.Owner.Exec(ctx, fmt.Sprintf(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
			CREATE ROLE %s LOGIN PASSWORD '%s' IN ROLE pharmachat_app;
		END IF; END $$`, appUser, appUser, appPassword)); err != nil {
		return fmt.Errorf("create app login role: %w", err)
	}
	u, err := url.Parse(e.OwnerURL)
	if err != nil {
		return fmt.Errorf("parse owner url: %w", err)
	}
	u.User = url.UserPassword(appUser, appPassword)
	e.AppURL = u.String()
	if e.App == nil {
		if e.App, err = db.Open(ctx, e.AppURL); err != nil {
			return err
		}
	}
	return nil
}

// Close stops the container and pools.
func (e *Env) Close() {
	if e.App != nil {
		e.App.Close()
	}
	if e.Owner != nil {
		e.Owner.Close()
	}
	if e.container != nil {
		_ = testcontainers.TerminateContainer(e.container)
	}
}

// Run is a TestMain helper: it skips integration tests under -short, otherwise starts
// a migrated database, runs the package's tests, and tears the container down.
func Run(m *testing.M, env **Env) int {
	flag.Parse()
	if testing.Short() {
		return m.Run()
	}
	e, err := Start(context.Background(), true)
	if err != nil {
		fmt.Println("dbtest:", err)
		return 1
	}
	defer e.Close()
	*env = e
	return m.Run()
}

// Require skips the test when no database was started (-short).
func Require(t *testing.T, env *Env) {
	t.Helper()
	if env == nil {
		t.Skip("integration test: needs Docker, skipped with -short")
	}
}
