# Execution Workflow Definitions

## Scenario: Editable agent execution workflows

### 1. Scope / Trigger

- Trigger: replace hardcoded assignment execution protocol templates with editable workflow definitions.
- Trigger: add workspace-level Workflows UI beside Runtimes and Skills.
- Trigger: add project workflow binding, issue workflow override, and queue-time workflow snapshots.
- Trigger: preserve current assignment execution behavior while making Standard Assignment, Trellis Task, Direct Task, Research Note, and Comment Response system-seeded workflow definitions.

This is cross-layer work. It changes database schema, sqlc queries, HTTP API contracts, daemon claim payloads, execution environment rendering, core TypeScript types, React Query hooks, and shared views.

Design records:

- `CONTEXT.md`
- `docs/adr/0001-workflow-definitions-bindings-and-overrides.md`
- `docs/agent-workflows-design.md`

### 2. Signatures

#### DB

Recommended tables and columns:

```sql
CREATE TABLE workflow_definition (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL CHECK (origin IN ('system_seeded', 'user')),
  system_key TEXT,
  forked_from_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
  current_published_revision_id UUID,
  created_by UUID REFERENCES "user"(id),
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
  created_by UUID REFERENCES "user"(id),
  published_at TIMESTAMPTZ,
  deprecated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workflow_definition_id, revision_number)
);

CREATE UNIQUE INDEX workflow_definition_system_key_unique
  ON workflow_definition(workspace_id, system_key)
  WHERE system_key IS NOT NULL;

ALTER TABLE workflow_definition
  ADD CONSTRAINT workflow_definition_current_revision_fk
  FOREIGN KEY (current_published_revision_id)
  REFERENCES workflow_revision(id)
  ON DELETE SET NULL;
```

Binding/snapshot fields:

```sql
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
```

Use nullable columns for backwards compatibility. Existing tasks and non-assignment trigger types may have null workflow fields until their trigger rules are implemented.

#### Workflow schema JSON

Canonical persisted schema lives in `workflow_revision.schema`:

```json
{
  "schema_version": 1,
  "applicability": ["assignment"],
  "source": {
    "format": "markdown",
    "body_template": "## Task Execution Protocol\n\nRun `multica issue get {{issue_id}} --output json`."
  },
  "variables": [
    { "name": "issue_id", "required": true, "source": "issue.id" },
    { "name": "project_title", "required": false, "source": "project.title" }
  ],
  "steps": [
    {
      "id": "context-first",
      "name": "Context First",
      "order": 1,
      "required": true,
      "depends_on": [],
      "body_template": "Read issue details and comments before editing.",
      "artifacts": []
    }
  ],
  "gates": [],
  "preview": {
    "default_trigger_type": "assignment"
  }
}
```

`workflow_revision.schema` is the only source of truth. Source editor, step graph editor, and preview must all read/write this schema. Do not persist an independent Markdown document beside it.

#### Workflow snapshot JSON

Queue-time resolver writes `agent_task_queue.workflow_snapshot`:

```json
{
  "schema_version": 1,
  "trigger_type": "assignment",
  "definition_id": "uuid",
  "revision_id": "uuid",
  "revision_number": 3,
  "workflow_name": "Trellis Task",
  "origin": "system_seeded",
  "resolved_at": "2026-05-17T00:00:00Z",
  "schema": {},
  "rendered_markdown": "## Trellis Task Protocol\n\n...",
  "capability_warnings": [
    {
      "code": "unsupported_step_graph",
      "message": "Agent has not declared step graph support."
    }
  ]
}
```

The daemon consumes `rendered_markdown`. The snapshot schema is retained for audit, preview, and future richer execution.

#### API

Use workspace-scoped routes:

```text
GET    /api/workflows?applicability=assignment&include_archived=false
POST   /api/workflows
GET    /api/workflows/{id}
PATCH  /api/workflows/{id}
POST   /api/workflows/{id}/draft
PUT    /api/workflows/{id}/draft
POST   /api/workflows/{id}/publish
POST   /api/workflows/{id}/fork
POST   /api/workflows/preview
```

Project and issue routes extend existing payloads:

```json
{
  "workflow_definition_id": "uuid-or-null"
}
```

```json
{
  "workflow_override_definition_id": "uuid-or-null"
}
```

Daemon claim payload extends `AgentTask`:

```json
{
  "workflow_definition_id": "uuid-or-null",
  "workflow_revision_id": "uuid-or-null",
  "workflow_snapshot": {
    "rendered_markdown": "..."
  }
}
```

