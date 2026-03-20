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
		convoy_id TEXT NOT NULL DEFAULT '',
		dependencies TEXT NOT NULL DEFAULT '[]',
		created_at INTEGER NOT NULL,
		closed_at INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_work_items_assignee_status ON work_items(assignee, status, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS convoys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		owner TEXT NOT NULL DEFAULT '',
		notify TEXT NOT NULL DEFAULT '',
		merge_strategy TEXT NOT NULL DEFAULT 'direct',
		status TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		closed_at INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_convoys_status_created_at ON convoys(status, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS convoy_tracks (
		convoy_id TEXT NOT NULL,
		work_item_id TEXT NOT NULL,
		added_at INTEGER NOT NULL,
		PRIMARY KEY (convoy_id, work_item_id)
	);`,
	`CREATE INDEX IF NOT EXISTS idx_convoy_tracks_work_item ON convoy_tracks(work_item_id, added_at DESC);`,
	`CREATE TABLE IF NOT EXISTS agent_hooks (
		agent_id TEXT PRIMARY KEY,
		hook_bead_id TEXT NOT NULL DEFAULT '',
		hooked_at INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'idle'
	);`,
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
		parent_checkpoint_id TEXT NOT NULL DEFAULT '',
		message_count INTEGER NOT NULL DEFAULT 0,
		summary_json TEXT NOT NULL DEFAULT '{}',
		audit_tail TEXT NOT NULL DEFAULT '',
		pending_tasks_json TEXT NOT NULL DEFAULT '[]',
		files_modified_json TEXT NOT NULL DEFAULT '[]',
		mail_cursor INTEGER NOT NULL DEFAULT 0,
		activity_cursor INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_session_checkpoints_session_agent_created ON session_checkpoints(session_id, agent_id, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS worktree_runs (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL DEFAULT '',
		agent_id TEXT NOT NULL DEFAULT '',
		parent_agent_id TEXT NOT NULL DEFAULT '',
		kind TEXT NOT NULL DEFAULT '',
		policy TEXT NOT NULL DEFAULT 'shared_repo',
		status TEXT NOT NULL DEFAULT '',
		repo_root TEXT NOT NULL DEFAULT '',
		worktree_path TEXT NOT NULL DEFAULT '',
		branch TEXT NOT NULL DEFAULT '',
		base_ref TEXT NOT NULL DEFAULT '',
		task_key TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		landed_at INTEGER NOT NULL DEFAULT 0,
		removed_at INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_worktree_runs_session_updated ON worktree_runs(session_id, updated_at DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_worktree_runs_agent_updated ON worktree_runs(agent_id, updated_at DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_worktree_runs_status_updated ON worktree_runs(status, updated_at DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_worktree_runs_path ON worktree_runs(worktree_path);`,
	`CREATE TABLE IF NOT EXISTS decisions (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		category TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		confidence TEXT NOT NULL DEFAULT 'tentative',
		source_checkpoint_id TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_decisions_session_created ON decisions(session_id, created_at DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_decisions_category_key_created ON decisions(category, key, created_at DESC);`,
	`CREATE TABLE IF NOT EXISTS user_preferences (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		confidence TEXT NOT NULL DEFAULT 'confirmed',
		source_session_id TEXT NOT NULL DEFAULT '',
		updated_at INTEGER NOT NULL
	);`,
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
	if err := ensureColumn(ctx, conn, "session_checkpoints", "parent_checkpoint_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "session_checkpoints", "message_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "session_checkpoints", "pending_tasks_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "session_checkpoints", "files_modified_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "work_items", "parent_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "work_items", "convoy_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "work_items", "dependencies", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "agent_hooks", "hook_bead_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "agent_hooks", "hooked_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, conn, "agent_hooks", "status", "TEXT NOT NULL DEFAULT 'idle'"); err != nil {
		return err
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_work_items_convoy_status ON work_items(convoy_id, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_hooks_status_hooked_at ON agent_hooks(status, hooked_at DESC);`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply orchestration schema: %w", err)
		}
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
