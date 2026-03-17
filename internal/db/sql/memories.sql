-- name: EnsureStage1JobForSession :exec
INSERT INTO memory_stage1_jobs (
    session_id,
    rollout_path,
    cwd,
    status,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    'eligible',
    unixepoch(),
    unixepoch()
)
ON CONFLICT(session_id) DO UPDATE SET
    rollout_path = excluded.rollout_path,
    cwd = excluded.cwd,
    updated_at = unixepoch();

-- name: ClaimStage1JobsForStartup :many
UPDATE memory_stage1_jobs
SET status = 'claimed',
    claimed_by = sqlc.arg(claimed_by),
    lease_expires_at = sqlc.arg(lease_expires_at),
    updated_at = sqlc.arg(now)
WHERE session_id IN (
    SELECT session_id
    FROM memory_stage1_jobs
    WHERE status IN ('eligible', 'failed')
      AND (retry_after = 0 OR retry_after <= sqlc.arg(now))
      AND (lease_expires_at IS NULL OR lease_expires_at <= sqlc.arg(now))
    ORDER BY updated_at ASC, created_at ASC
    LIMIT sqlc.arg(limit_count)
)
RETURNING session_id, rollout_path, cwd, status, claimed_by, lease_expires_at, retry_after, updated_at, created_at;

-- name: MarkStage1JobFailed :exec
UPDATE memory_stage1_jobs
SET status = 'failed',
    claimed_by = NULL,
    lease_expires_at = NULL,
    retry_after = ?,
    updated_at = unixepoch()
WHERE session_id = ?;

-- name: MarkStage1JobNoOutput :exec
UPDATE memory_stage1_jobs
SET status = 'succeeded_no_output',
    claimed_by = NULL,
    lease_expires_at = NULL,
    retry_after = 0,
    updated_at = unixepoch()
WHERE session_id = ?;

-- name: MarkStage1JobSucceeded :exec
UPDATE memory_stage1_jobs
SET status = 'succeeded_with_output',
    claimed_by = NULL,
    lease_expires_at = NULL,
    retry_after = 0,
    updated_at = unixepoch()
WHERE session_id = ?;

-- name: UpsertStage1Output :exec
INSERT INTO memory_stage1_outputs (
    session_id,
    source_updated_at,
    raw_memory,
    rollout_summary,
    rollout_slug,
    rollout_summary_file,
    last_usage,
    generated_at,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    NULL,
    unixepoch(),
    unixepoch(),
    unixepoch()
)
ON CONFLICT(session_id) DO UPDATE SET
    source_updated_at = excluded.source_updated_at,
    raw_memory = excluded.raw_memory,
    rollout_summary = excluded.rollout_summary,
    rollout_slug = excluded.rollout_slug,
    rollout_summary_file = excluded.rollout_summary_file,
    generated_at = unixepoch(),
    updated_at = unixepoch();

-- name: DeleteStage1OutputBySessionID :execrows
DELETE FROM memory_stage1_outputs
WHERE session_id = ?;

-- name: GetStage1OutputBySessionID :one
SELECT session_id, source_updated_at, raw_memory, rollout_summary, rollout_slug, rollout_summary_file, usage_count, last_usage, selected_for_phase2, selected_for_phase2_source_updated_at, generated_at, used_at, updated_at, created_at
FROM memory_stage1_outputs
WHERE session_id = ?;

-- name: ListStage1OutputsForPhase2 :many
SELECT session_id, source_updated_at, raw_memory, rollout_summary, rollout_slug, rollout_summary_file, usage_count, last_usage, selected_for_phase2, selected_for_phase2_source_updated_at, generated_at, used_at, updated_at, created_at
FROM memory_stage1_outputs
ORDER BY source_updated_at DESC, session_id DESC;

