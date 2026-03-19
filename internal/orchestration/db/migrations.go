package orchestrationdb

import (
	"context"
	"database/sql"
	"fmt"
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS agent_mail (
		id TEXT PRIMARY KEY,
		to_agent TEXT NOT NULL,
		from_agent TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 0,
		thread_id TEXT NOT NULL,
		read INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		read_at INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_agent_mail_to_created_at ON agent_mail(to_agent, created_at DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_agent_mail_thread_id ON agent_mail(thread_id, created_at ASC);`,
	`CREATE TABLE IF NOT EXISTS agent_state (
		agent_id TEXT PRIMARY KEY,
		role TEXT NOT NULL,
		status TEXT NOT NULL,
		session_id TEXT NOT NULL,
		worktree_path TEXT NOT NULL DEFAULT '',
		branch TEXT NOT NULL DEFAULT '',
		parent_agent_id TEXT NOT NULL DEFAULT '',
		last_heartbeat INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_agent_state_status ON agent_state(status, updated_at DESC);`,
	`CREATE TABLE IF NOT EXISTS agent_activity (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		details_json TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_agent_activity_agent_created_at ON agent_activity(agent_id, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS work_items (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		assignee TEXT NOT NULL DEFAULT '',
		parent_id TEXT NOT NULL DEFAULT '',
		dependencies TEXT NOT NULL DEFAULT '[]',
		created_at INTEGER NOT NULL,
		closed_at INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_work_items_assignee_status ON work_items(assignee, status, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS dispatch_queue (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		work_item_id TEXT NOT NULL DEFAULT '',
		target_scope TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 2,
		payload_json TEXT NOT NULL DEFAULT '{}',
		retry_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		available_at INTEGER NOT NULL,
		leased_by TEXT NOT NULL DEFAULT '',
		leased_at INTEGER NOT NULL DEFAULT 0,
		assigned_agent_id TEXT NOT NULL DEFAULT '',
		submission_id TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_dispatch_queue_status_available ON dispatch_queue(status, available_at, priority, created_at);`,
	`CREATE INDEX IF NOT EXISTS idx_dispatch_queue_session_status ON dispatch_queue(session_id, status, updated_at DESC);`,
	`CREATE TABLE IF NOT EXISTS session_checkpoints (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		work_item_id TEXT NOT NULL DEFAULT '',
		summary_json TEXT NOT NULL DEFAULT '{}',
		audit_tail TEXT NOT NULL DEFAULT '',
		mail_cursor INTEGER NOT NULL DEFAULT 0,
		activity_cursor INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_session_checkpoints_session_agent_created ON session_checkpoints(session_id, agent_id, created_at DESC);`,
}

func ensureSchema(ctx context.Context, conn *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply orchestration schema: %w", err)
		}
	}
	if err := ensureColumn(ctx, conn, "agent_state", "hook_bead_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "agent_state", "created_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(ctx context.Context, conn *sql.DB, tableName, columnName, columnDef string) error {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return fmt.Errorf("inspect columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			valueType string
			notNull   int
			defaultV  sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &valueType, &notNull, &defaultV, &pk); err != nil {
			return fmt.Errorf("scan column metadata for %s: %w", tableName, err)
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate column metadata for %s: %w", tableName, err)
	}

	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	if _, err := conn.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add column %s.%s: %w", tableName, columnName, err)
	}
	return nil
}
