-- name: UpsertTaskUsage :exec
-- Bumps `updated_at` on INSERT and on conflict so the daily-rollup worker
-- (migration 073) detects the row as dirty and re-aggregates its bucket.
-- Without the conflict-side bump, a correction to historical token counts
-- would never propagate to the rollup.
INSERT INTO task_usage (task_id, provider, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, metadata, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE(sqlc.narg('metadata'), '{}'::jsonb), now())
ON CONFLICT (task_id, provider, model)
DO UPDATE SET
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    cache_read_tokens = EXCLUDED.cache_read_tokens,
    cache_write_tokens = EXCLUDED.cache_write_tokens,
    metadata = EXCLUDED.metadata,
    updated_at = now();

-- name: GetTaskUsage :many
SELECT * FROM task_usage
WHERE task_id = $1
ORDER BY model;

-- name: GetWorkspaceUsageByDay :many
-- Bucket by tu.created_at (usage report time, ~= task completion time), not
-- atq.created_at (task enqueue time), so tasks that queue one day and execute
-- the next are attributed to the day tokens were actually produced. The since
-- cutoff is truncated to start-of-day so `days=N` yields full calendar days.
SELECT
    DATE(tu.created_at) AS date,
    tu.model,
    SUM(tu.input_tokens)::bigint AS total_input_tokens,
    SUM(tu.output_tokens)::bigint AS total_output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS total_cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS total_cache_write_tokens,
    COUNT(DISTINCT tu.task_id)::int AS task_count
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
GROUP BY DATE(tu.created_at), tu.model
ORDER BY DATE(tu.created_at) DESC, tu.model;

-- name: GetWorkspaceUsageSummary :many
-- Filter by tu.created_at (usage report time), aligned to start-of-day, so
-- `days=N` is interpreted as N full calendar days like the other usage queries.
SELECT
    tu.model,
    SUM(tu.input_tokens)::bigint AS total_input_tokens,
    SUM(tu.output_tokens)::bigint AS total_output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS total_cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS total_cache_write_tokens,
    COUNT(DISTINCT tu.task_id)::int AS task_count
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
GROUP BY tu.model
ORDER BY (SUM(tu.input_tokens) + SUM(tu.output_tokens)) DESC;

-- name: GetIssueUsageSummary :one
SELECT
    COALESCE(SUM(tu.input_tokens), 0)::bigint AS total_input_tokens,
    COALESCE(SUM(tu.output_tokens), 0)::bigint AS total_output_tokens,
    COALESCE(SUM(tu.cache_read_tokens), 0)::bigint AS total_cache_read_tokens,
    COALESCE(SUM(tu.cache_write_tokens), 0)::bigint AS total_cache_write_tokens,
    COUNT(DISTINCT tu.task_id)::int AS task_count
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
WHERE atq.issue_id = $1;

-- name: ListDashboardUsageDaily :many
-- Daily per-(date, model) token aggregates for the workspace, optionally
-- scoped to a single project via sqlc.narg('project_id'). Bucketed by
-- tu.created_at (token-production time) to match GetWorkspaceUsageByDay,
-- so a task that queues one day and finishes the next is attributed to
-- the day the tokens actually landed. Powers the workspace dashboard's
-- daily cost chart.
SELECT
    DATE(tu.created_at) AS date,
    tu.model,
    SUM(tu.input_tokens)::bigint AS input_tokens,
    SUM(tu.output_tokens)::bigint AS output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS cache_write_tokens,
    COUNT(DISTINCT tu.task_id)::int AS task_count
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY DATE(tu.created_at), tu.model
ORDER BY DATE(tu.created_at) DESC, tu.model;

-- name: ListDashboardUsageByAgent :many
-- Per-(agent, model) token aggregates for the workspace, optionally scoped
-- to a single project. Model dimension is preserved so the client can
-- compute cost from its per-model pricing table; the client folds rows by
-- agent for the "by agent" list on the dashboard.
SELECT
    atq.agent_id,
    tu.model,
    SUM(tu.input_tokens)::bigint AS input_tokens,
    SUM(tu.output_tokens)::bigint AS output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS cache_write_tokens,
    COUNT(DISTINCT tu.task_id)::int AS task_count
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY atq.agent_id, tu.model
ORDER BY atq.agent_id, tu.model;

