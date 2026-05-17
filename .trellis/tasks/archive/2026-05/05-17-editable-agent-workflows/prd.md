# Editable Agent Workflow Definitions

## Problem

Multica currently has assignment execution templates hardcoded in Go and selected primarily through agent-level execution protocol fields. That makes workflow behavior difficult to edit, difficult to preview, and too dependent on which agent receives an issue.

We need workflow definitions to become first-class workspace configuration, similar to Runtimes and Skills, while preserving deterministic execution through queue-time snapshots.

## Goals

- Replace hardcoded execution protocol templates with workspace-level editable workflow definitions.
- Seed built-in workflows into each workspace as read-only system definitions:
  - Standard Assignment
  - Trellis Task
  - Direct Task
  - Research Note
  - Comment Response
- Support project workflow binding so issues in the same project share the same work process across agent reassignment.
- Support issue workflow override for lightweight bug/research/one-off tasks.
- Resolve assignment workflow at `agent_task_queue` creation time and store an immutable workflow snapshot.
- Inject the snapshot's rendered Markdown into daemon execution environments.
- Add a workspace-level Workflows UI beside Runtimes and Skills, using Multica UI/design system.
- Preserve legacy agent execution protocol behavior during rollout.

## Non-Goals

- Do not build a separate workflow runner in the first implementation.
- Do not let daemon claim re-resolve workflow definitions.
- Do not make agent-level default workflow selection the primary model.
- Do not persist Markdown and step graph as separate truth sources.
- Do not copy ai-desk UI styling; use it only as product reference.

## Source Documents

- `CONTEXT.md`
- `docs/adr/0001-workflow-definitions-bindings-and-overrides.md`
- `docs/agent-workflows-design.md`
- `.trellis/spec/workflows/execution-workflow-definitions.md`

## Requirements

### Data model

- Add workflow definition and workflow revision storage.
- Workflow definition owns metadata such as workspace, name, description, origin, system key, fork source, current published revision, creator, archive state.
- Workflow revision owns versioned canonical schema with `draft`, `published`, and `deprecated` lifecycle.
- Add binding fields:
  - workspace default assignment workflow
  - workspace default comment workflow
  - project workflow binding
  - issue workflow override
- Add task snapshot fields:
  - workflow definition id
  - workflow revision id
  - workflow snapshot JSON
- Migrations must be backwards-compatible with existing rows and tasks.

### Workflow schema

- `workflow_revision.schema` is canonical.
- Source editor, step graph editor, and preview must read/write the same schema.
- Schema includes at least:
  - schema version
  - applicability
  - Markdown source template
  - variables
  - steps
  - gates/artifacts placeholder
- Workflow applicability drives selection filtering for assignment, comment response, chat, and autopilot run triggers.

### System seeds

- Seed system workflows idempotently per workspace by `(workspace_id, system_key)`.
- System-seeded workflow definitions are read-only through normal update endpoints.
- Users customize built-ins by forking/copying them into editable workflow definitions.
- Bindings reference workflow definition IDs, not hardcoded slugs.

### API/service

- Add workspace-scoped workflow endpoints for list, create, get, patch metadata, draft, publish, fork, and preview.
- Validate schema at draft/publish boundaries.
- Reject binding a project/issue/workspace default to a workflow without a current published revision for the requested trigger.
- Reject direct mutation of system-seeded definitions with `403`.
- Expose field-level validation errors for invalid workflow schemas.

### Resolution and snapshots

- Assignment workflow resolution order:
  1. issue override
  2. project binding
  3. workspace default assignment workflow
  4. built-in fallback workflow
- Comment mention workflow resolution:
  1. explicit full-workflow request
  2. workspace default comment workflow
  3. Comment Response system workflow
- Assignment tasks snapshot workflow at queue time, not claim time.
- Snapshot includes definition id, revision id, revision number, workflow name, origin, trigger type, resolved timestamp, schema, rendered Markdown, and capability warnings.
- Publishing a new workflow revision never mutates queued/running/historical snapshots.

### Daemon execution

- Daemon claim payload includes workflow snapshot fields.
- Daemon injects `workflow_snapshot.rendered_markdown` into the execution environment.
- Daemon must not resolve workflow definitions on claim.
- Null snapshot keeps legacy fallback behavior.

### Frontend

- Add `/:workspaceSlug/workflows` route for web and desktop.
- Add sidebar item beside Runtimes and Skills.
- Add core types, API client functions, React Query keys/options, and path builders.
- Workflows page includes list/search/filter/status/origin and create/fork actions.
- Workflow detail/editor includes Source, Steps, and Preview tabs.
- Preview renders from schema using Multica UI/design system.
- Project selector filters assignment-compatible published definitions.
- Issue override selector supports lightweight workflows such as Direct Task and Research Note.
- Capability mismatch is warning-only in v1: show warning and record it in snapshot metadata, but do not block assignment.

## Acceptance Criteria

- Existing assignment execution behavior still works after migration.
- A workspace contains system-seeded workflow definitions after setup/migration.
- A project can bind to Trellis Task; assignment snapshots Trellis current published revision.
- An issue in that project can override to Direct Task; reassignment to a different agent preserves Direct Task.
- Editing and publishing a workflow affects only tasks queued after publish.
- A queued task keeps its snapshot even when a workflow is later published.
- `@agent` comment tasks default to Comment Response, not project full workflow.
- Workflows UI is available in both web and desktop shells.
- System workflow definitions are read-only and forkable.
- Workflow preview is rendered from canonical schema, not from a separate persisted Markdown file.
- Capability mismatch warnings do not block execution.

## Suggested Implementation Slices

1. Database and system seed foundation.
2. Workflow service/API and schema validation.
3. Workflow resolver and queue-time snapshots.
4. Daemon claim payload and execution injection.
5. Frontend core contracts and query hooks.
6. Workflows list/editor UI.
7. Project/issue binding UI.
8. Legacy compatibility and cleanup.

## Verification

Minimum expected checks:

```bash
make sqlc
pnpm typecheck
pnpm test
go test ./internal/daemon/execenv
go test ./internal/daemon
go test ./internal/handler -run Workflow
```

Run broader checks when the implementation stabilizes. If unrelated flaky tests remain, document the exact package and reproduction.
