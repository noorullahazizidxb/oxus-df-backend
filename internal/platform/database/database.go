// File: internal/platform/database/database.go
// Usage: PostgreSQL connection pool and the transaction helper shared by all repositories.

// Package database owns the pgx pool and the transaction helper. Repositories
// call Queries(ctx): when a transaction was started with WithinTx the queries
// run inside it, otherwise on the pool. This lets a service make several
// modules' writes atomic without any module knowing about the others.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"

	"oxus-df-backend/internal/platform/database/sqlc"
)

const uniqueViolation = "23505"

// txKey is the private context key under which the open transaction is stored.
type txKey struct{}

// NewPool opens a pgx pool and verifies connectivity.
func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	// Money columns are NUMERIC; map them to shopspring/decimal, never float64.
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// DB wraps the pool for repositories and services.
type DB struct {
	pool *pgxpool.Pool
}

// New wraps a pgx pool in a DB.
func New(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

// Ping checks the database is reachable (used by /readyz).
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

// Queries returns sqlc queries bound to the transaction in ctx, or to the pool.
func (d *DB) Queries(ctx context.Context) *sqlc.Queries {
	if tx, ok := TxFromContext(ctx); ok {
		return sqlc.New(tx)
	}
	return sqlc.New(d.pool)
}

// WithinTx runs fn in a transaction. The ctx passed to fn carries the
// transaction; returning an error rolls it back. Nested calls join the outer
// transaction.
func (d *DB) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// TxFromContext returns the active transaction, if any. Used by the job
// publisher so River jobs are enqueued atomically with the business write.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// IsUniqueViolation reports whether err is a PostgreSQL unique constraint error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
