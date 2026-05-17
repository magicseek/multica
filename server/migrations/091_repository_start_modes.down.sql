DROP TABLE IF EXISTS task_output_metadata;
DROP TABLE IF EXISTS repository_operation;

ALTER TABLE chat_session
    DROP COLUMN IF EXISTS default_repository_id;

DROP TABLE IF EXISTS project_repository;
DROP TABLE IF EXISTS repository_binding;
DROP TABLE IF EXISTS repository;
