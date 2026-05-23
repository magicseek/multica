ALTER TABLE chat_plan_run
    DROP CONSTRAINT IF EXISTS chat_plan_run_consultation_wave_count_check;

ALTER TABLE chat_plan_run
    DROP COLUMN IF EXISTS consultation_wave_count;

DROP INDEX IF EXISTS idx_chat_session_directed_state_workspace;
DROP TABLE IF EXISTS chat_session_directed_state;

DROP INDEX IF EXISTS idx_chat_message_recipient_resolved_agent;
DROP INDEX IF EXISTS idx_chat_message_recipient_message;
DROP INDEX IF EXISTS idx_chat_message_recipient_session;
DROP TABLE IF EXISTS chat_message_recipient;

ALTER TABLE chat_message
    DROP COLUMN IF EXISTS author_member_id;
