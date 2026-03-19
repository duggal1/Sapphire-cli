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
}

func ensureSchema(ctx context.Context, conn *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply orchestration schema: %w", err)
		}
	}
	return nil
}
