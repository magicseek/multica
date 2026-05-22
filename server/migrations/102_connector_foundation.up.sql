-- Generic connector foundation. Provider definitions live in code and are
-- registered by deployment profile; these tables store workspace enablement,
-- user-owned encrypted credentials, and capability audit metadata.

CREATE TABLE workspace_connector (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider_id  TEXT NOT NULL CHECK (provider_id <> ''),
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    settings     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by   UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider_id)
);

CREATE INDEX idx_workspace_connector_workspace ON workspace_connector(workspace_id);

CREATE TABLE connector_credential (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider_id       TEXT NOT NULL CHECK (provider_id <> ''),
    owner_user_id     UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    encrypted_secret  BYTEA NOT NULL,
    secret_nonce      BYTEA NOT NULL,
    key_id            TEXT NOT NULL CHECK (key_id <> ''),
    status            TEXT NOT NULL DEFAULT 'never_validated'
        CHECK (status IN ('never_validated', 'valid', 'invalid')),
    upstream_identity JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_validated_at TIMESTAMPTZ,
    invalidated_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider_id, owner_user_id)
);

CREATE INDEX idx_connector_credential_workspace ON connector_credential(workspace_id);
CREATE INDEX idx_connector_credential_owner ON connector_credential(owner_user_id);

CREATE TABLE connector_capability_audit_event (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider_id        TEXT NOT NULL CHECK (provider_id <> ''),
    capability         TEXT NOT NULL CHECK (capability <> ''),
    task_id            UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    agent_id           UUID REFERENCES agent(id) ON DELETE SET NULL,
    delegated_user_id  UUID REFERENCES "user"(id) ON DELETE SET NULL,
    project_resource_id UUID REFERENCES project_resource(id) ON DELETE SET NULL,
    status             TEXT NOT NULL
        CHECK (status IN ('succeeded', 'rejected', 'failed')),
    reason             TEXT,
    metadata           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_connector_audit_workspace_created ON connector_capability_audit_event(workspace_id, created_at DESC);
CREATE INDEX idx_connector_audit_task ON connector_capability_audit_event(task_id);
