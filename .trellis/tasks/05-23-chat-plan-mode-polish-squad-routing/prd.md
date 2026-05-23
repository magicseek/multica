# Chat Plan Mode polish and squad routing

## Goal

Refine Chat Plan mode so it feels like a normal collaborative chat surface and so squad-based planning follows the intended lead-agent coordination model. The work should improve Plan mode control placement, show clear speaker identity on every message, and make `@squad` plan runs route through the squad lead before involving other squad agents.

## What I already know

* The current branch is `feat/chat-plan-runs`.
* The user wants the Plan mode settings bar moved to the lower edge of the chat input box.
* Plan mode controls should live as a footer row inside the chat input box rather than as a floating panel or a top slot.
* Agent replies in chat need visible avatar/name metadata, like Slack-style chat rows.
* Human messages should use the signed-in user's account identity.
* In Plan mode, `@squad` should first wake the squad lead. The lead should reason, then selectively mention other squad agents for consultation.
* Consulted agents must explicitly `@mention` the lead in their replies so the lead receives and processes the response.
* Human replies during an active squad plan run should be routed to the lead even when the human does not explicitly mention anyone.
* Plan mode should generate proposal issues as approval-gated drafts, not create real issues automatically.
* Initial code search found Chat Plan Run storage and APIs in `server/internal/handler/chat.go`, `server/internal/handler/chat_plan_runs.go`, `server/internal/handler/chat_structured_outputs.go`, and `server/migrations/105_chat_plan_runs.up.sql`.
* Initial code search found client Plan Run plumbing in `packages/core/chat/*` and shared chat UI in `packages/views/chat/*`.
* Existing issue/comment squad routing has a separate `squadOperatingProtocol` in `server/internal/handler/squad_briefing.go`; plan-mode squad routing may need an equivalent or shared rule set.
* `ChatInput` currently renders its `topSlot` inside the rounded input container above the editor. `ChatComposerPlanControls` is passed as that `topSlot` from both new-chat and session-chat pages, which is why the Plan bar appears at the top of the input box today.
* `ChatInput` already has bottom-left and bottom-right absolute adornment zones (`leftAdornment`, `rightAdornment`, upload, submit). Moving Plan mode to the lower edge likely means adding a dedicated bottom metadata/control row inside the input card rather than reusing the current top slot.
* `ChatMessageList` currently renders user messages as right-aligned bubbles with no visible identity row. Assistant messages render full-width; only plan consultation replies show a helper label such as "API Reviewer consulted".
* `ChatMessage` already includes `author_type`, `author_agent_id`, `plan_run_id`, and `consultation_id`. It does not include a member author id, so human identity is currently derivable from the current logged-in user and/or the chat session creator, not from the message row itself.
* Shared avatar/name infrastructure exists via `packages/views/common/actor-avatar.tsx` and `useActorName`; it can resolve members, agents, and squads from workspace query data.
* Server Plan Run send always enqueues `EnqueuePlanLeadTask` when `mode=plan` or `plan_run_id` is present, including human continuations. For squad actors, `resolvePlanActorForSend` stores the squad as actor and resolves `lead_agent_id` to the squad leader.
* Existing consultation machinery only creates helper tasks from agent mentions in the lead's assistant output, only if the plan run actor is a squad, and only when there are no existing consultations for that run. Helper replies are marked successful only if the helper output mentions the lead.
* Current tests verify that squad plan claims include squad helper context and lead mention, but they do not yet verify end-to-end consultation creation, helper lead-mention response handling, or human follow-up lead continuation behavior.

## Assumptions (temporary)

* Plan mode controls should remain inside the input surface, but attached to the bottom edge so the composer reads as one coherent control.
* Human chat identity can initially use the authenticated user for the current user's own messages; if future multi-human chat is needed, `chat_message` will need a member author id.
* The current squad plan flow is partially implemented, but it is under-enforced: it depends on the lead's generated output containing exact mention markdown, only allows one consultation wave per plan run, and lacks tests for the full closed loop.

## Open Questions


## Requirements (evolving)

