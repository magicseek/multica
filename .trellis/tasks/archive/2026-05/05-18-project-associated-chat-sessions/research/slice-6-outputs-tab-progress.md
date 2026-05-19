# Slice 6 Outputs Tab Progress

Date: 2026-05-18

## Completed

- Extended chat output aggregation to include:
  - direct chat task output metadata
  - output metadata from issue tasks whose issue has `origin_type='chat_session'`
  - source labels for each row: `chat_task` or `issue_task`
  - source issue ID, identifier, and title for issue-derived outputs.
- Kept aggregation independent of live Project rows so deleted Project snapshots do not break output display.
- Added backend response fields for chat output source metadata while preserving the existing task output metadata shape.
- Updated task output realtime payloads so issue-task output updates can invalidate the originating chat session Outputs tab.
- Added frontend schema/type support for chat output source metadata.
- Added Chat Session tab badges for Issues and Outputs counts.
- Updated the Outputs tab UI to show source labels and issue context for issue-derived outputs.
- Added English and Chinese copy for output source labels.
- Added backend coverage for direct chat outputs, chat-originated issue outputs, unrelated issue exclusion, and deleted Project snapshot resilience.
- Fixed older Slice 5 and sidebar tests to inject workspace context and cast project IDs consistently with the handlers.

## Verification

- `go test ./internal/handler -run 'TestListChatOutputs|TestTaskOutputMetadata|TestCompleteTask_ChatStructuredOutputs'`
- `go test ./internal/handler -run 'TestApproveChatIssueProposal|TestUpdateAndDismissChatIssueProposal|TestProjectDelete_PreservesChatProjectSnapshot|TestListChatSidebar_FiltersRecentActiveProjectAndLooseSessions|TestListChatOutputs'`
- `go test ./...` from `server/`
- `pnpm typecheck`
- `pnpm test`
- `pnpm lint` passed with existing warnings only.
- `git diff --check`

## Notes For Next Slice

- The Outputs tab currently lists metadata only. It does not preview or download artifacts beyond existing file link behavior.
- Issue-derived output source labeling depends on issue origin metadata. If future flows create issues from chat without `origin_type='chat_session'`, those outputs will not appear in the chat Outputs tab.
- Frontend tests rely on existing route/page coverage; no dedicated visual regression was added for the Outputs tab badge/source line.
