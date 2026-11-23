// Package store is the PostgreSQL storage of the product service.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Open creates a pool for databaseURL. It does not connect: the first query
// (Migrate or Ping) does, so an unreachable database is reported there.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Migrate applies the migrations (see migrate.go).
func (s *Store) Migrate(ctx context.Context, includeLocal bool) error {
	return Migrate(ctx, s.pool, includeLocal)
}

// Ping checks that the database answers; used by /readyz.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes every connection in the pool.
func (s *Store) Close() {
	s.pool.Close()
}