* Move the Plan mode controls to the bottom edge of the chat input box while preserving existing Plan mode selection/cancel behavior.
* Keep Plan mode controls visually integrated with the composer as an internal footer row that shows current mode/engine/target state.
* Render every agent reply with avatar and display name.
* Render every human reply with the logged-in user's avatar/name.
* Render chat messages as unified left-aligned Slack-like rows rather than mixing anonymous right-side human bubbles with agent rows.
* Render agent-to-agent exchanges as visible chat messages, not hidden background state.
* In Plan mode with a squad target, the lead agent is the primary recipient and coordinator.
* Squad member replies must make the lead the explicit next recipient.
* Agent-to-agent routing is triggered by visible `@agent` / `@squad` mentions in the chat message body.
* Agent mentions outside the allowed Plan Run / squad recipient scope must not silently expand collaboration.
* A Plan Run should allow at most 5 lead-to-helper consultation waves before forcing a lead summary back to the human.
* Legacy chat messages should not have directed recipient graph inferred from historical text.
* Human replies in an active squad Plan Run should be visible to and actionable by the lead even without a new `@mention`.
* Proposal issues produced by Plan mode remain drafts until the user approves them.
* Consultation flow should be testable without relying on model behavior alone.

## Acceptance Criteria (evolving)

* [ ] Plan mode controls are visually attached to the chat input lower edge and match the existing Multica design language.
* [ ] The composer footer row consistently shows Plan mode state and expands configuration near the input lower edge without becoming an unrelated floating layer.
* [ ] Chat messages from agents show the agent avatar and name.
* [ ] Chat messages from humans show the signed-in user's avatar/name.
* [ ] Human, agent, and agent-to-agent messages use the same left-aligned row structure with avatar/name header and content below.
* [ ] Agent-to-agent exchanges are visible in chat with enough directed-recipient context to understand who was addressing whom.
* [ ] A Plan Run started with `@squad` creates/continues work through the squad lead first.
* [ ] A lead can consult other squad agents and receive their responses through explicit lead mentions.
* [ ] A Plan Run stops automatic agent-to-agent consultation after 5 lead→helper→lead waves and asks the lead to summarize the current consensus for the human.
* [ ] Proposal issues generated by Plan mode are reviewable drafts and only become real issues after explicit user approval.
* [ ] A visible agent `@mention` creates graph recipient edges and task routing without requiring hidden model-emitted routing metadata.
* [ ] Out-of-scope agent mentions preserve the visible message but reject/skip the unauthorized routing edge with a clear warning state.
* [ ] Legacy chat messages remain readable after migration as undirected messages with sender identity where available.
* [ ] A human follow-up in an active squad Plan Run continues to the lead without requiring the human to mention the lead again.
* [ ] Unit or integration tests cover the routing semantics; UI tests cover the visual metadata and Plan mode control placement.

## Definition of Done (team quality bar)

* Tests added/updated for server routing and shared chat UI behavior.
* Lint, typecheck, SQL generation if needed, relevant Go tests, relevant Vitest tests, and final worktree check pass.
* Docs/spec notes updated if new Plan Run or squad routing contracts emerge.
* Rollback considered for any migration or API shape changes.

## Implementation Plan

See [`implementation-plan.md`](implementation-plan.md) for the Trellis delivery plan. The parent task is decomposed into these child tasks:

* `05-23-chat-directed-graph-storage-api` - graph persistence, sqlc/API, frontend type/parsing foundation.
* `05-23-chat-directed-routing-squad-orchestration` - directed mention routing, squad lead-first orchestration, ambiguity blocking, scoped mentions, and 5-wave limit.
* `05-23-plan-proposal-issue-drafts-approval` - approval-gated proposal issue drafts and real issue creation.
* `05-23-chat-message-identity-plan-footer-ui` - Slack-like message identity rows, recipient headers, routing warnings, and Plan composer footer.
* `05-23-chat-plan-graph-e2e-verification` - final migration, backend, frontend, manual, and regression verification.

## Out of Scope (explicit)

