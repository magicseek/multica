# Fix Bundle Transcript Status and Request Efficient Help

## Problem

Two request-efficient bundle UI details are confusing:

- The transcript inserts a `Bundle item N` divider at the start of a bundle item segment, but the divider also renders the item's final status. This makes a completed item look completed before its events run.
- The agent detail inspector exposes a `Request efficient` toggle without an inline explanation, so users cannot tell that it enables task bundles for request-priced providers.

## Requirements

- Bundle item transcript dividers must represent the start of an item segment, not its final checkpoint outcome.
- The divider should not show final statuses such as `Completed`/`Done` at the segment start.
- The transcript header should use a user-facing bundle item count instead of reusing issue selection copy.
- The agent detail inspector must show an info icon next to `Request efficient` with a concise tooltip explaining the toggle.
- English and Chinese locale bundles must remain in parity.

## Acceptance Criteria

- Transcript tests prove bundle dividers show `Started` and do not show `Done`/`Current item` as the divider detail.
- Agent inspector tests prove the request-efficient tooltip text is rendered.
- Typecheck, targeted tests, locale parity, and lint pass.

