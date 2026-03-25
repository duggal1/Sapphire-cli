-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS memory_provenance_new (
    id TEXT PRIMARY KEY,
    repo_scope_id TEXT DEFAULT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    source_kind TEXT NOT NULL,
    artifact_path TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    tool_output_ref TEXT NOT NULL DEFAULT '',
    handoff_id TEXT DEFAULT NULL,
    subagent_report_id TEXT DEFAULT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    symbol_key TEXT NOT NULL DEFAULT '',
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    head_commit TEXT NOT NULL DEFAULT '',
    index_epoch INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    FOREIGN KEY(handoff_id) REFERENCES memory_handoffs(id) ON DELETE SET NULL
);

INSERT INTO memory_provenance_new (
    id, repo_scope_id, session_id, agent_id, source_kind, artifact_path, tool_name, tool_output_ref,
    handoff_id, subagent_report_id, file_path, symbol_key, start_line, end_line, head_commit, index_epoch,
    metadata_json, created_at
)
SELECT
    id,
    NULLIF(repo_scope_id, ''),
    session_id,
    agent_id,
    source_kind,
    artifact_path,
    tool_name,
    tool_output_ref,
    NULLIF(handoff_id, ''),
    NULLIF(subagent_report_id, ''),
    file_path,
    symbol_key,
    start_line,
    end_line,
    head_commit,
    index_epoch,
    metadata_json,
    created_at
FROM memory_provenance;

DROP TABLE memory_provenance;
ALTER TABLE memory_provenance_new RENAME TO memory_provenance;

CREATE INDEX IF NOT EXISTS idx_memory_provenance_session_created
    ON memory_provenance(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_provenance_scope_file_symbol
    ON memory_provenance(repo_scope_id, file_path, symbol_key, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_findings_new (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    repo_scope_id TEXT DEFAULT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    symbol_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    source_report_id TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE SET NULL,
    FOREIGN KEY(source_report_id) REFERENCES memory_subagent_reports(id) ON DELETE SET NULL
);

INSERT INTO memory_findings_new (
    id, session_id, agent_id, repo_scope_id, kind, title, content, file_path, symbol_key, status, source_report_id, created_at, updated_at
)
SELECT
    id,
    session_id,
    agent_id,
    NULLIF(repo_scope_id, ''),
    kind,
    title,
    content,
    file_path,
    symbol_key,
    status,
    NULLIF(source_report_id, ''),
    created_at,
    updated_at
FROM memory_findings;

DROP TABLE memory_findings;
ALTER TABLE memory_findings_new RENAME TO memory_findings;

CREATE INDEX IF NOT EXISTS idx_memory_findings_session_kind_created
    ON memory_findings(session_id, kind, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_findings_scope_file
    ON memory_findings(repo_scope_id, file_path, kind, updated_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS memory_provenance_old (
    id TEXT PRIMARY KEY,
    repo_scope_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    source_kind TEXT NOT NULL,
    artifact_path TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    tool_output_ref TEXT NOT NULL DEFAULT '',
    handoff_id TEXT NOT NULL DEFAULT '',
    subagent_report_id TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    symbol_key TEXT NOT NULL DEFAULT '',
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    head_commit TEXT NOT NULL DEFAULT '',
    index_epoch INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE CASCADE,
    FOREIGN KEY(handoff_id) REFERENCES memory_handoffs(id) ON DELETE SET DEFAULT
);

INSERT INTO memory_provenance_old (
    id, repo_scope_id, session_id, agent_id, source_kind, artifact_path, tool_name, tool_output_ref,
    handoff_id, subagent_report_id, file_path, symbol_key, start_line, end_line, head_commit, index_epoch,
    metadata_json, created_at
)
SELECT
    id,
    COALESCE(repo_scope_id, ''),
    session_id,
    agent_id,
    source_kind,
    artifact_path,
    tool_name,
    tool_output_ref,
    COALESCE(handoff_id, ''),
    COALESCE(subagent_report_id, ''),
    file_path,
    symbol_key,
    start_line,
    end_line,
    head_commit,
    index_epoch,
    metadata_json,
    created_at
FROM memory_provenance;

DROP TABLE memory_provenance;
ALTER TABLE memory_provenance_old RENAME TO memory_provenance;

CREATE INDEX IF NOT EXISTS idx_memory_provenance_session_created
    ON memory_provenance(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_provenance_scope_file_symbol
    ON memory_provenance(repo_scope_id, file_path, symbol_key, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_findings_old (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    repo_scope_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    symbol_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    source_report_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE SET DEFAULT,
    FOREIGN KEY(source_report_id) REFERENCES memory_subagent_reports(id) ON DELETE SET DEFAULT
);

INSERT INTO memory_findings_old (
    id, session_id, agent_id, repo_scope_id, kind, title, content, file_path, symbol_key, status, source_report_id, created_at, updated_at
)
SELECT
    id,
    session_id,
    agent_id,
    COALESCE(repo_scope_id, ''),
    kind,
    title,
    content,
    file_path,
    symbol_key,
    status,
    COALESCE(source_report_id, ''),
    created_at,
    updated_at
FROM memory_findings;

DROP TABLE memory_findings;
ALTER TABLE memory_findings_old RENAME TO memory_findings;

CREATE INDEX IF NOT EXISTS idx_memory_findings_session_kind_created
    ON memory_findings(session_id, kind, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_findings_scope_file
    ON memory_findings(repo_scope_id, file_path, kind, updated_at DESC);
-- +goose StatementEnd
