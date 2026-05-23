# Implement chat directed graph storage and API

## Goal

Add the persistence and API foundation for first-class directed chat conversations so routing and UI no longer depend on parsing existing message/task fields ad hoc.

## Requirements

* Add additive database migrations for directed chat graph storage.
* Store one visible chat message once, with zero or more directed recipient edges.
* Preserve original recipient actor separately from resolved task recipient.
* Store routing warnings and continuation/ambiguity state in server-owned data.
* Keep legacy messages readable as undirected messages.
* Extend server list/send responses with graph-backed sender, recipient, warning, and continuation metadata.
* Update `packages/core` chat types and parsing with compatibility fallbacks.

## Acceptance Criteria

* [ ] Existing chats load after migration without inferred historical recipient edges.
* [ ] New messages can return sender actor metadata and zero or more recipient actors.
* [ ] API shape can represent one message with multiple recipient edges.
* [ ] API shape can represent `needs_target` candidate recipients for ambiguous no-mention sends.
* [ ] Frontend core types compile with legacy responses and new graph responses.
* [ ] sqlc generation and relevant Go tests pass.

## Suggested Files

* `server/migrations/*`
* `server/internal/db/queries/*`
* `server/internal/handler/chat.go`
* `server/internal/handler/chat_plan_runs.go`
* `packages/core/types/chat.ts`
* `packages/core/chat/*`

## Test Plan

* Go migration/query tests or handler tests for graph edge creation/read models.
* Go tests for legacy undirected transcript read behavior.
* TypeScript typecheck and targeted core chat parsing tests if existing patterns allow.

## Dependencies

None. This task should land before routing and UI tasks.