* Redesigning all chat layout/navigation beyond the Plan mode bar and per-message identity treatment.
* Replacing the existing squads model.
* Adding a fully general multi-agent consensus engine outside the Plan Run flow.

## Technical Notes

* UI files: `packages/views/chat/components/chat-input.tsx`, `packages/views/chat/components/chat-pages.tsx`, `packages/views/chat/components/chat-message-list.tsx`.
* Core types: `packages/core/types/chat.ts`.
* Server Plan Run send/claim: `server/internal/handler/chat.go`, `server/internal/handler/chat_plan_runs.go`.
* Server consultation lifecycle: `server/internal/service/task.go`.
* Existing squad issue/comment protocol reference: `server/internal/handler/squad_briefing.go`.
* Relevant tests: `packages/views/chat/components/chat-pages.test.tsx`, `packages/views/chat/components/chat-input.test.tsx`, `server/internal/handler/chat_plan_runs_test.go`.

## Complexity

Complex. This spans shared chat UI, existing API response consumption, server task routing, daemon prompts, and end-to-end Plan Run semantics.

## Feasible Approaches

### Approach A: UI polish + prompt-only routing

* Move the Plan controls and add identity rows.
* Strengthen daemon Plan Run instructions so leads mention helpers and helpers mention leads.
* Pros: smallest backend change.
* Cons: model-dependent; the failure the user observed can still happen when an agent omits exact mention markdown.

### Approach B: UI polish + backend-enforced lead orchestration (Recommended)

* Move the Plan controls and add identity rows.
* Keep mention-based helper selection, but make the backend enforce the loop: validate lead/helper roles, mark missing lead mentions visibly, resume lead after consultation close, and allow additional bounded consultation waves when the lead asks new helpers later.
* Add explicit tests for lead-first creation, helper task creation from lead mention, helper response requiring lead mention, and human follow-up continuing to the lead.
* Pros: matches product expectation and makes the behavior testable.
* Cons: more server work; needs careful loop guards to avoid runaway agent-to-agent chatter.

### Approach C: Full explicit multi-agent conversation graph

* Model plan participants, directed messages, and routing edges as first-class data rather than deriving consultation tasks from mention markdown.
* Pros: strongest long-term foundation.
* Cons: likely too broad for this fix; risks turning the task into a new orchestration subsystem.

## Decision (ADR-lite)

**Context**: The observed squad Plan mode behavior is not reliable enough when routing is inferred from free-form model output. The product expectation is that lead-agent coordination, helper replies, and human follow-ups are durable conversation semantics, not best-effort prompt behavior.

**Decision**: Build toward Approach C: a first-class multi-agent conversation graph for Plan Runs. Plan participants, directed exchanges, lead/helper roles, and continuation routing should be represented explicitly enough that the server can decide who should act next and the UI can show who is speaking to whom.

**Consequences**: This is larger than a UI polish pass. The task should preserve the immediate UI fixes, but the backend scope expands from mention-derived consultation tasks into an explicit graph/routing model with tests. To keep delivery practical, the MVP needs a tight boundary around which graph capabilities ship now versus later.

### MVP Boundary Decision

**Context**: The graph could be scoped narrowly to Plan Runs, broadly to all chat, or platform-wide across chat/issues/comments.

**Decision**: The MVP should implement a Chat-level directed conversation graph. It covers ordinary chat sessions and Plan mode sessions with the same graph primitives, but does not yet absorb issue comments or every workspace collaboration surface.

**Consequences**:

* Chat messages should gain explicit directed conversation metadata: author, intended recipients, relationship to prior message(s), and routing edges.
* Plan mode can use the same graph to represent lead-agent turns, helper consultations, helper-to-lead replies, and human-to-lead follow-ups.
* Ordinary chat uses the same graph for `@agent` / `@squad` interactions.
* Issue comments and existing issue-assignment squad protocols remain out of scope unless needed as compatibility references.

### Ordinary Chat Mention Routing Decision

**Context**: Ordinary chat already has a primary session agent, but a directed conversation graph makes explicit `@agent` and `@squad` mentions meaningful routing commands rather than plain text context.

