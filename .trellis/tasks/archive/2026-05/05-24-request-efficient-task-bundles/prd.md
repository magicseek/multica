# Implement request-efficient task bundles

## Goal

Implement first-class Task Bundles for request-priced agents so Multica can assign several related issue work items to one agent-owned execution without repeatedly starting the external provider. This is primarily for providers such as GitHub Copilot and Kiro, but the setting must be agent-level and available to every provider.

## Source Documents

* `CONTEXT.md` definitions and relationships for Request-priced Agent, Request-efficient Mode, Task Bundle, Bundle Item Checkpoints, Runtime Budget, Live Bundle Transcript, and Task Bundle Availability Gate.
* `docs/adr/0024-request-efficient-task-bundles-for-request-priced-agents.md` decision record.

## What I Already Know

* Current branch is `feat/chat-plan-runs`.
* Current queue model is one `agent_task_queue` row per daemon claim and one `Backend.Execute(...)` provider execution.
* Copilot starts a new `copilot -p ...` process on each `Execute` call, so separate issue tasks create separate provider connections.
* Kiro starts `kiro-cli acp` and sends a single prompt per `Execute` call.
* Copilot usage can report internal `premiumRequests`, so Multica can guarantee one provider execution boundary, not exactly one provider-billed request.
* Existing task timeout defaults are single-task oriented: daemon default is about 2 hours, and server stale running cleanup is about 2.5 hours.
* Existing live transcripts already flow through `ReportTaskMessages`, persisted task messages, and `task:message` websocket events.
* Existing issue task UI has `AgentLiveCard`, `TranscriptButton`, issue action/status menus, and issue task queries.
* Existing issue enqueue behavior treats backlog as staging; moving an assigned issue out of backlog triggers execution.
* Existing structured task outputs include `.multica/outputs.json`, `.multica/issue-proposals.json`, and chat-scoped `.multica/chats/<chat_session_id>/`.

## Requirements

* Add Request-efficient Mode as a concrete agent setting. Any provider can enable or disable it; Copilot and Kiro should be recommended-on candidates without hardcoding the feature only to those providers.
* Add Task Bundle Availability Gate: issue task status pages show a Task Bundle option only when at least one available agent has Request-efficient Mode enabled.
* Create Task Bundles only at explicit user-facing boundaries: assignment, Start Work Review, or chat proposal approval. Do not silently merge already queued or running issue tasks in the daemon.
* Model Task Bundle as a first-class domain object with ordered Task Bundle Items, not only JSON inside `agent_task_queue.context`.
* A Task Bundle owns exactly one Bundle Execution Task for the external provider execution.
* The daemon must launch the provider once for a request-efficient bundle and must not call `Backend.Execute` once per item.
* Pre-materialize bundle context before execution so the agent can inspect all issue items, resources, output paths, bundle order, and checkpoint commands inside one provider run.
* Execute bundle items sequentially by default. Only the active item moves its issue to `in_progress`; later items remain visibly queued inside the bundle.
* Provide a task-scoped Bundle Checkpoint Command/API so the provider can record per-item `completed`, `failed`, `blocked`, or `input_needed` outcomes during execution.
* Expose Live Bundle Transcript segmented by bundle item checkpoints.
* Add Bundle Runtime Budget so daemon timeout and server stale-task cleanup scale with the bounded item count instead of using only single-task defaults.
* Add Bundle Size Guardrail. Default maximum is five issue work items; larger selections split into multiple bundles unless bounded deployment configuration raises the max.
* Add Bundle Output Namespace with one bundle-level summary plus isolated per-issue item outputs.
* Add Bundle Changeset Mode. Default is one changeset per issue item; an explicit shared-bundle changeset mode is allowed for tightly coupled issue sets.
* Add Bundle Rerun Scope. Follow-up bundles default to failed, blocked, or input-needed items rather than rerunning completed items.
* If a request-efficient agent is a squad lead, it should execute squad-assigned work directly by default. Delegation is allowed only for explicit user delegation, another agent's unique required capability, or follow-up after the lead records a blocked outcome.

## Acceptance Criteria

