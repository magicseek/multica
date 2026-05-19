# Slice 1 Backend Progress

Date: 2026-05-18

## Completed

- Added `chat_session` Project context fields: `project_id`, `project_context_kind`, `project_snapshot`, `title_source`.
- Added first-class `chat_issue_proposal` and `chat_issue_proposal_item` tables.
- Extended `issue.origin_type` to allow `chat_session`.
- Changed user chat deletion semantics from hard delete to archive/soft delete.
- Added sqlc read/write shapes for:
  - Project/loose scoped chat session lists
  - recent sidebar Project and loose session groups
  - issue proposal reads
  - Chat Session output metadata aggregation
- Added backend handlers/routes for:
  - `GET /api/chat/sidebar`
  - filtered `GET /api/chat/sessions`
  - `GET /api/chat/sessions/{sessionId}/issue-proposals`
  - `GET /api/chat/sessions/{sessionId}/outputs`
- Preserved private creator-owned access gates on the new read endpoints.

## Verification

- `go test ./internal/handler -run 'Test(CreateChatSession_ProjectAssociation|ProjectDelete_PreservesChatProjectSnapshot|DeleteChatSession_Archives|ListChatSidebar|UpdateChatSession)'`
- `go test ./...`
- `pnpm typecheck`
- `pnpm test`

## Notes For Slice 2

- Frontend should treat `project_context_kind`, not `project_id == null`, as the source of truth for loose vs Project-associated sessions.
- Archived sessions remain available in `status=all` history but should stay out of active sidebar and normal active lists.
- Issue proposal mutation and approval flows are intentionally not implemented yet; the read model and persistence tables are ready for later slices.