**Decision**: In ordinary chat, an explicit `@agent` / `@squad` mention is a directed recipient. The server should enqueue replies for mentioned agents; `@squad` routes first to the squad lead. The session primary agent should not automatically remain the sole responder when the user explicitly directs the message elsewhere.

**Consequences**:

* Chat send should parse mention links and create graph recipient edges.
* Mentioned agents become task recipients for that message.
* Mentioned squads resolve to their lead agent as the first task recipient while preserving the squad recipient edge for UI and future delegation context.
* Messages without explicit mentions still need a deterministic default routing rule.

### No-Mention Continuation Decision

**Context**: After a chat has multiple participating agents, requiring the human to re-mention the correct agent every turn would make Plan mode and multi-agent chat brittle.

**Decision**: No-mention human replies use context-aware continuation. The server routes them to the active recipient of the current directed thread. In a Plan squad run, that means the lead agent by default. In ordinary chat, that means the most recent directly addressed agent/thread recipient. If there is no directed context, route to the session primary agent.

**Consequences**:

* The graph needs a notion of current active recipient or active directed thread for a chat session.
* Plan squad helper replies should not accidentally make the helper the new default human recipient; helper-to-lead replies return control to the lead.
* The UI can still allow the human to override context by explicitly mentioning a different agent or squad.

### Agent-to-Agent Visibility Decision

**Context**: Multi-agent planning is only useful to the human if the collaboration is inspectable. Hiding helper exchanges would make consensus feel like a black box and make routing failures harder to diagnose.

**Decision**: Agent-to-agent exchanges in chat are user-visible by default. Lead-to-helper requests, helper-to-lead replies, ordinary `@agent` replies, and squad-lead coordination should appear as normal chat messages with speaker identity and directed recipient context.

**Consequences**:

* The graph should persist visible message/edge rows for agent-to-agent turns.
* The chat UI should render every visible graph message with avatar/name metadata.
* Directed recipient information should be visible enough to explain routing, but compact enough not to overwhelm the main chat.
* Private/internal graph edges are out of scope for this MVP.

### Directed Recipient UI Decision

**Context**: Once chat supports multiple directed participants, a plain bubble stream is ambiguous. The UI needs to make speaker and intended recipient clear without turning the chat into a workflow graph.

**Decision**: Use a lightweight message header line: `Avatar Sender Name -> Recipient`. Every message gets sender identity; directed messages also show the recipient target in the same header. This should use Multica's existing avatar/name components and restrained typography.

**Consequences**:

* Human messages should no longer be anonymous right-aligned bubbles only.
* Agent messages should show the agent avatar/name consistently, not only for consultation messages.
* Directed recipient labels should be derived from graph edge metadata, not parsed from the markdown body.
* The UI should avoid indentation-heavy or flowchart-like layouts for this MVP.

### First-Class Graph Storage Decision

**Context**: A chat-level directed conversation graph can either be computed from existing message/task fields or persisted explicitly. Because routing and UI identity must become durable product semantics, a derived-only model would keep the current fragility.

**Decision**: Add database migrations for first-class graph storage in this task. The implementation should introduce explicit graph tables/columns, such as message edges, recipient metadata, and current directed thread state, rather than relying only on `chat_message.plan_run_id`, `consultation_id`, and parsed mention text.

**Consequences**:

* This task must include SQL migrations, sqlc query updates, API response type changes, and compatibility parsing on the frontend.
* Graph rows should become the source of truth for who a message is addressed to and who should act next.
* Existing Plan Run consultation fields can remain as compatibility or specialized summary fields, but directed routing should move toward graph-backed semantics.
* Verification must include migration generation, Go tests around graph writes/routing, and frontend tests for graph-backed message headers.

### Multi-Recipient Fan-Out Decision

**Context**: A user can mention multiple agents or squads in one chat message. The graph needs to represent that without duplicating user-visible content.

