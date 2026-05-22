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

- Long labels in auto-layout tables can overlap status badges or metadata
  columns. Treat truncation as a table-level layout decision, not just a text
  utility on the child node.
