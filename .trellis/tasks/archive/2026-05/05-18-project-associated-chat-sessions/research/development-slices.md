# Development Slices

Use this as the first planning pass before implementation. The full source of truth remains `docs/project-associated-chat-sessions-plan.md`.

## Slice 1: Backend Schema and Read APIs

Goal: make Project-associated Chat Sessions and proposal objects representable without changing the UI yet.

Deliver:

- chat session Project context fields and title source fields
- proposal/proposal item tables
- `issue.origin_type = chat_session`
- sqlc regeneration
- session list filters for loose/project scopes
- sidebar quick-access read endpoint

Acceptance:

- existing chat sessions migrate as loose
- Project deletion preserves project-associated session history through snapshot
- private creator-only session access remains enforced
- Project and agent are immutable after session creation

## Slice 2: Route Shell and Navigation

Goal: introduce page-level Chat routes and navigation without yet completing every tab behavior.

Deliver:

- path builders for `chats`, `chatNew`, and `chatSession`
- web and desktop routes
- Chat page shell with `Chat / Issues / Outputs` tabs
- empty new-chat page with page-level composer
- sidebar Projects tree and `Chats > Loose`
- Project detail `Issues / Chats` tabs

Acceptance:

- Project row body toggles expansion
- Project row hover actions can open Project detail and new Project chat
- recent five-day sidebar filter works
- complete lists remain available from Chats page and Project detail Chats tab

## Slice 3: Functional Chat Migration

Goal: replace the floating Chat experience with route-owned session behavior.

Deliver:

- reuse `ChatInput` and `ChatMessageList`
- lazy-create sessions from `/chats/new`
- send messages and uploads from page routes
- title editing with user-edited protection
- remove dashboard-mounted Chat FAB/window

Acceptance:

- first message creates one session
- upload/send race does not create duplicate sessions
- route state, not floating Zustand state, selects the active session
- user-edited title is not overwritten by agent summary title

## Slice 4: Structured Output Handoff

Goal: let daemon task completion carry structured chat outputs.

Deliver:

- manifest loaders for chat summary and issue proposals
- completion payload extension
- backend validation and persistence
- compatibility path for existing `.multica/outputs.json`
- runtime instructions for agents to write manifests

Acceptance:

- valid summary title updates eligible sessions
- valid proposals persist with task/message/agent provenance
- invalid proposal payload does not leave task running
- output metadata does not double-insert during compatibility period

## Slice 5: Proposal Approval and Issues Tab

Goal: turn persisted proposals into approved backlog issues.

Deliver:

- proposal item editing
- approve selected items transaction
- skipped/restore/dismiss flows
- created issue list in Chat Session Issues tab
- inline proposal card under the proposing assistant message

Acceptance:

- selected items create backlog issues atomically
- partial approval marks unselected items skipped
- skipped items can be restored
- approving user is the issue creator
- issue keeps lightweight chat session origin; proposal tables keep detailed provenance

## Slice 6: Outputs Tab

Goal: expose output metadata produced by the chat and by issues created from it.

Deliver:

- chat output aggregation endpoint
- Outputs tab list
- tab output count
- realtime invalidation for output updates

Acceptance:

- direct chat task outputs appear
- outputs from chat-originated issue tasks appear
- unrelated Project issue outputs do not appear
- deleted Project snapshot does not break output display
