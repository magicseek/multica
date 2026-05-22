# Polish project/chat tabs and first-message titles

## Goal

Fix two project-associated chat UX regressions:

1. Project and Chat header tabs should feel like a deliberate segmented control: equal-width items, stable label positions, and a smooth active-indicator animation.
2. Project chat sessions should derive their title from the first user message in that chat, not reuse a stale/project-level title across multiple sessions.

## Requirements

* Replace ad hoc header `TabsList` usage in Project detail and Chat session headers with a reusable, equal-width header tab switcher.
* Preserve existing tab routing and active tab behavior for Project (`issues`, `chats`, `analytics`) and Chat (`chat`, `issues`, `outputs`, `analytics`).
* Keep the control compact enough for the current desktop header and responsive on narrow widths.
* Counts for Issues/Outputs remain visible without changing tab width.
* The active tab should animate via a sliding indicator rather than just swapping button backgrounds.
* Chat title generation must be server-authoritative on first user message so a stale client-created title can be corrected.
* User-renamed chat titles must not be overwritten.
* Later messages must not keep changing the title after the first user message.

## Acceptance Criteria

* [x] Project detail header tabs are equal width with animated active indicator.
* [x] Chat session header tabs are equal width with animated active indicator.
* [x] A project chat session created with a stale title is renamed from the first actual user message on first send.
* [x] User-renamed titles are preserved.
* [x] Later messages do not retitle an existing first-message-titled session.
* [x] Focused backend tests and frontend typecheck/lint pass.

## Out of Scope

* Redesigning page content inside Issues/Chats/Outputs/Analytics tabs.
* Adding LLM-based title summarization.
* Changing sidebar recents ranking.
