DROP INDEX IF EXISTS workflow_quality_gate_result_run_idx;
DROP TABLE IF EXISTS workflow_quality_gate_result;

DROP INDEX IF EXISTS workflow_review_run_status_idx;
DROP TABLE IF EXISTS workflow_review;

DROP INDEX IF EXISTS workflow_artifact_run_idx;
DROP TABLE IF EXISTS workflow_artifact;

DROP INDEX IF EXISTS workflow_step_run_run_status_idx;
DROP INDEX IF EXISTS workflow_step_run_run_order_idx;
DROP TABLE IF EXISTS workflow_step_run;

DROP INDEX IF EXISTS workflow_run_autopilot_run_idx;
DROP INDEX IF EXISTS workflow_run_chat_session_idx;
DROP INDEX IF EXISTS workflow_run_issue_idx;
DROP INDEX IF EXISTS workflow_run_workspace_status_idx;
DROP TABLE IF EXISTS workflow_run;
