-- Workflow definitions

-- name: ListWorkflowDefinitionsByWorkspace :many
SELECT
    wd.*,
    wr.id AS current_revision_id,
    wr.revision_number AS current_revision_number,
    wr.status AS current_revision_status,
    wr.schema AS current_revision_schema,
    wr.created_by AS current_revision_created_by,
    wr.published_at AS current_revision_published_at,
    wr.deprecated_at AS current_revision_deprecated_at,
    wr.created_at AS current_revision_created_at,
    wr.updated_at AS current_revision_updated_at
FROM workflow_definition wd
LEFT JOIN workflow_revision wr ON wr.id = wd.current_published_revision_id
WHERE wd.workspace_id = $1
  AND (sqlc.arg('include_archived')::boolean = true OR wd.archived_at IS NULL)
ORDER BY
    CASE wd.origin WHEN 'system_seeded' THEN 0 ELSE 1 END,
    wd.name ASC;

-- name: ListWorkflowDefinitionsByApplicability :many
SELECT
    wd.*,
    wr.id AS current_revision_id,
    wr.revision_number AS current_revision_number,
    wr.status AS current_revision_status,
    wr.schema AS current_revision_schema,
    wr.created_by AS current_revision_created_by,
    wr.published_at AS current_revision_published_at,
    wr.deprecated_at AS current_revision_deprecated_at,
    wr.created_at AS current_revision_created_at,
    wr.updated_at AS current_revision_updated_at
FROM workflow_definition wd
JOIN workflow_revision wr ON wr.id = wd.current_published_revision_id
WHERE wd.workspace_id = $1
  AND wd.archived_at IS NULL
  AND (wr.schema->'applicability') ? sqlc.arg('applicability')::text
ORDER BY
    CASE wd.origin WHEN 'system_seeded' THEN 0 ELSE 1 END,
    wd.name ASC;

-- name: GetWorkflowDefinitionInWorkspace :one
SELECT * FROM workflow_definition
WHERE id = $1 AND workspace_id = $2;

-- name: GetWorkflowDefinitionWithCurrentRevision :one
SELECT
    wd.*,
    wr.id AS current_revision_id,
    wr.revision_number AS current_revision_number,
    wr.status AS current_revision_status,
    wr.schema AS current_revision_schema,
    wr.created_by AS current_revision_created_by,
    wr.published_at AS current_revision_published_at,
    wr.deprecated_at AS current_revision_deprecated_at,
    wr.created_at AS current_revision_created_at,
    wr.updated_at AS current_revision_updated_at
FROM workflow_definition wd
LEFT JOIN workflow_revision wr ON wr.id = wd.current_published_revision_id
WHERE wd.id = $1 AND wd.workspace_id = $2;

-- name: GetWorkflowDefinitionBySystemKey :one
SELECT * FROM workflow_definition
WHERE workspace_id = $1 AND system_key = $2;

-- name: UpsertSystemWorkflowDefinition :one
INSERT INTO workflow_definition (
    workspace_id, name, description, origin, system_key
)
VALUES ($1, $2, $3, 'system_seeded', $4)
ON CONFLICT (workspace_id, system_key) WHERE system_key IS NOT NULL
DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = now()
RETURNING *;

-- name: CreateUserWorkflowDefinition :one
INSERT INTO workflow_definition (
    workspace_id, name, description, origin, forked_from_definition_id, created_by
)
VALUES ($1, $2, $3, 'user', sqlc.narg('forked_from_definition_id'), $4)
RETURNING *;

-- name: UpdateWorkflowDefinitionMetadata :one
UPDATE workflow_definition SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND origin = 'user'
RETURNING *;

-- name: ArchiveWorkflowDefinition :one
UPDATE workflow_definition SET
    archived_at = now(),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND origin = 'user'
RETURNING *;

-- name: DeleteDraftOnlyWorkflowDefinition :exec
DELETE FROM workflow_definition
WHERE id = $1
  AND workspace_id = $2
  AND origin = 'user'
  AND current_published_revision_id IS NULL;

-- name: GetWorkflowRevision :one
SELECT * FROM workflow_revision
WHERE id = $1;

-- name: GetCurrentWorkflowRevision :one
SELECT wr.* FROM workflow_revision wr
JOIN workflow_definition wd ON wd.current_published_revision_id = wr.id
WHERE wd.id = $1 AND wd.workspace_id = $2;

-- name: GetLatestDraftWorkflowRevision :one
SELECT wr.* FROM workflow_revision wr
JOIN workflow_definition wd ON wd.id = wr.workflow_definition_id
WHERE wd.id = $1 AND wd.workspace_id = $2 AND wr.status = 'draft'
ORDER BY wr.revision_number DESC
LIMIT 1;

-- name: GetNextWorkflowRevisionNumber :one
SELECT COALESCE(MAX(revision_number), 0)::int + 1 AS next_revision_number
FROM workflow_revision
WHERE workflow_definition_id = $1;

-- name: CreateWorkflowRevision :one
INSERT INTO workflow_revision (
    workflow_definition_id, revision_number, status, schema, created_by, published_at
)
VALUES (
    $1, $2, $3, $4, sqlc.narg('created_by'),
    CASE WHEN $3 = 'published' THEN now() ELSE NULL END
)
RETURNING *;

-- name: UpdateWorkflowRevisionSchema :one
UPDATE workflow_revision SET
    schema = $2,
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: DeleteWorkflowDraftRevision :exec
DELETE FROM workflow_revision
WHERE id = $1 AND status = 'draft';

-- name: PublishWorkflowRevision :one
UPDATE workflow_revision SET
    status = 'published',
    published_at = COALESCE(published_at, now()),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeprecateOtherPublishedWorkflowRevisions :exec
UPDATE workflow_revision SET
    status = 'deprecated',
    deprecated_at = COALESCE(deprecated_at, now()),
    updated_at = now()
WHERE workflow_definition_id = $1
  AND id <> $2
  AND status = 'published';

-- name: UpsertSystemWorkflowRevision :one
INSERT INTO workflow_revision (
    workflow_definition_id, revision_number, status, schema, published_at
)
VALUES ($1, 1, 'published', $2, now())
ON CONFLICT (workflow_definition_id, revision_number)
DO UPDATE SET
    status = 'published',
    schema = EXCLUDED.schema,
    published_at = COALESCE(workflow_revision.published_at, now()),
    deprecated_at = NULL,
    updated_at = now()
RETURNING *;

-- name: SetWorkflowCurrentPublishedRevision :one
UPDATE workflow_definition SET
    current_published_revision_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetWorkspaceWorkflowDefaultsIfNull :one
UPDATE workspace SET
    default_assignment_workflow_definition_id = COALESCE(default_assignment_workflow_definition_id, $2),
    default_comment_workflow_definition_id = COALESCE(default_comment_workflow_definition_id, $3),
    updated_at = CASE
        WHEN default_assignment_workflow_definition_id IS NULL
          OR default_comment_workflow_definition_id IS NULL
        THEN now()
        ELSE updated_at
    END
WHERE id = $1
RETURNING *;
