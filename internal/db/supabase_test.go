package db

import (
	"context"
	"os"
	"testing"
)

func TestConnectSupabase(t *testing.T) {
	// Only run if the env var is set to avoid CI failures
	url := os.Getenv("SUPABASE_DATABASE_URL")
	if url == "" {
		t.Skip("SUPABASE_DATABASE_URL not set, skipping connection test")
	}

	ctx := context.Background()
	pool, err := ConnectSupabase(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to Supabase: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping Supabase: %v", err)
	}
}
