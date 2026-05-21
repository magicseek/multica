ALTER TABLE agent_task_queue
  DROP CONSTRAINT IF EXISTS agent_task_queue_status_check,
  ADD CONSTRAINT agent_task_queue_status_check
    CHECK (status IN ('queued', 'dispatched', 'running', 'waiting', 'completed', 'failed', 'cancelled'));

ALTER TABLE workflow_step_run
  DROP CONSTRAINT IF EXISTS workflow_step_run_status_check,
  ADD CONSTRAINT workflow_step_run_status_check
    CHECK (status IN (
        'pending',
        'ready',
        'running',
        'waiting_input',
        'waiting_review',
        'waiting_quality',
        'waiting_manual',
        'waiting_external',
        'paused',
        'blocked',
        'failed',
        'completed',
        'skipped'
    ));

CREATE TABLE workflow_input_request (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
  workflow_step_run_id UUID NOT NULL REFERENCES workflow_step_run(id) ON DELETE CASCADE,
  issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
  chat_session_id UUID REFERENCES chat_session(id) ON DELETE SET NULL,
  question_comment_id UUID REFERENCES comment(id) ON DELETE SET NULL,
  answer_comment_id UUID REFERENCES comment(id) ON DELETE SET NULL,
  requester_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
  responder_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
  status TEXT NOT NULL CHECK (status IN ('requested', 'answered', 'cancelled', 'expired')),
  question_text TEXT NOT NULL,
  answer_text TEXT,
  round_index INT NOT NULL DEFAULT 1,
  max_rounds INT NOT NULL DEFAULT 1,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  answered_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workflow_input_request_run_status_idx
  ON workflow_input_request(workflow_run_id, status, requested_at DESC);

CREATE INDEX workflow_input_request_step_idx
  ON workflow_input_request(workflow_step_run_id, requested_at DESC);

CREATE UNIQUE INDEX workflow_input_request_open_step_unique
  ON workflow_input_request(workflow_step_run_id)
  WHERE status = 'requested';
