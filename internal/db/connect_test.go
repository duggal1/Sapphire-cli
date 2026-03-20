package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestConnectBackfillsSessionWorktreePolicy(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "crush.db")

	conn, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	_, err = conn.ExecContext(ctx, `CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		parent_session_id TEXT,
		title TEXT NOT NULL,
		message_count INTEGER NOT NULL DEFAULT 0,
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		cost REAL NOT NULL DEFAULT 0.0,
		updated_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		mode TEXT DEFAULT 'pair_programming'
	);`)
	require.NoError(t, err)
	require.NoError(t, goose.SetDialect("sqlite3"))
	_, err = goose.EnsureDBVersionContext(ctx, conn)
	require.NoError(t, err)
	for _, version := range []int64{
		20250424200609,
		20250515105448,
		20250624000000,
		20250627000000,
		20250810000000,
		20250812000000,
		20260127000000,
		20260309000000,
		20260317000000,
	} {
		_, err = conn.ExecContext(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)`, version)
		require.NoError(t, err)
	}

	require.NoError(t, conn.Close())

	db, err := Connect(ctx, dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	ok, err := tableHasColumn(ctx, db, "sessions", "worktree_policy")
	require.NoError(t, err)
	require.True(t, ok)
}
