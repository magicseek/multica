# Add Trellis execution protocol template

## Goal

Let users choose Trellis as an optional execution protocol template for assignment-triggered task agents, while preserving the existing default legacy workflow and the existing standard assignment protocol behavior.

## What I already know

* The user explicitly does not want AETHER included.
* The desired product shape is a template strategy: protocol behavior should be selectable, not hard-coded behind a single boolean.
* Current product state:
  * `agent.execution_protocol_enabled` is a boolean feature flag.
  * Ordinary assignment tasks with the flag enabled render a hard-coded `Task Execution Protocol`.
  * comment, chat, quick-create, autopilot, and squad leader tasks have specialized workflows and must remain higher priority.
* `agent-workflow` provides a useful pattern:
  * `ExecutionProtocolTemplate` records name/content/version/isActive/project scope.
  * active-template selection falls back from project-specific to global.
  * rendering uses placeholders plus plan/resume helpers.
* For this task we should borrow the template strategy, not copy the AETHER protocol or full step workflow engine.

## Requirements

* Add first-party execution protocol templates:
  * `standard-assignment`: the current hard-coded seven-step assignment protocol.
  * `trellis-task`: a new Trellis-oriented protocol for assignment-triggered issue work.
* Add an agent-level protocol template choice while keeping backward compatibility:
  * `execution_protocol_enabled=false` keeps the legacy assignment workflow.
  * `execution_protocol_enabled=true` with no slug keeps the existing standard assignment behavior.
  * `execution_protocol_enabled=true` with `execution_protocol_slug="trellis-task"` renders the Trellis protocol.
* Do not include AETHER as a product option.
* Do not parse `.trellis/workflow.md` into Multica workflow steps. Trellis remains runtime-native; the template only instructs the local agent to use Trellis when present.
* Preserve specialized task workflows:
  * comment-triggered
  * chat
  * quick-create
  * autopilot run-only
  * squad leader `no_action`
* Expose the template choice through backend API, daemon claim payload, core types/schemas, and agent create/detail UI.
* Validate unknown protocol slugs server-side.
* Agent templates may later preselect a protocol slug, but this task only needs the core protocol-selection plumbing.

## Acceptance Criteria

* [x] Existing agents and clients that only set `execution_protocol_enabled=true` still receive the standard assignment protocol.
* [x] Agents created or updated with `execution_protocol_slug="trellis-task"` receive a Trellis protocol on ordinary assignment tasks.
* [x] Disabled agents do not receive any execution protocol template.
* [x] Unknown protocol slugs are rejected by create/update handlers.
* [x] Claim responses include the resolved agent protocol slug so daemon execution can render the selected template.
* [x] comment/chat/quick-create/autopilot/squad paths do not render the Trellis protocol even if selected.
* [x] Frontend create and detail settings let users choose Off / Standard assignment / Trellis task.
* [x] Targeted handler, daemon, execenv, and frontend tests cover the new selection behavior.

## Definition of Done

* Tests added/updated for backend and frontend behavior.
* `make sqlc` run after DB/query changes.
* `pnpm typecheck`, `pnpm lint`, `pnpm test` run.
* Focused Go tests for handler/daemon/execenv run.
* Trellis spec updated if the protocol-template contract becomes reusable project knowledge.

## Out of Scope

* AETHER protocol support.
* Full workflow engine import from `agent-workflow`.
* StepDefinition / artifact / work item / assignment lease implementation.
* Workspace/project-scoped custom protocol templates.
* Admin UI for authoring protocol templates.

## Technical Approach

* Add a static execution protocol template registry, separate from the existing agent template catalog.
* Add `agent.execution_protocol_slug` with a nullable or empty-string default.
* Thread the slug through:
  * sqlc generated agent model/query params
  * create/update/list agent API responses
  * daemon task claim payload
  * `TaskContextForEnv`
  * frontend core types/schemas
  * create/detail UI controls
* Update execenv rendering:
  * keep the existing predicate exclusions
  * resolve empty slug to `standard-assignment`
  * render the selected template via placeholder replacement

## Decision (ADR-lite)

**Context**: The current flag is too coarse; users need Trellis-specific execution guidance without adopting AETHER or importing a full workflow engine.

**Decision**: Implement a first-party static protocol template registry and let agents select a protocol slug. Keep the existing boolean as the enable/disable switch and compatibility fallback.

**Consequences**: This is a narrow change that preserves current behavior and opens a path for richer protocol catalogs later. It intentionally does not solve custom protocol authoring or step-based workflow execution.

## Technical Notes

* Current hard-coded protocol: `server/internal/daemon/execenv/execution_protocol.go`
* Current branch predicate: `server/internal/daemon/execenv/runtime_config.go`
* Current claim payload path:
  * `server/internal/handler/daemon.go`
  * `server/internal/daemon/types.go`
  * `server/internal/daemon/daemon.go`
* Current agent API/types:
  * `server/internal/handler/agent.go`
  * `server/pkg/db/queries/agent.sql`
  * `packages/core/types/agent.ts`
  * `packages/core/api/schemas.ts`
* Current frontend controls:
  * `packages/views/agents/components/create-agent-dialog.tsx`
  * `packages/views/agents/components/agent-detail-inspector.tsx`
* External reference inspected:
  * `/Users/troy.huang/workspace/RingCentral/AI/agent-workflow/workflow/protocol.go`
  * `/Users/troy.huang/workspace/RingCentral/AI/agent-workflow/cmd/workflowd/main.go`
  * `/Users/troy.huang/workspace/RingCentral/AI/agent-workflow/workflow/types.go`
