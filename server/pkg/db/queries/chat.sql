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
WHERE chat_session.id = $1
  AND title_source <> 'user'
  AND NOT (
      title_source = 'first_message'
      AND (
          SELECT count(*)
          FROM chat_message
          WHERE chat_message.chat_session_id = chat_session.id
            AND chat_message.role = 'user'
      ) <= 1
  )
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
INSERT INTO chat_message (
    chat_session_id,
    role,
    content,
    task_id,
    failure_reason,
    elapsed_ms,
    author_type,
    author_member_id,
    author_agent_id,
    plan_run_id,
    consultation_id,
    reply_to_message_id
)
VALUES (
    $1,
    $2,
    $3,
    sqlc.narg(task_id),
    sqlc.narg(failure_reason),
    sqlc.narg(elapsed_ms),
    COALESCE(sqlc.narg('author_type'), CASE WHEN $2 = 'assistant' THEN 'agent' ELSE 'member' END),
    sqlc.narg('author_member_id'),
    sqlc.narg('author_agent_id'),
    sqlc.narg('plan_run_id'),
    sqlc.narg('consultation_id'),
    sqlc.narg('reply_to_message_id')
)
RETURNING *;

-- name: ListChatMessages :many
SELECT * FROM chat_message
WHERE chat_session_id = $1
ORDER BY created_at ASC;

-- name: CreateChatMessageRecipient :one
INSERT INTO chat_message_recipient (
    workspace_id,
    chat_session_id,
    message_id,
    recipient_type,
    recipient_id,
    resolved_agent_id,
    source,
    status,
    task_id,
    warning_code,
    warning_message
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    sqlc.narg('resolved_agent_id'),
    COALESCE(sqlc.narg('source'), 'explicit_mention'),
    COALESCE(sqlc.narg('status'), 'pending'),
    sqlc.narg('recipient_task_id'),
    COALESCE(sqlc.narg('warning_code'), ''),
    COALESCE(sqlc.narg('warning_message'), '')
)
ON CONFLICT (message_id, recipient_type, recipient_id, source)
DO UPDATE SET
    resolved_agent_id = COALESCE(EXCLUDED.resolved_agent_id, chat_message_recipient.resolved_agent_id),
    status = EXCLUDED.status,
    task_id = COALESCE(EXCLUDED.task_id, chat_message_recipient.task_id),
    warning_code = EXCLUDED.warning_code,
    warning_message = EXCLUDED.warning_message,
    updated_at = now()
RETURNING *;

-- name: ListChatMessageRecipientsBySession :many
SELECT *
FROM chat_message_recipient
WHERE chat_session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListChatMessageRecipientsByMessages :many
SELECT *
FROM chat_message_recipient
WHERE message_id = ANY($1::uuid[])
ORDER BY created_at ASC, id ASC;

-- name: UpsertChatSessionDirectedState :one
INSERT INTO chat_session_directed_state (
    chat_session_id,
    workspace_id,
    state,
    active_recipient_type,
    active_recipient_id,
    active_message_id,
    candidate_recipients
) VALUES (
    $1,
    $2,
    $3,
    sqlc.narg('active_recipient_type'),
    sqlc.narg('active_recipient_id'),
    sqlc.narg('active_message_id'),
    COALESCE(sqlc.narg('candidate_recipients'), '[]'::jsonb)
)
ON CONFLICT (chat_session_id)
DO UPDATE SET
    state = EXCLUDED.state,
    active_recipient_type = EXCLUDED.active_recipient_type,
    active_recipient_id = EXCLUDED.active_recipient_id,
    active_message_id = EXCLUDED.active_message_id,
    candidate_recipients = EXCLUDED.candidate_recipients,
    updated_at = now()
RETURNING *;

-- name: GetChatSessionDirectedState :one
SELECT *
FROM chat_session_directed_state
WHERE chat_session_id = $1;

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

-- name: SetChatMessagePlanRun :one
UPDATE chat_message
SET plan_run_id = $2
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
    trigger_chat_message_id,
    chat_plan_run_id,
    chat_plan_consultation_id,
    chat_task_kind,
    connector_delegated_user_id
)
VALUES (
    $1,
    $2,
    NULL,
    'queued',
    $3,
    $4,
    $5,
    sqlc.narg('chat_plan_run_id'),
    sqlc.narg('chat_plan_consultation_id'),
    COALESCE(sqlc.narg('chat_task_kind'), 'normal'),
    sqlc.narg('connector_delegated_user_id')
)
RETURNING *;

-- name: CreateChatPlanRun :one
INSERT INTO chat_plan_run (
    workspace_id,
    chat_session_id,
    creator_user_id,
    actor_type,
    actor_id,
    lead_agent_id,
    plan_engine,
    engine_version,
    status,
    initial_message_id,
    latest_message_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    'brainstorming',
    $9,
    $9
)
RETURNING *;

-- name: GetChatPlanRun :one
SELECT *
FROM chat_plan_run
WHERE id = $1;

