ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_origin_type_check;
UPDATE issue
SET origin_type = NULL,
    origin_id = NULL
WHERE origin_type = 'chat_session';
ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
    CHECK (origin_type IN ('autopilot', 'quick_create'));

DROP INDEX IF EXISTS idx_chat_issue_proposal_item_issue;
DROP INDEX IF EXISTS idx_chat_issue_proposal_item_proposal;
DROP TABLE IF EXISTS chat_issue_proposal_item;

DROP INDEX IF EXISTS idx_chat_issue_proposal_workspace;
DROP INDEX IF EXISTS idx_chat_issue_proposal_session;
DROP TABLE IF EXISTS chat_issue_proposal;

DROP INDEX IF EXISTS idx_chat_session_project_context;
DROP INDEX IF EXISTS idx_chat_session_workspace_creator_status_updated;

ALTER TABLE chat_session
DROP CONSTRAINT IF EXISTS chat_session_project_snapshot_object_check,
DROP CONSTRAINT IF EXISTS chat_session_title_source_check,
DROP CONSTRAINT IF EXISTS chat_session_project_context_kind_check;

ALTER TABLE chat_session
DROP COLUMN IF EXISTS title_source,
DROP COLUMN IF EXISTS project_snapshot,
DROP COLUMN IF EXISTS project_context_kind,
DROP COLUMN IF EXISTS project_id;
