# ai-desk Flows to Multica Workflows Migration Plan

> Status: Draft implementation plan
> Date: 2026-05-19
> Scope: Migrate ai-desk workflow definitions and core Flow runtime semantics into Multica-native workflows.
> Decision record: `docs/adr/0003-migrate-ai-desk-flows-through-multica-workflow-runs.md`

## TL;DR

Multica should migrate ai-desk Flows by extending the existing Multica workflow control plane, not by copying ai-desk's browser-driven step orchestration.

Workflow definitions remain versioned JSON schemas in `workflow_revision.schema`. When an agent task is queued, Multica immediately creates an immutable workflow snapshot, a server-owned workflow run, and initial step runs. Daemons execute workflow work and report state through Multica CLI commands. Browser UI edits definitions and observes runs, but does not orchestrate execution.

First-version migration includes step runs, artifacts, human reviews, quality gates, YAML import/export, and a Multica-native builder. It does not migrate ai-desk Party Mode, `WorkflowEngine`, workflow-level sidecar agents, `ticket_required`, workflow participants, historical task/run data, or AI-generated workflow creation.

## Goals

- Preserve ai-desk workflow definition value: steps, dependencies, artifact templates, agent prompts, rules, reviews, quality gates, and import/export.
- Use Multica-native architecture: Go/Postgres control plane, `agent_task_queue`, daemon pull, React Query, `packages/ui`, and Settings-based configuration.
- Keep workflow definitions draftable, publishable, forkable, exportable, and snapshot-safe.
- Make workflow execution observable at step level without forcing a cold-started agent invocation for every small step.
- Store explicit workflow artifacts as reviewable server data while preserving existing Output Metadata privacy boundaries for ordinary local outputs.

## Non-Goals

- Do not migrate ai-desk historical tasks, steps, artifacts, or runs.
- Do not migrate Party Mode fields or keep them as legacy metadata.
- Do not migrate ai-desk `WorkflowEngine` labels (`NONE`, `AETHER`).
- Do not migrate ai-desk `pre_added_agents` / sidecar agent behavior.
- Do not migrate ai-desk workflow participants or per-workflow ACL.
- Do not migrate `WORKFLOW` / `AUTOMATION` as Multica workflow types; automation maps to Autopilot.
- Do not add ReactFlow or copy ai-desk UI components in the first version.
- Do not implement AI-generated workflow creation in the first version.

## Current Source Systems

ai-desk core files:

- `backend/app/models/workflow.py`
- `backend/app/schemas/workflow.py`
- `backend/app/services/workflow_service.py`
- `backend/app/services/workflow_schema_import.py`
- `backend/app/services/workflow_schema_export.py`
- `backend/app/services/task_service.py`
- `backend/app/services/step_context_service.py`
- `backend/app/services/remote_agent_executor.py`
- `frontend/src/components/workflows/WorkflowEditorV2.tsx`
- `frontend/src/components/workflows/workflowSchemaYaml.ts`
- `frontend/src/services/DaemonService.ts`
- `ai-desk-daemon/daemon/internal/daemonapp/*`

Multica target files and surfaces:

- `server/migrations/093_workflow_definitions.up.sql`
- `server/internal/service/workflow.go`
- `server/internal/workflowdefs/workflowdefs.go`
- `server/internal/service/task.go`
- `server/internal/daemon/*`
- `packages/core/types/workflow.ts`
- `packages/core/workflows/*`
- `packages/views/workflows/components/workflows-page.tsx`
- `packages/views/workflows/components/workflow-graph-preview.tsx`
- `packages/views/settings/components/settings-page.tsx`

## Target Architecture

### Definition Layer

`workflow_revision.schema` remains the canonical workflow definition payload.

Definition schema v2 should be self-contained:

- metadata: name, description, category, source origin, import metadata
- applicability: `assignment`, `comment`, `chat`, `autopilot`
- source: editable Markdown/template body
- variables
- steps
- gates
- artifact templates embedded per step
- import warnings metadata, when useful for UI review

Do not normalize definition steps, artifact templates, or gates into editable relational definition tables in the first version.

### Runtime Layer

When `agent_task_queue` is created, Multica resolves the workflow and creates:

- `workflow_snapshot`
- `workflow_run`
- initial `workflow_step_run` rows

Runtime records are server-owned. The daemon may execute and report, but it does not mutate workflow definition content or re-resolve workflow revisions.