* [ ] Agent create/edit/detail APIs and UI expose a Request-efficient Mode toggle.
* [ ] Copilot and Kiro agents show a recommendation to enable Request-efficient Mode, while other providers can still enable it.
* [ ] Issue task status pages do not show Task Bundle controls when no available agent has Request-efficient Mode enabled.
* [ ] Issue task status pages show Task Bundle controls when at least one available agent has Request-efficient Mode enabled.
* [ ] Creating a Task Bundle persists a durable bundle row, ordered item rows, and one linked `agent_task_queue` execution task.
* [ ] Bundle creation is available at a pre-enqueue boundary and does not regroup already queued/running tasks silently.
* [ ] Daemon claim response includes enough bundle context for one provider execution to process all bundle items sequentially.
* [ ] A request-efficient bundle starts the provider once and does not invoke `Backend.Execute` per item.
* [ ] During execution, only the active bundle item issue moves to `in_progress`; later item issues remain non-running with visible bundled queued state.
* [ ] Bundle checkpoint command/API records per-item status, result metadata, transcript boundary, and output namespace.
* [ ] Bundle transcript UI shows live messages and per-item segmentation.
* [ ] Bundle runtime budget is honored by daemon execution timeout and server stale-task cleanup.
* [ ] Bundle rerun defaults to failed, blocked, and input-needed items and excludes completed items unless explicitly selected.
* [ ] Bundle outputs cannot overwrite each other across issue items.
* [ ] Request-efficient squad lead tasks include instructions and enforcement that the lead executes directly unless a delegation exception applies.
* [ ] Tests cover database persistence, enqueue/claim semantics, daemon single-execution behavior, checkpointing, UI gating, and rerun scope.

## Definition of Done

* SQL migrations and sqlc output are updated.
* Backend tests cover bundle creation, issue status transitions, checkpoint APIs, runtime budgets, rerun scope, and squad-lead behavior.
* Frontend tests cover agent setting UI and issue task status availability gating.
* Relevant Go tests, TypeScript typecheck, Vitest tests, and final worktree check pass.
* Documentation/spec notes are updated if new task runtime or issue status contracts are introduced.

## Out of Scope

* Guaranteeing exact external provider billing counts beyond one Multica provider execution boundary.
* Hidden daemon-side batching of already queued/running issue tasks.
* Parallel execution of issue items inside one bundle.
* Waiting mid-provider-run for human clarification; use blocked or input-needed outcomes instead.
* Making all providers request-efficient by default.

## Technical Notes

* ADR: `docs/adr/0024-request-efficient-task-bundles-for-request-priced-agents.md`.
* Shared language: `CONTEXT.md`.
* Server specs: `.trellis/spec/server/backend/agent-execution-protocol.md`, `.trellis/spec/server/backend/daemon-runtime-contracts.md`.
* Current enqueue/claim/runtime files: `server/internal/service/task.go`, `server/internal/daemon/daemon.go`, `server/pkg/db/queries/agent.sql`, `server/cmd/server/runtime_sweeper.go`.
* Provider execution references: `server/pkg/agent/copilot.go`, `server/pkg/agent/kiro.go`.
* Issue status and activity UI references: `packages/views/issues/components/agent-live-card.tsx`, `packages/views/common/task-transcript`, `packages/views/issues/actions/issue-actions-menu-items.tsx`.
* Agent types/query references: `packages/core/types/agent.ts`, `packages/core/agents/queries.ts`.

## Suggested Implementation Phases

1. Data model and API: add request-efficient agent setting, task bundle tables, bundle item rows, task linkage, and list/detail endpoints.
2. Creation flow: add pre-enqueue bundle creation at Start Work Review / issue task status surface and enforce the availability gate.
3. Runtime flow: extend daemon claim payload, prompt/context rendering, output namespace, checkpoint command/API, and single provider execution behavior.
4. Progress and retry: add live transcript segmentation, item status transitions, runtime budget, stale cleanup, and rerun scope.
5. UI polish and verification: agent setting recommendation copy, issue task status controls, bundle detail/progress, transcript display, tests, and manual smoke.
