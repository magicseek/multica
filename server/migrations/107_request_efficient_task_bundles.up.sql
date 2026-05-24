ALTER TABLE agent
ADD COLUMN request_efficient_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE task_bundle (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    runtime_id UUID NOT NULL REFERENCES agent_runtime(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'blocked', 'cancelled')),
    changeset_mode TEXT NOT NULL DEFAULT 'per_issue'
        CHECK (changeset_mode IN ('per_issue', 'shared')),
    max_items INT NOT NULL DEFAULT 5 CHECK (max_items > 0),
    runtime_budget_seconds INT NOT NULL CHECK (runtime_budget_seconds > 0),
    rerun_of_bundle_id UUID REFERENCES task_bundle(id) ON DELETE SET NULL,
    rerun_scope JSONB NOT NULL DEFAULT '[]',
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

ALTER TABLE agent_task_queue
ADD COLUMN task_bundle_id UUID REFERENCES task_bundle(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX agent_task_queue_task_bundle_unique
ON agent_task_queue(task_bundle_id)
WHERE task_bundle_id IS NOT NULL;

CREATE INDEX task_bundle_workspace_status_idx
ON task_bundle(workspace_id, status, created_at DESC);

CREATE INDEX task_bundle_agent_status_idx
ON task_bundle(agent_id, status, created_at DESC);

CREATE TABLE task_bundle_item (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bundle_id UUID NOT NULL REFERENCES task_bundle(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    position INT NOT NULL CHECK (position > 0),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'in_progress', 'completed', 'failed', 'blocked', 'input_needed', 'cancelled')),
    output_namespace TEXT NOT NULL,
    checkpoint_seq INT,
    result JSONB,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(bundle_id, issue_id),
    UNIQUE(bundle_id, position)
);

CREATE INDEX task_bundle_item_issue_idx
ON task_bundle_item(issue_id, created_at DESC);

CREATE INDEX task_bundle_item_bundle_status_idx
ON task_bundle_item(bundle_id, status, position);
