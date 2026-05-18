# Project-associated Chat Sessions

## Handoff

This Trellis task owns the next development planning and implementation for the page-level Chat Session redesign discussed on 2026-05-18.

Use the new worktree:

```text
/Users/troy.huang/workspace/AI/multica-project-associated-chat-sessions
```

Branch:

```text
trellis/project-associated-chat-sessions
```

Base branch:

```text
trellis/repository-start-modes
```

## Source Documents

The complete product and architecture handoff is in these documents:

- `CONTEXT.md`
- `docs/adr/0002-private-project-associated-chat-sessions-and-issue-proposals.md`
- `docs/project-associated-chat-sessions-plan.md`
- `.trellis/tasks/05-18-project-associated-chat-sessions/sidebar-proposed-settings-plan.md`

Treat `docs/project-associated-chat-sessions-plan.md` as the implementation plan. Treat the ADR as the durable architectural decision record. Treat `CONTEXT.md` as the glossary and domain boundary source of truth.
Treat `sidebar-proposed-settings-plan.md` as the post-QA handoff for Chat Issues `Proposed`, sidebar IA, Recents pagination, and Settings Configure relocation.

## Scope

Implement a page-level private Chat Session experience with optional Project association.

Major work areas:

- replace the global Chat FAB/floating window with workspace routes
- add Project-associated Chat Session data model and sidebar navigation
- add Project detail `Issues` / `Chats` tabs
- add Chat Session `Chat` / `Issues` / `Outputs` tabs
- present pending Chat Issue Proposal Items in a Chat Session `Issues` `Proposed` lane before Backlog without adding an Issue status
- update sidebar IA with flat primary navigation, Projects, Recents, fixed bottom Settings, and inline loose-chat pagination
- move Runtimes, Workflows, and Skills discovery into Settings under Configure while preserving direct URLs
- let agents submit structured issue proposals through task completion metadata
- let users edit and approve proposal items to create backlog issues atomically
- aggregate output metadata from the chat task and from issues created by that chat

## Non-Goals

- no shared Project team chat
- no chat move between Projects
- no agent switching after session creation
- no backend inference of issues from prose
- no proposal artifacts in the Outputs tab
- no new dependencies unless explicitly approved

## Required Planning Steps

Before implementation, split the plan into Trellis-sized development slices. A likely split is:

1. Backend schema and read APIs.
2. Route-based Chat shell and sidebar navigation.
3. Functional migration from floating Chat to page Chat.
4. Structured output handoff for summary title and issue proposals.
5. Proposal approval and Issues tab.
6. Outputs tab aggregation.

For each slice, define acceptance criteria and the focused test set before coding.

## Context Notes

Important current-code constraints from the design session:

- Existing chat is private and creator-owned.
- Existing Project deletion is hard delete; `issue.project_id` is `ON DELETE SET NULL`.
- Existing chat deletion is hard-delete oriented and must change before durable issue/output provenance depends on chat rows.
- Existing output metadata upload currently uses `.multica/outputs.json` plus a separate daemon `/outputs` endpoint; the target architecture is structured outputs in one task completion payload, with compatibility during migration.
- Route-based Chat state should come from URL params and React Query, not from floating-panel Zustand state.

## Verification Baseline

Expected verification after implementation:

```bash
make test
pnpm typecheck
pnpm test
```

Add focused backend, route, sidebar, proposal approval, and output aggregation tests as described in the implementation plan.
