CREATE TABLE repository (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    source_state TEXT NOT NULL
        CHECK (source_state IN ('remote_git', 'local_dir', 'agent_managed', 'local_git')),
    remote_url TEXT,
    remote_key TEXT,
    default_branch TEXT,
    lead_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_by_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'ready'
        CHECK (status IN ('initializing', 'ready', 'error', 'archived')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(metadata) = 'object'),
    CHECK (source_state <> 'remote_git' OR remote_url IS NOT NULL)
);

CREATE INDEX idx_repository_workspace ON repository(workspace_id, status, created_at DESC);
CREATE UNIQUE INDEX idx_repository_workspace_remote_key
    ON repository(workspace_id, remote_key)
    WHERE remote_key IS NOT NULL AND status <> 'archived';

CREATE TABLE repository_binding (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    daemon_id TEXT NOT NULL,
    runtime_id UUID REFERENCES agent_runtime(id) ON DELETE SET NULL,
    machine_label TEXT NOT NULL DEFAULT '',
    binding_kind TEXT NOT NULL
        CHECK (binding_kind IN ('local_dir', 'daemon_workdir', 'git_cache_worktree')),
    local_path TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'initializing'
        CHECK (state IN ('ready', 'missing', 'inaccessible', 'initializing', 'error')),
    last_seen_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (local_path <> ''),
    CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX idx_repository_binding_repository ON repository_binding(repository_id);
CREATE INDEX idx_repository_binding_workspace ON repository_binding(workspace_id);
CREATE INDEX idx_repository_binding_daemon ON repository_binding(workspace_id, daemon_id);

CREATE TABLE project_repository (
    project_id UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    repository_id UUID NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'secondary' CHECK (role IN ('primary', 'secondary')),
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, repository_id)
);

CREATE INDEX idx_project_repository_project ON project_repository(project_id, position);
CREATE INDEX idx_project_repository_repository ON project_repository(repository_id);
CREATE UNIQUE INDEX idx_project_repository_primary
    ON project_repository(project_id)
    WHERE role = 'primary';

ALTER TABLE chat_session
    ADD COLUMN default_repository_id UUID REFERENCES repository(id) ON DELETE SET NULL;

CREATE INDEX idx_chat_session_default_repository
    ON chat_session(default_repository_id)
    WHERE default_repository_id IS NOT NULL;

CREATE TABLE repository_operation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL
        CHECK (operation_type IN ('create_binding', 'init_git', 'publish_remote', 'refresh_binding', 'archive')),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    requested_by_type TEXT NOT NULL CHECK (requested_by_type IN ('member', 'agent', 'system')),
    requested_by_id UUID,
    target_daemon_id TEXT,
    target_runtime_id UUID REFERENCES agent_runtime(id) ON DELETE SET NULL,
    binding_id UUID REFERENCES repository_binding(id) ON DELETE SET NULL,
    request JSONB NOT NULL DEFAULT '{}',
    result JSONB NOT NULL DEFAULT '{}',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK (jsonb_typeof(request) = 'object'),
    CHECK (jsonb_typeof(result) = 'object')
);

CREATE INDEX idx_repository_operation_workspace_status
    ON repository_operation(workspace_id, status, created_at ASC);
CREATE INDEX idx_repository_operation_repository
    ON repository_operation(repository_id, created_at DESC);
CREATE INDEX idx_repository_operation_target_daemon
    ON repository_operation(workspace_id, target_daemon_id, status)
    WHERE target_daemon_id IS NOT NULL;

CREATE TABLE task_output_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    repository_id UUID REFERENCES repository(id) ON DELETE SET NULL,
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    relative_path TEXT NOT NULL,
    filename TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'unknown'
        CHECK (kind IN ('source', 'doc', 'artifact', 'log', 'report', 'unknown')),
    size_bytes BIGINT CHECK (size_bytes IS NULL OR size_bytes >= 0),
    mime_type TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (relative_path <> ''),
    CHECK (relative_path !~ '(^/|(^|/)\.\.(/|$)|\\|^[A-Za-z]:|^~(/|$))'),
    CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX idx_task_output_metadata_workspace
    ON task_output_metadata(workspace_id, created_at DESC);
CREATE INDEX idx_task_output_metadata_repository
    ON task_output_metadata(repository_id, created_at DESC)
    WHERE repository_id IS NOT NULL;
CREATE INDEX idx_task_output_metadata_task
    ON task_output_metadata(task_id);
