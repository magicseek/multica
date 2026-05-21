-- Workflow runtime state

-- name: CreateWorkflowRun :one
INSERT INTO workflow_run (
    workspace_id,
    agent_task_queue_id,
    issue_id,
    chat_session_id,
    autopilot_run_id,
    workflow_definition_id,
    workflow_revision_id,
    trigger_type,
    snapshot,
    status
)
VALUES (
    $1,
    $2,
    sqlc.narg('issue_id'),
    sqlc.narg('chat_session_id'),
    sqlc.narg('autopilot_run_id'),
    sqlc.narg('workflow_definition_id'),
    sqlc.narg('workflow_revision_id'),
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetWorkflowRunByTask :one
SELECT * FROM workflow_run
WHERE agent_task_queue_id = $1;

-- name: GetWorkflowRunByStepRun :one
SELECT wr.* FROM workflow_run wr
JOIN workflow_step_run wsr ON wsr.workflow_run_id = wr.id
WHERE wsr.id = $1;

-- name: GetWorkflowRun :one
SELECT * FROM workflow_run
WHERE id = $1;

-- name: ListWorkflowRunsByIssue :many
SELECT * FROM workflow_run
WHERE issue_id = $1
ORDER BY created_at DESC;

-- name: ListWorkflowRunsByChatSession :many
SELECT * FROM workflow_run
WHERE chat_session_id = $1
  AND workspace_id = $2
ORDER BY created_at DESC;

-- name: ListWorkflowRunsByAutopilotRun :many
SELECT * FROM workflow_run
WHERE autopilot_run_id = $1
  AND workspace_id = $2
ORDER BY created_at DESC;

-- name: UpdateWorkflowRunStatus :one
UPDATE workflow_run SET
    status = $2,
    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('failed', 'completed') THEN COALESCE(completed_at, now()) ELSE completed_at END,
    cancelled_at = CASE WHEN $2 = 'cancelled' THEN COALESCE(cancelled_at, now()) ELSE cancelled_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateWorkflowRunStatusByTask :one
UPDATE workflow_run SET
    status = $2,
    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('failed', 'completed') THEN COALESCE(completed_at, now()) ELSE completed_at END,
    cancelled_at = CASE WHEN $2 = 'cancelled' THEN COALESCE(cancelled_at, now()) ELSE cancelled_at END,
    updated_at = now()
WHERE agent_task_queue_id = $1
RETURNING *;

-- name: CreateWorkflowStepRun :one
INSERT INTO workflow_step_run (
    workflow_run_id,
    step_definition_id,
    title,
    order_index,
    required,
    status,
    execution_kind,
    depends_on_step_ids,
    artifact_inputs,
    snapshot,
    attempt
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11
)
RETURNING *;

-- name: ListWorkflowStepRunsByRun :many
SELECT * FROM workflow_step_run
WHERE workflow_run_id = $1
ORDER BY order_index ASC, created_at ASC;

-- name: GetWorkflowStepRun :one
SELECT * FROM workflow_step_run
WHERE id = $1;

-- name: UpdateWorkflowStepRunStatus :one
UPDATE workflow_step_run SET
    status = $2,
    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'skipped') THEN COALESCE(completed_at, now()) ELSE completed_at END,
    error = COALESCE(sqlc.narg('error'), error),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateWorkflowStepRunRetry :one
WITH source AS (
    SELECT *
    FROM workflow_step_run
    WHERE workflow_step_run.id = $1
),
next_attempt AS (
    SELECT COALESCE(MAX(attempt), 0) + 1 AS attempt
    FROM workflow_step_run
    WHERE workflow_run_id = (SELECT workflow_run_id FROM source)
      AND step_definition_id = (SELECT step_definition_id FROM source)
)
INSERT INTO workflow_step_run (
    workflow_run_id,
    step_definition_id,
    title,
    order_index,
    required,
    status,
    execution_kind,
    depends_on_step_ids,
    artifact_inputs,
    snapshot,
    attempt
)
SELECT
    source.workflow_run_id,
    source.step_definition_id,
    source.title,
    source.order_index,
    source.required,
    $2,
    source.execution_kind,
    source.depends_on_step_ids,
    source.artifact_inputs,
    source.snapshot,
    next_attempt.attempt
FROM source, next_attempt
RETURNING *;

-- name: CreateWorkflowArtifactVersion :one
WITH next_version AS (
    SELECT COALESCE(MAX(version), 0) + 1 AS version
    FROM workflow_artifact
    WHERE workflow_step_run_id = $2
      AND logical_name = $3
)
INSERT INTO workflow_artifact (
    workflow_run_id,
    workflow_step_run_id,
    logical_name,
    version,
    content_kind,
    content_text,
    content_json,
    producer_type,
    producer_id
)
VALUES (
    $1,
    $2,
    $3,
    (SELECT version FROM next_version),
    $4,
    sqlc.narg('content_text'),
    sqlc.narg('content_json'),
    $5,
    sqlc.narg('producer_id')
)
RETURNING *;