Workflow run content is immutable. If the workflow definition is wrong, users publish a new revision and start a new run.

### Execution Layer

A workflow step run is the state unit. A workflow execution batch is an execution pass that may process multiple ready step runs using the same workdir/session until a blocking boundary:

- human review required
- blocking quality gate
- manual step
- external step
- pause / user input
- failure / blocked state

The first version executes DAG-ready steps in deterministic topological order, not in parallel.

### Agent-Facing Contract

First version uses Multica CLI commands, not MCP-only workflow tools:

```text
multica workflow run get <run-id> --output json
multica workflow step get <step-run-id> --output json
multica workflow artifact save <step-run-id> --name <name> --file <path|-> --format markdown|json|text
multica workflow quality report <step-run-id> --artifact <artifact-id> --status pass|fail|warning --blocking true|false --file <path|->
multica workflow step complete <step-run-id>
multica workflow step fail <step-run-id> --reason "..."
multica workflow step pause <step-run-id> --reason "..."
```

MCP can be added later as an ergonomic wrapper over the same backend contracts.

## Proposed Schema Shape

Sketch only; exact names should be finalized in implementation.

```json
{
  "schema_version": 2,
  "name": "Workflow name",
  "description": "Workflow description",
  "applicability": ["assignment"],
  "source": {
    "format": "markdown",
    "body_template": "..."
  },
  "steps": [
    {
      "id": "design",
      "title": "Write design",
      "description": "Produce a design artifact",
      "order": 1,
      "required": true,
      "depends_on": [],
      "execution": {
        "kind": "agent",
        "prompt": "...",
        "rules": "..."
      },
      "artifact": {
        "name": "Design",
        "template": {
          "format": "markdown",
          "content": "...",
          "files": []
        },
        "inputs": []
      },
      "review": {
        "required": true
      },
      "quality_gate": {
        "enabled": true,
        "blocking": false,
        "prompt": "...",
        "report_mode": "summary"
      }
    }
  ],
  "gates": []
}
```

Supported execution kinds:

- `agent`: run through the current Multica agent assignment/runtime path
- `manual`: human member completes through Multica UI
- `external`: represented as waiting/blocked; no first-version integration

## Proposed Runtime Data Model

### `workflow_run`

Suggested fields:

- `id`
- `workspace_id`
- `agent_task_queue_id UNIQUE`
- `issue_id NULL`
- `chat_session_id NULL`
- `autopilot_run_id NULL`
- `workflow_definition_id`
- `workflow_revision_id`
- `trigger_type`
- `snapshot JSONB`
- `status`: `queued`, `running`, `waiting`, `blocked`, `failed`, `completed`, `cancelled`
- `started_at`, `completed_at`, `cancelled_at`
- `created_at`, `updated_at`

### `workflow_step_run`

Suggested fields:

- `id`
- `workflow_run_id`
- `definition_id`
- `title`
- `order_index`
- `required`
- `status`
- `execution_kind`: `agent`, `manual`, `external`
- `depends_on_step_ids JSONB`
- `artifact_inputs JSONB`
- `snapshot JSONB` for step-level immutable config
- `attempt`
- `started_at`, `completed_at`
- `execution_metadata JSONB`
- `error`
- `created_at`, `updated_at`

Step statuses:

```text
pending
ready
running
waiting_review
waiting_quality
waiting_manual
waiting_external
paused
blocked
failed
completed
skipped
```

### `workflow_artifact`

Suggested fields:

- `id`
- `workflow_run_id`
- `workflow_step_run_id`
- `logical_name`
- `version`
- `content_kind`: `text`, `markdown`, `json`
- `content_text`
- `content_json`
- `producer_type`: `agent`, `member`, `system`
- `producer_id`
- `supersedes_artifact_id NULL`
- `created_at`

Artifact versions are immutable. Step retry creates a new artifact version.

### `workflow_review`

Suggested fields:

- `id`
- `workflow_artifact_id`
- `workflow_step_run_id`
- `reviewer_id`
- `status`: `approved`, `rejected`, `changes_requested`
- `comment`
- `created_at`

Required reviews are human-approved in the first version. Agent self-approval is not supported.

### `workflow_quality_gate_result`

Suggested fields:

