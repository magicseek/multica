# Migrate ai-desk Flows through Multica workflow runs

Multica will migrate ai-desk Flow capabilities into the existing Multica workflow control plane instead of copying ai-desk's browser-driven step orchestration, workflow engine labels, or UI stack. Workflow definitions remain versioned JSON schemas with import/export support; queued work creates immutable workflow snapshots, server-owned workflow runs, and materialized step runs with artifacts, human reviews, and quality gates. Daemons execute and report workflow work, while browser surfaces edit definitions and observe runs.

## Considered Options

- Copy ai-desk's task/step orchestration and browser-controlled execution flow: rejected because Multica already owns dispatch through `agent_task_queue`, daemon pull, queue-time snapshots, and server-side runtime recovery.
- Store workflow definition steps, artifacts, and gates as editable relational records: rejected because draft, publish, fork, import, export, and snapshot behavior are simpler and safer when the workflow revision stores one canonical schema payload.
- Treat workflow steps as prompt-only markdown sections: rejected because ai-desk Flows rely on step-level dependencies, artifacts, human reviews, quality gates, pause/retry, and observable execution state.
- Cold-start one daemon task per step: rejected because it would increase token and runtime overhead; step runs remain state units, but execution batches may reuse work directory and agent session until a blocking boundary.

## Consequences

- `workflow_revision.schema` remains the canonical definition representation.
- Workflow snapshots, workflow runs, and initial workflow step runs are created when the agent task is queued, not when a daemon claims work.
- First-version workflow runs have a 1:1 relationship with the `agent_task_queue` row that created them.
- Workflow run content is immutable after creation; corrections require a new workflow revision and a newly queued run.
- Incorrect workflow content in a running workflow is handled by cancelling or rerunning from a corrected workflow revision, not by repairing the current run in place.
- Step retry re-executes a retryable step inside the same workflow run and snapshot; workflow rerun creates a new queue row, snapshot, and run.
- Runtime state is materialized through workflow runs, workflow step runs, workflow artifacts, workflow reviews, and workflow quality gates.
- Workflow completion does not automatically post issue comments or change issue status; final user-visible reporting remains an explicit workflow step.
- Workflow artifacts are explicit server-stored reviewable content; ordinary local outputs remain governed by Output Metadata privacy boundaries.
- Workflow artifact versions are immutable; step retries create new artifact versions rather than overwriting prior reviewable content.
- Required workflow reviews are approved or rejected by human workspace members in the first version.
- Quality gates declare whether they block dependent step runs.
- First-version quality gate results may be produced by the executing agent and must expose provenance; they are not presented as server-trusted platform verdicts.
- First-version DAG execution is deterministic and serial, even when multiple branches are ready.
- YAML import/export is included for definition migration and review, but runtime execution uses normalized schemas, snapshots, and runs.
- The first-version Workflow Builder uses Multica shared UI components and graph preview rather than introducing a new graph-canvas dependency.
- ai-desk Party Mode, `WorkflowEngine` labels, workflow-level sidecar agents, and `WORKFLOW`/`AUTOMATION` workflow types are not migrated as Multica workflow concepts.
- ai-desk `AUTOMATION` behavior maps to Multica Autopilot rather than to a workflow type.
- ai-desk `OFFICIAL` workflow source maps to system-seeded workflow definitions; `PUBLIC` and `PERSONAL` imports map to user-owned workflow definitions in the target workspace.
- ai-desk workflow participants and editor collaborators are not migrated as workflow-specific ACLs; imported workflows follow Multica workspace permissions.
- ai-desk `ticket_required` is not migrated; issue, chat, comment, and autopilot fit is expressed through workflow applicability and Autopilot behavior.
- ai-desk step `ExecutionMode` is reshaped into Multica step execution intent (`agent`, `manual`, or `external`) rather than preserving ai-desk runtime-routing labels.
- Manual steps get a minimal human completion loop in the first version; external steps are represented as waiting or blocked without external-system execution.
- Step dependencies and artifact inputs remain separate: dependencies control readiness, while artifact inputs describe data flow and generate import warnings when they appear inconsistent.
- Artifact templates are embedded into the workflow schema during first-version migration; unresolved ai-desk template catalog references produce import warnings rather than external definition dependencies.
- Workflow run and step run statuses use Multica runtime language rather than preserving ai-desk uppercase task and step status enums.
- Workflow definition editing lives under Settings > Workflows; workflow run observation lives on the issue, task, chat, or autopilot surface that created the run.
- Migration imports ai-desk workflow definitions/templates only; historical ai-desk tasks, steps, artifacts, and runs are not loaded into Multica runtime tables.
- Workflow import fails on structural errors that would make execution invalid, but intentionally skipped or recoverable ai-desk fields produce import warnings and continue.
- ai-desk AI-generated workflow creation is excluded from first-version migration and can be redesigned later as a Multica-native proposal flow.
- First-version agent-facing workflow control is exposed through Multica CLI commands rather than requiring MCP-only workflow tools.
