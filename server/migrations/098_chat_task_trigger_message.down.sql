DROP INDEX IF EXISTS idx_agent_task_queue_trigger_chat_message;

ALTER TABLE agent_task_queue
  DROP COLUMN IF EXISTS trigger_chat_message_id;
