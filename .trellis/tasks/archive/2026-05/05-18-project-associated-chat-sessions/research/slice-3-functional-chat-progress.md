# Slice 3 Functional Chat Migration Progress

Date: 2026-05-18

## Completed

- Reused the shared rich `ChatInput` on route-owned chat pages:
  - `/chats/new`
  - `/chats/new?project_id=...`
  - `/chats/{id}`.
- Reused `ChatMessageList` on the chat-session route with per-session pending-task and agent presence state.
- Added explicit draft/editor identity overrides to `ChatInput` so route state, not the floating chat Zustand active session, owns page composition state.
- Implemented lazy session creation from `/chats/new` with an in-flight promise guard so upload/send races reuse one session.
- Wired uploads from route-owned pages:
  - new chat uploads create the session first and attach files to it
  - existing chat uploads attach to the route session.
- Wired route-owned send and stop flows with pending-task cache seeding and invalidation.
- Added inline session title editing with `TitleEditor`; PATCH writes `title_source='user'`.
- Added first-message title backfill for upload-created empty sessions:
  - only updates sessions with empty title and `title_source='legacy'`
  - does not overwrite user-edited titles.
- Removed dashboard-mounted `ChatWindow` and `ChatFab` from web and desktop shells.

## Verification

- `pnpm --filter @multica/views typecheck`
- `pnpm --filter @multica/views exec vitest run chat-input.test.tsx`
- `go test ./internal/handler -run 'TestSendChatMessage_(LinksAttachments|InvalidAttachmentIDs|SetsFirstMessageTitleForUntitledLegacySession|DoesNotOverwriteUserTitle)'`
- `go test ./...` from `server/`
- `pnpm typecheck`
- `pnpm test`
- `pnpm lint` passed with existing warnings only.
- `git diff --check`

## Local Runtime Check

- Attempted `make setup-worktree` to start a local dev server for browser QA.
- `.env.worktree` was generated with:
  - Backend: `http://localhost:18716`
  - Frontend: `http://localhost:13636`
- Setup could not continue because Docker/OrbStack was not running:
  - `failed to connect to the docker API at unix:///Users/troy.huang/.orbstack/run/docker.sock`

## Notes For Slice 4

- `UpdateChatSessionTitle` remains available for structured summary-title updates.
- Summary-title updates should keep the same protection rule: never overwrite `title_source='user'`.
- Issue proposal and output metadata ingestion remains deferred to Slice 4.
