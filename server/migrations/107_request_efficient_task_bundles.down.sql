DROP INDEX IF EXISTS task_bundle_item_bundle_status_idx;
DROP INDEX IF EXISTS task_bundle_item_issue_idx;
DROP TABLE IF EXISTS task_bundle_item;

DROP INDEX IF EXISTS task_bundle_agent_status_idx;
DROP INDEX IF EXISTS task_bundle_workspace_status_idx;
DROP INDEX IF EXISTS agent_task_queue_task_bundle_unique;

ALTER TABLE agent_task_queue
DROP COLUMN IF EXISTS task_bundle_id;

DROP TABLE IF EXISTS task_bundle;

ALTER TABLE agent
DROP COLUMN IF EXISTS request_efficient_enabled;
