ALTER TABLE agent_task_queue
  ADD COLUMN trigger_chat_message_id UUID REFERENCES chat_message(id) ON DELETE SET NULL;

CREATE INDEX idx_agent_task_queue_trigger_chat_message
  ON agent_task_queue(trigger_chat_message_id)
  WHERE trigger_chat_message_id IS NOT NULL;
