// Package db opens Postgres pools and scopes work to a tenant.
//
// Services connect as a member of pharmachat_app, which is subject to row-level
// security. Every tenant-scoped query must run inside WithTenant, which sets
// app.tenant_id for the transaction only; outside it, RLS returns no rows.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a pool and verifies connectivity.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s@%s/%s: %w", cfg.ConnConfig.User, cfg.ConnConfig.Host, cfg.ConnConfig.Database, err)
	}
	return pool, nil
}

// Beginner is satisfied by *pgxpool.Pool and pgx.Conn.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTenant runs fn in a transaction whose RLS scope is tenantID. The setting is
// transaction-local, so a pooled connection never carries it to another request.
func WithTenant(ctx context.Context, b Beginner, tenantID uuid.UUID, fn func(pgx.Tx) error) (err error) {
	if tenantID == uuid.Nil {
		return errors.New("with tenant: tenant id is empty")
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
		return fmt.Errorf("set tenant scope: %w", err)
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant tx: %w", err)
	}
	return nil
}
