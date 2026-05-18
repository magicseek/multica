# Project-Associated Chat Sessions and Issue Proposals

> Status: Draft implementation plan
> Date: 2026-05-18
> Scope: Chat session navigation, Project association, chat-originated issues, proposal approval, chat outputs, daemon structured output handoff

## TL;DR

Multica should move Chat from the global floating panel into page-level workspace routes.

The new model keeps Chat Sessions private, but lets a session be associated with a Project at creation time. Project-associated sessions appear under Projects in the sidebar and in the owning Project detail page. Loose sessions appear under the workspace Chats entry.

Agents do not directly create issues from chat. They produce structured issue proposals. The backend validates and persists those proposals, users may edit proposal items, and user approval creates normal backlog issues in a single transaction.

The Chat Session page has fixed tabs:

- `Chat`: transcript and composer
- `Issues`: pending proposals plus issues created from this chat
- `Outputs`: output metadata from the chat task and from tasks on issues created by this chat

## Current State

Chat is currently implemented as a global floating window:

- `packages/views/chat/components/chat-fab.tsx`
- `packages/views/chat/components/chat-window.tsx`
- Mounted by:
  - `apps/web/app/[workspaceSlug]/(dashboard)/layout.tsx`
  - `apps/desktop/src/renderer/src/components/desktop-layout.tsx`

Chat server state already exists:

- `chat_session`
- `chat_message`
- `agent_task_queue.chat_session_id`
- `packages/core/chat/queries.ts`
- `packages/core/chat/mutations.ts`
- `server/internal/handler/chat.go`
- `server/internal/service/task.go`

Important current constraints:

- Chat sessions are creator-owned and private.
- A chat session has one `agent_id`.
- `ListChatSessions` filters sessions by creator and accessible agent.
- Chat tasks are completed through `CompleteTask`; assistant messages are saved from the daemon final output.
- `task_output_metadata` already exists, but daemon uploads `.multica/outputs.json` through a separate `/outputs` endpoint after task execution.
- Projects are hard-deleted today. `issue.project_id` uses `ON DELETE SET NULL`.
- There is no shared page-level Chat route in web or desktop.

## Product Decisions

### Privacy

Project-associated Chat Sessions stay private to the creator. Project association changes organization and defaults, not visibility.

Issues created from chat follow normal issue visibility. Output metadata from chat stays under the chat privacy boundary until explicitly attached to a shared issue, project, or published artifact.

### Project Association

A Chat Session may be either:

- loose
- project-associated

Project association is chosen at creation time and is not movable later.

Project-associated sessions:

- appear under Projects in the sidebar quick tree
- appear in the Project detail `Chats` tab for the creator
- use the Project as the default target for chat-originated issues
- keep a creation-time Project snapshot so deleted Projects can still be represented in chat history

Loose sessions:

- appear under the workspace `Chats` entry
- can have no Project context
- create no-Project issues unless the issue is later edited normally

### Agent Selection

A Chat Session must have an agent before the first send.

For Project-associated sessions, recommend the Project lead agent when the lead is an agent and accessible to the user. The user still explicitly selects or confirms the agent. The selected agent is fixed for the session.

Do not support changing the agent after session creation.

### Title

Do not include the Project name in the Chat Session title. Navigation already provides that hierarchy.

Initial title fallback:

- first user message, truncated

After the first agent execution, the agent may return a summary title through structured completion metadata. The backend applies it only if the user has not edited the title.

Track title source with either:

- `title_source`
- or `title_edited_by_user`

### Sidebar

The sidebar quick tree shows only active Chat Sessions updated within the last five rolling days.

This five-day rule only affects quick access:

- complete loose history remains in `Chats`
- complete project-associated history remains in Project detail `Chats`

Suggested shape:

```text
Workspace
  Issues
  Projects
    Project A
      Chat session 1
      Chat session 2
    Project B
      Chat session 3
  Chats
    Loose
      Chat session 4
```

Project row behavior:

- clicking the row toggles expand/collapse
- hover actions include:
  - new chat in this Project
  - open Project detail

### Project Detail

Project detail gets two main tabs:

- `Issues`
- `Chats`

`Chats` lists the current user's private Chat Sessions under that Project. Rows navigate to the Chat Session page.

