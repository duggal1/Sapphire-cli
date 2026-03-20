-- Add mode column to sessions table for Codex plan mode architecture
-- Reference: Codex CLI v0.88.0+ collaboration modes (plan, pair_programming, execute)

-- +goose Up
ALTER TABLE sessions ADD COLUMN mode TEXT DEFAULT 'pair_programming';
CREATE INDEX IF NOT EXISTS idx_sessions_mode ON sessions(mode);
ALTER TABLE sessions ADD COLUMN worktree_policy TEXT DEFAULT 'shared_repo';
CREATE INDEX IF NOT EXISTS idx_sessions_worktree_policy ON sessions(worktree_policy);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_worktree_policy;
ALTER TABLE sessions DROP COLUMN worktree_policy;
DROP INDEX IF EXISTS idx_sessions_mode;
ALTER TABLE sessions DROP COLUMN mode;