-- name: ListDashboardUsageDailyRollup :many
-- Daily token rollup, served from `task_usage_dashboard_daily` (migration
-- 084). Same wire shape as ListDashboardUsageDaily so the handler can
-- swap them on the `UseDailyRollupForDashboard` flag with no other
-- changes. The rollup is up to ~10 min stale (5 min cron + 5 min lag),
-- which is fine for a dashboard read path.
SELECT
    bucket_date AS date,
    model,
    SUM(input_tokens)::bigint        AS input_tokens,
    SUM(output_tokens)::bigint       AS output_tokens,
    SUM(cache_read_tokens)::bigint   AS cache_read_tokens,
    SUM(cache_write_tokens)::bigint  AS cache_write_tokens,
    SUM(task_count)::int             AS task_count
FROM task_usage_dashboard_daily
WHERE workspace_id = $1
  AND bucket_date >= DATE_TRUNC('day', @since::timestamptz)::date
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
GROUP BY bucket_date, model
ORDER BY bucket_date DESC, model;

-- name: ListDashboardUsageByAgentRollup :many
-- Per-(agent, model) token rollup from `task_usage_dashboard_daily`.
-- task_count here is the SUM of per-bucket distinct counts; one task that
-- spans multiple days lands in multiple buckets, so this can over-count
-- by date. The frontend prefers `ListDashboardAgentRunTime`'s per-agent
-- distinct figure for the user-facing "tasks" column, so this value is
-- informational only.
SELECT
    agent_id,
    model,
    SUM(input_tokens)::bigint        AS input_tokens,
    SUM(output_tokens)::bigint       AS output_tokens,
    SUM(cache_read_tokens)::bigint   AS cache_read_tokens,
    SUM(cache_write_tokens)::bigint  AS cache_write_tokens,
    SUM(task_count)::int             AS task_count
FROM task_usage_dashboard_daily
WHERE workspace_id = $1
  AND bucket_date >= DATE_TRUNC('day', @since::timestamptz)::date
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
GROUP BY agent_id, model
ORDER BY agent_id, model;

-- name: ListDashboardRunTimeDaily :many
-- Daily per-date run time + task counts for the workspace, optionally
-- scoped to a single project. Powers the workspace dashboard's "Time"
-- and "Tasks" metrics on the same toggle as Tokens / Cost. Bucketed by
-- completed_at (terminal time) — same anchor as ListDashboardAgentRunTime
-- so the day boundaries line up with the per-agent run-time card. Only
-- terminal tasks (completed or failed) with both started_at and
-- completed_at populated contribute.
SELECT
    DATE(atq.completed_at) AS date,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (atq.completed_at - atq.started_at)))::bigint,
        0
    )::bigint AS total_seconds,
    COUNT(*)::int AS task_count,
    COUNT(*) FILTER (WHERE atq.status = 'failed')::int AS failed_count
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.started_at IS NOT NULL
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= DATE_TRUNC('day', @since::timestamptz)
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY DATE(atq.completed_at)
ORDER BY DATE(atq.completed_at) DESC;

-- name: ListDashboardAgentRunTime :many
-- Per-agent total task run time and task count for the workspace, optionally
-- scoped to a single project. Counts only terminal runs (completed or failed)
-- with both started_at and completed_at populated — queued/running tasks have
-- no finite duration. Anchored on completed_at so the window matches the
-- token cost window (which is anchored on tu.created_at, ~= completion time).
SELECT
    atq.agent_id,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (atq.completed_at - atq.started_at)))::bigint,
        0
    )::bigint AS total_seconds,
    COUNT(*)::int AS task_count,
    COUNT(*) FILTER (WHERE atq.status = 'failed')::int AS failed_count
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.started_at IS NOT NULL
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= DATE_TRUNC('day', @since::timestamptz)
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY atq.agent_id
ORDER BY total_seconds DESC;