### Empty Chat Page

Use a page-level Codex-style composer, not the old floating-window shell.

Do not show recommendation/plugin cards.

Project-associated empty title:

```text
What should we work on in {Project Name}?
```

Loose empty title:

```text
What should we work on?
```

Session creation is lazy. `/chats/new` or `/chats/new?project_id=...` does not create a row until the first send or first attachment upload requires a session id.

## Data Model

### `chat_session`

Add Project context and title source fields.

Suggested fields:

| Field | Notes |
|---|---|
| `project_id` | Nullable FK to `project(id) ON DELETE SET NULL` |
| `project_context_kind` | `loose` or `project`; do not infer loose from `project_id IS NULL` |
| `project_snapshot` | JSONB with creation-time Project display data |
| `title_source` | `legacy`, `first_message`, `agent_summary`, or `user` |
| `deleted_at` | Nullable soft-delete marker, if deletion stays distinct from archive |

Existing sessions migrate to:

```text
project_context_kind = loose
project_snapshot = null
title_source = legacy
```

Rules:

- `project_id` is writable only at creation.
- `agent_id` is writable only at creation.
- Project deletion sets `project_id` null but keeps `project_context_kind = project` and keeps `project_snapshot`.
- User title edits set `title_source = user`.
- Agent summary title may update title only when `title_source != user`.

### `chat_issue_proposal`

Persist agent proposals separately from chat prose.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `workspace_id` | Workspace scope |
| `chat_session_id` | Source session |
| `source_chat_message_id` | Assistant message that surfaced the proposal, nullable if unavailable |
| `source_task_id` | Chat task that produced the proposal |
| `proposer_agent_id` | Agent that proposed it |
| `title` | Proposal group title |
| `summary` | Optional proposal summary |
| `status` | `pending`, `accepted`, `partially_accepted`, `dismissed` |
| `created_at`, `updated_at` | Timestamps |

### `chat_issue_proposal_item`

Persist item-level approval state.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `proposal_id` | FK |
| `position` | Stable ordering |
| `title` | Editable issue draft title |
| `description` | Editable issue draft description |
| `priority` | Optional issue priority |
| `labels` | Optional JSONB label draft metadata |
| `assignee_id` | Optional member or agent assignment target, depending on existing issue conventions |
| `status` | `pending`, `created`, `skipped` |
| `issue_id` | Created issue, nullable |
| `approved_snapshot` | JSONB snapshot of item content used for issue creation |
| `created_at`, `updated_at` | Timestamps |

Rules:

- Proposal items do not store editable `project_id`.
- Created issue Project is derived from the source Chat Session context.
- Agent cannot specify issue `status`; backend creates issues in backlog.
- Partial approval marks unselected pending items as `skipped`.
- Skipped items can be restored to `pending`.
- Approval is atomic. If any selected item fails validation, no issues are created.

### `issue.origin_type`

Extend the existing origin constraint to include:

```text
chat_session
```

Created chat-originated issues use:

```text
origin_type = chat_session
origin_id = chat_session.id
```

Detailed provenance stays in the proposal tables:

- source chat message
- source task
- proposer agent
- approval snapshot

### Output Metadata Aggregation

`task_output_metadata` already exists.

Add server queries for a Chat Session output view:

1. output metadata from tasks where `agent_task_queue.chat_session_id = chat_session.id`
2. output metadata from tasks whose issue was created with `origin_type = chat_session` and `origin_id = chat_session.id`

Exclude unrelated issues in the same Project.

## Structured Output Handoff

The backend does not infer issues from prose. It only validates structured payloads handed off by the daemon.

### Local Manifests

Suggested fixed manifest paths:

```text
.multica/chat-summary.json
.multica/issue-proposals.json
.multica/outputs.json
```

The path is fixed per task workdir. Durable association is stored in DB through task/session/message/proposal rows, not encoded in file paths.

### Summary Title Manifest

```json
{
  "version": 1,
  "title": "Summarize Slack hierarchy"
}
```

Validation:

- title is required
- trim whitespace
- cap to `chatSessionTitleMaxLen`
- apply only if the session title was not user-edited

### Issue Proposal Manifest

