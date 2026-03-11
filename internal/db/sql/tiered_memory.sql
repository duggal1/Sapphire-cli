-- name: GetProjectConstitution :one
SELECT * FROM project_constitution WHERE id = ? LIMIT 1;

-- name: UpsertProjectConstitution :one
INSERT INTO project_constitution (id, content, updated_at, created_at)
VALUES (?, ?, strftime('%s', 'now'), strftime('%s', 'now'))
ON CONFLICT(id) DO UPDATE SET
    content = excluded.content,
    updated_at = strftime('%s', 'now')
RETURNING *;

-- name: GetCodebaseKnowledgeByFilePath :many
SELECT * FROM codebase_knowledge WHERE file_path = ?;

-- name: UpsertCodebaseKnowledge :one
INSERT INTO codebase_knowledge (
    id, file_path, symbol_name, symbol_type, signature, documentation, location_range, updated_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))
ON CONFLICT(file_path, symbol_name, symbol_type) DO UPDATE SET
    signature = excluded.signature,
    documentation = excluded.documentation,
    location_range = excluded.location_range,
    updated_at = strftime('%s', 'now')
RETURNING *;

-- name: GetStructuredSummaryBySessionID :one
SELECT * FROM structured_summaries WHERE session_id = ? ORDER BY created_at DESC LIMIT 1;

-- name: CreateStructuredSummary :one
INSERT INTO structured_summaries (id, session_id, summary_data, updated_at, created_at)
VALUES (?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))
RETURNING *;

-- name: ListStructuredSummaries :many
SELECT * FROM structured_summaries ORDER BY created_at DESC LIMIT ?;

-- name: SearchCodebaseKnowledge :many
SELECT * FROM codebase_knowledge
WHERE symbol_name LIKE ? OR documentation LIKE ? OR file_path LIKE ?
ORDER BY updated_at DESC LIMIT ?;
