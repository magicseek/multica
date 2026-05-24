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

---

## Testing Requirements

<!-- What level of testing is expected -->

- Runtime sidebar regressions need component tests that cover both the action
  path and the read-only history path. Tests should assert that raw internal
  identifiers are not visible and that completed/non-startable issues do not
  render stale creation controls.

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
