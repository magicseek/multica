# Implement chat directed routing and squad orchestration

## Goal

Use the directed graph as the server source of truth for ordinary chat routing and Plan mode squad collaboration, including lead-first flow, helper replies, scoped mentions, ambiguity blocking, and loop limits.

## Requirements

* Parse user-authored visible `@agent` / `@squad` mentions into graph recipient edges.
* Route user-authored `@squad` first to the squad lead while preserving the squad recipient edge for UI.
* Route no-mention human follow-up to the active directed thread recipient.
* Reject ambiguous no-mention follow-up with structured `needs_target` metadata when multiple recipients are active.
* Parse agent-authored visible mentions as routing commands.
* Block agent-authored out-of-scope mentions in Plan squad runs while preserving visible messages and warning state.
* Require helper replies in Plan squad runs to explicitly mention the lead before returning control.
* Allow at most 5 lead-to-helper-to-lead consultation waves per Plan Run, then force lead summary handoff.
* Keep routing idempotent per message/recipient edge.

## Acceptance Criteria

* [ ] Ordinary chat explicit single-agent mention enqueues exactly one recipient task.
* [ ] Ordinary chat explicit multi-agent mention stores one user message and multiple recipient edges/tasks.
* [ ] Ordinary chat `@squad` routes to squad lead first.
* [ ] No-mention continuation follows active directed recipient when unambiguous.
* [ ] No-mention continuation after parallel replies returns structured `needs_target`.
* [ ] Plan `@squad` starts with the lead agent.
* [ ] Lead visible helper mentions create helper tasks only for allowed squad members.
* [ ] Helper lead mention returns control to the lead.
* [ ] Out-of-scope agent mentions create warnings and no tasks.
* [ ] The sixth automatic consultation wave is not created; lead is asked to summarize.

## Suggested Files

* `server/internal/handler/chat.go`
* `server/internal/handler/chat_plan_runs.go`
* `server/internal/service/task.go`
* `server/internal/handler/squad_briefing.go` as a reference only
* `server/internal/handler/chat_plan_runs_test.go`

## Test Plan

* Go handler/service tests for explicit mention fan-out.
* Go tests for squad lead resolution and helper-to-lead loop.
* Go tests for ambiguous no-target server rejection.
* Go tests for unauthorized mention warning and no task enqueue.
* Go tests for consultation wave limit.

## Dependencies

Depends on `05-23-chat-directed-graph-storage-api`.
