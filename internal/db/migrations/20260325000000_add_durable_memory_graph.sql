-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS memory_repo_scopes (
    id TEXT PRIMARY KEY,
    repo_root TEXT NOT NULL,
    scope_path TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    head_commit TEXT NOT NULL DEFAULT '',
    dirty INTEGER NOT NULL DEFAULT 0,
    changed_files_json TEXT NOT NULL DEFAULT '[]',
    latest_epoch INTEGER NOT NULL DEFAULT 0,
    last_indexed_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(repo_root, scope_path, branch)
);

CREATE INDEX IF NOT EXISTS idx_memory_repo_scopes_root_branch
    ON memory_repo_scopes(repo_root, branch, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_index_epochs (
    id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    epoch INTEGER NOT NULL,
    head_commit TEXT NOT NULL DEFAULT '',
    changed_files_json TEXT NOT NULL DEFAULT '[]',
    removed_files_json TEXT NOT NULL DEFAULT '[]',
    file_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'ready',
    created_at INTEGER NOT NULL,
    completed_at INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    UNIQUE(scope_id, epoch)
);

CREATE INDEX IF NOT EXISTS idx_memory_index_epochs_scope_epoch
    ON memory_index_epochs(scope_id, epoch DESC);

CREATE TABLE IF NOT EXISTS memory_repo_files (
    id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    path TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'text',
    role TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    content_hash TEXT NOT NULL,
    mod_time_unix INTEGER NOT NULL DEFAULT 0,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    symbol_count INTEGER NOT NULL DEFAULT 0,
    imports_json TEXT NOT NULL DEFAULT '[]',
    facts_json TEXT NOT NULL DEFAULT '{}',
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    UNIQUE(scope_id, path)
);

CREATE INDEX IF NOT EXISTS idx_memory_repo_files_scope_path
    ON memory_repo_files(scope_id, path);

CREATE INDEX IF NOT EXISTS idx_memory_repo_files_scope_hash
    ON memory_repo_files(scope_id, content_hash);

CREATE TABLE IF NOT EXISTS memory_repo_symbols (
    id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    stable_key TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    signature TEXT NOT NULL DEFAULT '',
    doc TEXT NOT NULL DEFAULT '',
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    exported INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    fingerprint TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    FOREIGN KEY(file_id) REFERENCES memory_repo_files(id) ON DELETE CASCADE,
    UNIQUE(scope_id, stable_key)
);

CREATE INDEX IF NOT EXISTS idx_memory_repo_symbols_scope_name
    ON memory_repo_symbols(scope_id, name, kind);

CREATE INDEX IF NOT EXISTS idx_memory_repo_symbols_file
    ON memory_repo_symbols(file_id, start_line);

CREATE TABLE IF NOT EXISTS memory_repo_edges (
    id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    from_file_path TEXT NOT NULL DEFAULT '',
    from_symbol_key TEXT NOT NULL DEFAULT '',
    edge_type TEXT NOT NULL,
    to_file_path TEXT NOT NULL DEFAULT '',
    to_symbol_name TEXT NOT NULL DEFAULT '',
    to_symbol_key TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    UNIQUE(scope_id, from_file_path, from_symbol_key, edge_type, to_file_path, to_symbol_name, to_symbol_key)
);

CREATE INDEX IF NOT EXISTS idx_memory_repo_edges_scope_from
    ON memory_repo_edges(scope_id, from_file_path, from_symbol_key, edge_type);

CREATE INDEX IF NOT EXISTS idx_memory_repo_edges_scope_to
    ON memory_repo_edges(scope_id, to_file_path, to_symbol_name, edge_type);

CREATE TABLE IF NOT EXISTS memory_handoffs (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    repo_scope_id TEXT NOT NULL DEFAULT '',
    checkpoint_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    objective TEXT NOT NULL DEFAULT '',
    plan_json TEXT NOT NULL DEFAULT '[]',
    blockers_json TEXT NOT NULL DEFAULT '[]',
    uncertainties_json TEXT NOT NULL DEFAULT '[]',
    touched_files_json TEXT NOT NULL DEFAULT '[]',
    touched_symbols_json TEXT NOT NULL DEFAULT '[]',
    subagents_json TEXT NOT NULL DEFAULT '[]',
    validation_json TEXT NOT NULL DEFAULT '{}',
    repo_snapshot_json TEXT NOT NULL DEFAULT '{}',
    next_actions_json TEXT NOT NULL DEFAULT '[]',
    artifact_path TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_handoffs_session_created
    ON memory_handoffs(session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_boot_packets (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    repo_scope_id TEXT NOT NULL DEFAULT '',
    task_hash TEXT NOT NULL DEFAULT '',
    artifact_path TEXT NOT NULL DEFAULT '',
    required_reads_json TEXT NOT NULL DEFAULT '[]',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_boot_packets_session_created
    ON memory_boot_packets(session_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS memory_boot_packets;
DROP TABLE IF EXISTS memory_handoffs;
DROP TABLE IF EXISTS memory_repo_edges;
DROP TABLE IF EXISTS memory_repo_symbols;
DROP TABLE IF EXISTS memory_repo_files;
DROP TABLE IF EXISTS memory_index_epochs;
DROP TABLE IF EXISTS memory_repo_scopes;
-- +goose StatementEnd
