-- +goose Up
-- +goose StatementBegin

-- Project Constitution (Tier 1 - Hot Memory)
CREATE TABLE IF NOT EXISTS project_constitution (
    id TEXT PRIMARY KEY, -- usually 'default' or project-specific
    content TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

-- Codebase Knowledge (Tier 3 - Cold Memory, persistent semantic index)
CREATE TABLE IF NOT EXISTS codebase_knowledge (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    symbol_type TEXT NOT NULL, -- function, struct, interface, etc.
    signature TEXT,
    documentation TEXT,
    location_range TEXT, -- JSON range {start: {line, col}, end: {line, col}}
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(file_path, symbol_name, symbol_type)
);

CREATE INDEX IF NOT EXISTS idx_codebase_knowledge_file_path ON codebase_knowledge (file_path);
CREATE INDEX IF NOT EXISTS idx_codebase_knowledge_symbol_name ON codebase_knowledge (symbol_name);

-- Structured Summaries (Semantic compression of history)
CREATE TABLE IF NOT EXISTS structured_summaries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    summary_data TEXT NOT NULL, -- JSON structured summary
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_structured_summaries_session_id ON structured_summaries (session_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS structured_summaries;
DROP TABLE IF EXISTS codebase_knowledge;
DROP TABLE IF EXISTS project_constitution;
-- +goose StatementEnd
