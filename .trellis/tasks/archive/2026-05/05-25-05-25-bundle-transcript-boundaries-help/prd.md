# Fix Bundle Transcript Boundaries and Request Efficient Help

## Problem

The request-efficient bundle transcript now marks each bundle item as started, but it does not show where completed items end. That makes the timeline incomplete and still forces users to infer completion from raw checkpoint command output. The transcript also does not visibly explain why two bundle items prove request efficiency: they are both events inside one `agent_task_queue` task/provider execution.

The Agent detail inspector uses an info icon for Request efficient mode, but the explanation is not reliably visible on click or hover in the desktop app.

## Requirements

- Bundle transcripts must show both item start and item completion/end boundaries when checkpoint data exists.
- Completion/end markers must render after the checkpoint command/result that completed that item, not at the start of the item.
- Transcript metadata must make the single provider execution evidence visible in user-facing language without exposing raw UUIDs.
- The Request efficient info icon must reveal the explanation on click and be discoverable on hover/focus.
- Existing English and Chinese locale parity must be preserved.

## Acceptance Criteria

- Transcript component tests prove item completion appears after checkpoint output and before the next item starts.
- Transcript component tests prove the header displays one provider run for a task bundle.
- Agent inspector tests prove the help text is rendered in a user-visible disclosure, not only an inaccessible tooltip node.
- Typecheck, targeted tests, locale parity, lint, and diff check pass.