- `id`
- `workflow_artifact_id`
- `workflow_step_run_id`
- `producer_type`: `agent`, `server`
- `producer_id`
- `blocking`
- `status`: `pass`, `fail`, `warning`
- `score NULL`
- `summary`
- `report_markdown`
- `report_json`
- `created_at`

Agent-produced gate results must be displayed as agent-produced, not as server-trusted platform verdicts.

## API Surface

Extend existing workflow definition endpoints:

```text
GET    /api/workflows
POST   /api/workflows
GET    /api/workflows/{id}
PATCH  /api/workflows/{id}
POST   /api/workflows/{id}/draft
PUT    /api/workflows/{id}/draft
POST   /api/workflows/{id}/publish
POST   /api/workflows/{id}/fork
POST   /api/workflows/preview
POST   /api/workflows/import
GET    /api/workflows/{id}/export?format=yaml
```

Add runtime endpoints:

```text
GET    /api/workflow-runs?issue_id=...
GET    /api/workflow-runs/{id}
POST   /api/workflow-runs/{id}/cancel
POST   /api/workflow-runs/{id}/rerun

GET    /api/workflow-step-runs/{id}
POST   /api/workflow-step-runs/{id}/retry
POST   /api/workflow-step-runs/{id}/manual-complete
POST   /api/workflow-step-runs/{id}/skip
POST   /api/workflow-step-runs/{id}/fail

POST   /api/workflow-step-runs/{id}/artifacts
GET    /api/workflow-artifacts/{id}
POST   /api/workflow-artifacts/{id}/reviews
POST   /api/workflow-artifacts/{id}/quality-gate-results
```

Daemon claim payload should include:

- `workflow_run_id`
- current execution batch context
- ready step run summaries
- artifact inputs needed for the batch
- existing snapshot metadata for compatibility

## ai-desk Migration Mapping

| ai-desk field/concept | Multica target |
|---|---|
| `Workflow.name`, `description`, `category`, `version` | workflow schema metadata |
| `step_definitions[]` | `schema.steps[]` |
| `depends_on_steps` | `steps[].depends_on` readiness edges |
| `input_artifacts` | `steps[].artifact.inputs` data-flow declarations |
| `agent_prompt` | `steps[].execution.prompt` |
| `rules` | `steps[].execution.rules` |
| `quality_gate_prompt` | `steps[].quality_gate.prompt` |
| `quality_report_mode` | `steps[].quality_gate.report_mode` |
| `review_required` | `steps[].review.required` |
| `artifact_name` | `steps[].artifact.name` |
| `artifact_template_content` | embedded artifact template content |
| `artifact_template_files` | embedded artifact template files |
| `artifact_template_id` | resolve and embed content when possible; otherwise warning |
| `default_execution_mode=MANUAL` | `execution.kind=manual` |
| `BUILT_IN_AGENT` / `LOCAL_AGENT` | `execution.kind=agent` |
| `EXTERNAL_AGENT` | `execution.kind=external` |
| `source=OFFICIAL` | System-Seeded Workflow Definition |
| `source=PUBLIC/PERSONAL` | user Workflow Definition in target workspace |
| `workflow_type=AUTOMATION` | Autopilot behavior, not workflow type |
| `engine=NONE/AETHER` | not migrated; represented through capabilities/run semantics |
| `party_mode_discuss/review` | ignored with warning |
| `pre_added_agents` | ignored with warning |
| `participants` | ignored with warning |
| `ticket_required` | ignored; represented by applicability or Autopilot |

Import fails on structural errors:

- schema cannot parse
- missing/duplicate step ids
- dependency references unknown step ids
- unsupported schema version
- required fields absent after normalization

Import warns and continues on intentionally skipped or recoverable content:

- Party Mode
- pre-added sidecar agents
- engine labels
- participants
- ticket_required
- Automation workflow type
- unresolved artifact template references
- suspicious artifact input without clear producing step

## UI Plan

### Settings > Workflows

Use existing Multica page shell and components:

- `packages/views/workflows/components/workflows-page.tsx`
- `packages/views/workflows/components/workflow-graph-preview.tsx`
- `@multica/ui/components/ui/*`

First-version builder:

- workflow list with search/filter
- source/metadata tab
- structured step inspector
- execution kind selector
- artifact template editor
- artifact inputs editor
- review required controls
- quality gate prompt/report/blocking controls
- import/export actions
- preview with graph and rendered workflow

