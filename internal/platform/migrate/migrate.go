// Package migrate applies the schema: goose migrations from /migrations, then River's
// job-queue tables, then grants the River tables to pharmachat_app.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/ngoctb61/pharmachat-backend/migrations"
)

// River creates its tables after our migrations, so the default grants in
// 00001_init.sql do not cover them.
const grantRiver = `
DO $$
DECLARE t text;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables
             WHERE schemaname = 'public' AND tablename LIKE 'river\_%' AND tablename <> 'river_migration'
    LOOP
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO pharmachat_app', t);
    END LOOP;
END
$$;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO pharmachat_app;`

func provider(pool *pgxpool.Pool) (*goose.Provider, *sql.DB, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("init goose: %w", err)
	}
	return p, sqlDB, nil
}

// Up applies every pending migration. It is idempotent.
func Up(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply goose migrations: %w", err)
	}
	for _, r := range results {
		log.InfoContext(ctx, "migration applied", "version", r.Source.Version, "file", r.Source.Path, "duration_ms", r.Duration.Milliseconds())
	}

	rm, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("init river migrator: %w", err)
	}
	res, err := rm.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("apply river migrations: %w", err)
	}
	for _, v := range res.Versions {
		log.InfoContext(ctx, "river migration applied", "version", v.Version)
	}

	if _, err := pool.Exec(ctx, grantRiver); err != nil {
		return fmt.Errorf("grant river tables to pharmachat_app: %w", err)
	}
	return nil
}

// Down rolls back the most recent goose migration. River tables are left alone.
func Down(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	r, err := p.Down(ctx)
	if err != nil {
		return fmt.Errorf("roll back goose migration: %w", err)
	}
	log.InfoContext(ctx, "migration rolled back", "version", r.Source.Version, "file", r.Source.Path)
	return nil
}

// Status logs whether each goose migration is applied.
func Status(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	for _, s := range statuses {
		log.InfoContext(ctx, "migration status", "version", s.Source.Version, "file", s.Source.Path, "state", string(s.State))
	}
	return nil
}