**Decision**: Use one message with multiple recipient edges. The original user message is stored once. The graph edge table stores one directed recipient edge per mentioned recipient. The server creates one task per resolved agent recipient. Each agent response is a visible chat message linked back to its recipient edge/thread.

**Consequences**:

* Chat UI should not duplicate the user's message for each recipient.
* Edge rows need enough metadata to distinguish original recipient (`agent` or `squad`) from resolved task recipient (`agent`, e.g. squad lead).
* API responses should expose recipient labels/actors for rendering `Sender -> Recipient`.
* Task creation needs to be idempotent per message/recipient edge to avoid duplicate agent replies on retries.

### Parallel Reply Continuation Decision

**Context**: If a user sends one message to multiple recipients and more than one agent replies, a later human message without an explicit target is ambiguous. Routing by last responder would make behavior depend on timing.

**Decision**: After parallel recipient replies, a no-mention human follow-up should not automatically enqueue a task. The product should require the user to choose or mention a target before continuing that directed thread.

**Consequences**:

* The graph/thread state needs an `ambiguous` or multi-active-recipient state.
* The UI should make the ambiguity clear and guide the user to pick a recipient or mention an agent/squad.
* The server should avoid silently choosing a target when the graph has multiple active recipients.
* Plan squad lead/helper flows remain non-ambiguous because helper-to-lead replies return control to the lead.

### Ambiguous No-Target Block Decision

**Context**: Ambiguous no-mention follow-ups are a routing correctness issue, not just a UI affordance. Desktop clients can lag behind the server, and API callers can bypass client-side checks.

**Decision**: Use server-authoritative blocking with UI precheck. The UI should detect ambiguous directed-thread state and prompt the user to choose or mention a target before send. The server should also reject ambiguous no-target sends with a structured response such as `needs_target`, including candidate recipients when available.

**Consequences**:

* The API response/error contract needs to carry structured ambiguity metadata.
* Frontend send flow should render this as an actionable target-selection prompt rather than a generic toast.
* Server routing remains authoritative for old desktop builds and direct API calls.
* Tests should cover both UI precheck behavior and server rejection.

### Agent-to-Agent Routing Trigger Decision

**Context**: Agent-to-agent collaboration must be visible and debuggable. Requiring a hidden structured recipient payload from the model would make the human-visible transcript less authoritative and would duplicate routing intent across two channels.

**Decision**: Visible `@agent` / `@squad` mentions in an agent's chat output are the authoritative routing trigger. The backend should parse those mentions, write graph recipient edges, and enqueue recipient tasks from the parsed result. Structured metadata can store the parsed recipient edges, task ids, and routing state, but the model should not need to emit a separate hidden routing command for MVP correctness.

**Consequences**:

* What the user sees in chat is the same command surface the server routes from.
* Agent prompts must instruct helpers to explicitly `@mention` the lead when returning work.
* Server parsing and validation become important: malformed, unknown, or unauthorized mentions must not silently create background work.
* Tests should cover agent-visible mention parsing, edge creation, task fan-out, and helper-to-lead continuation.

### Out-of-Scope Mention Decision

**Context**: In a squad Plan Run, agents should collaborate within the selected squad unless the human explicitly changes the target. Allowing a lead or helper to mention arbitrary workspace agents would let model output expand the collaboration boundary without user intent.

**Decision**: For MVP, agent-authored mentions outside the allowed recipient scope are not routed. The chat message remains visible, but the server should not create graph recipient edges or tasks for unauthorized recipients. The UI/API should surface a clear warning state explaining that the mentioned recipient is outside the allowed scope.

**Consequences**:

* Plan squad runs remain bounded to the selected squad members and lead unless the user explicitly starts/targets a different recipient.
* Ordinary chat can still route user-authored mentions to arbitrary allowed workspace agents/squads.
* Agent-authored out-of-scope mentions become debuggable transcript events rather than silent failures.
* Tests should cover unauthorized agent mention parsing, skipped edge creation, no task enqueue, and warning rendering.

### Legacy Message Migration Decision

**Context**: Existing chat messages were not stored with first-class recipient edges. Inferring historical graph structure from plain text or old plan consultation fields would be lossy and could create misleading routing history.

