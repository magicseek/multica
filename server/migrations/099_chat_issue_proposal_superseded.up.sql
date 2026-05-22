ALTER TABLE chat_issue_proposal DROP CONSTRAINT IF EXISTS chat_issue_proposal_status_check;

ALTER TABLE chat_issue_proposal ADD CONSTRAINT chat_issue_proposal_status_check
CHECK (status IN ('pending', 'accepted', 'partially_accepted', 'dismissed', 'superseded'));
