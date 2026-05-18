# Slice 4 Structured Output Handoff Progress

Date: 2026-05-18

## Completed

- Added daemon manifest ingestion for:
  - `.multica/chat-summary.json`
  - `.multica/issue-proposals.json`
  - `.multica/outputs.json`.
- Extended daemon task completion to send a `structured_outputs` payload in the existing complete request.
- Kept the legacy `.multica/outputs.json` upload path for compatibility, while skipping the separate upload when outputs were already included in completion.
- Added backend completion handling for structured outputs after the task is finalized:
  - summary titles update eligible chat sessions with `title_source='agent_summary'`
  - user-edited titles remain protected
  - issue proposals persist with task, assistant message, and proposer agent provenance
  - task output metadata uses replace semantics.
- Made issue proposal persistence idempotent per task by replacing existing proposals for the same task before inserting the new payload.
- Added realtime events/invalidation for chat issue proposal updates and chat output updates.
- Aligned chat issue proposal status typing/copy with backend statuses.
- Updated runtime instructions so agents know which manifests to write and that issue creation remains backend/user-controlled.

## Verification

- `make sqlc`
- `go test ./internal/handler -run 'TestCompleteTask_ChatStructured|TestCompleteTask_InvalidIssueProposal|TestTaskOutputMetadata'`
- `go test ./internal/daemon -run 'TestLoadTaskOutputManifest|TestLoadStructuredTaskOutputs'`
- `go test ./...` from `server/`
- `pnpm --filter @multica/core typecheck`
- `pnpm --filter @multica/views typecheck`
- `pnpm typecheck`
- `pnpm test`
- `pnpm lint` passed with existing warnings only.
- `git diff --check`

## Notes For Slice 5

- Proposal approval/editing is still deferred.
- Proposals are now available through the existing session proposals endpoint and refresh via `chat:issue_proposals_updated`.
- Proposal item statuses currently persist as `pending` until the approval flow is implemented.