**Decision**: Legacy messages should migrate as undirected chat messages. The implementation should preserve existing content and timestamps, expose sender identity where it can be known safely, and leave recipient graph edges empty unless they were created by the new routing system.

**Consequences**:

* Existing conversations remain readable without pretending old messages had precise directed recipients.
* New graph-backed routing starts at the migration boundary.
* The UI must gracefully render messages without recipient headers.
* Tests should cover mixed transcripts containing legacy undirected messages and new directed graph messages.

### Chat Message Row UI Decision

**Context**: Directed multi-agent chat needs a consistent place to show who spoke and who was addressed. The current UI mixes anonymous right-aligned human bubbles with fuller assistant rows, which breaks down once human messages and agent messages can both have explicit recipients.

**Decision**: Use a unified left-aligned Slack-like chat row for human, agent, and agent-to-agent messages. Each row shows avatar and sender display name in a compact header, with directed recipient context (`Sender -> Recipient`) when recipient edges exist. Message content renders below the header. Current-user messages should use the logged-in account avatar/name rather than an anonymous right-side bubble.

**Consequences**:

* The message list becomes easier to scan in multi-agent conversations because every row uses one identity pattern.
* Existing human bubble styling should be removed or scoped away from the directed chat view.
* Legacy undirected messages show only sender identity; new directed messages additionally show recipient targets.
* UI tests should assert sender identity for human and agent rows and recipient header rendering for directed messages.

### Plan Mode Composer Footer Decision

**Context**: The current Plan mode controls render through `ChatInput.topSlot`, which places the bar above the editor inside the rounded composer. The requested visual direction is to make Plan mode feel attached to the lower edge of the input box and consistent with the rest of Multica's composer controls.

**Decision**: Render Plan mode controls as an internal composer footer row. The footer should always present the current mode, selected planning engine, and target state when Plan mode is active. Expanded configuration should stay visually connected to the lower edge of the input, not become a detached floating panel.

**Consequences**:

* `ChatInput` likely needs a footer/bottom slot instead of reusing `topSlot` for Plan controls.
* `ChatComposerPlanControls` should switch from a top border bar to compact footer styling that coexists with upload/submit affordances.
* The design should preserve existing Plan Run cancel/continue state while avoiding vertical crowding in the text editor area.
* UI tests should assert the Plan control placement through stable labels/roles rather than fragile visual-only selectors.

### Consultation Loop Limit Decision

**Context**: Explicit agent-to-agent routing can create a useful deliberation loop, but the system needs a hard stop to avoid runaway task creation or an endless debate hidden behind normal-looking chat activity.

**Decision**: For MVP, each Plan Run can run at most 5 lead→helper→lead consultation waves. After the fifth wave, the server should stop creating new automatic helper consultation tasks for that Plan Run and instruct the lead to summarize the current consensus, unresolved disagreements, and proposed next step for the human.

**Consequences**:

* The graph/routing state needs a consultation wave counter or equivalent derivation that is authoritative on the server.
* Reaching the limit should be visible in the transcript or Plan Run state rather than failing silently.
* The human can continue by sending a new instruction or explicit target, but the same automatic loop should not continue indefinitely.
* Tests should cover wave counting, the fifth-wave boundary, no sixth automatic helper task, and lead summary handoff.

### Proposal Issue Approval Decision

**Context**: Plan mode is meant to brainstorm with the user and produce proposed implementation issues. Automatically creating real issues from an agent conversation would make exploratory planning mutate the workspace before the human has reviewed the output.

**Decision**: Plan mode outputs proposal issues as approval-gated drafts. The system should show the proposed issues for review, but no real issue records should be created until the user explicitly approves creation.

**Consequences**:

* Plan Run storage/API needs to distinguish proposal issue drafts from created workspace issues.
* The UI should make the draft/approval state clear and support approving creation as an explicit action.
* Agent/squad consensus can update the draft proposal, but cannot directly write to the issue tracker.
* Tests should cover draft creation, approval creating issues, and no issue creation before approval.
