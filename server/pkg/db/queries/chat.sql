-- name: CreateChatSession :one
INSERT INTO chat_session (
    workspace_id,
    agent_id,
    creator_id,
    title,
    runtime_id,
    default_repository_id,
    project_id,
    project_context_kind,
    project_snapshot,
    title_source
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    (SELECT runtime_id FROM agent WHERE id = $2),
    sqlc.narg('default_repository_id'),
    sqlc.narg('project_id'),
    sqlc.arg('project_context_kind'),
    sqlc.narg('project_snapshot'),
    sqlc.arg('title_source')
)
RETURNING *;

-- name: GetChatSession :one
SELECT * FROM chat_session
WHERE id = $1;

-- name: GetChatSessionInWorkspace :one
SELECT * FROM chat_session
WHERE id = $1 AND workspace_id = $2;

-- name: ListChatSessionsByCreator :many
-- Returns active sessions with a boolean unread flag. Unread is strictly
-- per-session: either the user has uncleared assistant replies in this
-- session or they don't. Counting messages would be misleading.
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1 AND cs.creator_id = $2 AND cs.status = 'active'
ORDER BY cs.updated_at DESC;

-- name: ListAllChatSessionsByCreator :many
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1 AND cs.creator_id = $2
ORDER BY cs.updated_at DESC;

-- name: ListChatSessionsByCreatorFiltered :many
-- Complete list variant for route-owned chat views. `scope` narrows to loose
-- sessions or one Project's sessions; absent scope preserves the old active
-- list behavior.
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1
  AND cs.creator_id = $2
  AND cs.status = 'active'
  AND (
      sqlc.narg('scope')::text IS NULL
      OR (sqlc.narg('scope')::text = 'loose' AND cs.project_context_kind = 'loose')
      OR (
          sqlc.narg('scope')::text = 'project'
          AND cs.project_context_kind = 'project'
          AND cs.project_id = sqlc.narg('project_id')::uuid
      )
  )
ORDER BY cs.updated_at DESC;

-- name: ListAllChatSessionsByCreatorFiltered :many
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1
  AND cs.creator_id = $2
  AND (
      sqlc.narg('scope')::text IS NULL
      OR (sqlc.narg('scope')::text = 'loose' AND cs.project_context_kind = 'loose')
      OR (
          sqlc.narg('scope')::text = 'project'
          AND cs.project_context_kind = 'project'
          AND cs.project_id = sqlc.narg('project_id')::uuid
      )
  )
ORDER BY cs.updated_at DESC;

-- name: ListSidebarProjects :many
-- Sidebar Projects tree: active Projects are visible even when they have no
-- recent Project-associated Chat Sessions.
SELECT id, title, icon, status
FROM project
WHERE workspace_id = $1
  AND status NOT IN ('completed', 'cancelled')
ORDER BY updated_at DESC, title ASC, id DESC;

-- name: ListRecentProjectChatSessionsByCreator :many
-- Sidebar quick-access tree: active, private sessions updated within the
-- requested rolling window, capped per active Project.
SELECT
    ranked.id,
    ranked.workspace_id,
    ranked.agent_id,
    ranked.creator_id,
    ranked.title,
    ranked.status,
    ranked.session_id,
    ranked.work_dir,
    ranked.runtime_id,
    ranked.default_repository_id,
    ranked.project_id,
    ranked.project_context_kind,
    ranked.project_snapshot,
    ranked.title_source,
    ranked.unread_since,
    ranked.created_at,
    ranked.updated_at,
    ranked.has_unread,
    ranked.group_project_id
FROM (
    SELECT
        cs.*,
        (cs.unread_since IS NOT NULL)::bool AS has_unread,
        p.id AS group_project_id,
        row_number() OVER (
            PARTITION BY p.id
            ORDER BY cs.updated_at DESC, cs.id DESC
        ) AS project_session_rank
    FROM chat_session cs
    JOIN project p ON p.id = cs.project_id AND p.workspace_id = cs.workspace_id
    WHERE cs.workspace_id = $1
      AND cs.creator_id = $2
      AND cs.status = 'active'
      AND cs.project_context_kind = 'project'
      AND cs.agent_id = ANY(sqlc.arg('agent_ids')::uuid[])
      AND cs.updated_at >= now() - (sqlc.arg('recent_days')::int * interval '1 day')
      AND p.status NOT IN ('completed', 'cancelled')
) ranked
WHERE ranked.project_session_rank <= sqlc.arg('project_chat_limit')::int
ORDER BY ranked.group_project_id, ranked.updated_at DESC, ranked.id DESC;

