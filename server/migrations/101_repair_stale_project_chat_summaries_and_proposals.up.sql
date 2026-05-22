-- Repair project chat sessions polluted by stale legacy structured manifests.
-- One-message project chats should keep the first user message as their title,
-- and pending proposals whose every item already exists as a Project issue are
-- stale duplicate handoffs rather than useful new review work.

WITH first_user_message AS (
    SELECT
        cm.chat_session_id,
        count(*) AS user_message_count,
        left(regexp_replace(btrim((array_agg(cm.content ORDER BY cm.created_at ASC, cm.id ASC))[1]), '\s+', ' ', 'g'), 80) AS title
    FROM chat_message cm
    WHERE cm.role = 'user'
      AND btrim(cm.content) <> ''
    GROUP BY cm.chat_session_id
)
UPDATE chat_session cs
SET title = first_user_message.title,
    title_source = 'first_message',
    updated_at = now()
FROM first_user_message
WHERE first_user_message.chat_session_id = cs.id
  AND first_user_message.user_message_count = 1
  AND btrim(first_user_message.title) <> ''
  AND cs.project_context_kind = 'project'
  AND cs.title_source = 'agent_summary';

WITH project_issue_titles AS (
    SELECT
        workspace_id,
        project_id,
        lower(btrim(regexp_replace(title, '[[:space:]]+', ' ', 'g'))) AS title_key
    FROM issue
    WHERE project_id IS NOT NULL
      AND status <> 'cancelled'
), pending_proposal_stats AS (
    SELECT
        cip.id AS proposal_id,
        count(*) AS item_count,
        count(*) FILTER (WHERE pit.title_key IS NOT NULL) AS duplicate_item_count
    FROM chat_issue_proposal cip
    JOIN chat_session cs ON cs.id = cip.chat_session_id
    JOIN chat_issue_proposal_item cipi ON cipi.proposal_id = cip.id
    LEFT JOIN project_issue_titles pit
      ON pit.workspace_id = cip.workspace_id
     AND pit.project_id = cs.project_id
     AND pit.title_key = lower(btrim(regexp_replace(cipi.title, '[[:space:]]+', ' ', 'g')))
    WHERE cip.status = 'pending'
      AND cs.project_context_kind = 'project'
      AND cs.project_id IS NOT NULL
      AND cipi.status = 'pending'
    GROUP BY cip.id
), duplicate_pending_proposals AS (
    SELECT proposal_id
    FROM pending_proposal_stats
    WHERE item_count > 0
      AND item_count = duplicate_item_count
)
DELETE FROM chat_issue_proposal
WHERE id IN (SELECT proposal_id FROM duplicate_pending_proposals);
