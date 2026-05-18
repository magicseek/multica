# Sidebar, Proposed Lane, and Settings Configure Handoff

Status: Trellis implementation handoff
Date: 2026-05-18
Owner: Trellis

This handoff records the post-QA product decisions for the current
Project-associated Chat Sessions branch. It supplements:

- `CONTEXT.md`
- `docs/adr/0002-private-project-associated-chat-sessions-and-issue-proposals.md`
- `docs/project-associated-chat-sessions-plan.md`

Trellis owns implementation. This document is planning and acceptance
criteria only.

## Locked Decisions

### Chat Session Issues Kanban

- `Proposed` is a review lane in the Chat Session `Issues` tab, not a new
  `IssueStatus`.
- The Chat Session `Issues` Kanban orders lanes as:
  `Proposed`, `Backlog`, `Todo`, `In Progress`, `In Review`, `Done`,
  `Blocked`.
- `Proposed` contains pending `Chat Issue Proposal Items`.
- Proposal cards in `Proposed` use proposal review controls, not normal issue
  drag-and-drop.
- Approving proposal items removes them from `Proposed` and creates real
  backlog issues.
- Batch approval defaults to selected pending proposal items across the
  entire Chat Session, while preserving proposal grouping for context.

### Sidebar Information Architecture

- Remove the old `Workspace` section title from the primary sidebar.
- Primary sidebar order:
  `Inbox`, `My Issues`, `Issues`, `Projects`, `Agents`, `Squads`,
  `Autopilot`, `Usage`, `Recents`.
- `Projects` remains expandable and lists all active Projects, even Projects
  with no recent chats.
- Each expanded Project may show up to three recent Project-associated Chat
  Sessions.
- Project-associated chat history beyond the sidebar subset is recovered from
  the owning Project detail `Chats` tab.
- `Recents` is the sidebar label for recent loose Chat Sessions only. The
  product concept and full archive remain `Chats`.
- `Recents` initially shows active loose Chat Sessions updated in the last
  five days.
- `Recents` groups rows by recency labels such as Today, Yesterday, Last five
  days, and Older.
- `Recents` has an inline "Show more" pagination control. Each click appends
  10 older active loose Chat Sessions beyond the current list.
- `Recents` must not include Project-associated Chat Sessions.
- Sidebar layout has fixed top chrome, a scrollable main navigation area, and
  fixed bottom Settings. Long Projects/Recents lists scroll without moving
  Settings.

### Settings Configure Relocation

- `Runtimes`, `Workflows`, and `Skills` move out of primary sidebar
  navigation.
- `Settings` is a single fixed bottom sidebar entry.
- Settings page middle navigation groups are:
  `Account`, `Configure`, `Workspace`.
- `Configure` contains `Runtimes`, `Workflows`, and `Skills`.
- Existing direct URLs for `/:workspace/runtimes`, `/:workspace/workflows`,
  and `/:workspace/skills` remain addressable for compatibility. Preferred
  behavior for list routes is a `replace` redirect to
  `/:workspace/settings?tab=runtimes|workflows|skills`; detail routes may
  remain direct if detail surfaces are not yet embedded in Settings.

## Trellis Implementation Slices

### Slice 1: Chat Issues Kanban Proposed Lane

Scope:

- Update the Chat Session `Issues` tab to render a Kanban-style board.
- Add a leading `Proposed` lane backed by pending `Chat Issue Proposal Items`.
- Reuse existing issue board presentation where practical, but do not extend
  `IssueStatus`.
- Keep proposal state in React Query. Do not introduce Zustand proposal state.
- Preserve existing proposal editing, restore, dismiss, and approval behavior.

Acceptance criteria:

- Pending proposal items appear in `Proposed`.
- Created chat-originated issues appear in normal issue lanes by their real
  issue status.
- `Proposed` cards cannot be dragged into issue status lanes.
- Normal issues keep existing drag-and-drop status behavior.
- Approving selected items across the Chat Session creates backlog issues and
  removes those cards from `Proposed`.
- Created issue count still counts only real issues, not pending proposal
  items.

Focused tests:

- Chat `Issues` tab renders `Proposed` before `Backlog`.
- Pending proposal items are not treated as issues.
- Batch approval can select items from multiple proposal groups.
- Approval calls remain atomic through the backend proposal approval contract.
- Dragging a proposal item is disabled or ignored.

Likely files:

- `packages/views/chat/components/chat-pages.tsx`
- `packages/views/issues/components/board-view.tsx`
- `packages/views/issues/components/board-column.tsx`
- `packages/core/chat/mutations.ts`
- `packages/core/chat/queries.ts`
- `packages/views/locales/*/chat.json`

### Slice 2: Sidebar Data Shape and Pagination API

Scope:

- Adjust sidebar data so `Projects` can list all active Projects, not only
  Projects with recent chats.
- Limit per-Project recent chat rows to three.
- Add or extend an API for loose chat pagination:
  `limit=10`, cursor by `updated_at` plus stable `id`, active loose sessions
  only.
- Keep the initial five-day window as the first Recents page.
- Preserve private creator and accessible-agent filtering.
- Parse frontend responses defensively with schemas/fallbacks.

Recommended API shape:

```text
GET /api/chat/sidebar
```

Returns all active Projects with up to three recent Project-associated
sessions per Project, plus the initial loose Recents set.

```text
GET /api/chat/sidebar/recents?limit=10&cursor=...
```

Returns older active loose Chat Sessions only.

Acceptance criteria:

