CREATE TABLE chat_plan_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    chat_session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    creator_user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('agent', 'squad')),
    actor_id UUID NOT NULL,
    lead_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    plan_engine TEXT NOT NULL,
    engine_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'brainstorming'
        CHECK (status IN ('brainstorming', 'consulting', 'ready_for_approval', 'completed', 'cancelled', 'failed')),
    initial_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    latest_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    CHECK (jsonb_typeof(summary) = 'object')
);

CREATE INDEX idx_chat_plan_run_session_status
ON chat_plan_run(chat_session_id, status, updated_at DESC);

CREATE INDEX idx_chat_plan_run_workspace
ON chat_plan_run(workspace_id, updated_at DESC);

CREATE INDEX idx_chat_plan_run_lead
ON chat_plan_run(lead_agent_id, status)
WHERE status IN ('brainstorming', 'consulting', 'ready_for_approval');

CREATE TABLE chat_plan_consultation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_run_id UUID NOT NULL REFERENCES chat_plan_run(id) ON DELETE CASCADE,
    requester_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    target_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    request_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    response_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'responded', 'failed', 'timed_out', 'skipped')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    UNIQUE (plan_run_id, request_message_id, target_agent_id)
);

CREATE INDEX idx_chat_plan_consultation_run_status
ON chat_plan_consultation(plan_run_id, status, created_at);

CREATE INDEX idx_chat_plan_consultation_task
ON chat_plan_consultation(task_id)
WHERE task_id IS NOT NULL;

ALTER TABLE chat_message
    ADD COLUMN author_type TEXT NOT NULL DEFAULT 'member',
    ADD COLUMN author_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    ADD COLUMN plan_run_id UUID REFERENCES chat_plan_run(id) ON DELETE SET NULL,
    ADD COLUMN consultation_id UUID REFERENCES chat_plan_consultation(id) ON DELETE SET NULL,
    ADD COLUMN reply_to_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL;

UPDATE chat_message cm
SET author_type = 'agent',
    author_agent_id = atq.agent_id
FROM agent_task_queue atq
WHERE cm.task_id = atq.id
  AND cm.role = 'assistant';

ALTER TABLE chat_message
    ADD CONSTRAINT chat_message_author_type_check
    CHECK (author_type IN ('member', 'agent', 'system'));

CREATE INDEX idx_chat_message_plan_run
ON chat_message(plan_run_id, created_at)
WHERE plan_run_id IS NOT NULL;

CREATE INDEX idx_chat_message_author_agent
ON chat_message(author_agent_id, created_at)
WHERE author_agent_id IS NOT NULL;

ALTER TABLE chat_issue_proposal
    ADD COLUMN source_plan_run_id UUID REFERENCES chat_plan_run(id) ON DELETE SET NULL;

CREATE INDEX idx_chat_issue_proposal_plan_run
ON chat_issue_proposal(source_plan_run_id, created_at)
WHERE source_plan_run_id IS NOT NULL;

ALTER TABLE agent_task_queue
    ADD COLUMN chat_plan_run_id UUID REFERENCES chat_plan_run(id) ON DELETE SET NULL,
    ADD COLUMN chat_plan_consultation_id UUID REFERENCES chat_plan_consultation(id) ON DELETE SET NULL,
    ADD COLUMN chat_task_kind TEXT NOT NULL DEFAULT 'normal';

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_chat_task_kind_check
    CHECK (chat_task_kind IN ('normal', 'plan_lead', 'plan_consultation'));

CREATE INDEX idx_agent_task_queue_chat_plan_run
ON agent_task_queue(chat_plan_run_id, status, created_at)
WHERE chat_plan_run_id IS NOT NULL;

CREATE INDEX idx_agent_task_queue_chat_plan_consultation
ON agent_task_queue(chat_plan_consultation_id)
WHERE chat_plan_consultation_id IS NOT NULL;
