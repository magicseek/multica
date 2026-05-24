-- name: CreateTaskBundle :one
INSERT INTO task_bundle (
    workspace_id,
    agent_id,
    runtime_id,
    status,
    changeset_mode,
    max_items,
    runtime_budget_seconds,
    rerun_of_bundle_id,
    rerun_scope,
    created_by
) VALUES (
    $1,
    $2,
    $3,
    'queued',
    $4,
    $5,
    $6,
    sqlc.narg('rerun_of_bundle_id'),
    COALESCE(sqlc.narg('rerun_scope'), '[]'::jsonb),
    sqlc.narg('created_by')
)
RETURNING *;

-- name: GetTaskBundle :one
SELECT * FROM task_bundle
WHERE id = $1;

-- name: GetTaskBundleForTask :one
SELECT tb.* FROM task_bundle tb
JOIN agent_task_queue atq ON atq.task_bundle_id = tb.id
WHERE atq.id = $1;

-- name: GetTaskForBundle :one
SELECT * FROM agent_task_queue
WHERE task_bundle_id = $1;

-- name: ListTaskBundlesByIssue :many
SELECT DISTINCT tb.* FROM task_bundle tb
JOIN task_bundle_item tbi ON tbi.bundle_id = tb.id
WHERE tbi.issue_id = $1
ORDER BY tb.created_at DESC;

-- name: ListTaskBundleItems :many
SELECT * FROM task_bundle_item
WHERE bundle_id = $1
ORDER BY position ASC;

-- name: GetTaskBundleItem :one
SELECT * FROM task_bundle_item
WHERE id = $1;

-- name: CreateTaskBundleItem :one
INSERT INTO task_bundle_item (
    bundle_id,
    issue_id,
    position,
    output_namespace
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: StartTaskBundle :one
UPDATE task_bundle
SET status = 'running',
    updated_at = now()
WHERE id = $1
  AND status IN ('queued', 'running')
RETURNING *;

-- name: StartNextTaskBundleItem :one
UPDATE task_bundle_item
SET status = 'in_progress',
    started_at = COALESCE(started_at, now()),
    updated_at = now()
WHERE id = (
    SELECT next_item.id FROM task_bundle_item AS next_item
    WHERE next_item.bundle_id = $1
      AND next_item.status = 'queued'
      AND NOT EXISTS (
          SELECT 1 FROM task_bundle_item AS active_item
          WHERE active_item.bundle_id = $1
            AND active_item.status = 'in_progress'
      )
    ORDER BY next_item.position ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CheckpointTaskBundleItem :one
UPDATE task_bundle_item
SET status = $2,
    result = sqlc.narg('result'),
    error = sqlc.narg('error'),
    checkpoint_seq = sqlc.narg('checkpoint_seq'),
    completed_at = now(),
    updated_at = now()
WHERE id = $1
  AND status IN ('queued', 'in_progress')
RETURNING *;

-- name: FinishUnfinishedTaskBundleItems :many
UPDATE task_bundle_item
SET status = $2,
    error = sqlc.narg('error'),
    completed_at = now(),
    updated_at = now()
WHERE bundle_id = $1
  AND status IN ('queued', 'in_progress')
RETURNING *;

-- name: CompleteTaskBundleIfDone :one
UPDATE task_bundle tb
SET status = CASE
        WHEN EXISTS (
            SELECT 1 FROM task_bundle_item i
            WHERE i.bundle_id = tb.id
              AND i.status IN ('failed', 'blocked', 'input_needed')
        ) THEN 'blocked'
        ELSE 'completed'
    END,
    completed_at = now(),
    updated_at = now()
WHERE tb.id = $1
  AND NOT EXISTS (
      SELECT 1 FROM task_bundle_item i
      WHERE i.bundle_id = tb.id
        AND i.status IN ('queued', 'in_progress')
  )
RETURNING *;

-- name: FailTaskBundle :one
UPDATE task_bundle
SET status = 'failed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1
  AND status IN ('queued', 'running')
RETURNING *;
