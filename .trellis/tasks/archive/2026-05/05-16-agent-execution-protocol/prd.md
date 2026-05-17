# Enable Protocol-Gated Task Agent Execution Workflow

## Objective

Move task-agent execution workflow control into an opt-in, product-configurable path so only agents with the setting enabled receive the new execution protocol prompt. Existing agents and non-task workflows must continue to run through the legacy prompt path.

This task is the worktree-scoped handoff for the Phase 1 implementation already applied on branch `trellis/agent-execution-protocol`.

## User Direction

- Use a git worktree, not the current repository checkout.
- Transfer the work to Trellis for follow-up workflow tracking.
- Apply the new execution layer only to agents that enable the setting.
- Continue the larger enhancement sequence in the preferred order: `1 -> 3 -> 2`.

## Scope

In scope:

- Add a persistent `agents.execution_protocol_enabled` setting with default `false`.
- Expose the setting through agent create, update, detail, and list flows.
- Propagate the setting from claimed tasks to daemon runtime context.
- Inject a centralized execution protocol into ordinary assignment-triggered task-agent prompts only when the setting is enabled.
- Keep legacy prompt behavior for agents without the setting.
- Add UI controls and localized labels for the agent detail/settings and create-agent flows.
- Keep the implementation compatible with current sqlc, daemon, and frontend package boundaries.

Out of scope for this phase:

- Checkpoint/resume storage.
- Work item splitting and assignment leases.
- Agent squad task decomposition, assignment, or merge behavior changes.
- Provider-specific session continuation.
- Enabling the protocol globally by default.

## Behavioral Requirements

1. Agents default to `execution_protocol_enabled = false`.
2. Create-agent and update-agent requests may set the flag explicitly.
3. API responses include the flag so frontend state can reflect server truth.
4. Task queue claim results include the flag so daemon task execution can decide prompt composition without extra queries.
5. `TaskContextForEnv` carries the flag to `execenv`.
6. `execenv` injects the task execution protocol only when:
   - a normal task run is being executed, and
   - the task's agent has `execution_protocol_enabled = true`.
7. Comment-triggered, chat-triggered, autopilot, quick-create, and squad leader paths stay on their existing prompt behavior unless separately changed later.
8. The frontend exposes the setting without moving server state into Zustand or app-level platform code.

## Acceptance Criteria

- Database migration adds and removes the new agent setting cleanly.
- sqlc generated files match the query changes.
- Go daemon/handler tests pass for the touched packages.
- TypeScript typecheck passes.
- Existing task execution tests cover both disabled legacy behavior and enabled protocol injection.
- UI duplicate/create flows preserve explicit enabled state.
- Current repo checkout no longer carries this task's implementation diff; work continues only from this worktree.

## Current Implementation State

The following code changes already exist in this worktree as an uncommitted patch:

- `server/migrations/091_agent_execution_protocol.up.sql`
- `server/migrations/091_agent_execution_protocol.down.sql`
- `server/pkg/db/queries/agent.sql`
- `server/pkg/db/generated/agent.sql.go`
- `server/pkg/db/generated/models.go`
- `server/internal/handler/agent.go`
- `server/internal/handler/daemon.go`
- `server/internal/daemon/types.go`
- `server/internal/daemon/daemon.go`
- `server/internal/daemon/execenv/execenv.go`
- `server/internal/daemon/execenv/runtime_config.go`
- `server/internal/daemon/execenv/execution_protocol.go`
- `server/internal/daemon/execenv/execenv_test.go`
- `packages/core/types/agent.ts`
- `packages/views/agents/components/agent-detail-inspector.tsx`
- `packages/views/agents/components/create-agent-dialog.tsx`
- `packages/views/locales/en/agents.json`
- `packages/views/locales/zh-Hans/agents.json`
- `CONTEXT.md`

## Verification Already Run Before Worktree Handoff

- `make sqlc`
- `cd server && go test ./internal/daemon/execenv`
- `cd server && go test ./internal/daemon ./internal/handler`
- `pnpm install --frozen-lockfile`
- `pnpm typecheck`
- `git diff --check`

## Verification Re-Run In Worktree

- `git diff --check`
- `cd server && go test ./internal/daemon/execenv ./internal/daemon ./internal/handler`
- `pnpm install --frozen-lockfile`
- `pnpm typecheck`
- `make sqlc`
- `pnpm lint`
- `pnpm test`

Note: `make sqlc` exits successfully, but the local generator also rewrites the pre-existing `server/pkg/db/generated/issue.sql.go` duplicate-issue parameter names. That drift is unrelated to this task and was removed from the worktree diff.

Note: `pnpm lint` exits successfully with existing warnings in unrelated files. `pnpm test` exits successfully with existing stderr warnings from existing tests.

## Recommended Next Trellis Steps

1. Run `trellis-check` or the Phase 2.2 quality check against this worktree.
2. Re-run focused verification in this worktree if the local dependency setup is available.
3. Review whether `CONTEXT.md` should remain as repo documentation or be moved into Trellis task research/spec material.
4. If Phase 1 is accepted, continue the larger roadmap in the user's chosen sequence:
   - Step 3: checkpoint/resume module.
   - Step 2: work-item splitting and assignment leases.