Do not add ReactFlow in the first version. Use graph preview and structured inspectors.

### Issue Detail / Task Context

Workflow run viewer belongs with the work item that created the run:

- current run status
- step timeline
- ready/running/waiting/failed/completed states
- artifact versions
- review approve/reject controls
- quality reports with provenance
- retry step / rerun workflow actions

Settings remains definition editing only.

## Implementation Phases

### Phase 1: Schema v2 and import/export

Deliverables:

- workflow schema v2 types in Go and TypeScript
- normalization and validation in `workflowdefs`
- YAML import/export
- ai-desk migration converter with warnings
- tests for mapping and validation

Acceptance:

- ai-desk workflow YAML imports into a normalized Multica schema
- unsupported fields produce warnings, not silent loss
- structural errors fail import
- exported YAML round-trips through import without semantic loss for supported fields

### Phase 2: Runtime tables and queue-time creation

Deliverables:

- migrations for workflow runs, step runs, artifacts, reviews, quality gate results
- sqlc queries
- task enqueue changes to create workflow snapshot/run/step runs immediately
- rerun and cancel service methods

Acceptance:

- assignment enqueue creates exactly one workflow run and initial step runs
- publish after queue does not affect the run snapshot
- run content cannot be patched in place
- run is visible before daemon claim

### Phase 3: CLI and daemon execution contract

Deliverables:

- workflow CLI commands for run/step/artifact/quality operations
- daemon claim payload includes workflow run context
- execution batch loop executes ready serial steps until a blocking boundary
- usage/session/workdir reuse preserved

Acceptance:

- daemon can complete a simple agent workflow with multiple step runs
- daemon stops at required human review
- retry step reuses run/snapshot and creates new artifact version
- workflow rerun creates a new queue row and run

### Phase 4: Review, quality, manual, and external step UI

Deliverables:

- issue/task workflow run viewer
- artifact version viewer
- human review controls
- quality report display with provenance
- manual step complete/attach artifact flow
- external step waiting/blocked display

Acceptance:

- human member can approve/reject required review
- agent cannot approve its own required review
- blocking quality failure prevents dependent steps
- non-blocking quality warning is visible but does not block
- manual step can be completed without daemon execution

### Phase 5: Workflow Builder upgrade

Deliverables:

- structured step inspector
- artifact template editor
- dependency and artifact-input editors
- quality gate controls
- import/export buttons and warning display
- graph preview updates for schema v2

Acceptance:

- builder uses only Multica UI/design system components
- no new graph-canvas dependency
- UI writes canonical workflow schema
- preview renders from server-normalized schema

### Phase 6: System seed and migration rollout

Deliverables:

- system-seeded workflows derived from migrated ai-desk definitions where appropriate
- import reports for unsupported fields
- migration script or admin import flow
- docs for workflow authors and reviewers

Acceptance:

- official ai-desk workflows can be imported as system-seeded definitions
- personal/public workflows import as user definitions
- skipped fields are visible in import reports
- historical ai-desk task/run data is not imported into runtime tables

## Verification Plan

Backend:

- migration tests
- workflow schema normalization tests
- import/export round-trip tests
- enqueue snapshot/run creation tests
- run immutability tests
- step readiness/topological execution tests
- review and quality gate blocking tests

Daemon:

- claim payload compatibility tests
- execution batch stop-boundary tests
- session/workdir reuse tests
- cancel/retry/rerun tests

Frontend:

- workflow builder rendering and form tests
- graph preview layout tests
- import warning display tests
- run viewer state tests
- review controls tests

End-to-end:

- import ai-desk workflow
- publish workflow
- bind project
- create issue assigned to agent
- verify snapshot/run/steps created at queue time
- daemon executes agent steps
- human approves artifact
- blocking quality gate behavior works
- final report remains explicit workflow step

## Remaining Risks

- Token overhead can rise if execution batches are implemented as cold starts. Mitigation: reuse workdir/session and batch consecutive ready agent steps.
- Artifact content storage may need size limits. First version should support text/markdown/json and defer large binaries.
- Quality gate provenance must be visually clear. Agent-produced reports are not platform-verified.
- Autopilot mapping for ai-desk automation templates needs a separate Autopilot design slice.
- Multi-agent workflow behavior should be redesigned through Multica Squads/assignment/mentions, not inferred from ai-desk sidecar agents.
