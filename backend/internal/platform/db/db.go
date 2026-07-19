// Package db is the tenant-aware Postgres access layer (ADR-07 Phase 1).
//
// It provides three things the modules wire against:
//
//   - NewPool: a pgxpool configured for PgBouncer transaction pooling.
//   - BeginTenantScope + WithTx/TxFrom: a per-request transaction that pins
//     app.current_tenant (RLS reads this GUC in a later increment; today it is
//     set but unenforced because the app connects as a superuser).
//   - Conn: a context-aware sqlc DBTX that routes each query onto the request
//     transaction when one is bound, else the pool. One Conn is handed to every
//     module's repository NewAdapter; when no request tx is present it behaves
//     exactly like running on the raw pool.
//
// This package holds NO business logic (platform layer) and imports no module.
package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds a pgx pool configured for PgBouncer transaction-pooling mode.
// QueryExecModeExec disables pgx's implicit prepared-statement cache, which is
// incompatible with transaction pooling (ADR-07 §3 / ADR-03). It is harmless on
// a direct postgres:5432 connection, so it is safe to set unconditionally.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: new pool: %w", err)
	}
	return pool, nil
}

// txKey carries the per-request tenant transaction in the context.
type txKey struct{}

// WithTx returns a context carrying tx. Queries run through a Conn on that
// context execute on tx instead of the pool.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFrom returns the request tx bound by WithTx, if any.
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// DB wraps the pool and provides the tenant-scoping helpers. One per process.
type DB struct {
	pool *pgxpool.Pool
}

// New wraps an existing pool.
func New(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

// Pool exposes the raw pool for liveness checks (healthz Ping). Do NOT run
// tenant-scoped queries directly on it — use Conn so the request tx is honoured.
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Conn returns the context-aware DBTX to hand to every repository NewAdapter.
// It structurally satisfies each module's (identical) sqlc DBTX interface.
func (d *DB) Conn() *Conn { return &Conn{pool: d.pool} }

// BeginTenantScope opens a transaction and pins app.current_tenant on it.
// set_config(..., true) is transaction-local, so it is correct under PgBouncer
// transaction pooling and is discarded at COMMIT/ROLLBACK. Bind the returned tx
// into the request context with WithTx; the caller owns commit/rollback.
func (d *DB) BeginTenantScope(ctx context.Context, orgID uuid.UUID) (pgx.Tx, error) {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("db: begin tenant scope: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", orgID.String()); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("db: set tenant guc: %w", err)
	}
	return tx, nil
}

// RunInTx runs fn inside a transaction. If the request already opened a tenant
// tx (WithTx), fn runs on a savepoint of it so multi-statement work stays on the
// tenant-scoped connection; otherwise a fresh pool tx is opened. Used by the
// bank/comic/journal adapters for their multi-statement methods.
func (d *DB) RunInTx(ctx context.Context, fn func(pgx.Tx) error) error {
	if outer, ok := TxFrom(ctx); ok {
		sp, err := outer.Begin(ctx) // savepoint on the request tx
		if err != nil {
			return err
		}
		if err := fn(sp); err != nil {
			_ = sp.Rollback(ctx)
			return err
		}
		return sp.Commit(ctx)
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Conn is a context-aware sqlc DBTX: each query dispatches to the request tx
// (WithTx) when present, else the pool.
type Conn struct {
	pool *pgxpool.Pool
}

func (c *Conn) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if tx, ok := TxFrom(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	return c.pool.Exec(ctx, sql, args...)
}

func (c *Conn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if tx, ok := TxFrom(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return c.pool.Query(ctx, sql, args...)
}

func (c *Conn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if tx, ok := TxFrom(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return c.pool.QueryRow(ctx, sql, args...)
}
