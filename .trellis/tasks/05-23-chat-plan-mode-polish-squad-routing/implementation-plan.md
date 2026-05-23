# Implementation Plan: Chat Plan Mode polish and squad routing

## Objective

Deliver a chat-level directed conversation graph that supports ordinary `@agent` / `@squad` routing, Plan mode squad lead orchestration, visible multi-agent exchanges, Slack-like message identity rows, Plan composer footer controls, and approval-gated proposal issue creation.

## Delivery Strategy

Implement in small, reviewable slices. The directed graph storage/API foundation should land first because server routing and UI rendering both depend on it. Routing should land before UI relies on recipient metadata. Proposal drafts can be implemented after the graph exists, then the UI can expose the complete experience. Final verification should validate the full flow across backend, frontend, migration compatibility, and desktop/web behavior.

## Child Tasks

1. `05-23-chat-directed-graph-storage-api`
   - Adds first-class directed graph persistence, sqlc queries, API response fields, and frontend parsing/types.
   - Dependency: none.

2. `05-23-chat-directed-routing-squad-orchestration`
   - Uses graph edges for mention routing, no-mention continuation, ambiguity blocking, squad lead-first flow, helper replies, out-of-scope warnings, and the 5-wave consultation limit.
   - Dependency: graph storage/API.

3. `05-23-plan-proposal-issue-drafts-approval`
   - Persists Plan output as proposal issue drafts and creates real issues only after explicit approval.
   - Dependency: graph storage/API; can be developed after or alongside routing once Plan Run state is stable.

4. `05-23-chat-message-identity-plan-footer-ui`
   - Moves Plan controls to a composer footer row and renders all chat messages as left-aligned identity rows with directed recipient headers.
   - Dependency: graph API shape; can use compatibility fallbacks while backend is still in progress.

5. `05-23-chat-plan-graph-e2e-verification`
   - Runs final regression, integration, migration, and visual/manual verification.
   - Dependency: all implementation tasks.

## Suggested Backend Contract

### Directed graph data model

Introduce explicit graph storage instead of deriving routing from message text:

* Message sender identity: enough metadata to render a human, agent, or squad-originated message.
* Recipient edges: one row per intended recipient, linked to the source message.
* Resolved task recipient: preserve original recipient (`agent` or `squad`) separately from resolved agent recipient (for example, squad lead).
* Thread/continuation state: records the active directed recipient or ambiguous multi-recipient state for no-mention human follow-up.
* Routing warning state: records skipped unauthorized mentions, malformed mentions, missing lead mentions, or ambiguity failures.
* Consultation wave count: authoritative server-side count for Plan Run agent-to-agent waves.

Legacy chat messages should remain readable as undirected messages. Do not infer historical recipient edges from old text.

### Routing semantics

* User-authored explicit `@agent` / `@squad` mentions create recipient edges and enqueue one task per resolved agent recipient.
* User-authored `@squad` routes first to the squad lead while preserving the squad as the displayed recipient.
* No-mention human follow-up routes to the active directed thread recipient.
* If multiple recipients are active after parallel replies, no-mention follow-up is blocked with structured `needs_target` metadata.
* Agent-authored visible mentions are authoritative routing commands.
* Agent-authored mentions outside allowed Plan Run / squad scope preserve the visible message but do not enqueue tasks.
* In Plan squad runs, helper replies must explicitly mention the lead to return control.
* Each Plan Run allows at most 5 lead-to-helper-to-lead consultation waves before forcing lead summary handoff.

### Proposal issue semantics

Plan mode creates proposal issue drafts, not real issues. Approval creates real issue records in a single explicit user action. Agent/squad turns can update proposal drafts, but cannot directly create workspace issues.

## Suggested Frontend Contract

* Extend chat message types with sender actor, recipient actors, routing warnings, and optional proposal draft metadata.
* Parse API responses with schema/fallbacks in `packages/core`; do not use bare casts for new response bodies.
* Render all chat messages as left-aligned rows:
  - Avatar + sender name header.
  - Optional `Sender -> Recipient` directed context from graph edges.
  - Message content below.
  - Routing warnings as compact inline states, not generic toasts only.
* Move Plan mode controls from `ChatInput.topSlot` into a bottom/footer slot inside the composer.
* Add UI precheck for ambiguous no-target sends, with server rejection as the authority.

## Verification Plan

Backend:

* Migration and sqlc generation succeed.
* Go tests cover graph writes, explicit mention fan-out, squad lead resolution, no-mention continuation, ambiguous blocking, unauthorized agent mention warning, missing lead mention handling, 5-wave limit, and proposal approval.

Frontend:

* Vitest tests cover message identity rows for human/agent/legacy messages, recipient header rendering, routing warning display, Plan composer footer placement, ambiguous target precheck, and proposal draft approval UI.
* Typecheck succeeds after API/type changes.

End-to-end/manual:

* Start a Plan Run with a squad and verify lead-first response.
* Verify lead can mention helpers, helpers mention lead back, and the lead resumes.
* Verify no-mention human follow-up in a Plan squad run routes to the lead.
* Verify multi-recipient ordinary chat blocks ambiguous no-target continuation.
* Verify proposal issue drafts do not create issues before approval and do create issues after approval.

## Risk Controls

* Keep graph storage additive and backward compatible.
* Do not remove existing Plan Run fields until graph-backed flow is proven.
* Gate new route behavior with server-side validation so older clients cannot bypass ambiguity or scope rules.
* Use tests to lock routing semantics before polishing UI.