-- name: ListWorkflowArtifactsByRun :many
SELECT * FROM workflow_artifact
WHERE workflow_run_id = $1
ORDER BY created_at DESC, logical_name ASC, version DESC;

-- name: ListWorkflowArtifactsByStep :many
SELECT * FROM workflow_artifact
WHERE workflow_step_run_id = $1
ORDER BY logical_name ASC, version DESC;

-- name: GetWorkflowArtifact :one
SELECT * FROM workflow_artifact
WHERE id = $1;

-- name: GetWorkflowArtifactVersionByAnchor :one
SELECT versioned.* FROM workflow_artifact anchor
JOIN workflow_artifact versioned
  ON versioned.workflow_step_run_id = anchor.workflow_step_run_id
 AND versioned.logical_name = anchor.logical_name
WHERE anchor.id = $1
  AND versioned.version = $2;

-- name: CreateWorkflowReview :one
INSERT INTO workflow_review (
    workflow_run_id,
    workflow_step_run_id,
    workflow_artifact_id,
    status
)
VALUES (
    $1,
    sqlc.narg('workflow_step_run_id'),
    sqlc.narg('workflow_artifact_id'),
    'requested'
)
RETURNING *;

-- name: ListWorkflowReviewsByRun :many
SELECT * FROM workflow_review
WHERE workflow_run_id = $1
ORDER BY created_at DESC;

-- name: GetWorkflowReview :one
SELECT * FROM workflow_review
WHERE id = $1;

-- name: UpdateWorkflowReviewDecision :one
UPDATE workflow_review SET
    status = $2,
    reviewer_id = $3,
    decision_notes = COALESCE(sqlc.narg('decision_notes'), decision_notes),
    reviewed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateWorkflowQualityGateResult :one
INSERT INTO workflow_quality_gate_result (
    workflow_run_id,
    workflow_step_run_id,
    workflow_artifact_id,
    status,
    blocking,
    producer_type,
    producer_id,
    report_text,
    report_json
)
VALUES (
    $1,
    $2,
    sqlc.narg('workflow_artifact_id'),
    $3,
    $4,
    $5,
    sqlc.narg('producer_id'),
    sqlc.narg('report_text'),
    sqlc.narg('report_json')
)
RETURNING *;

-- name: ListWorkflowQualityGateResultsByRun :many
SELECT * FROM workflow_quality_gate_result
WHERE workflow_run_id = $1
ORDER BY created_at DESC;

-- name: ListWorkflowQualityGateResultsByStep :many
SELECT * FROM workflow_quality_gate_result
WHERE workflow_step_run_id = $1
ORDER BY created_at DESC;

-- name: CreateWorkflowInputRequest :one
WITH next_round AS (
    SELECT COALESCE(MAX(round_index), 0) + 1 AS round_index
    FROM workflow_input_request
    WHERE workflow_step_run_id = @workflow_step_run_id
)
INSERT INTO workflow_input_request (
    workspace_id,
    workflow_run_id,
    workflow_step_run_id,
    issue_id,
    chat_session_id,
    question_comment_id,
    requester_agent_id,
    status,
    question_text,
    round_index,
    max_rounds
)
VALUES (
    @workspace_id,
    @workflow_run_id,
    @workflow_step_run_id,
    sqlc.narg('issue_id'),
    sqlc.narg('chat_session_id'),
    sqlc.narg('question_comment_id'),
    sqlc.narg('requester_agent_id'),
    'requested',
    @question_text,
    (SELECT round_index FROM next_round),
    @max_rounds
)
RETURNING *;

-- name: GetWorkflowInputRequest :one
SELECT * FROM workflow_input_request
WHERE id = $1;

-- name: ListWorkflowInputRequestsByRun :many
SELECT * FROM workflow_input_request
WHERE workflow_run_id = $1
ORDER BY requested_at DESC, created_at DESC;

-- name: ListWorkflowInputRequestsByStepRun :many
SELECT * FROM workflow_input_request
WHERE workflow_step_run_id = $1
ORDER BY round_index DESC, requested_at DESC;

-- name: CountWorkflowInputRequestRoundsByStepRun :one
SELECT count(*)::int FROM workflow_input_request
WHERE workflow_step_run_id = $1;

-- name: GetOpenWorkflowInputRequestByStepRun :one
SELECT * FROM workflow_input_request
WHERE workflow_step_run_id = $1 AND status = 'requested';

-- name: AnswerWorkflowInputRequest :one
UPDATE workflow_input_request SET
    status = 'answered',
    answer_text = @answer_text,
    answer_comment_id = sqlc.narg('answer_comment_id'),
    responder_id = @responder_id,
    answered_at = now(),
    updated_at = now()
WHERE id = @id AND status = 'requested'
RETURNING *;

-- name: CancelWorkflowInputRequest :one
UPDATE workflow_input_request SET
    status = 'cancelled',
    cancelled_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'requested'
RETURNING *;
