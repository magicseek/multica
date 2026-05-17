CREATE TABLE workflow_definition (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    origin TEXT NOT NULL CHECK (origin IN ('system_seeded', 'user')),
    system_key TEXT,
    forked_from_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
    current_published_revision_id UUID,
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workflow_revision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_definition_id UUID NOT NULL REFERENCES workflow_definition(id) ON DELETE CASCADE,
    revision_number INT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'deprecated')),
    schema JSONB NOT NULL,
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ,
    deprecated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workflow_definition_id, revision_number)
);

ALTER TABLE workflow_definition
ADD CONSTRAINT workflow_definition_current_published_revision_fk
FOREIGN KEY (current_published_revision_id)
REFERENCES workflow_revision(id)
ON DELETE SET NULL;

CREATE UNIQUE INDEX workflow_definition_system_key_unique
ON workflow_definition(workspace_id, system_key)
WHERE system_key IS NOT NULL;

CREATE INDEX workflow_definition_workspace_idx
ON workflow_definition(workspace_id, archived_at, origin, name);

CREATE INDEX workflow_revision_definition_idx
ON workflow_revision(workflow_definition_id, status, revision_number DESC);

ALTER TABLE workspace
ADD COLUMN default_assignment_workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
ADD COLUMN default_comment_workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE project
ADD COLUMN workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE issue
ADD COLUMN workflow_override_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE agent_task_queue
ADD COLUMN workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
ADD COLUMN workflow_revision_id UUID REFERENCES workflow_revision(id) ON DELETE SET NULL,
ADD COLUMN workflow_snapshot JSONB;

CREATE INDEX project_workflow_definition_idx
ON project(workflow_definition_id)
WHERE workflow_definition_id IS NOT NULL;

CREATE INDEX issue_workflow_override_definition_idx
ON issue(workflow_override_definition_id)
WHERE workflow_override_definition_id IS NOT NULL;

CREATE INDEX agent_task_queue_workflow_definition_idx
ON agent_task_queue(workflow_definition_id)
WHERE workflow_definition_id IS NOT NULL;
