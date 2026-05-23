# Implement Plan proposal issue drafts and approval

## Goal

Ensure Plan mode produces reviewable proposal issue drafts and only creates real workspace issues after explicit user approval.

## Requirements

* Persist Plan mode proposal issues as drafts associated with the Plan Run.
* Let agent/squad consensus update draft proposal content without creating real issues.
* Add an explicit approval API/action that creates real issues from selected drafts.
* Make approval idempotent so retries do not duplicate issues.
* Show draft vs created state in API/UI.
* Preserve existing structured output behavior where possible, but route issue creation through approval.

## Acceptance Criteria

* [ ] Plan output can create/update proposal issue drafts without creating issue records.
* [ ] Draft proposals are listable from the Plan Run/chat context.
* [ ] Explicit approval creates real issues.
* [ ] Approval retry does not duplicate issues.
* [ ] Created proposals link back to created issue ids.
* [ ] Tests prove no issue is created before approval.

## Suggested Files

* `server/internal/handler/chat_plan_runs.go`
* `server/internal/handler/chat_structured_outputs.go`
* `server/internal/handler/issues.go`
* `server/migrations/*`
* `packages/core/types/chat.ts`
* `packages/views/chat/components/*`

## Test Plan

* Go tests for draft persistence, draft update, approval, idempotent retry, and created issue linkage.
* Frontend tests for draft/approval UI if the UI surface changes in this task.

## Dependencies

Depends on stable Plan Run state and preferably the graph storage task. Can run in parallel with routing after storage primitives exist.
