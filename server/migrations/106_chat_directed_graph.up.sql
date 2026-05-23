ALTER TABLE chat_message
    ADD COLUMN author_member_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

UPDATE chat_message cm
SET author_member_id = cs.creator_id
FROM chat_session cs
WHERE cm.chat_session_id = cs.id
  AND cm.author_type = 'member'
  AND cm.author_member_id IS NULL;

CREATE TABLE chat_message_recipient (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    chat_session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES chat_message(id) ON DELETE CASCADE,
    recipient_type TEXT NOT NULL CHECK (recipient_type IN ('member', 'agent', 'squad')),
    recipient_id UUID NOT NULL,
    resolved_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    source TEXT NOT NULL DEFAULT 'explicit_mention'
        CHECK (source IN ('explicit_mention', 'continuation', 'default', 'system')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'routed', 'blocked', 'skipped', 'failed')),
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    warning_code TEXT NOT NULL DEFAULT '',
    warning_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id, recipient_type, recipient_id, source)
);

CREATE INDEX idx_chat_message_recipient_session
ON chat_message_recipient(chat_session_id, created_at, id);

CREATE INDEX idx_chat_message_recipient_message
ON chat_message_recipient(message_id, created_at, id);

CREATE INDEX idx_chat_message_recipient_resolved_agent
ON chat_message_recipient(resolved_agent_id, status, created_at)
WHERE resolved_agent_id IS NOT NULL;

CREATE TABLE chat_session_directed_state (
    chat_session_id UUID PRIMARY KEY REFERENCES chat_session(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'idle' CHECK (state IN ('idle', 'active', 'ambiguous')),
    active_recipient_type TEXT CHECK (active_recipient_type IN ('member', 'agent', 'squad')),
    active_recipient_id UUID,
    active_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL,
    candidate_recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(candidate_recipients) = 'array'),
    CHECK (
        (state = 'active' AND active_recipient_type IS NOT NULL AND active_recipient_id IS NOT NULL)
        OR (state <> 'active')
    )
);

CREATE INDEX idx_chat_session_directed_state_workspace
ON chat_session_directed_state(workspace_id, updated_at DESC);

ALTER TABLE chat_plan_run
    ADD COLUMN consultation_wave_count INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT chat_plan_run_consultation_wave_count_check
        CHECK (consultation_wave_count >= 0);
