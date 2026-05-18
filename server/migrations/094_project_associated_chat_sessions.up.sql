ALTER TABLE chat_session
ADD COLUMN project_id UUID REFERENCES project(id) ON DELETE SET NULL,
ADD COLUMN project_context_kind TEXT NOT NULL DEFAULT 'loose',
ADD COLUMN project_snapshot JSONB,
ADD COLUMN title_source TEXT NOT NULL DEFAULT 'legacy';

ALTER TABLE chat_session
ADD CONSTRAINT chat_session_project_context_kind_check
CHECK (project_context_kind IN ('loose', 'project'));

ALTER TABLE chat_session
ADD CONSTRAINT chat_session_title_source_check
CHECK (title_source IN ('legacy', 'first_message', 'agent_summary', 'user'));

ALTER TABLE chat_session
ADD CONSTRAINT chat_session_project_snapshot_object_check
CHECK (project_snapshot IS NULL OR jsonb_typeof(project_snapshot) = 'object');

CREATE INDEX idx_chat_session_workspace_creator_status_updated
ON chat_session(workspace_id, creator_id, status, updated_at DESC);

CREATE INDEX idx_chat_session_project_context
ON chat_session(workspace_id, creator_id, project_context_kind, project_id, updated_at DESC);

CREATE TABLE chat_issue_proposal (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    chat_session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    source_chat_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    source_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    proposer_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    summary TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'partially_accepted', 'dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(title) <> '')
);

CREATE INDEX idx_chat_issue_proposal_session
ON chat_issue_proposal(chat_session_id, status, created_at);

CREATE INDEX idx_chat_issue_proposal_workspace
ON chat_issue_proposal(workspace_id, created_at DESC);

CREATE TABLE chat_issue_proposal_item (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID NOT NULL REFERENCES chat_issue_proposal(id) ON DELETE CASCADE,
    position INT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority TEXT CHECK (priority IN ('urgent', 'high', 'medium', 'low', 'none')),
    labels JSONB NOT NULL DEFAULT '[]',
    assignee_type TEXT CHECK (assignee_type IN ('member', 'agent')),
    assignee_id UUID,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'created', 'skipped')),
    issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    approved_snapshot JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (position >= 0),
    CHECK (btrim(title) <> ''),
    CHECK (jsonb_typeof(labels) = 'array'),
    CHECK (approved_snapshot IS NULL OR jsonb_typeof(approved_snapshot) = 'object'),
    CHECK ((assignee_type IS NULL AND assignee_id IS NULL) OR (assignee_type IS NOT NULL AND assignee_id IS NOT NULL))
);

CREATE INDEX idx_chat_issue_proposal_item_proposal
ON chat_issue_proposal_item(proposal_id, position);

CREATE INDEX idx_chat_issue_proposal_item_issue
ON chat_issue_proposal_item(issue_id)
WHERE issue_id IS NOT NULL;

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_origin_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
    CHECK (origin_type IN ('autopilot', 'quick_create', 'chat_session'));
