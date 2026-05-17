-- name: ListRepositories :many
SELECT * FROM repository
WHERE workspace_id = $1 AND status <> 'archived'
ORDER BY created_at DESC;

-- name: GetRepositoryInWorkspace :one
SELECT * FROM repository
WHERE id = $1 AND workspace_id = $2 AND status <> 'archived';

-- name: CreateRepository :one
INSERT INTO repository (
    workspace_id, name, source_state, remote_url, remote_key, default_branch,
    lead_agent_id, created_by, created_by_agent_id, status, metadata
) VALUES (
    $1, $2, $3, sqlc.narg('remote_url'), sqlc.narg('remote_key'), sqlc.narg('default_branch'),
    sqlc.narg('lead_agent_id'), sqlc.narg('created_by'), sqlc.narg('created_by_agent_id'), $4, $5
) RETURNING *;

-- name: UpdateRepository :one
UPDATE repository SET
    name = COALESCE(sqlc.narg('name'), name),
    source_state = COALESCE(sqlc.narg('source_state'), source_state),
    remote_url = sqlc.narg('remote_url'),
    remote_key = sqlc.narg('remote_key'),
    default_branch = sqlc.narg('default_branch'),
    lead_agent_id = sqlc.narg('lead_agent_id'),
    status = COALESCE(sqlc.narg('status'), status),
    metadata = COALESCE(sqlc.narg('metadata'), metadata),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ArchiveRepository :one
UPDATE repository SET status = 'archived', updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ListRepositoryBindings :many
SELECT * FROM repository_binding
WHERE repository_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: GetRepositoryBindingInWorkspace :one
SELECT * FROM repository_binding
WHERE id = $1 AND repository_id = $2 AND workspace_id = $3;

-- name: CreateRepositoryBinding :one
INSERT INTO repository_binding (
    repository_id, workspace_id, owner_user_id, daemon_id, runtime_id,
    machine_label, binding_kind, local_path, state, last_seen_at, metadata
) VALUES (
    $1, $2, sqlc.narg('owner_user_id'), $3, sqlc.narg('runtime_id'),
    $4, $5, $6, $7, sqlc.narg('last_seen_at'), $8
) RETURNING *;

-- name: DeleteRepositoryBinding :exec
DELETE FROM repository_binding
WHERE id = $1 AND repository_id = $2 AND workspace_id = $3;

-- name: ListProjectRepositoryRefs :many
SELECT * FROM project_repository
WHERE project_id = $1
ORDER BY position ASC, created_at ASC;

-- name: CreateProjectRepositoryRef :one
INSERT INTO project_repository (
    project_id, repository_id, workspace_id, role, position
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: DeleteProjectRepositoryRefs :exec
DELETE FROM project_repository
WHERE project_id = $1;

-- name: ListRepositoryOperations :many
SELECT * FROM repository_operation
WHERE repository_id = $1 AND workspace_id = $2
ORDER BY created_at DESC;

-- name: GetRepositoryOperationInWorkspace :one
SELECT * FROM repository_operation
WHERE id = $1 AND workspace_id = $2;

-- name: GetRepositoryOperationForDaemon :one
SELECT * FROM repository_operation
WHERE id = $1 AND workspace_id = $2 AND target_daemon_id = $3;

-- name: CreateRepositoryOperation :one
INSERT INTO repository_operation (
    repository_id, workspace_id, operation_type, status, requested_by_type,
    requested_by_id, target_daemon_id, target_runtime_id, binding_id, request
) VALUES (
    $1, $2, $3, $4, $5,
    sqlc.narg('requested_by_id'), sqlc.narg('target_daemon_id'), sqlc.narg('target_runtime_id'),
    sqlc.narg('binding_id'), $6
) RETURNING *;

-- name: ClaimRepositoryOperationForDaemon :one
WITH candidate AS (
    SELECT ro.id FROM repository_operation ro
    WHERE ro.workspace_id = $1
      AND ro.target_daemon_id = $2
      AND ro.status = 'queued'
      AND (
        sqlc.narg('target_runtime_id')::uuid IS NULL
        OR ro.target_runtime_id IS NULL
        OR ro.target_runtime_id = sqlc.narg('target_runtime_id')::uuid
      )
    ORDER BY ro.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE repository_operation op
SET status = 'running', updated_at = now()
FROM candidate
WHERE op.id = candidate.id
RETURNING op.*;

-- name: MarkRepositoryOperationRunning :one
UPDATE repository_operation
SET status = 'running', updated_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND target_daemon_id = $3
  AND status IN ('queued', 'running')
RETURNING *;

-- name: CompleteRepositoryOperation :one
UPDATE repository_operation
SET
    status = 'succeeded',
    binding_id = COALESCE(sqlc.narg('binding_id'), binding_id),
    result = $4,
    error = NULL,
    updated_at = now(),
    completed_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND target_daemon_id = $3
  AND status = 'running'
RETURNING *;

-- name: FailRepositoryOperation :one
UPDATE repository_operation
SET
    status = 'failed',
    result = $4,
    error = $5,
    updated_at = now(),
    completed_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND target_daemon_id = $3
  AND status = 'running'
RETURNING *;

-- name: ListGithubProjectResourceURLsInWorkspace :many
SELECT DISTINCT btrim(resource_ref->>'url') AS url
FROM project_resource
WHERE workspace_id = $1
  AND resource_type = 'github_repo'
  AND btrim(resource_ref->>'url') <> ''
ORDER BY url ASC;

-- name: ListGithubProjectResourceURLsForProject :many
SELECT DISTINCT btrim(resource_ref->>'url') AS url
FROM project_resource
WHERE project_id = $1
  AND resource_type = 'github_repo'
  AND btrim(resource_ref->>'url') <> ''
ORDER BY url ASC;

-- name: ListTaskOutputMetadata :many
SELECT * FROM task_output_metadata
WHERE task_id = $1 AND workspace_id = $2
ORDER BY created_at ASC, filename ASC;

-- name: DeleteTaskOutputMetadataForTask :exec
DELETE FROM task_output_metadata
WHERE task_id = $1 AND workspace_id = $2;

-- name: CreateTaskOutputMetadata :one
INSERT INTO task_output_metadata (
    workspace_id, repository_id, task_id, relative_path, filename,
    kind, size_bytes, mime_type, metadata
) VALUES (
    $1, sqlc.narg('repository_id'), $2, $3, $4,
    $5, sqlc.narg('size_bytes'), sqlc.narg('mime_type'), $6
) RETURNING *;
