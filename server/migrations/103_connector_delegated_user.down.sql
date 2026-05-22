DROP INDEX IF EXISTS idx_autopilot_connector_delegated_user;
DROP INDEX IF EXISTS idx_agent_task_queue_connector_delegated_user;

ALTER TABLE autopilot DROP COLUMN IF EXISTS connector_delegated_user_id;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS connector_delegated_user_id;