-- name: GetChatPlanRunInSession :one
SELECT *
FROM chat_plan_run
WHERE id = $1
  AND chat_session_id = $2
  AND workspace_id = $3;

-- name: ListChatPlanRunsBySession :many
SELECT *
FROM chat_plan_run
WHERE chat_session_id = $1
  AND workspace_id = $2
ORDER BY updated_at DESC, created_at DESC;

-- name: GetActiveChatPlanRunBySession :one
SELECT *
FROM chat_plan_run
WHERE chat_session_id = $1
  AND workspace_id = $2
  AND status IN ('brainstorming', 'consulting', 'ready_for_approval')
ORDER BY updated_at DESC, created_at DESC
LIMIT 1;

-- name: UpdateChatPlanRunLatestMessage :one
UPDATE chat_plan_run
SET latest_message_id = $2,
    status = CASE
        WHEN status IN ('cancelled', 'failed', 'completed') THEN status
        ELSE 'brainstorming'
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateChatPlanRunStatus :one
UPDATE chat_plan_run
SET status = $2,
    updated_at = now(),
    completed_at = CASE WHEN $2 = 'completed' THEN now() ELSE completed_at END,
    cancelled_at = CASE WHEN $2 = 'cancelled' THEN now() ELSE cancelled_at END,
    failed_at = CASE WHEN $2 = 'failed' THEN now() ELSE failed_at END
WHERE id = $1
RETURNING *;

-- name: UpdateChatPlanRunSummary :one
UPDATE chat_plan_run
SET summary = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: IncrementChatPlanRunConsultationWaveCount :one
UPDATE chat_plan_run
SET consultation_wave_count = consultation_wave_count + 1,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CancelChatPlanRun :one
UPDATE chat_plan_run
SET status = 'cancelled',
    cancelled_at = now(),
    updated_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND status IN ('brainstorming', 'consulting', 'ready_for_approval')
RETURNING *;

-- name: CreateChatPlanConsultation :one
INSERT INTO chat_plan_consultation (
    plan_run_id,
    requester_agent_id,
    target_agent_id,
    request_message_id,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    'pending'
)
ON CONFLICT (plan_run_id, request_message_id, target_agent_id)
DO UPDATE SET updated_at = chat_plan_consultation.updated_at
RETURNING *;

-- name: SetChatPlanConsultationTask :one
UPDATE chat_plan_consultation
SET task_id = $2,
    status = 'running',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkChatPlanConsultationResponded :one
UPDATE chat_plan_consultation
SET response_message_id = $2,
    status = 'responded',
    responded_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkChatPlanConsultationFailedByTask :one
UPDATE chat_plan_consultation
SET status = CASE WHEN sqlc.arg('failure_reason') = 'timeout' THEN 'timed_out' ELSE 'failed' END,
    failed_at = now(),
    updated_at = now()
WHERE task_id = $1
  AND status IN ('pending', 'running')
RETURNING *;

-- name: MarkChatPlanConsultationFailed :one
UPDATE chat_plan_consultation
SET status = $2,
    failed_at = now(),
    updated_at = now()
WHERE id = $1
  AND status IN ('pending', 'running')
RETURNING *;

-- name: GetChatPlanConsultation :one
SELECT *
FROM chat_plan_consultation
WHERE id = $1;

-- name: ListChatPlanConsultationsByRun :many
SELECT *
FROM chat_plan_consultation
WHERE plan_run_id = $1
ORDER BY created_at ASC;

-- name: CountOpenChatPlanConsultations :one
SELECT count(*)::int
FROM chat_plan_consultation
WHERE plan_run_id = $1
  AND status IN ('pending', 'running');

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
SELECT id, status, created_at, agent_id FROM agent_task_queue
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
    source_plan_run_id,
    proposer_agent_id,
    title,
    summary
) VALUES (
    $1,
    $2,
    sqlc.narg('source_chat_message_id'),
    sqlc.narg('source_task_id'),
    sqlc.narg('source_plan_run_id'),
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

-- name: ListProjectChatProposalDuplicateTitleKeys :many
SELECT DISTINCT title_key::text
FROM (
    SELECT lower(btrim(regexp_replace(issue.title, '[[:space:]]+', ' ', 'g'))) AS title_key
    FROM issue
    WHERE issue.workspace_id = $1
      AND issue.project_id = $2
      AND issue.status <> 'cancelled'
    UNION
    SELECT lower(btrim(regexp_replace(cipi.title, '[[:space:]]+', ' ', 'g'))) AS title_key
    FROM chat_issue_proposal_item cipi
    JOIN chat_issue_proposal cip ON cip.id = cipi.proposal_id
    JOIN chat_session cs ON cs.id = cip.chat_session_id
    WHERE cip.workspace_id = $1
      AND cs.project_context_kind = 'project'
      AND cs.project_id = $2
      AND cip.chat_session_id <> $3
      AND cip.status IN ('pending', 'accepted', 'partially_accepted')
      AND cipi.status IN ('pending', 'created')
) duplicate_titles
WHERE title_key <> '';

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