```json
{
  "version": 1,
  "proposals": [
    {
      "title": "Implementation tasks",
      "summary": "Break the chat plan into backlog issues.",
      "items": [
        {
          "title": "Add chat session project context",
          "description": "Persist project association and snapshot on chat sessions.",
          "priority": "medium",
          "labels": ["backend"],
          "assignee_id": null
        }
      ]
    }
  ]
}
```

Validation:

- ignore unknown fields
- reject missing item title
- cap title and description lengths
- validate priority against existing issue priority values
- validate assignee access if provided
- reject `status` if present or ignore it explicitly; backend owns created issue status
- do not allow project fields

### Completion Payload

Target shape:

```json
{
  "output": "...",
  "session_id": "...",
  "work_dir": "...",
  "structured_outputs": {
    "chat_summary": { "version": 1, "title": "..." },
    "issue_proposals": { "version": 1, "proposals": [] },
    "outputs": { "outputs": [] }
  }
}
```

Current code uploads output metadata through `/api/daemon/tasks/{taskId}/outputs`. Implementation should either:

1. migrate daemon output metadata reporting into completion payload in the same change, or
2. keep the existing `/outputs` endpoint as compatibility while new daemon versions include output metadata in completion payload.

The target architecture is one task completion payload so summary title, proposals, and output metadata are processed from the same completion event.

Recommended processing order in `CompleteTask` for chat tasks:

1. complete task and update resume pointer
2. create assistant chat message from final output
3. apply summary title if allowed
4. persist issue proposals with `source_chat_message_id`, `source_task_id`, `proposer_agent_id`
5. store output metadata
6. broadcast chat/task/proposal/output invalidation events

Invalid proposal payloads should not make the task look still-running. Save the assistant reply, reject the invalid structured proposal, log the validation error, and surface a non-blocking structured-output warning if needed.

## API Surface

### Chat Sessions

Extend:

```text
POST   /api/chat/sessions
PATCH  /api/chat/sessions/{id}
GET    /api/chat/sessions
GET    /api/chat/sessions/{id}
DELETE /api/chat/sessions/{id}
```

`POST /api/chat/sessions` adds:

```json
{
  "agent_id": "uuid",
  "title": "optional fallback title",
  "project_id": "optional uuid",
  "default_repository_id": "optional uuid"
}
```

Rules:

- validate Project exists in workspace when `project_id` is provided
- write Project snapshot at creation
- require accessible, non-archived agent
- project lead agent can be recommended in UI, but backend requires explicit `agent_id`

`PATCH /api/chat/sessions/{id}`:

- allow `title`
- allow `default_repository_id`
- disallow `project_id`
- disallow `agent_id`
- user title edits mark title source as user

`DELETE /api/chat/sessions/{id}`:

- should become soft-delete or archive, not hard delete
- keep row, messages, created issue provenance, and output metadata discoverability
- hide from active sidebar and normal active lists

### Sidebar Navigation

Add a dedicated endpoint or filtered list query for sidebar quick access.

Recommended endpoint:

```text
GET /api/chat/sidebar?recent_days=5
```

Response:

```json
{
  "projects": [
    {
      "project": { "id": "uuid", "title": "Project A", "icon": null },
      "sessions": []
    }
  ],
  "loose": []
}
```

Rules:

- only current user's private sessions
- active sessions only
- `updated_at >= now() - recent_days`
- active Projects only
- project-associated sessions whose Project is deleted do not appear in active Projects tree

### Complete Lists

Use filtered `GET /api/chat/sessions` for complete history:

```text
GET /api/chat/sessions?scope=loose&status=all
GET /api/chat/sessions?scope=project&project_id={id}&status=all
```

### Issue Proposals

Add:

```text
GET  /api/chat/sessions/{sessionId}/issue-proposals
PATCH /api/chat/issue-proposals/{proposalId}/items/{itemId}
POST /api/chat/issue-proposals/{proposalId}/approve
POST /api/chat/issue-proposals/{proposalId}/items/{itemId}/restore
POST /api/chat/issue-proposals/{proposalId}/dismiss
```

Approval request:

```json
{
  "item_ids": ["uuid", "uuid"]
}
```

Approval response:

```json
{
  "issues": [],
  "proposal": {}
}
```