-- name: ListEligibleStage1OutputsForPhase2 :many
SELECT session_id, source_updated_at, raw_memory, rollout_summary, rollout_slug, rollout_summary_file, usage_count, last_usage, selected_for_phase2, selected_for_phase2_source_updated_at, generated_at, used_at, updated_at, created_at
FROM memory_stage1_outputs
WHERE (length(trim(raw_memory)) > 0 OR length(trim(rollout_summary)) > 0)
  AND (
        (last_usage IS NOT NULL AND last_usage >= sqlc.arg(cutoff))
        OR (last_usage IS NULL AND source_updated_at >= sqlc.arg(cutoff))
  )
ORDER BY usage_count DESC, COALESCE(last_usage, source_updated_at) DESC, source_updated_at DESC, session_id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListPhase2BaselineOutputs :many
SELECT session_id, source_updated_at, raw_memory, rollout_summary, rollout_slug, rollout_summary_file, usage_count, last_usage, selected_for_phase2, selected_for_phase2_source_updated_at, generated_at, used_at, updated_at, created_at
FROM memory_stage1_outputs
WHERE selected_for_phase2 = 1
ORDER BY source_updated_at DESC, session_id DESC;

-- name: PruneStage1OutputsForRetention :execrows
DELETE FROM memory_stage1_outputs
WHERE session_id IN (
    SELECT o.session_id
    FROM memory_stage1_outputs AS o
    WHERE o.selected_for_phase2 = 0
      AND COALESCE(o.last_usage, o.source_updated_at) < ?
    ORDER BY COALESCE(o.last_usage, o.source_updated_at) ASC, o.source_updated_at ASC, o.session_id ASC
    LIMIT ?
);

-- name: RecordStage1OutputUsage :exec
UPDATE memory_stage1_outputs
SET usage_count = usage_count + 1,
    last_usage = unixepoch()
WHERE session_id = ?;

-- name: EnsureGlobalPhase2Job :exec
INSERT INTO memory_phase2_jobs (
    singleton_id,
    dirty,
    status,
    updated_at,
    created_at
) VALUES (
    1,
    1,
    'idle',
    unixepoch(),
    unixepoch()
)
ON CONFLICT(singleton_id) DO NOTHING;

-- name: MarkPhase2Dirty :exec
UPDATE memory_phase2_jobs
SET dirty = 1,
    input_watermark = CASE
        WHEN input_watermark >= unixepoch() THEN input_watermark + 1
        ELSE unixepoch()
    END,
    updated_at = unixepoch()
WHERE singleton_id = 1;

-- name: ClaimGlobalPhase2Job :one
UPDATE memory_phase2_jobs
SET status = 'claimed',
    claim_token = sqlc.arg(claim_token),
    lease_expires_at = sqlc.arg(lease_expires_at),
    dirty = 0,
    updated_at = sqlc.arg(now)
WHERE singleton_id IN (
    SELECT singleton_id
    FROM memory_phase2_jobs
    WHERE singleton_id = 1
      AND input_watermark > last_output_watermark
      AND (retry_after = 0 OR retry_after <= sqlc.arg(now))
      AND (lease_expires_at IS NULL OR lease_expires_at <= sqlc.arg(now))
)
RETURNING singleton_id, dirty, status, claim_token, lease_expires_at, retry_after, input_watermark, last_output_watermark, last_error, updated_at, created_at;

-- name: HeartbeatGlobalPhase2Job :execrows
UPDATE memory_phase2_jobs
SET lease_expires_at = ?,
    updated_at = unixepoch()
WHERE singleton_id = 1
  AND claim_token = ?;

-- name: FailGlobalPhase2Job :execrows
UPDATE memory_phase2_jobs
SET status = 'failed',
    claim_token = NULL,
    lease_expires_at = NULL,
    retry_after = ?,
    last_error = ?,
    updated_at = unixepoch()
WHERE singleton_id = 1
  AND claim_token = ?;

