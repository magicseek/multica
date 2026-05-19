# Slice 2 Route and Navigation Progress

Date: 2026-05-18

## Completed

- Added workspace path builders for:
  - `/chats`
  - `/chats/new`
  - `/chats/new?project_id=...`
  - `/chats/{id}` with optional `tab=issues|outputs`.
- Added typed Chat API/client/query support for:
  - scoped session lists
  - recent sidebar hierarchy
  - issue proposal reads
  - output metadata reads.
- Added shared page-level Chat views:
  - `ChatsPage`
  - `ChatNewPage`
  - `ChatSessionPage`
  - `ProjectChatsSurface`.
- Wired web routes under `apps/web/app/[workspaceSlug]/(dashboard)/chats`.
- Wired desktop routes under `apps/desktop/src/renderer/src/routes.tsx`.
- Reworked sidebar Workspace navigation into the requested hierarchy:
  - `Issues`
  - `Projects` expandable tree with recent project-associated sessions
  - `Chats > Loose` with recent loose sessions
  - remaining workspace sections.
- Project row body now toggles expansion; hover actions open Project detail and create a Project-associated chat.
- Project detail now exposes `Issues / Chats` tabs; the Chats tab lists Project-associated sessions and links to create a new one.
- Chat session page now exposes `Chat / Issues / Outputs` tabs, with read-only Issues/Outputs panels backed by the Slice 1 APIs.
- Added `chats` to reserved route segments and link-prefix handling.

## Verification

- `pnpm typecheck`
- `pnpm lint`
- `pnpm --filter @multica/views test -- app-sidebar chat`
- `pnpm --filter @multica/core test -- paths`
- `pnpm test`
- `go test ./...` from `server/`
- `git diff --check`

## Notes For Slice 3

- The floating Chat FAB/window is still mounted; Slice 3 should migrate final send/upload/title behavior and then remove the dashboard-mounted floating UI.
- The new page composer already supports basic first-message session creation and follow-up sending, but it does not yet reuse the rich attachment-aware `ChatInput`.
- Project/agent movement remains intentionally unsupported.
- Issue proposal approval/editing remains deferred to Slice 5.
