-- name: GetLongHorizonRun :one
SELECT session_id, status, runbook_md, frozen_spec_md, audit_log, activated, updated_at, created_at
FROM long_horizon_runs
WHERE session_id = ?;

-- name: UpsertLongHorizonRun :one
INSERT INTO long_horizon_runs (
    session_id,
    status,
    runbook_md,
    frozen_spec_md,
    audit_log,
    activated,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    unixepoch(),
    unixepoch()
)
ON CONFLICT(session_id) DO UPDATE SET
    status = excluded.status,
    runbook_md = excluded.runbook_md,
    frozen_spec_md = excluded.frozen_spec_md,
    audit_log = excluded.audit_log,
    activated = excluded.activated,
    updated_at = unixepoch()
RETURNING session_id, status, runbook_md, frozen_spec_md, audit_log, activated, updated_at, created_at;

-- name: ListLongHorizonMilestones :many
SELECT session_id, milestone_id, position, name, condition, status, updated_at, created_at
FROM long_horizon_milestones
WHERE session_id = ?
ORDER BY position ASC, created_at ASC;

-- name: ReplaceLongHorizonMilestones :exec
DELETE FROM long_horizon_milestones
WHERE session_id = ?;

-- name: UpsertLongHorizonMilestone :exec
INSERT INTO long_horizon_milestones (
    session_id,
    milestone_id,
    position,
    name,
    condition,
    status,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    unixepoch(),
    unixepoch()
)
ON CONFLICT(session_id, milestone_id) DO UPDATE SET
    position = excluded.position,
    name = excluded.name,
    condition = excluded.condition,
    status = excluded.status,
    updated_at = unixepoch();
