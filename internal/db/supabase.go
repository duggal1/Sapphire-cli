package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectSupabase opens a Supabase (Postgres) connection pool.
// TEST_EDIT_NORMAL
func ConnectSupabase(ctx context.Context) (*pgxpool.Pool, error) {
	connStr := os.Getenv("SUPABASE_DATABASE_URL")
	if connStr == "" {
		return nil, fmt.Errorf("SUPABASE_DATABASE_URL is not set")
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
