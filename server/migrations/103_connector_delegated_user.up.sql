ALTER TABLE agent_task_queue
  ADD COLUMN IF NOT EXISTS connector_delegated_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_agent_task_queue_connector_delegated_user
  ON agent_task_queue(connector_delegated_user_id)
  WHERE connector_delegated_user_id IS NOT NULL;

ALTER TABLE autopilot
  ADD COLUMN IF NOT EXISTS connector_delegated_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_autopilot_connector_delegated_user
  ON autopilot(connector_delegated_user_id)
  WHERE connector_delegated_user_id IS NOT NULL;