-- name: SucceedGlobalPhase2Job :execrows
UPDATE memory_phase2_jobs
SET status = 'idle',
    claim_token = NULL,
    lease_expires_at = NULL,
    retry_after = 0,
    input_watermark = CASE
        WHEN input_watermark > ? THEN input_watermark
        ELSE ?
    END,
    last_output_watermark = CASE
        WHEN last_output_watermark > ? THEN last_output_watermark
        ELSE ?
    END,
    last_error = '',
    updated_at = unixepoch()
WHERE singleton_id = 1
  AND claim_token = ?;

-- name: GetPhase2InputSelection :many
SELECT session_id, source_updated_at, raw_memory, rollout_summary, rollout_slug, rollout_summary_file, usage_count, last_usage, selected_for_phase2, selected_for_phase2_source_updated_at, generated_at, used_at, updated_at, created_at
FROM memory_stage1_outputs
WHERE updated_at > ?
ORDER BY updated_at DESC;

-- name: ClearPhase2BaselineSelection :exec
UPDATE memory_stage1_outputs
SET selected_for_phase2 = 0,
    selected_for_phase2_source_updated_at = NULL
WHERE selected_for_phase2 = 1;

-- name: MarkStage1OutputSelectedForPhase2 :execrows
UPDATE memory_stage1_outputs
SET selected_for_phase2 = 1,
    selected_for_phase2_source_updated_at = source_updated_at
WHERE session_id = ?
  AND source_updated_at = ?;

-- name: UpsertMemoryRegistryEntry :one
INSERT INTO memory_registry_entries (
    id,
    canonical_key,
    kind,
    title,
    body,
    source_session_id,
    rollout_summary_file,
    stale,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    unixepoch(),
    unixepoch()
)
ON CONFLICT(canonical_key) DO UPDATE SET
    kind = excluded.kind,
    title = excluded.title,
    body = excluded.body,
    source_session_id = excluded.source_session_id,
    rollout_summary_file = excluded.rollout_summary_file,
    stale = excluded.stale,
    updated_at = unixepoch()
RETURNING id, canonical_key, kind, title, body, source_session_id, rollout_summary_file, stale, updated_at, created_at;

-- name: ListMemoryRegistryEntries :many
SELECT id, canonical_key, kind, title, body, source_session_id, rollout_summary_file, stale, updated_at, created_at
FROM memory_registry_entries
ORDER BY updated_at DESC, created_at DESC;

-- name: ReplaceMemoryRegistryCitations :exec
DELETE FROM memory_registry_citations
WHERE registry_entry_id = ?;

-- name: InsertMemoryRegistryCitation :exec
INSERT INTO memory_registry_citations (
    registry_entry_id,
    session_id,
    citation_type,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    unixepoch()
);

-- name: ListMemoryRegistryCitationsByEntry :many
SELECT registry_entry_id, session_id, citation_type, created_at
FROM memory_registry_citations
WHERE registry_entry_id = ?
ORDER BY created_at ASC;

-- name: GetMemorySummaryMaterialization :one
SELECT path, kind, content, session_id, updated_at, created_at
FROM memory_materializations
WHERE kind = 'summary'
LIMIT 1;

-- name: GetMemoryRegistryMaterialization :one
SELECT path, kind, content, session_id, updated_at, created_at
FROM memory_materializations
WHERE kind = 'registry'
LIMIT 1;

-- name: UpsertMemoryMaterialization :one
INSERT INTO memory_materializations (
    path,
    kind,
    content,
    session_id,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    unixepoch(),
    unixepoch()
)
ON CONFLICT(path) DO UPDATE SET
    kind = excluded.kind,
    content = excluded.content,
    session_id = excluded.session_id,
    updated_at = unixepoch()
RETURNING path, kind, content, session_id, updated_at, created_at;

-- name: ListMemoryMaterializations :many
SELECT path, kind, content, session_id, updated_at, created_at
FROM memory_materializations
ORDER BY path ASC;
