CREATE TABLE workflow_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_task_queue_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    chat_session_id UUID REFERENCES chat_session(id) ON DELETE SET NULL,
    autopilot_run_id UUID REFERENCES autopilot_run(id) ON DELETE SET NULL,
    workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
    workflow_revision_id UUID REFERENCES workflow_revision(id) ON DELETE SET NULL,
    trigger_type TEXT NOT NULL DEFAULT '',
    snapshot JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'waiting', 'blocked', 'failed', 'completed', 'cancelled')),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(agent_task_queue_id)
);

CREATE INDEX workflow_run_workspace_status_idx
ON workflow_run(workspace_id, status, created_at DESC);

CREATE INDEX workflow_run_issue_idx
ON workflow_run(issue_id, created_at DESC)
WHERE issue_id IS NOT NULL;

CREATE INDEX workflow_run_chat_session_idx
ON workflow_run(chat_session_id, created_at DESC)
WHERE chat_session_id IS NOT NULL;

CREATE INDEX workflow_run_autopilot_run_idx
ON workflow_run(autopilot_run_id, created_at DESC)
WHERE autopilot_run_id IS NOT NULL;

CREATE TABLE workflow_step_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    step_definition_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    order_index INT NOT NULL DEFAULT 0,
    required BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL CHECK (status IN (
        'pending',
        'ready',
        'running',
        'waiting_review',
        'waiting_quality',
        'waiting_manual',
        'waiting_external',
        'paused',
        'blocked',
        'failed',
        'completed',
        'skipped'
    )),
    execution_kind TEXT NOT NULL CHECK (execution_kind IN ('agent', 'manual', 'external')),
    depends_on_step_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    artifact_inputs JSONB NOT NULL DEFAULT '[]'::jsonb,
    snapshot JSONB NOT NULL,
    attempt INT NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    execution_metadata JSONB,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workflow_run_id, step_definition_id, attempt)
);

CREATE INDEX workflow_step_run_run_order_idx
ON workflow_step_run(workflow_run_id, order_index, created_at);

CREATE INDEX workflow_step_run_run_status_idx
ON workflow_step_run(workflow_run_id, status, order_index);

CREATE TABLE workflow_artifact (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    workflow_step_run_id UUID NOT NULL REFERENCES workflow_step_run(id) ON DELETE CASCADE,
    logical_name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    content_kind TEXT NOT NULL CHECK (content_kind IN ('text', 'markdown', 'json')),
    content_text TEXT,
    content_json JSONB,
    producer_type TEXT NOT NULL CHECK (producer_type IN ('agent', 'member', 'system')),
    producer_id UUID,
    supersedes_artifact_id UUID REFERENCES workflow_artifact(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workflow_step_run_id, logical_name, version)
);

CREATE INDEX workflow_artifact_run_idx
ON workflow_artifact(workflow_run_id, created_at DESC);

CREATE TABLE workflow_review (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    workflow_step_run_id UUID REFERENCES workflow_step_run(id) ON DELETE CASCADE,
    workflow_artifact_id UUID REFERENCES workflow_artifact(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('requested', 'approved', 'rejected', 'cancelled')),
    reviewer_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    decision_notes TEXT,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (workflow_step_run_id IS NOT NULL OR workflow_artifact_id IS NOT NULL)
);

CREATE INDEX workflow_review_run_status_idx
ON workflow_review(workflow_run_id, status, created_at DESC);

CREATE TABLE workflow_quality_gate_result (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    workflow_step_run_id UUID NOT NULL REFERENCES workflow_step_run(id) ON DELETE CASCADE,
    workflow_artifact_id UUID REFERENCES workflow_artifact(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('pass', 'fail', 'warning')),
    blocking BOOLEAN NOT NULL DEFAULT FALSE,
    producer_type TEXT NOT NULL CHECK (producer_type IN ('agent', 'member', 'system')),
    producer_id UUID,
    report_text TEXT,
    report_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workflow_quality_gate_result_run_idx
ON workflow_quality_gate_result(workflow_run_id, created_at DESC);
