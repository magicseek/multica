DROP INDEX IF EXISTS workflow_input_request_open_step_unique;
DROP INDEX IF EXISTS workflow_input_request_step_idx;
DROP INDEX IF EXISTS workflow_input_request_run_status_idx;
DROP TABLE IF EXISTS workflow_input_request;

UPDATE workflow_step_run
SET status = 'waiting_manual'
WHERE status = 'waiting_input';

ALTER TABLE workflow_step_run
  DROP CONSTRAINT IF EXISTS workflow_step_run_status_check,
  ADD CONSTRAINT workflow_step_run_status_check
    CHECK (status IN (
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
    ));

UPDATE agent_task_queue
SET status = 'queued'
WHERE status = 'waiting';

ALTER TABLE agent_task_queue
  DROP CONSTRAINT IF EXISTS agent_task_queue_status_check,
  ADD CONSTRAINT agent_task_queue_status_check
    CHECK (status IN ('queued', 'dispatched', 'running', 'completed', 'failed', 'cancelled'));
