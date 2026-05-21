ALTER TABLE task_usage
ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
