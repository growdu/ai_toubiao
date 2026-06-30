// Package db provides pgxpool helpers with tenant-aware query helpers.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds database connection settings.
type Config struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxConns:        20,
		MinConns:        2,
		MaxConnLifetime: 1 * time.Hour,
		MaxConnIdleTime: 10 * time.Minute,
	}
}

// New creates a new pgxpool with the given config.
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// InTx runs fn in a transaction with automatic rollback on error.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // safe no-op after commit

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}