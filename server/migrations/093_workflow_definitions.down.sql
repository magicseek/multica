DROP INDEX IF EXISTS agent_task_queue_workflow_definition_idx;
DROP INDEX IF EXISTS issue_workflow_override_definition_idx;
DROP INDEX IF EXISTS project_workflow_definition_idx;

ALTER TABLE agent_task_queue
DROP COLUMN IF EXISTS workflow_snapshot,
DROP COLUMN IF EXISTS workflow_revision_id,
DROP COLUMN IF EXISTS workflow_definition_id;

ALTER TABLE issue
DROP COLUMN IF EXISTS workflow_override_definition_id;

ALTER TABLE project
DROP COLUMN IF EXISTS workflow_definition_id;

ALTER TABLE workspace
DROP COLUMN IF EXISTS default_comment_workflow_definition_id,
DROP COLUMN IF EXISTS default_assignment_workflow_definition_id;

DROP INDEX IF EXISTS workflow_revision_definition_idx;
DROP INDEX IF EXISTS workflow_definition_workspace_idx;
DROP INDEX IF EXISTS workflow_definition_system_key_unique;

ALTER TABLE workflow_definition
DROP CONSTRAINT IF EXISTS workflow_definition_current_published_revision_fk;

DROP TABLE IF EXISTS workflow_revision;
DROP TABLE IF EXISTS workflow_definition;
