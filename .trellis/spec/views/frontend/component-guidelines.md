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
