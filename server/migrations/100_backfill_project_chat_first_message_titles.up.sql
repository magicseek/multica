-- Repair Project-associated chat sessions whose client-created title was the
-- Project title instead of the first user message title.
WITH first_user_message AS (
    SELECT DISTINCT ON (cm.chat_session_id)
        cm.chat_session_id,
        left(regexp_replace(btrim(cm.content), '\s+', ' ', 'g'), 80) AS title
    FROM chat_message cm
    WHERE cm.role = 'user'
      AND btrim(cm.content) <> ''
    ORDER BY cm.chat_session_id, cm.created_at ASC, cm.id ASC
)
UPDATE chat_session cs
SET title = first_user_message.title,
    title_source = 'first_message',
    updated_at = now()
FROM first_user_message
WHERE first_user_message.chat_session_id = cs.id
  AND cs.title_source = 'first_message'
  AND cs.project_context_kind = 'project'
  AND btrim(first_user_message.title) <> ''
  AND cs.project_snapshot IS NOT NULL
  AND btrim(cs.title) = btrim(COALESCE(cs.project_snapshot->>'title', ''))
  AND btrim(cs.title) <> btrim(first_user_message.title);
