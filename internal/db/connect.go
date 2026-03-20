package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

var pragmas = map[string]string{
	"foreign_keys":  "ON",
	"journal_mode":  "WAL",
	"page_size":     "4096",
	"cache_size":    "-8000",
	"synchronous":   "NORMAL",
	"secure_delete": "ON",
	"busy_timeout":  "5000",
}

// Connect opens a SQLite database connection and runs migrations.
func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}
	dbPath := filepath.Join(dataDir, "crush.db")

	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// SQLite performs best with a single shared writer connection in this CLI.
	// Without this, concurrent title/session/message writes can fight over locks
	// and trigger busy_timeout stalls in the live render path.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	goose.SetBaseFS(FS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("Failed to set dialect", "error", err)
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}
	if err := repairLegacySessionMigrationState(ctx, db); err != nil {
		slog.Error("Failed to repair legacy session migration state", "error", err)
		return nil, fmt.Errorf("failed to repair legacy session migration state: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}
	if err := ensureSessionSchema(ctx, db); err != nil {
		slog.Error("Failed to backfill session schema", "error", err)
		return nil, fmt.Errorf("failed to backfill session schema: %w", err)
	}

	return db, nil
}

func repairLegacySessionMigrationState(ctx context.Context, db *sql.DB) error {
	const modeMigrationVersion int64 = 20260317000000

	modeExists, err := tableHasColumn(ctx, db, "sessions", "mode")
	if err != nil {
		return err
	}
	if !modeExists {
		return nil
	}

	applied, err := migrationApplied(ctx, db, modeMigrationVersion)
	if err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "sessions", "worktree_policy", "TEXT DEFAULT 'shared_repo'"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_sessions_mode ON sessions(mode)`); err != nil {
		return fmt.Errorf("create sessions mode index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_sessions_worktree_policy ON sessions(worktree_policy)`); err != nil {
		return fmt.Errorf("create sessions worktree policy index: %w", err)
	}
	if applied {
		return nil
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)`, modeMigrationVersion); err != nil {
		return fmt.Errorf("record repaired migration %d: %w", modeMigrationVersion, err)
	}
	return nil
}

func ensureSessionSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureColumn(ctx, db, "sessions", "mode", "TEXT DEFAULT 'pair_programming'"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_sessions_mode ON sessions(mode)`); err != nil {
		return fmt.Errorf("create sessions mode index: %w", err)
	}
	if err := ensureColumn(ctx, db, "sessions", "worktree_policy", "TEXT DEFAULT 'shared_repo'"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_sessions_worktree_policy ON sessions(worktree_policy)`); err != nil {
		return fmt.Errorf("create sessions worktree policy index: %w", err)
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, tableName, columnName, columnDef string) error {
	hasColumn, err := tableHasColumn(ctx, db, tableName, columnName)
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add %s.%s: %w", tableName, columnName, err)
	}
	return nil
}

func tableHasColumn(ctx context.Context, db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultV, &primaryKey); err != nil {
			return false, fmt.Errorf("scan table info %s: %w", tableName, err)
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info %s: %w", tableName, err)
	}
	return false, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version int64) (bool, error) {
	if _, err := goose.EnsureDBVersionContext(ctx, db); err != nil {
		return false, fmt.Errorf("ensure goose version table: %w", err)
	}
	row := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM goose_db_version WHERE version_id = ? AND is_applied = 1`, version)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("query goose version %d: %w", version, err)
	}
	return count > 0, nil
}
