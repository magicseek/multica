# Polish chat message identity UI and Plan composer footer

## Goal

Make chat feel like a coherent collaborative surface by showing sender identity on every message, showing directed recipient context where available, and moving Plan mode controls into the composer footer.

## Requirements

* Render human, agent, and agent-to-agent messages as unified left-aligned chat rows.
* Show avatar and display name for every message sender.
* Show directed recipient context from graph metadata, using `Sender -> Recipient` semantics.
* Render legacy undirected messages without recipient headers.
* Show routing warnings compactly in the transcript.
* Add a `ChatInput` footer/bottom slot for Plan mode controls.
* Move `ChatComposerPlanControls` from top-slot styling to an internal composer footer row.
* Keep Plan Run cancel/continue and engine/target state visible without crowding the editor.
* Add UI precheck for ambiguous no-target continuation where graph metadata exposes candidates.

## Acceptance Criteria

* [ ] Current user messages show the signed-in user's avatar/name and are not anonymous right-side bubbles.
* [ ] Agent messages show agent avatar/name consistently.
* [ ] Directed messages render recipient context from metadata, not parsed markdown.
* [ ] Legacy messages render safely without recipient context.
* [ ] Plan controls appear attached to the bottom edge of the input box.
* [ ] Expanded Plan controls remain visually connected to the composer.
* [ ] Ambiguous no-target UI prompts the user to choose/mention a target before sending.
* [ ] Targeted Vitest tests cover row rendering and composer footer placement.

## Suggested Files

* `packages/views/chat/components/chat-input.tsx`
* `packages/views/chat/components/chat-pages.tsx`
* `packages/views/chat/components/chat-message-list.tsx`
* `packages/views/common/actor-avatar.tsx`
* `packages/core/types/chat.ts`
* `packages/views/chat/components/*.test.tsx`

## Test Plan

* Vitest tests for human sender identity, agent sender identity, directed recipient header, legacy undirected fallback, routing warning display, Plan footer placement, and ambiguity precheck.
* Manual visual check in web and desktop dev app after implementation.

## Dependencies

Depends on the graph API shape from `05-23-chat-directed-graph-storage-api`. Can begin with compatibility fixtures if backend is not complete.
