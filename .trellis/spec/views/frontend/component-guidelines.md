# Component Guidelines

> How components are built in this project.

---

## Overview

<!--
Document your project's component conventions here.

Questions to answer:
- What component patterns do you use?
- How are props defined?
- How do you handle composition?
- What accessibility standards apply?
-->

(To be filled by the team)

---

## Component Structure

<!-- Standard structure of a component file -->

(To be filled by the team)

---

## Props Conventions

<!-- How props should be defined and typed -->

(To be filled by the team)

---

## Styling Patterns

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

### Header Segmented Tabs

Header-level navigation tabs are a visual contract, not just content tabs.
When a `PageHeader` owns a route or panel switcher, use the shared
`HeaderTabs` component from `packages/views/common/header-tabs.tsx` instead
of hand-assembling `TabsList` / `TabsTrigger` controls in the feature file.

Required pattern for Project, Chat, and similar dense app headers:

- Keep tab items equal width with a grid layout so label length and count
  badges do not shift neighboring tabs.
- Use one sliding active indicator with `transition-transform`; do not rely
  only on swapping individual trigger backgrounds.
- Keep labels truncated inside `min-w-0` tab buttons. Counts may be shown as
  badges, but they must not participate in width decisions.
- Give the tablist a localized `aria-label` and keep `role="tab"` /
  `aria-selected` semantics intact.
- Allow the header to wrap on narrow widths. The segmented control can become
  full-width below the desktop breakpoint, but it should not squeeze the
  title/breadcrumb until text overlaps.

### Dense Tables and Long Text

When a table contains user- or agent-generated text, column containment is part
of the component contract. Do not rely on `max-w-*` alone inside the default
auto table layout: long labels can still influence column sizing or paint over
neighboring cells.

Required pattern for dense shared tables:

- Use `table-fixed` when rows include potentially long titles, prompts, file
  names, or source labels.
- Define column widths at the table level (`colgroup`, header widths, or an
  existing data-table sizing API) rather than only on individual cells.
- Put truncating text inside a block-level child with
  `block min-w-0 max-w-full truncate`; `truncate` on an inline link/span is not
  sufficient.
- Add `overflow-hidden` to cells that own long text so decorative hover states
  and links cannot paint into the next column.
- Keep the table inside an `overflow-x-auto` container when the fixed minimum
  width is wider than the available panel.

### Streaming Chat Rows

In-flight agent output is a chat message surface, not a loader-only surface.
When a pending task streams timeline content before the final persisted
`chat_message` exists, render it with the same actor row used by persisted
assistant messages.

Required pattern:

- Keep avatar, display name, provider/runtime icon, and timestamp layout
  consistent between live pending rows and persisted message rows.
- Resolve actor identity from the pending task payload first. Use the session
  agent only as a legacy single-agent fallback, never as the source of truth
  for squad consultation tasks.
- Render the pending stage line (`Queued`, `Thinking`, tool activity, elapsed
  timer) inside the same live row, directly under the actor header. It should
  never appear as a detached full-width row below the message.
- Put the pending stage line inside the same body stack used for normal
  assistant content so header-to-body spacing matches persisted messages.
- Render the live row as soon as a pending task exists, even when the timeline
  stream is still empty. The initial no-output state should still show
  avatar, name, and stage.
- Place live timeline content inside the message row body so spacing, borders,
  and hover affordances do not shift when the final message arrives.
- Render message-scoped addons such as final Plan summaries, proposal cards,
  workflow review prompts, or generated artifact previews inside the owning
  message row body. They must align under the sender name and avatar, not
  become separate top-level rows in the message list.
- Add a component regression that seeds a pending task and proves the running
  agent identity and stage are visible before any final message or timeline
  output is present.
- Add a component regression that attaches an addon to an assistant message and
  proves the addon is inside that message row.

### Issue Comment Threads

Issue comments are thread rows with a stable avatar/name gutter. Root comments
have a collapse affordance before the avatar; nested replies must reserve the
same leading gutter so an agent-to-agent reply does not jump left relative to
the thread header.

Required pattern:

- Render root comment headers and nested reply headers with the same visual
  avatar start position. If the root row has a disclosure control, replies need
  an invisible spacer of the same width.
- Keep reply body, attachments, and reaction rows aligned with the same
  content column used by the root comment body.
- Do not special-case agent-authored replies into a denser layout. Agent and
  member replies share the same row geometry; only actor identity changes.
- Add a component regression for an agent-authored root comment that mentions
  another agent and an agent-authored nested reply, proving the reply row keeps
  the thread-header gutter.

---

## Accessibility

<!-- A11y requirements and patterns -->

(To be filled by the team)

---

## Common Mistakes

<!-- Component-related mistakes your team has made -->

- Reusing generic `TabsList` inside a header creates uneven tab widths and no
  route-switching motion. Header tabs need a shared segmented-control surface
  so Project and Chat stay visually consistent.
- Long labels in auto-layout tables can overlap status badges or metadata
  columns. Treat truncation as a table-level layout decision, not just a text
  utility on the child node.
- Nested issue replies without the root comment gutter make agent-to-agent
  handoff replies look detached from the thread. Preserve the gutter even when
  the reply itself has no disclosure control.
