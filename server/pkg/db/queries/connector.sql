-- =====================
-- Workspace Connectors
-- =====================

-- name: ListWorkspaceConnectors :many
SELECT * FROM workspace_connector
WHERE workspace_id = $1
ORDER BY provider_id ASC;

-- name: GetWorkspaceConnector :one
SELECT * FROM workspace_connector
WHERE workspace_id = $1 AND provider_id = $2;

-- name: UpsertWorkspaceConnector :one
INSERT INTO workspace_connector (
    workspace_id, provider_id, enabled, settings, created_by
) VALUES (
    $1, $2, $3, $4, sqlc.narg('created_by')
)
ON CONFLICT (workspace_id, provider_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    settings = EXCLUDED.settings,
    updated_at = now()
RETURNING *;

-- =====================
-- Connector Credentials
-- =====================

-- name: GetConnectorCredential :one
SELECT * FROM connector_credential
WHERE workspace_id = $1 AND provider_id = $2 AND owner_user_id = $3;

-- name: ListConnectorCredentialsByWorkspace :many
SELECT * FROM connector_credential
WHERE workspace_id = $1
ORDER BY provider_id ASC, created_at ASC;

-- name: UpsertConnectorCredential :one
INSERT INTO connector_credential (
    workspace_id, provider_id, owner_user_id, encrypted_secret, secret_nonce,
    key_id, status, upstream_identity, last_validated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg('last_validated_at')
)
ON CONFLICT (workspace_id, provider_id, owner_user_id) DO UPDATE SET
    encrypted_secret = EXCLUDED.encrypted_secret,
    secret_nonce = EXCLUDED.secret_nonce,
    key_id = EXCLUDED.key_id,
    status = EXCLUDED.status,
    upstream_identity = EXCLUDED.upstream_identity,
    last_validated_at = EXCLUDED.last_validated_at,
    invalidated_at = NULL,
    updated_at = now()
RETURNING *;

-- name: MarkConnectorCredentialInvalid :one
UPDATE connector_credential
SET status = 'invalid',
    invalidated_at = now(),
    updated_at = now()
WHERE workspace_id = $1 AND provider_id = $2 AND owner_user_id = $3
RETURNING *;

-- name: DeleteConnectorCredential :exec
DELETE FROM connector_credential
WHERE workspace_id = $1 AND provider_id = $2 AND owner_user_id = $3;

-- =====================
-- Connector Capability Audit
-- =====================

-- name: CreateConnectorCapabilityAuditEvent :one
INSERT INTO connector_capability_audit_event (
    workspace_id, provider_id, capability, task_id, agent_id,
    delegated_user_id, project_resource_id, status, reason, metadata
) VALUES (
    $1, $2, $3,
    sqlc.narg('task_id'),
    sqlc.narg('agent_id'),
    sqlc.narg('delegated_user_id'),
    sqlc.narg('project_resource_id'),
    $4,
    sqlc.narg('reason'),
    $5
)
RETURNING *;