-- name: ListAgentAnalyticsUsageRows :many
-- Raw per-(task, model) usage rows for Project/Chat Analytics. The handler
-- folds this into summary, daily, agent, and source aggregates so the client
-- can compute cost from the preserved model dimension while the run table
-- itself remains separately paginated.
SELECT
    tu.task_id,
    atq.agent_id,
    a.name AS agent_name,
    atq.status,
    CASE
      WHEN atq.chat_session_id IS NOT NULL THEN 'chat'
      WHEN atq.issue_id IS NOT NULL THEN 'issue'
      ELSE 'task'
    END::text AS source_type,
    DATE(tu.created_at) AS date,
    atq.created_at,
    atq.dispatched_at,
    atq.started_at,
    atq.completed_at,
    i.id AS issue_id,
    CASE
      WHEN i.id IS NOT NULL THEN (ws.issue_prefix || '-' || i.number)::text
      ELSE ''
    END::text AS issue_identifier,
    i.title AS issue_title,
    cs.id AS chat_session_id,
    cs.title AS chat_title,
    tu.provider,
    tu.model,
    tu.input_tokens,
    tu.output_tokens,
    tu.cache_read_tokens,
    tu.cache_write_tokens,
    COALESCE(NULLIF(tu.metadata->>'prompt_bytes', '')::bigint, 0)::bigint AS prompt_bytes,
    COALESCE(NULLIF(tu.metadata->>'first_text_ms', '')::bigint, 0)::bigint AS first_text_ms,
    COALESCE(NULLIF(tu.metadata->>'task_message_tool_use_count', '')::bigint, 0)::bigint AS task_message_tool_use_count,
    COALESCE(NULLIF(tu.metadata->>'tool_result_bytes', '')::bigint, 0)::bigint AS tool_result_bytes
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
JOIN workspace ws ON ws.id = a.workspace_id
LEFT JOIN issue i ON i.id = atq.issue_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id
WHERE a.workspace_id = @workspace_id
  AND atq.status IN ('completed', 'failed', 'cancelled')
  AND atq.completed_at IS NOT NULL
  AND (
    NOT @has_since::boolean
    OR tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
  )
  AND (
    NOT @has_before::boolean
    OR tu.created_at < DATE_TRUNC('day', @before::timestamptz)
  )
  AND (
    (@scope::text = 'project' AND (i.project_id = @scope_id OR cs.project_id = @scope_id))
    OR (@scope::text = 'chat' AND atq.chat_session_id = @scope_id)
  )
  AND (
    @source::text = 'all'
    OR (@source::text = 'issues' AND atq.chat_session_id IS NULL AND atq.issue_id IS NOT NULL)
    OR (@source::text = 'chats' AND atq.chat_session_id IS NOT NULL)
  )
ORDER BY tu.created_at DESC, tu.model;

-- name: CountAgentAnalyticsRuns :one
-- Count terminal runs with recorded usage for the current analytics scope.
-- Matches ListAgentAnalyticsRuns exactly except for projection/pagination.
SELECT COUNT(DISTINCT atq.id)::int AS total
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id
WHERE a.workspace_id = @workspace_id
  AND atq.status IN ('completed', 'failed', 'cancelled')
  AND atq.completed_at IS NOT NULL
  AND (
    NOT @has_since::boolean
    OR tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
  )
  AND (
    NOT @has_before::boolean
    OR tu.created_at < DATE_TRUNC('day', @before::timestamptz)
  )
  AND (
    (@scope::text = 'project' AND (i.project_id = @scope_id OR cs.project_id = @scope_id))
    OR (@scope::text = 'chat' AND atq.chat_session_id = @scope_id)
  )
  AND (
    @source::text = 'all'
    OR (@source::text = 'issues' AND atq.chat_session_id IS NULL AND atq.issue_id IS NOT NULL)
    OR (@source::text = 'chats' AND atq.chat_session_id IS NOT NULL)
  );

