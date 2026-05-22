DROP INDEX IF EXISTS idx_agent_task_queue_chat_plan_consultation;
DROP INDEX IF EXISTS idx_agent_task_queue_chat_plan_run;

ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_chat_task_kind_check;

ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS chat_task_kind,
    DROP COLUMN IF EXISTS chat_plan_consultation_id,
    DROP COLUMN IF EXISTS chat_plan_run_id;

DROP INDEX IF EXISTS idx_chat_issue_proposal_plan_run;

ALTER TABLE chat_issue_proposal
    DROP COLUMN IF EXISTS source_plan_run_id;

DROP INDEX IF EXISTS idx_chat_message_author_agent;
DROP INDEX IF EXISTS idx_chat_message_plan_run;

ALTER TABLE chat_message
    DROP CONSTRAINT IF EXISTS chat_message_author_type_check;

ALTER TABLE chat_message
    DROP COLUMN IF EXISTS reply_to_message_id,
    DROP COLUMN IF EXISTS consultation_id,
    DROP COLUMN IF EXISTS plan_run_id,
    DROP COLUMN IF EXISTS author_agent_id,
    DROP COLUMN IF EXISTS author_type;

DROP TABLE IF EXISTS chat_plan_consultation;
DROP TABLE IF EXISTS chat_plan_run;
