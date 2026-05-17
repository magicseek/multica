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

-- name: CreateRepositoryOperation :one
INSERT INTO repository_operation (
    repository_id, workspace_id, operation_type, status, requested_by_type,
    requested_by_id, target_daemon_id, target_runtime_id, binding_id, request
) VALUES (
    $1, $2, $3, $4, $5,
    sqlc.narg('requested_by_id'), sqlc.narg('target_daemon_id'), sqlc.narg('target_runtime_id'),
    sqlc.narg('binding_id'), $6
) RETURNING *;

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