### 3. Contracts

#### Selection precedence

For issue assignment tasks:

```text
issue.workflow_override_definition_id
> project.workflow_definition_id
> workspace.default_assignment_workflow_definition_id
> built-in fallback system workflow
```

For comment-triggered `@agent` tasks:

```text
explicit full-workflow request
> workspace.default_comment_workflow_definition_id
> Comment Response system workflow
```

Comment-triggered tasks do not silently inherit project workflow.

#### Publish lifecycle

- `draft`: editable, cannot be selected by project/issue/workspace binding.
- `published`: current selectable revision for a workflow definition.
- `deprecated`: retained for audit/history, cannot be newly selected.

Publishing a draft:

- validates schema and applicability,
- marks the new revision as published,
- updates `workflow_definition.current_published_revision_id`,
- affects only tasks queued after publish,
- never mutates existing `agent_task_queue.workflow_snapshot`.

#### System-seeded definitions

- Seed into every workspace.
- `origin = 'system_seeded'`.
- System seed rows are read-only through normal update endpoints.
- Users customize through `POST /api/workflows/{id}/fork`.
- Bindings reference definition IDs, not hardcoded slugs.
- Seed operation must be idempotent by `(workspace_id, system_key)`.

#### Rendering

- Server resolves and renders workflow snapshot when `agent_task_queue` is created.
- Daemon claim must not re-resolve workflow definitions.
- Daemon injects `workflow_snapshot.rendered_markdown` into the execution environment.
- Legacy agent `execution_protocol_enabled` / `execution_protocol_slug` can remain as compatibility input only until migration is complete; it must not be the primary workflow selector.

#### Frontend

- Add workspace route `/:workspaceSlug/workflows`.
- Add sidebar item beside Runtimes and Skills.
- Use Multica's shared UI library and design system.
- Workflows page includes list/search/filter and create/fork actions.
- Workflow detail/editor includes `Source`, `Steps`, and `Preview` tabs.
- `Preview` includes both a graph preview of the structured step dependencies
  and the rendered Markdown from the server preview contract.
- Graph preview renders nodes from `schema.steps` and edges from
  `steps[].depends_on`; it must be derived from the same schema object as the
  Steps editor and must update when steps or dependencies change.
- Graph preview provides an expand/collapse control in the header. Expanded
  mode fills the Preview workspace and can return to the normal split view.
- Graph preview must recalculate viewport scale and center the graph content
  whenever the Preview pane changes size, including expand and collapse
  transitions. Do not leave the graph top-left anchored after a size change.
- Preview renders Markdown through the shared app Markdown renderer. It must
  not display the rendered Markdown string inside a raw `<pre>` block.
- Graph preview and rendered Markdown preview use independent scroll containers
  inside the Preview tab. Long rendered Markdown must not resize, push, or hide
  the graph preview, and scrolling Markdown must not scroll the graph pane.
- Source, Steps, and Preview tab triggers use consistent icon treatment.
- Steps editor presents steps as a numbered, low-chrome ordered editing list;
  avoid separate card boxes around every step unless the visual system changes.
- Project and issue workflow selectors filter by applicability.
- Project and issue workflow selectors submit workflow definition IDs, but their
  collapsed trigger labels must resolve those IDs back to workflow names from
  the loaded workflow metadata. Do not show raw UUIDs to users except as a
  last-resort fallback for missing metadata.
- Capability mismatch is warning-only in v1 and must not block assignment.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
|-----------|-------------------|
| Create workflow with empty name | `400`, field error |
| Create/update schema with unsupported `schema_version` | `400`, schema error |
| Schema missing `applicability` | `400`, schema error |
| Schema has duplicate step IDs | `400`, schema error |
| Step dependency references unknown step | `400`, schema error |
| Publish workflow with invalid schema | `400`, no revision status change |
| Bind project to workflow without assignment applicability | `400` |
| Bind project to workflow without current published revision | `400` |
| Set issue override to workflow without assignment applicability | `400` |
| Edit system-seeded workflow directly | `403`, suggest fork in response detail |
| Fork workflow not visible in workspace | `404` |
| Queue assignment when selected workflow was deleted/archived | fall back by precedence and record warning |
| Capability mismatch | `200`, warning returned in UI context and stored in snapshot |
| Daemon claims task with null workflow snapshot | legacy behavior/fallback, no panic |

### 5. Good/Base/Bad Cases