Rules:

- only the Chat Session creator can view or approve the proposal
- selected items must be pending
- create issues in backlog
- all selected items create successfully or none do
- unselected pending items become skipped when the proposal is partially approved

### Chat Outputs

Add:

```text
GET /api/chat/sessions/{sessionId}/outputs
```

Returns output metadata from the aggregation rules above.

## Frontend Plan

### Routes

Add path builders in `packages/core/paths/paths.ts`:

```ts
chats: () => `${ws}/chats`
chatNew: (params?: { project_id?: string }) => ...
chatSession: (id: string, tab?: "chat" | "issues" | "outputs") => ...
```

Web routes:

```text
apps/web/app/[workspaceSlug]/(dashboard)/chats/page.tsx
apps/web/app/[workspaceSlug]/(dashboard)/chats/new/page.tsx
apps/web/app/[workspaceSlug]/(dashboard)/chats/[id]/page.tsx
```

Desktop routes:

```text
/:workspaceSlug/chats
/:workspaceSlug/chats/new
/:workspaceSlug/chats/:id
```

Shared views:

```text
packages/views/chat/components/chats-page.tsx
packages/views/chat/components/chat-session-page.tsx
packages/views/chat/components/chat-empty-page.tsx
packages/views/chat/components/chat-issues-tab.tsx
packages/views/chat/components/chat-outputs-tab.tsx
packages/views/chat/components/chat-issue-proposal-card.tsx
```

### Reuse and Refactor

Reuse:

- `ChatInput`
- `ChatMessageList`
- `OfflineBanner`
- `NoAgentBanner`
- agent availability hooks
- file upload hook
- repository selector logic
- existing React Query chat cache keys where possible

Refactor out of `ChatWindow`:

- lazy session creation
- send message handling
- upload handling
- active agent resolution
- repository selector state

New route-based session state should come from URL params, not Zustand `activeSessionId`.

Keep in Zustand only:

- drafts
- selected agent preference for new loose sessions
- optional composer preferences

Remove or stop using:

- global `isOpen`
- floating window size
- floating expanded state
- FAB pending indicator

### Layout

Remove `ChatWindow` and `ChatFab` from:

- web dashboard layout
- desktop layout

Use sidebar and routes as the only primary Chat entry.

### Sidebar

Update `packages/views/layout/app-sidebar.tsx`:

- add `Chats` workspace item
- add expandable Projects tree with recent sessions
- add `Chats > Loose` recent session group
- row click toggles Project expansion
- hover actions:
  - new Project chat
  - open Project detail
- navigate session rows to `chatSession(id)`
- navigate top-level New Chat to `chatNew()`

### Project Detail

Update `ProjectDetail`:

- introduce `Issues` and `Chats` tabs
- keep existing issues surface under `Issues`
- new `Chats` tab lists all current user's sessions for that Project
- include `New chat` action linking to `chatNew({ project_id })`

### Chat Session Page

Header:

- title editor
- Project context chip when project-associated
- agent chip
- repository chip/selector if allowed
- status chip if archived/deleted

Tabs:

- `Chat`
- `Issues`
- `Outputs`

Counts:

- `Issues` badge counts created chat-originated issues only
- pending proposal items do not increment the issue count
- `Outputs` badge counts output metadata records

`Chat` tab:

- transcript
- inline proposal cards under the relevant assistant message
- composer at bottom

`Issues` tab:

- pending proposals first
- then proposal groups by creation time
- created issues linked as normal issue rows/cards
- restore skipped items
- approve selected items

`Outputs` tab:

- output metadata list
- source labels:
  - chat task
  - issue task
- no proposal artifacts

## Realtime and Cache

Extend event handling in `packages/core/realtime/use-realtime-sync.ts`.

Events to add or reuse:

- `chat:session_updated`
- `chat:session_deleted` or new soft-delete event
- `chat:issue_proposals_updated`
- `chat:issues_updated`
- `task:outputs_updated`

Invalidation targets:

- session detail
- session list
- sidebar tree
- chat messages
- issue proposals
- chat-created issues
- chat outputs
- project detail chat list

Avoid writing proposal state into Zustand. Proposal state is server state owned by React Query.

## Implementation Phases