-- name: ListRecentLooseChatSessionsByCreator :many
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1
  AND cs.creator_id = $2
  AND cs.status = 'active'
  AND cs.project_context_kind = 'loose'
  AND cs.agent_id = ANY(sqlc.arg('agent_ids')::uuid[])
  AND cs.updated_at >= now() - (sqlc.arg('recent_days')::int * interval '1 day')
ORDER BY cs.updated_at DESC, cs.id DESC;

-- name: ListOlderLooseChatSessionsByCreator :many
SELECT cs.*,
       (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs
WHERE cs.workspace_id = $1
  AND cs.creator_id = $2
  AND cs.status = 'active'
  AND cs.project_context_kind = 'loose'
  AND cs.agent_id = ANY(sqlc.arg('agent_ids')::uuid[])
  AND (
      sqlc.narg('before_updated_at')::timestamptz IS NULL
      OR cs.updated_at < sqlc.narg('before_updated_at')::timestamptz
      OR (
          cs.updated_at = sqlc.narg('before_updated_at')::timestamptz
          AND cs.id < sqlc.narg('before_id')::uuid
      )
  )
ORDER BY cs.updated_at DESC, cs.id DESC
LIMIT sqlc.arg('limit')::int;

-- name: UpdateChatSessionTitle :one
UPDATE chat_session SET title = $2, title_source = $3, updated_at = now()
WHERE chat_session.id = $1
RETURNING *;

-- name: SetChatSessionFirstMessageTitle :one
UPDATE chat_session
SET title = $2,
    title_source = 'first_message',
    updated_at = now()
WHERE chat_session.id = $1
  AND title_source IN ('legacy', 'first_message')
  AND (
      SELECT count(*)
      FROM chat_message
      WHERE chat_message.chat_session_id = chat_session.id
        AND chat_message.role = 'user'
  ) = 1
RETURNING *;

-- name: SetChatSessionAgentSummaryTitle :one
UPDATE chat_session
SET title = $2,
    title_source = 'agent_summary',
    updated_at = now()
WHERE id = $1
  AND title_source <> 'user'
RETURNING *;

-- name: UpdateChatSessionFields :one
UPDATE chat_session SET
    title = COALESCE(sqlc.narg('title'), title),
    title_source = CASE
        WHEN sqlc.narg('title')::text IS NOT NULL THEN 'user'
        ELSE title_source
    END,
    default_repository_id = CASE
        WHEN sqlc.arg('set_default_repository_id')::bool THEN sqlc.narg('default_repository_id')
        ELSE default_repository_id
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateChatSessionSession :exec
-- Updates the resume pointer for a chat session. Empty/NULL inputs are
-- ignored via COALESCE so a task that completes without a session_id (e.g.
-- the agent crashed before establishing one) cannot wipe out a previously
-- recorded resume pointer. This makes the chat memory robust against
-- intermittent agent failures.
UPDATE chat_session
SET session_id = COALESCE(sqlc.narg('session_id'), session_id),
    work_dir = COALESCE(sqlc.narg('work_dir'), work_dir),
    runtime_id = COALESCE(sqlc.narg('runtime_id'), runtime_id),
    updated_at = now()
WHERE id = sqlc.arg('id');

-- name: LockChatSessionForDelete :one
-- Acquires an exclusive (FOR UPDATE) row lock on chat_session(id). Used by
-- the delete path so that a concurrent SendChatMessage cannot enqueue a new
-- agent_task_queue row referencing this session between our cancel and
-- delete steps. The FK from agent_task_queue.chat_session_id takes a
-- KEY SHARE lock on the parent row during INSERT validation, which
-- conflicts with FOR UPDATE — concurrent inserts block here and then fail
-- their FK check after we commit the delete.
SELECT id FROM chat_session
WHERE id = $1
FOR UPDATE;

-- name: ArchiveChatSession :exec
-- Soft-delete/archive. Keep the session row, messages, proposal provenance,
-- chat-originated issue links, and output metadata discoverable while hiding
-- the session from normal active lists.
UPDATE chat_session
SET status = 'archived',
    updated_at = now()
WHERE id = $1;

-- name: TouchChatSession :exec
UPDATE chat_session SET updated_at = now()
WHERE id = $1;

-- name: CreateChatMessage :one
INSERT INTO chat_message (chat_session_id, role, content, task_id, failure_reason, elapsed_ms)
VALUES ($1, $2, $3, sqlc.narg(task_id), sqlc.narg(failure_reason), sqlc.narg(elapsed_ms))
RETURNING *;

-- name: ListChatMessages :many
SELECT * FROM chat_message
WHERE chat_session_id = $1
ORDER BY created_at ASC;

-- name: GetChatMessage :one
SELECT * FROM chat_message
WHERE id = $1;

-- name: GetChatMessageInSession :one
SELECT * FROM chat_message
WHERE id = $1 AND chat_session_id = $2;

-- name: SetChatMessageTaskID :one
UPDATE chat_message
SET task_id = $2
WHERE id = $1
RETURNING *;

-- name: GetAssistantChatMessageByTask :one
SELECT * FROM chat_message
WHERE chat_session_id = $1
  AND task_id = $2
  AND role = 'assistant'
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateChatTask :one
INSERT INTO agent_task_queue (
    agent_id,
    runtime_id,
    issue_id,
    status,
    priority,
    chat_session_id,
    trigger_chat_message_id
)
VALUES ($1, $2, NULL, 'queued', $3, $4, $5)
RETURNING *;

-- name: GetLastChatTaskSession :one
-- Returns the most recent task in this chat session that managed to record a
-- session_id. Includes both completed and failed tasks: even a failed task
-- may have established a real agent session before failing, and we'd rather
-- resume there than start over and lose conversation memory. Used as a
-- fallback when chat_session.session_id is NULL.
SELECT session_id, work_dir, runtime_id FROM agent_task_queue
WHERE chat_session_id = $1
  AND status IN ('completed', 'failed')
  AND session_id IS NOT NULL
ORDER BY completed_at DESC
LIMIT 1;

-- name: GetPendingChatTask :one
-- Returns the most recent in-flight task for a chat session, if any.
-- Used by the frontend to recover pending state after refresh / reopen.
-- created_at is the anchor for the chat StatusPill timer (it computes
-- elapsed = now - task.created_at), so the pill survives refresh / reopen
-- without "resetting to 0s".
SELECT id, status, created_at FROM agent_task_queue
WHERE chat_session_id = $1 AND status IN ('queued', 'dispatched', 'running', 'waiting')
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPendingChatTasksByCreator :many
-- Aggregate view of all in-flight chat tasks owned by a given creator in a
-- workspace. Drives the FAB's "running" indicator when the chat window is
-- closed and no single session's query is active.
SELECT atq.id AS task_id, atq.status, atq.chat_session_id
FROM agent_task_queue atq
JOIN chat_session cs ON cs.id = atq.chat_session_id
WHERE cs.workspace_id = $1
  AND cs.creator_id = $2
  AND atq.status IN ('queued', 'dispatched', 'running', 'waiting')
ORDER BY atq.created_at DESC;

-- name: MarkChatSessionRead :exec
-- Clears unread_since, dropping the session's unread count to 0.
UPDATE chat_session SET unread_since = NULL
WHERE id = $1;

-- name: SetUnreadSinceIfNull :exec
-- Atomically stamps the first unread assistant message's arrival time.
-- No-op if the session is already in "has unread" state — keeps the earliest
-- unread boundary stable across multiple incoming replies.
UPDATE chat_session SET unread_since = now()
WHERE id = $1 AND unread_since IS NULL;

-- name: CreateChatIssueProposal :one
INSERT INTO chat_issue_proposal (
    workspace_id,
    chat_session_id,
    source_chat_message_id,
    source_task_id,
    proposer_agent_id,
    title,
    summary
) VALUES (
    $1,
    $2,
    sqlc.narg('source_chat_message_id'),
    sqlc.narg('source_task_id'),
    sqlc.narg('proposer_agent_id'),
    $3,
    sqlc.narg('summary')
) RETURNING *;

-- name: DeleteChatIssueProposalsForTask :exec
DELETE FROM chat_issue_proposal
WHERE chat_session_id = $1
  AND source_task_id = $2;

-- name: SupersedePendingChatIssueProposalsForSession :exec
WITH candidates AS (
    SELECT cip.id
    FROM chat_issue_proposal cip
    WHERE cip.chat_session_id = $1
      AND cip.status = 'pending'
      AND cip.source_task_id IS DISTINCT FROM $2
      AND NOT EXISTS (
          SELECT 1
          FROM chat_issue_proposal_item cipi
          WHERE cipi.proposal_id = cip.id
            AND cipi.status <> 'pending'
      )
), skipped_items AS (
    UPDATE chat_issue_proposal_item
    SET status = 'skipped',
        updated_at = now()
    WHERE proposal_id IN (SELECT id FROM candidates)
      AND status = 'pending'
    RETURNING proposal_id
)
UPDATE chat_issue_proposal
SET status = 'superseded',
    updated_at = now()
WHERE id IN (SELECT id FROM candidates);

-- name: ListChatIssueProposalItemsForTaskForUpdate :many
SELECT cip.status AS proposal_status,
       cipi.status AS item_status
FROM chat_issue_proposal cip
JOIN chat_issue_proposal_item cipi ON cipi.proposal_id = cip.id
WHERE cip.chat_session_id = $1
  AND cip.source_task_id = $2
ORDER BY cip.created_at ASC, cipi.position ASC, cipi.created_at ASC
FOR UPDATE OF cip, cipi;

-- name: CreateChatIssueProposalItem :one
INSERT INTO chat_issue_proposal_item (
    proposal_id,
    position,
    title,
    description,
    priority,
    labels,
    assignee_type,
    assignee_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    sqlc.narg('priority'),
    sqlc.arg('labels'),
    sqlc.narg('assignee_type'),
    sqlc.narg('assignee_id')
) RETURNING *;

-- name: GetChatIssueProposal :one
SELECT *
FROM chat_issue_proposal
WHERE id = $1;

-- name: ListChatIssueProposalItemsByProposal :many
SELECT *
FROM chat_issue_proposal_item
WHERE proposal_id = $1
ORDER BY position ASC, created_at ASC;

-- name: ListChatIssueProposalItemsByProposalForUpdate :many
SELECT *
FROM chat_issue_proposal_item
WHERE proposal_id = $1
ORDER BY position ASC, created_at ASC
FOR UPDATE;

-- name: UpdateChatIssueProposalItemDraft :one
UPDATE chat_issue_proposal_item
SET title = $3,
    description = $4,
    priority = sqlc.narg('priority'),
    labels = sqlc.arg('labels'),
    assignee_type = sqlc.narg('assignee_type'),
    assignee_id = sqlc.narg('assignee_id'),
    updated_at = now()
WHERE id = $1
  AND proposal_id = $2
  AND status <> 'created'
RETURNING *;

-- name: MarkChatIssueProposalItemCreated :one
UPDATE chat_issue_proposal_item
SET status = 'created',
    issue_id = $3,
    approved_snapshot = $4,
    updated_at = now()
WHERE id = $1
  AND proposal_id = $2
  AND status = 'pending'
RETURNING *;

-- name: MarkUnselectedPendingChatIssueProposalItemsSkipped :exec
UPDATE chat_issue_proposal_item
SET status = 'skipped',
    updated_at = now()
WHERE proposal_id = $1
  AND status = 'pending'
  AND NOT (id = ANY(sqlc.arg('item_ids')::uuid[]));

-- name: RestoreChatIssueProposalItem :one
UPDATE chat_issue_proposal_item
SET status = 'pending',
    issue_id = NULL,
    approved_snapshot = NULL,
    updated_at = now()
WHERE id = $1
  AND proposal_id = $2
  AND status = 'skipped'
RETURNING *;

-- name: SkipPendingChatIssueProposalItems :exec
UPDATE chat_issue_proposal_item
SET status = 'skipped',
    updated_at = now()
WHERE proposal_id = $1
  AND status = 'pending';

-- name: SetChatIssueProposalStatus :one
UPDATE chat_issue_proposal
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListChatIssueProposalsBySession :many
SELECT *
FROM chat_issue_proposal
WHERE workspace_id = $1
  AND chat_session_id = $2
ORDER BY
  CASE status
    WHEN 'pending' THEN 0
    WHEN 'partially_accepted' THEN 1
    WHEN 'accepted' THEN 2
    ELSE 3
  END,
  created_at ASC;

-- name: ListChatIssueProposalItemsBySession :many
SELECT cipi.*
FROM chat_issue_proposal_item cipi
JOIN chat_issue_proposal cip ON cip.id = cipi.proposal_id
WHERE cip.workspace_id = $1
  AND cip.chat_session_id = $2
ORDER BY cip.created_at ASC, cipi.position ASC, cipi.created_at ASC;

-- name: ListChatSessionIssues :many
SELECT *
FROM issue
WHERE workspace_id = $1
  AND origin_type = 'chat_session'
  AND origin_id = $2
ORDER BY created_at ASC;

-- name: ListChatSessionOutputMetadata :many
SELECT
  tom.*,
  CASE
    WHEN atq.chat_session_id = $2 THEN 'chat_task'::text
    ELSE 'issue_task'::text
  END AS source_type,
  source_issue.id AS source_issue_id,
  source_issue.title AS source_issue_title,
  source_issue.number AS source_issue_number
FROM task_output_metadata tom
JOIN agent_task_queue atq ON atq.id = tom.task_id
LEFT JOIN issue source_issue ON source_issue.id = atq.issue_id
  AND source_issue.workspace_id = $1
  AND source_issue.origin_type = 'chat_session'
  AND source_issue.origin_id = $2
WHERE tom.workspace_id = $1
  AND (
      atq.chat_session_id = $2
      OR source_issue.id IS NOT NULL
  )
ORDER BY tom.created_at ASC, tom.filename ASC;
