-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS memory_stage1_jobs (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    rollout_path TEXT NOT NULL DEFAULT '',
    cwd TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'eligible',
    claimed_by TEXT,
    lease_expires_at INTEGER,
    retry_after INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS memory_stage1_outputs (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    raw_memory TEXT NOT NULL DEFAULT '',
    rollout_summary TEXT NOT NULL DEFAULT '',
    rollout_slug TEXT NOT NULL DEFAULT '',
    rollout_summary_file TEXT NOT NULL DEFAULT '',
    used_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS memory_phase2_jobs (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    dirty INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'idle',
    claim_token TEXT,
    lease_expires_at INTEGER,
    retry_after INTEGER NOT NULL DEFAULT 0,
    input_watermark INTEGER NOT NULL DEFAULT 0,
    last_output_watermark INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS memory_registry_entries (
    id TEXT PRIMARY KEY,
    canonical_key TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    source_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
    rollout_summary_file TEXT NOT NULL DEFAULT '',
    stale INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS memory_registry_citations (
    registry_entry_id TEXT NOT NULL REFERENCES memory_registry_entries(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    citation_type TEXT NOT NULL DEFAULT 'thread',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (registry_entry_id, session_id, citation_type)
);

CREATE TABLE IF NOT EXISTS memory_materializations (
    path TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    content TEXT NOT NULL,
    session_id TEXT REFERENCES sessions(id) ON DELETE CASCADE,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS long_horizon_runs (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active',
    runbook_md TEXT NOT NULL DEFAULT '',
    frozen_spec_md TEXT NOT NULL DEFAULT '',
    audit_log TEXT NOT NULL DEFAULT '',
    activated INTEGER NOT NULL DEFAULT 1,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS long_horizon_milestones (
    session_id TEXT NOT NULL REFERENCES long_horizon_runs(session_id) ON DELETE CASCADE,
    milestone_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    condition TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (session_id, milestone_id)
);

CREATE INDEX IF NOT EXISTS idx_memory_stage1_jobs_status_retry ON memory_stage1_jobs (status, retry_after, lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_memory_stage1_outputs_updated_at ON memory_stage1_outputs (updated_at);
CREATE INDEX IF NOT EXISTS idx_memory_registry_entries_updated_at ON memory_registry_entries (updated_at);
CREATE INDEX IF NOT EXISTS idx_memory_registry_entries_source_session ON memory_registry_entries (source_session_id);
CREATE INDEX IF NOT EXISTS idx_memory_materializations_kind ON memory_materializations (kind);
CREATE INDEX IF NOT EXISTS idx_long_horizon_milestones_session_position ON long_horizon_milestones (session_id, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS long_horizon_milestones;
DROP TABLE IF EXISTS long_horizon_runs;
DROP TABLE IF EXISTS memory_materializations;
DROP TABLE IF EXISTS memory_registry_citations;
DROP TABLE IF EXISTS memory_registry_entries;
DROP TABLE IF EXISTS memory_phase2_jobs;
DROP TABLE IF EXISTS memory_stage1_outputs;
DROP TABLE IF EXISTS memory_stage1_jobs;

CREATE TABLE IF NOT EXISTS project_constitution (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS codebase_knowledge (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    symbol_type TEXT NOT NULL,
    signature TEXT,
    documentation TEXT,
    location_range TEXT,
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_codebase_knowledge_file_path ON codebase_knowledge (file_path);
CREATE INDEX IF NOT EXISTS idx_codebase_knowledge_symbol_name ON codebase_knowledge (symbol_name);

CREATE TABLE IF NOT EXISTS structured_summaries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    summary_data TEXT NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_structured_summaries_session_id ON structured_summaries (session_id);
-- +goose StatementEnd
