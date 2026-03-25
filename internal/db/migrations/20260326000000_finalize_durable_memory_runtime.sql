-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS memory_provenance (
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

CREATE INDEX IF NOT EXISTS idx_memory_provenance_session_created
    ON memory_provenance(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_provenance_scope_file_symbol
    ON memory_provenance(repo_scope_id, file_path, symbol_key, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_fact_provenance (
    fact_kind TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    provenance_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (fact_kind, fact_id, provenance_id),
    FOREIGN KEY(provenance_id) REFERENCES memory_provenance(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_memory_fact_provenance_fact
    ON memory_fact_provenance(fact_kind, fact_id, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_subagent_reports (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    parent_session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL,
    assignment_id TEXT NOT NULL DEFAULT '',
    submission_id TEXT NOT NULL DEFAULT '',
    repo_scope_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    progress TEXT NOT NULL DEFAULT '',
    risks TEXT NOT NULL DEFAULT '',
    blockers TEXT NOT NULL DEFAULT '',
    next_action TEXT NOT NULL DEFAULT '',
    files_json TEXT NOT NULL DEFAULT '[]',
    commands_json TEXT NOT NULL DEFAULT '[]',
    touched_symbols_json TEXT NOT NULL DEFAULT '[]',
    raw_result TEXT NOT NULL DEFAULT '',
    artifact_path TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE SET DEFAULT
);

CREATE INDEX IF NOT EXISTS idx_memory_subagent_reports_session_created
    ON memory_subagent_reports(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_subagent_reports_parent_created
    ON memory_subagent_reports(parent_session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_subagent_reports_agent_status
    ON memory_subagent_reports(agent_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_findings (
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

CREATE INDEX IF NOT EXISTS idx_memory_findings_session_kind_created
    ON memory_findings(session_id, kind, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_findings_scope_file
    ON memory_findings(repo_scope_id, file_path, kind, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_resume_points (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    repo_scope_id TEXT NOT NULL DEFAULT '',
    handoff_id TEXT NOT NULL DEFAULT '',
    boot_packet_artifact_path TEXT NOT NULL DEFAULT '',
    handoff_artifact_path TEXT NOT NULL DEFAULT '',
    continuation_prompt TEXT NOT NULL DEFAULT '',
    original_prompt TEXT NOT NULL DEFAULT '',
    resume_reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at INTEGER NOT NULL,
    resumed_at INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(repo_scope_id) REFERENCES memory_repo_scopes(id) ON DELETE SET DEFAULT,
    FOREIGN KEY(handoff_id) REFERENCES memory_handoffs(id) ON DELETE SET DEFAULT
);

CREATE INDEX IF NOT EXISTS idx_memory_resume_points_session_status_created
    ON memory_resume_points(session_id, status, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS memory_resume_points;
DROP TABLE IF EXISTS memory_findings;
DROP TABLE IF EXISTS memory_subagent_reports;
DROP TABLE IF EXISTS memory_fact_provenance;
DROP TABLE IF EXISTS memory_provenance;
-- +goose StatementEnd