-- name: ListAgentAnalyticsRuns :many
-- One paginated row per terminal agent run. Tracing metadata is normalized
-- into typed numeric columns so views do not parse arbitrary metadata JSON.
WITH filtered AS (
  SELECT
      atq.id AS task_id,
      atq.agent_id,
      a.name AS agent_name,
      atq.status,
      atq.created_at,
      atq.dispatched_at,
      atq.started_at,
      atq.completed_at,
      atq.autopilot_run_id,
      atq.workflow_definition_id,
      wr.id AS workflow_run_id,
      CASE
        WHEN atq.chat_session_id IS NOT NULL THEN 'chat'
        WHEN atq.issue_id IS NOT NULL THEN 'issue'
        ELSE 'task'
      END::text AS source_type,
      i.id AS issue_id,
      CASE
        WHEN i.id IS NOT NULL THEN (ws.issue_prefix || '-' || i.number)::text
        ELSE ''
      END::text AS issue_identifier,
      i.title AS issue_title,
      cs.id AS chat_session_id,
      cs.title AS chat_title,
      tu.provider,
      tu.model,
      tu.input_tokens,
      tu.output_tokens,
      tu.cache_read_tokens,
      tu.cache_write_tokens,
      tu.metadata
  FROM task_usage tu
  JOIN agent_task_queue atq ON atq.id = tu.task_id
  JOIN agent a ON a.id = atq.agent_id
  JOIN workspace ws ON ws.id = a.workspace_id
  LEFT JOIN issue i ON i.id = atq.issue_id
  LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id
  LEFT JOIN LATERAL (
    SELECT id
    FROM workflow_run
    WHERE agent_task_queue_id = atq.id
    ORDER BY created_at DESC
    LIMIT 1
  ) wr ON TRUE
  WHERE a.workspace_id = @workspace_id
    AND atq.status IN ('completed', 'failed', 'cancelled')
    AND atq.completed_at IS NOT NULL
    AND (
      NOT @has_since::boolean
      OR tu.created_at >= DATE_TRUNC('day', @since::timestamptz)
    )
    AND (
      NOT @has_before::boolean
      OR tu.created_at < DATE_TRUNC('day', @before::timestamptz)
    )
    AND (
      (@scope::text = 'project' AND (i.project_id = @scope_id OR cs.project_id = @scope_id))
      OR (@scope::text = 'chat' AND atq.chat_session_id = @scope_id)
    )
    AND (
      @source::text = 'all'
      OR (@source::text = 'issues' AND atq.chat_session_id IS NULL AND atq.issue_id IS NOT NULL)
      OR (@source::text = 'chats' AND atq.chat_session_id IS NOT NULL)
    )
)
SELECT
    task_id,
    agent_id,
    agent_name,
    status,
    source_type,
    issue_id,
    issue_identifier,
    issue_title,
    chat_session_id,
    chat_title,
    autopilot_run_id,
    workflow_definition_id,
    workflow_run_id,
    created_at,
    dispatched_at,
    started_at,
    completed_at,
    COALESCE(EXTRACT(EPOCH FROM (completed_at - created_at)) * 1000, 0)::bigint AS total_duration_ms,
    COALESCE(EXTRACT(EPOCH FROM (dispatched_at - created_at)) * 1000, 0)::bigint AS queue_ms,
    COALESCE(EXTRACT(EPOCH FROM (started_at - dispatched_at)) * 1000, 0)::bigint AS startup_ms,
    COALESCE(EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000, 0)::bigint AS execution_ms,
    COALESCE(SUM(input_tokens), 0)::bigint AS input_tokens,
    COALESCE(SUM(output_tokens), 0)::bigint AS output_tokens,
    COALESCE(SUM(cache_read_tokens), 0)::bigint AS cache_read_tokens,
    COALESCE(SUM(cache_write_tokens), 0)::bigint AS cache_write_tokens,
    COALESCE(
      jsonb_agg(
        jsonb_build_object(
          'provider', provider,
          'model', model,
          'input_tokens', input_tokens,
          'output_tokens', output_tokens,
          'cache_read_tokens', cache_read_tokens,
          'cache_write_tokens', cache_write_tokens,
          'task_count', 1
        )
        ORDER BY provider, model
      ) FILTER (WHERE model IS NOT NULL),
      '[]'::jsonb
    ) AS model_usage,
    COALESCE(MAX(NULLIF(metadata->>'prompt_bytes', '')::bigint), 0)::bigint AS prompt_bytes,
    COALESCE(MAX(NULLIF(metadata->>'system_prompt_bytes', '')::bigint), 0)::bigint AS system_prompt_bytes,
    COALESCE(MAX(NULLIF(metadata->>'chat_message_bytes', '')::bigint), 0)::bigint AS chat_message_bytes,
    COALESCE(MAX(NULLIF(metadata->>'chat_attachment_count', '')::bigint), 0)::bigint AS chat_attachment_count,
    COALESCE(MAX(NULLIF(metadata->>'agent_instructions_bytes', '')::bigint), 0)::bigint AS agent_instructions_bytes,
    COALESCE(MAX(NULLIF(metadata->>'agent_skill_count', '')::bigint), 0)::bigint AS agent_skill_count,
    COALESCE(MAX(NULLIF(metadata->>'agent_skill_bytes', '')::bigint), 0)::bigint AS agent_skill_bytes,
    COALESCE(MAX(NULLIF(metadata->>'repo_count', '')::bigint), 0)::bigint AS repo_count,
    COALESCE(MAX(NULLIF(metadata->>'repository_count', '')::bigint), 0)::bigint AS repository_count,
    COALESCE(MAX(NULLIF(metadata->>'project_resource_count', '')::bigint), 0)::bigint AS project_resource_count,
    COALESCE(MAX(NULLIF(metadata->>'workflow_snapshot_bytes', '')::bigint), 0)::bigint AS workflow_snapshot_bytes,
    COALESCE(MAX(NULLIF(metadata->>'workflow_step_snapshot_bytes', '')::bigint), 0)::bigint AS workflow_step_snapshot_bytes,
    COALESCE(MAX(NULLIF(metadata->>'autopilot_description_bytes', '')::bigint), 0)::bigint AS autopilot_description_bytes,
    COALESCE(MAX(NULLIF(metadata->>'autopilot_payload_bytes', '')::bigint), 0)::bigint AS autopilot_payload_bytes,
    COALESCE(MAX(NULLIF(metadata->>'quick_create_prompt_bytes', '')::bigint), 0)::bigint AS quick_create_prompt_bytes,
    COALESCE(MAX(NULLIF(metadata->>'exec_env_ms', '')::bigint), 0)::bigint AS exec_env_ms,
    COALESCE(MAX(NULLIF(metadata->>'runtime_config_ms', '')::bigint), 0)::bigint AS runtime_config_ms,
    COALESCE(MAX(NULLIF(metadata->>'backend_create_ms', '')::bigint), 0)::bigint AS backend_create_ms,
    COALESCE(MAX(NULLIF(metadata->>'agent_run_ms', '')::bigint), 0)::bigint AS agent_run_ms,
    COALESCE(MAX(NULLIF(metadata->>'daemon_run_ms', '')::bigint), 0)::bigint AS daemon_run_ms,
    COALESCE(MAX(NULLIF(metadata->>'first_event_ms', '')::bigint), 0)::bigint AS first_event_ms,
    COALESCE(MAX(NULLIF(metadata->>'first_text_ms', '')::bigint), 0)::bigint AS first_text_ms,
    COALESCE(MAX(NULLIF(metadata->>'first_tool_use_ms', '')::bigint), 0)::bigint AS first_tool_use_ms,
    COALESCE(MAX(NULLIF(metadata->>'first_tool_result_ms', '')::bigint), 0)::bigint AS first_tool_result_ms,
    COALESCE(MAX(NULLIF(metadata->>'task_message_text_count', '')::bigint), 0)::bigint AS task_message_text_count,
    COALESCE(MAX(NULLIF(metadata->>'task_message_thinking_count', '')::bigint), 0)::bigint AS task_message_thinking_count,
    COALESCE(MAX(NULLIF(metadata->>'task_message_tool_use_count', '')::bigint), 0)::bigint AS task_message_tool_use_count,
    COALESCE(MAX(NULLIF(metadata->>'task_message_tool_result_count', '')::bigint), 0)::bigint AS task_message_tool_result_count,
    COALESCE(MAX(NULLIF(metadata->>'task_message_error_count', '')::bigint), 0)::bigint AS task_message_error_count,
    COALESCE(MAX(NULLIF(metadata->>'assistant_text_bytes', '')::bigint), 0)::bigint AS assistant_text_bytes,
    COALESCE(MAX(NULLIF(metadata->>'thinking_bytes', '')::bigint), 0)::bigint AS thinking_bytes,
    COALESCE(MAX(NULLIF(metadata->>'tool_input_bytes', '')::bigint), 0)::bigint AS tool_input_bytes,
    COALESCE(MAX(NULLIF(metadata->>'tool_result_bytes', '')::bigint), 0)::bigint AS tool_result_bytes,
    COALESCE(MAX(NULLIF(metadata->>'agent_result_output_bytes', '')::bigint), 0)::bigint AS agent_result_output_bytes
FROM filtered
GROUP BY
    task_id,
    agent_id,
    agent_name,
    status,
    source_type,
    issue_id,
    issue_identifier,
    issue_title,
    chat_session_id,
    chat_title,
    autopilot_run_id,
    workflow_definition_id,
    workflow_run_id,
    created_at,
    dispatched_at,
    started_at,
    completed_at
ORDER BY
    CASE WHEN @sort::text = 'duration_desc' THEN COALESCE(EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000, 0)::bigint END DESC NULLS LAST,
    CASE WHEN @sort::text = 'tokens_desc' THEN COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_write_tokens), 0)::bigint END DESC NULLS LAST,
    completed_at DESC,
    task_id DESC
LIMIT @limit_count::int
OFFSET @offset_count::int;
