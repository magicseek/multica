# Migrate ai-desk Flows to Multica Workflows

**Created**: 2026-05-19
**Assignee**: troy
**Priority**: P1
**Status**: Planning

## Problem

ai-desk has a mature Flow model with workflow definitions, a builder, YAML import/export, step dependencies, artifact templates, human review, quality gates, and daemon-backed execution. Multica already has editable workflow definitions, project/issue bindings, queue-time workflow snapshots, and daemon pull execution, but it currently lacks materialized workflow runs and step-level runtime semantics.

Multica needs to migrate the useful ai-desk Flow capabilities into its own workflow control plane without copying ai-desk's browser-driven orchestration, UI stack, engine labels, or legacy multi-agent concepts.

## Source Documents

- `CONTEXT.md`
- `docs/adr/0003-migrate-ai-desk-flows-through-multica-workflow-runs.md`
- `docs/ai-desk-flows-to-multica-workflows-plan.md`
- `.trellis/spec/workflows/execution-workflow-definitions.md`

## Goals

- Extend Multica workflows with server-owned workflow runs and workflow step runs.
- Preserve ai-desk definition value: steps, dependencies, prompts, rules, artifact templates, reviews, quality gates, and YAML import/export.
- Create workflow snapshots, workflow runs, and initial step runs immediately when `agent_task_queue` rows are created.
- Keep workflow run content immutable after creation.
- Execute step runs through Multica daemon pull and CLI-based workflow control, not browser orchestration.
- Use Multica's design system and existing `packages/ui`, `packages/core`, and `packages/views` patterns.
- Make running workflows observable from issue, task, chat, and Autopilot contexts.

## Non-Goals

- Do not migrate ai-desk historical tasks, steps, artifacts, or runs.
- Do not migrate Party Mode fields or keep them as legacy metadata.
- Do not migrate ai-desk `WorkflowEngine` labels.
- Do not migrate ai-desk `pre_added_agents` or workflow-level sidecar agents.
- Do not migrate ai-desk workflow participants or per-workflow ACL.
- Do not preserve `WORKFLOW` / `AUTOMATION` as Multica workflow types; automation maps to Multica Autopilot.
- Do not add ReactFlow or copy ai-desk builder UI in the first version.
- Do not implement AI-generated workflow creation in the first version.
- Do not allow workflow content to be changed mid-run; incorrect workflow content is handled by cancel/rerun with a corrected revision.

## Required Architecture

- `workflow_revision.schema` remains the canonical definition payload.
- Runtime state is materialized in relational records for workflow runs, step runs, artifacts, reviews, and quality gate results.
- First-version workflow run has a 1:1 relationship with the `agent_task_queue` row that created it.
- Step dependencies control readiness; artifact inputs describe data flow and stay separate.
- Step DAG execution is deterministic and serial in the first version.
- A workflow execution batch may process multiple ready step runs in the same workdir/session until a blocking boundary.
- Required workflow review is approved or rejected by a human workspace member.
- Agent-produced quality gate results must expose provenance and must not be presented as server-trusted platform verdicts.
- Explicit workflow artifacts are server-stored and versioned immutably; ordinary local outputs remain governed by Output Metadata privacy boundaries.

## Implementation Phases

1. Finalize workflow schema v2 and ai-desk YAML import/export.
2. Add runtime database tables, sqlc queries, and service-layer run/step creation at queue time.
3. Add workflow CLI commands and daemon execution contract for step run progress, artifacts, reviews, quality gates, pause, failure, completion, retry, and rerun.
4. Add API and UI for workflow run observation in issue/task/chat/Autopilot contexts.
5. Upgrade Settings > Workflows builder using Multica UI components, structured inspectors, and derived graph preview.
6. Seed or import migrated workflow definitions and add rollout tests.

## Acceptance Criteria

- New agent tasks create immutable workflow snapshots, workflow runs, and initial step runs at queue time.
- Existing workflow definition/binding behavior remains compatible with current assignment/comment workflows.
- Daemon claim and execution can process workflow step runs without re-resolving workflow definitions.
- Blocking review and quality gate states stop dependent step execution.
- Step retry stays inside the same workflow run and creates new artifact versions.
- Workflow rerun creates a new queue row, snapshot, and run.
- YAML import fails on structural execution errors and warns on intentionally skipped ai-desk fields.
- Builder and run UI use Multica shared UI components and do not introduce ReactFlow.
- Package boundaries are preserved: server logic in `server/`, shared types/hooks in `packages/core`, shared UI in `packages/views` and `packages/ui`, app wiring in app packages.
- Verification includes backend tests, sqlc regeneration, TypeScript typecheck/tests for touched frontend packages, and focused UI tests where run/builder views change.

## Known Risks

- Token/runtime overhead if batching and session reuse are not implemented carefully.
- Artifact size and retention policy need explicit product limits during implementation.
- Quality gate provenance must be visible enough to avoid implying server-certified correctness.
- Autopilot mapping is related but should remain a separate design lane.
- Future multi-agent workflow behavior should be designed through Multica-native assignment, mentions, or Squads rather than ai-desk sidecar agents.