- Projects with zero recent chat sessions still appear.
- Completed/cancelled/deleted Projects do not appear in active sidebar tree.
- Project-associated sessions remain grouped under Projects only.
- Recents initial set is five-day loose active sessions.
- "Show more" can load older active loose chats beyond five days.
- No unauthorized private-agent sessions leak into sidebar data.

Focused tests:

- Backend sidebar endpoint includes active Project with no chats.
- Backend caps project-associated chat rows to three per Project.
- Backend loose pagination returns older rows in deterministic order.
- Backend loose pagination does not return Project-associated sessions.
- Frontend API schema tolerates missing optional pagination fields.

Likely files:

- `server/pkg/db/queries/chat.sql`
- `server/internal/handler/chat.go`
- `server/internal/handler/chat_test.go`
- `packages/core/api/client.ts`
- `packages/core/api/schemas.ts`
- `packages/core/types/chat.ts`
- `packages/core/chat/queries.ts`

### Slice 3: Sidebar IA and Fixed Settings Layout

Scope:

- Remove `Workspace` and `Configure` section headings from the primary
  sidebar.
- Reorder primary navigation as locked above.
- Keep `Projects` expandable.
- Rename the loose chat sidebar section to `Recents`.
- Add date grouping and inline "Show more" for Recents.
- Move Settings to a fixed bottom sidebar entry.
- Ensure only the middle navigation body scrolls.

Acceptance criteria:

- Sidebar matches the locked order.
- Settings stays visible at the bottom when Projects/Recents overflow.
- Sidebar scroll area includes primary navigation, pinned items if present,
  Projects, and Recents.
- Recents date grouping is presentation-only.
- "Show more" appends rows in place without navigating away.
- Empty states are still clear for no Projects and no recent chats.

Focused tests:

- AppSidebar renders nav items in the locked order.
- AppSidebar no longer renders Workspace/Configure group labels.
- Settings is rendered in the fixed footer area and links to Settings.
- Recents renders only loose sessions and groups by date.
- Clicking "Show more" appends the next page and preserves current page
  context.

Likely files:

- `packages/views/layout/app-sidebar.tsx`
- `packages/views/layout/app-sidebar.test.tsx`
- `packages/views/locales/en/layout.json`
- `packages/views/locales/zh-Hans/layout.json`
- `packages/core/chat/queries.ts`

### Slice 4: Settings Configure Relocation

Scope:

- Add `Configure` group to Settings middle navigation.
- Add tabs for `Runtimes`, `Workflows`, and `Skills`.
- Remove `Runtimes`, `Workflows`, and `Skills` from primary sidebar.
- Keep direct routes addressable. Prefer route-level `replace` redirects for
  list routes to Settings tabs.
- Preserve desktop-specific runtime page behavior and extra Settings tabs.

Acceptance criteria:

- Settings middle navigation order is `Account`, `Configure`, `Workspace`.
- Configure contains `Runtimes`, `Workflows`, and `Skills`.
- Sidebar has only one Settings entry at the fixed bottom.
- Direct list URLs for Runtimes/Workflows/Skills do not 404.
- Existing detail URLs still work or route to an equivalent detail surface.
- Desktop daemon/update tabs remain account-level extras unless a separate
  product decision changes them.

Focused tests:

- Settings page renders the Configure group.
- `?tab=runtimes`, `?tab=workflows`, and `?tab=skills` select the right tab.
- Old list routes resolve to the new Settings tab behavior.
- Desktop routes still preserve runtime detail and skill detail navigation.

Likely files:

- `packages/views/settings/components/settings-page.tsx`
- `packages/views/settings/components/index.ts`
- `packages/views/locales/en/settings.json`
- `packages/views/locales/zh-Hans/settings.json`
- `packages/core/paths/paths.ts`
- `apps/web/app/[workspaceSlug]/(dashboard)/runtimes/page.tsx`
- `apps/web/app/[workspaceSlug]/(dashboard)/workflows/page.tsx`
- `apps/web/app/[workspaceSlug]/(dashboard)/skills/page.tsx`
- `apps/desktop/src/renderer/src/routes.tsx`

## Sequencing Recommendation

1. Implement Slice 2 first because sidebar UI depends on the corrected data
   shape and pagination contract.
2. Implement Slice 3 next to land the visible sidebar IA.
3. Implement Slice 4 after Sidebar IA so Settings owns Configure discovery.
4. Implement Slice 1 independently if a separate Trellis lane is available;
   it touches Chat Issues and proposal review rather than global navigation.

## Verification Gate

Minimum verification before Trellis marks the work complete:

```bash
pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx
pnpm --filter @multica/views exec vitest run chat
pnpm --filter @multica/core exec vitest run api
pnpm typecheck
pnpm test
make test
```

If a command target does not match the final file layout, run the closest
focused tests plus the full project command listed above.

Manual QA:

- Long Projects list scrolls while Settings remains fixed.
- Long Recents list scrolls while Settings remains fixed.
- Recents initial list shows last five days only.
- Recents "Show more" appends older loose chats in pages of 10.
- Project-associated chats never appear in Recents.
- Each expanded Project shows at most three recent chats plus a path to full
  Project chat history.
- Visiting old Runtimes/Workflows/Skills URLs still lands on a working
  surface.
- Chat Issues shows `Proposed` before `Backlog`; approval creates backlog
  issues and removes proposal cards.

## Non-Goals

- Do not add a real `proposed` issue status.
- Do not make Project-associated chats shared.
- Do not move Project-associated chat history into Recents.
- Do not remove direct Runtimes/Workflows/Skills URLs.
- Do not add dependencies for sidebar grouping, pagination, or Kanban.
