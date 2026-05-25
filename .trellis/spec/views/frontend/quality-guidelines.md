# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

### Runtime Sidebar Summaries

Issue-detail sidebar sections that summarize runtime state, task bundles, or
agent executions must separate read-only history from action controls. Existing
runs should render a compact status summary first; creation or rerun controls
should appear only when the current issue is eligible for that action.

User-facing labels in these sections must be derived from stable product
concepts such as issue identifier/title, agent display name, status label, and
progress count. Do not show raw backend identifiers, runtime IDs, bundle IDs,
output namespaces, or implementation limits as primary labels. If an internal
limit affects a disabled state, explain the user action in plain language
instead of rendering `count/max` style diagnostics.

When a completed or otherwise non-startable issue has historical runtime data,
the section should remain read-only unless there is a concrete rerun affordance
for a failed or blocked run.

### Transcript Timeline Semantics

Timeline and transcript dividers must describe the event boundary where they
are rendered. If a divider is inserted before the first event for a task bundle
item, it should say that the item started; it must not display the item's final
checkpoint status (`completed`, `blocked`, `done`, etc.) at that start
position. Final outcomes need an end-boundary marker or summary after the
checkpoint/result event so users can see both where the item began and where it
finished.

Do not reuse selection-copy such as `N selected` for historical transcript
metadata. Bundle transcripts should say how many bundle items exist; creation
forms should say how many items are selected.

Request-efficient bundle transcripts must also make the single execution
contract visible. If multiple bundle items appear in one task transcript, the
header should show that the transcript represents one provider request/run
rather than making users infer it from raw task IDs or checkpoint commands.

### Inspector Toggle Help

Agent settings toggles that change execution behavior need an inline help
affordance next to the control. The help must be reachable by click and
discoverable by hover or focus, and it must explain the user-visible effect of
the toggle instead of restating the label. This is required for
request-efficient mode because it changes how issue tasks are grouped into
provider requests/runs and affects billing behavior for request-priced
providers.

The affordance belongs in the property row, but the explanatory panel must not
participate in the inspector grid layout. Use a floating/portaled disclosure
for icon-only inspector help so opening the copy does not resize, wrap, or
misalign the surrounding property rows.

---

## Testing Requirements

<!-- What level of testing is expected -->

- Runtime sidebar regressions need component tests that cover both the action
  path and the read-only history path. Tests should assert that raw internal
  identifiers are not visible and that completed/non-startable issues do not
  render stale creation controls.
- Transcript regressions need component tests that prove time-positioned
  dividers do not display future/final state at the start of a segment, and
  that completion/end markers render after the checkpoint output that ends the
  segment.
- Request-efficient transcript regressions should assert that the single
  provider request/run evidence is visible in the transcript header.
- Inspector behavior toggles need component tests for their help affordance so
  the explanatory copy is not accidentally removed during layout changes. For
  icon-only help, tests should exercise the disclosure path instead of only
  asserting an accessible name, and should prove the help panel is outside the
  inspector row/sidebar layout when it opens.

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
