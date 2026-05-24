# Fix Task Bundle Sidebar Summary

## Problem

The issue detail sidebar task bundle section mixes two different concepts:

- historical bundle run status for the current issue
- controls for creating a new bundle run

After a bundle has completed, the sidebar still renders the creation form. This exposes internal UUID values, shows the current issue as checked for a new bundle even though the prior bundle is complete, and displays the internal max-item guardrail as `1/5 selected`.

## Requirements

- Existing bundle runs render as a read-only, compact status summary.
- Completed bundle pages do not show new-bundle checkboxes unless the current issue is actually eligible for a new bundle run.
- The UI must never expose raw bundle IDs, agent IDs, runtime IDs, or output namespaces as primary labels.
- Selection count must not expose the max bundle size as `count/max`; if shown, it should be user-facing and concise.
- New bundle controls remain available for eligible request-efficient agents on startable issues.
- Failed or blocked bundle runs can still be rerun from the summary.

## Acceptance Criteria

- A completed bundle with two items shows a readable `Completed 2/2` style summary and item labels based on issue identifier/title.
- The completed bundle summary does not render raw UUID text.
- The completed bundle summary does not render the `Start bundle` action or selection checkboxes when the current issue is not startable.
- The create flow for eligible todo issues still calls `createTaskBundle` with the selected issue IDs.
- Tests cover the completed read-only case and the create case.

