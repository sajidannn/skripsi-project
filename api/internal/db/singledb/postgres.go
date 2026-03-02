// Package singledb provides a single shared PostgreSQL connection pool used
// when the API runs in "single" DB mode (all tenants share one database).
package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates and validates a pgxpool connection from the given DSN.
//
// Example DSN:
//
//	postgres://user:password@localhost:5432/pos_db?sslmode=disable
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("singledb: SINGLE_DSN is not set")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("singledb: failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("singledb: failed to ping database: %w", err)
	}

	return pool, nil
}