- Good: A project binds to Trellis Task. Issue A is assigned to Agent 1 and snapshots Trellis revision 2. The workflow is then published to revision 3. Issue B assigned later snapshots revision 3. Issue A still runs revision 2.
- Good: A small bug inside the same project sets issue override to Direct Task. Reassigning from Agent 1 to Agent 2 keeps Direct Task because workflow belongs to the issue, not the agent.
- Good: A user forks the system Trellis Task, edits source/steps, publishes it, and binds the project to the fork.
- Good: A project binding select stores the selected workflow definition ID
  while the collapsed control displays the workflow name, for example
  `Trellis Task`, not the UUID.
- Good: A user opens Preview and sees the workflow step graph with dependency
  arrows plus the rendered Markdown generated from the same schema.
- Good: The rendered Markdown preview shows headings, lists, code blocks, and
  tables as readable Markdown output rather than raw source text.
- Good: The graph preview can expand to fill the Preview tab and then collapse
  back to the normal graph plus rendered Markdown layout.
- Good: Expanding or collapsing the graph preview keeps the workflow graph
  scaled to the available viewport and centered in the graph pane.
- Good: A long rendered Markdown preview scrolls inside the Markdown pane while
  the graph remains visible and independently scrollable.
- Base: Workspace has no project binding and no issue override. Assignment uses workspace default assignment workflow.
- Base: Existing task has null workflow snapshot after migration. Daemon falls back to current legacy prompt path or standard assignment fallback.
- Bad: A comment mention automatically runs the full project workflow. This is wrong; comments default to Comment Response unless full workflow execution is explicit.
- Bad: Frontend stores Markdown and step graph as separate independent models. This is wrong; both edit the same workflow schema.
- Bad: Preview shows only rendered Markdown and omits the graph view. This hides
  dependency structure even though the workflow schema has step graph data.

### 6. Tests Required

Backend/db:

- Migration applies and rolls back cleanly.
- sqlc queries create, list, fork, draft, publish, and fetch current published revisions.
- System seed is idempotent per workspace.
- Direct update of system-seeded workflow returns `403`.
- Publish moves current pointer and leaves prior snapshots untouched.

Resolver/service:

- Selection precedence: issue override, project binding, workspace default, fallback.
- Applicability filter rejects invalid binding/override.
- Queue-time snapshot stores definition id, revision id, schema, rendered Markdown, and warnings.
- Publishing after queue does not affect queued task snapshot.
- Capability mismatch emits warning but does not block queueing.

Daemon/execenv:

- Claim payload with workflow snapshot injects rendered Markdown.
- Claim payload without workflow snapshot keeps legacy fallback behavior.
- Daemon does not call workflow resolution on claim.

Frontend/core/views:

- TypeScript types cover workflow definition, revision, schema, snapshot, applicability, and warnings.
- React Query keys invalidate on workflow create/update/publish/fork.
- Workflows page follows Runtimes/Skills page structure and shared components.
- Source/Steps/Preview tabs round-trip one schema object.
- Preview tab includes a graph view derived from `steps[].depends_on` and a
  rendered Markdown view; tests or visual QA must verify both are present.
- Preview UI verification covers readable Markdown rendering, graph
  expand/collapse, numbered step rows, and consistent Source/Steps/Preview tab
  icons.
- Preview UI verification covers independent graph and Markdown scrolling with
  long rendered Markdown content.
- Graph preview tests or manual QA cover both collapsed and expanded viewport
  sizes so centering/scale regressions are caught.
- Project and issue selectors filter by applicability and display capability warnings.
- Project and issue selector tests or manual QA verify that selected workflow
  IDs render as workflow names in the collapsed control after binding changes.

Regression:

- Existing agent creation/update tests pass while deprecated execution protocol fields remain.
- Existing assignment task tests pass with workflow snapshot present.
- `pnpm typecheck`, `pnpm test`, focused Go handler/service tests, and daemon execenv tests run.

### 7. Wrong vs Correct

#### Wrong

```text
agent.execution_protocol_slug = "trellis-task"
daemon claim resolves slug from hardcoded Go map
```

This makes workflow selection agent-owned and hardcoded. Reassigning the same issue to another agent can change the process.

#### Correct

```text
issue/project/workspace resolves workflow_definition_id
server snapshots current published workflow_revision at queue time
daemon injects agent_task_queue.workflow_snapshot.rendered_markdown
```

This keeps workflow ownership with the work, preserves auditability, and prevents delayed claims from observing unintended template changes.