### Phase 1: Data Model and Backend Read Shapes

Deliver:

- chat session Project fields and title source fields
- proposal tables
- origin constraint update
- sqlc regeneration
- API response types include Project context and counts where needed
- list filters for loose/project sessions
- sidebar quick-access endpoint

Tests:

- migrations apply
- existing chats migrate as loose
- Project deletion leaves project-associated chat identifiable through snapshot
- private chat filtering still applies
- cannot move session Project or change agent

### Phase 2: Page Routes and Sidebar Navigation

Deliver:

- path builders
- web routes
- desktop routes
- remove floating Chat mount
- route-based Chat Session page shell
- empty new-chat page
- sidebar Project tree and Chats/Loose group
- Project detail `Issues` / `Chats` tabs

Tests:

- path builder unit tests
- desktop route tests
- sidebar grouping tests
- Project row click toggles while hover actions navigate
- empty Project chat requires explicit agent and lazy-creates on first send

### Phase 3: Chat Page Functional Migration

Deliver:

- reuse ChatInput and ChatMessageList in page layout
- lazy-create sessions from `/chats/new`
- send messages from route page
- uploads create session when needed
- title editing and title source handling
- no global FAB/window state dependency

Tests:

- first message creates exactly one session
- upload and send race dedupes session creation
- user-edited title is not overwritten
- active route session renders cached messages without first-message flash

### Phase 4: Structured Output Handoff

Deliver:

- daemon manifest loaders for chat summary and issue proposals
- completion payload extension
- backend parsing and validation
- summary title update
- proposal persistence
- output metadata processing from completion payload, with compatibility for existing `/outputs`
- runtime prompt update telling agents how to write manifests

Tests:

- valid manifests persist structured outputs
- oversized or invalid manifests are ignored with warnings
- invalid proposal does not leave task running
- summary title applies only when allowed
- existing `.multica/outputs.json` behavior remains compatible during migration

### Phase 5: Proposal Approval and Issues Tab

Deliver:

- proposal item editing
- approve selected items transaction
- skipped/restore/dismiss flows
- issue creation with `origin_type = chat_session`
- `Issues` tab proposal management and created issue list
- inline proposal card in Chat transcript

Tests:

- approval creates backlog issues
- partial approval marks unselected items skipped
- restore returns skipped item to pending
- validation failure creates no partial issue batch
- created issue creator is approving user
- proposal provenance records task/message/agent
- issue only stores lightweight chat session origin

### Phase 6: Outputs Tab

Deliver:

- chat output aggregation endpoint
- output count
- Outputs tab UI
- event invalidation

Tests:

- direct chat task outputs appear
- outputs from chat-originated issue tasks appear
- unrelated Project issue outputs do not appear
- deleted Project snapshot does not break output aggregation

## Verification Plan

Run, at minimum:

```bash
make test
pnpm typecheck
pnpm test
```

For frontend route work, also run the web and desktop route-specific tests touched by the change.

Manual QA checklist:

- create loose chat from top-level New Chat
- create Project chat from sidebar hover action
- create Project chat from Project detail Chats tab
- verify sidebar recent tree filters to five days
- verify complete loose history in Chats page
- verify complete Project chat history in Project detail
- agent returns summary title and proposal
- user edits proposal item and approves selected items
- created issues are backlog and linked to source chat
- outputs appear only in the right Chat Session Outputs tab
- delete Project and confirm chat still shows deleted Project snapshot
- delete/archive chat and confirm created issues remain

## Non-Goals

- No shared Project team chat in this version.
- No chat move between Projects.
- No agent switching after session creation.
- No backend inference of issues from prose.
- No proposal storage as markdown-only chat text.
- No proposal artifacts in the Outputs tab.
- No new dependencies.

## Main Risks

- `CompleteTask` currently handles chat message creation after task completion. Adding structured outputs needs careful ordering so proposals can reference the assistant message when available.
- Existing chat delete is hard-delete oriented. It must be changed before chat-originated issue and output provenance depends on durable session rows.
- Route-based Chat should not keep using floating-panel state as a second source of truth.
- Sidebar tree queries must preserve the existing private-agent access filter.
- Output metadata migration should avoid double-inserting when old and new daemon paths overlap.
