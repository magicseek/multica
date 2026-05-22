# Optimize chat turn speed and prompt cache reuse

## Goal

Improve each Chat Session turn's end-to-end responsiveness, reduce repeated task-run token spend, and improve provider prompt cache rate while preserving existing Multica task, workflow, repository, and chat behavior.

## What I already know

- The user wants all previously identified optimization points implemented through Trellis, one by one.
- After each optimization point, run a build and focused unit tests, fix regressions, then commit that optimization before moving to the next.
- Existing dirty changes already touch daemon env-root reuse, CODEX_HOME preparation, user Codex skill syncing, and copy-on-write file clone helpers. These appear aligned with the Codex Home Snapshot / env reuse optimization and must be validated before committing.
- The final completion gate is not only unit tests: it requires a system demo from an empty project where Chat is used with `grill-with-docs` to discuss an arcade tank game, proposals are turned into Multica issues, agents are dispatched, and the resulting game is playable. During that demo verification, this observer must not contribute code to the demo project.
- Relevant project rules come from `CLAUDE.md`, `AGENTS.md`, `.trellis/workflow.md`, `.trellis/spec/server/backend/index.md`, `.trellis/spec/server/backend/daemon-runtime-contracts.md`, `.trellis/spec/server/backend/agent-execution-protocol.md`, and `.trellis/spec/guides/code-reuse-thinking-guide.md`.

## Requirements

### R1. Provider Session Runner

- Deepen provider execution so Chat Session turns can reuse a hot provider runner when safe, starting with Codex app-server.
- Preserve the existing per-task execution path as fallback for stale, incompatible, cancelled, or failed runners.
- Prevent concurrent turns from corrupting one runner's JSON-RPC stream.
- Respect provider args, runtime config, auth/config identity, cwd/env root, cancellation, timeout, and resume fallback semantics.

### R2. Runtime Brief / Prompt Cache

- Split large stable runtime guidance from per-turn dynamic facts so Chat Session turns have smaller dynamic prompt surfaces.
- Keep provider-specific overlays and task-mode overlays deterministic and testable.
- Avoid removing instructions required for issue, workflow, quick-create, Autopilot, comment, or chat behavior.
- For providers that need inline system prompts, avoid inlining unrelated static bulk when a slimmer prompt is sufficient.

### R3. Codex Home Snapshot / Env Reuse

- Reduce repeated CODEX_HOME setup cost across Chat Session turns.
- Reuse local Repository Binding env roots safely for Chat Session turns.
- Avoid recopying unchanged user skills and config payloads while preserving freshness for auth, managed config sections, sandbox policy, plugin cache, and workspace skills.
- Keep stale skill/config cleanup deterministic and tested.

### R4. Prompt Cache Observability

- Record enough execution facts to explain cache rate changes: prompt size, runtime brief size, stable/dynamic segment hashes, env/root reuse, runner reuse, resume hit/fallback, provider/model usage, and cache read/write tokens where available.
- Bind Codex token usage to the correct session/thread or otherwise make attribution safer than "last modified session file after start time" under concurrency.
- Make the metrics useful for comparing optimizations before and after rollout.

### R5. Chat Turn Context

- Bind a queued chat task to the exact user message that triggered it, or otherwise make daemon claim resolve the triggering message without scanning the whole Chat Session transcript.
- Preserve attachment lookup and proposal-first chat behavior.
- Avoid mismatching messages when multiple Chat Session turns are queued quickly.

### R6. Chat Turn Client

- Share the optimistic send path across ChatWindow, ChatNewPage, and ChatSessionPage.
- Keep React Query as the single source of truth for server state; do not duplicate server state into Zustand.
- Reduce unnecessary invalidation/refetch on each send while preserving WS recovery behavior.

### R7. Workflow Step Context

- Reduce Workflow Run token overhead by giving agents a current-step context instead of making them infer the next step from full workflow state on every phase.
- Preserve ADR-0003 and ADR-0004 constraints: server-owned Workflow Runs, immutable Workflow Snapshots, same-run resume after Workflow Input Requests, and no one-daemon-task-per-step cold-start model.

## Acceptance Criteria

- [ ] Each optimization point is implemented as an isolated, reviewable change set.
- [ ] Each optimization point has focused unit tests for the changed behavior.
- [ ] After each optimization point, a relevant build command and focused unit tests pass before committing.
- [ ] Each optimization point is committed separately using the repository's Lore Commit Protocol.
- [ ] Existing Chat Session behavior remains intact: send, pending task, task messages, completion, failure, cancellation, attachments, Chat Issue Proposals, and Output Metadata behavior are not regressed.
- [ ] Existing issue/comment/quick-create/Autopilot/Workflow Run task behavior remains intact.
- [ ] The final system verification starts from an empty project and uses Chat with `grill-with-docs` to discuss arcade tank game requirements.
- [ ] The chat-generated proposals are turned into Multica issues.
- [ ] Agents are dispatched to complete the decomposed issues.
- [ ] The generated demo project produces a playable arcade tank game.
- [ ] The observer does not contribute code to the demo project during final validation.

## Verification Plan

- For backend/daemon changes: run focused Go tests under `server/internal/daemon`, `server/internal/daemon/execenv`, `server/pkg/agent`, and `server/internal/handler` as relevant.
- For frontend chat changes: run focused package tests plus `pnpm typecheck` for touched packages.
- For full backend safety after each backend-heavy optimization: run `make build` and at least the relevant `go test` packages.
- For cross-layer changes: run `pnpm typecheck`, `pnpm test`, and `make test` when scope requires it.
- For final system verification: start a clean local app stack, create or open an empty Multica project, conduct the Chat workflow with `grill-with-docs`, approve proposals into issues, dispatch agents, observe completion, and verify the generated arcade tank game is playable without observer code contribution.

## Implementation Order

1. Codex Home Snapshot / local Chat Session env reuse, because aligned dirty changes already exist and can be validated first.
2. Provider Session Runner for Codex app-server reuse.
3. Runtime Brief / Prompt Cache split.
4. Prompt Cache Observability.
5. Chat Turn Context.
6. Chat Turn Client.
7. Workflow Step Context.
8. Final system demo verification.

## Out of Scope

- No new external dependencies unless an optimization cannot be implemented safely without one.
- No provider-wide rewrite before Codex proves the runner abstraction.
- No change to Workflow Snapshot immutability or server-owned Workflow Run semantics.
- No manual code contribution to the final demo game project during validation.

## Technical Notes

- Current dirty files before this task was created:
  - `server/internal/daemon/daemon.go`
  - `server/internal/daemon/daemon_test.go`
  - `server/internal/daemon/execenv/codex_home.go`
  - `server/internal/daemon/execenv/codex_home_link_test.go`
  - `server/internal/daemon/execenv/codex_user_skills.go`
  - `server/internal/daemon/execenv/execenv.go`
  - `server/internal/daemon/execenv/execenv_test.go`
  - `server/internal/daemon/execenv/file_clone*.go`
- The first implementation pass should validate and, if needed, repair these existing changes rather than rewriting them.
