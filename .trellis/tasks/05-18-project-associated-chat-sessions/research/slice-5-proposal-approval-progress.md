# Slice 5 Proposal Approval and Issues Tab Progress

Date: 2026-05-18

## Completed

- Added chat issue proposal approval/edit/restore/dismiss backend endpoints.
- Added an atomic approval transaction:
  - validates selected pending proposal items before creating any issue
  - creates selected items as `backlog` issues
  - marks unselected pending items as `skipped`
  - records `approved_snapshot` and created `issue_id`
  - recalculates proposal status as `accepted`, `partially_accepted`, `pending`, or `dismissed`.
- Created issues use:
  - `creator_type='member'`
  - approving user as `creator_id`
  - `origin_type='chat_session'`
  - `origin_id=chat_session.id`
  - project context inherited from the owning chat session when present.
- Added backend route for listing issues created from a chat session.
- Added realtime event `chat:issues_updated` and frontend invalidation for chat session issues plus workspace issue caches.
- Added frontend API schemas, client methods, query keys, and mutations for:
  - listing chat-origin issues
  - editing proposal items
  - approving selected proposal items
  - restoring skipped items
  - dismissing pending proposal items.
- Reworked the Chat Session Issues tab to show:
  - editable proposal cards
  - selected-item approval
  - dismiss/restore flows
  - created issue links.
- Added inline proposal cards under the assistant message that produced the proposal.
- Added English and Chinese copy for proposal approval UI.

## Verification

- `gofmt -w server/internal/handler/chat_issue_proposals.go server/internal/handler/chat_issue_proposals_test.go server/internal/handler/chat_structured_outputs.go server/cmd/server/router.go server/pkg/protocol/events.go`
- `go test ./internal/handler -run 'TestApproveChatIssueProposal|TestUpdateAndDismissChatIssueProposal|TestCompleteTask_ChatStructured'`
- `go test ./...` from `server/`
- `pnpm --filter @multica/core typecheck`
- `pnpm --filter @multica/views typecheck`
- `pnpm typecheck`
- `pnpm test`
- `pnpm lint` passed with existing warnings only.
- `git diff --check`

## Notes For Slice 6

- Outputs tab still uses direct chat session output metadata; aggregation from chat-originated issue tasks remains Slice 6.
- Proposal labels are currently captured in proposal snapshots only. They are not attached as first-class issue labels during approval.
- Proposal assignee editing supports `member`, `agent`, or unassigned. The proposal table validation intentionally excludes `squad` for now.
- The inline proposal card links to the Issues tab. Full inline approval was not added to keep mutation controls centralized in the tab.
