# Chat Plan Runs

**Created**: 2026-05-23
**Assignee**: troy.huang
**Priority**: P1
**Status**: Planning
**Branch**: `feat/chat-plan-runs`
**Base branch**: `feat/ringcentral-connectors`

## Problem

Multica Chat can already produce reviewable issue proposals, but it does not have a first-class planning mode for turning a rough idea into a refined set of proposal issues through multi-turn brainstorming. Users need a Plan mode that can run different planning protocols, keep state across several question/answer turns, and, when a Squad is selected, let the lead agent consult squad members before synthesizing the final proposal.

## Source Documents

- `CONTEXT.md`
- `docs/adr/0004-chat-plan-runs-and-squad-consensus.md`
- `docs/chat-plan-runs-plan.md`
- `docs/project-associated-chat-sessions-plan.md`
- `docs/ai-desk-flows-to-multica-workflows-plan.md`
- `~/workspace/AI/ai-symphony/docs/research/slock-multica-research-report.md`
- `~/workspace/AI/ai-symphony/docs/ai-symphony-design.md`

## Goals

- Add a stateful **Chat Plan Run** lifecycle inside existing Chat Sessions.
- Provide Plan mode in Chat with three server-owned **Plan Engine** presets: `grill_with_docs`, `brainstorming`, and `office_hours`, defaulting to `grill_with_docs`.
- Keep Plan Engine presets authoritative in the cloud and inject the chosen protocol into every claimed plan task.
- Support multi-turn planning without requiring the user to reselect Plan mode for every reply.
- Persist a lightweight **Plan Summary** with confirmed requirements, rejected options, consensus notes, and open questions.
- Continue using existing **Chat Issue Proposal** approval before creating issues.
- Support squad-backed planning where the selected squad's lead agent can consult squad members through chat mentions and then resume to synthesize **Squad Plan Consensus**.
- Keep agent-to-agent consultation bounded so missing, failed, or timed-out helper replies do not leave the Plan Run stuck indefinitely.

## Non-Goals

- Do not introduce a separate Plan Chat Session type.
- Do not require workspace Skills or local runtime Skills to exist for Plan Engines.
- Do not daemon-wide sync Plan Engine presets ahead of task claim.
- Do not introduce unbounded roundtable or free-form multi-agent chat in MVP.
- Do not auto-create issues when planning completes.
- Do not replace existing Chat Issue Proposal approval UI.
- Do not introduce full reviewable workflow artifacts for Plan Summary in MVP.
- Do not make ordinary Chat support squad actors unless needed by the Plan mode path.

## Required Architecture

### Domain Model

- `chat_plan_run` is the source of truth for Plan Run state.
- `chat_plan_run` belongs to one `chat_session`.
- `chat_plan_run.actor_type` is `agent` or `squad`.
- `chat_plan_run.lead_agent_id` is the executing agent for all lead planning turns.
- `chat_plan_run.plan_engine` stores the selected preset id.
- `chat_plan_run.engine_version` snapshots the preset version/hash at run start.
- `chat_plan_run.summary` stores lightweight structured plan summary JSON.
- `chat_plan_consultation` records each bounded lead-to-helper consultation.
- `chat_message` must be able to attribute agent-authored helper messages inside the same transcript.
- `chat_issue_proposal` can link back to `source_plan_run_id`.

### Plan Engine Contract

- Plan Engine presets live in server code as a registry with id, display label, description, protocol text, version/hash, and optional synthetic skill content.
- The task prompt is the authoritative synchronization mechanism from cloud Multica to local agents.
- Synthetic ephemeral Skills may be projected to local runtimes later, but implementation must not depend on native skill discovery.
- Every plan task prompt must include the active plan status, engine protocol, engine version, user message, relevant plan summary, transcript context, proposal output path, and rules forbidding direct issue creation without explicit bypass.

### Plan Run Lifecycle

- Starting Plan mode creates a `chat_plan_run`, persists the first user message, and enqueues a lead plan task.
- Continuing an active Plan Run persists the user reply and enqueues a continuation task for the same lead.
- A Plan Run stays active until the agent produces proposal(s), the user cancels, or failure/cancellation terminal state is reached.
- Existing chat message/proposal websocket invalidation paths should be extended rather than replaced.
- A Plan Run may produce multiple Chat Issue Proposals over time, but MVP should optimize for one final proposal set.

### Squad Consultation

- If the planning actor is a squad, resolve the squad lead as `lead_agent_id`.
- Lead planning prompt includes squad roster and exact mention markdown for eligible squad helper agents.
- Server parses lead-authored assistant chat messages for `mention://agent/<id>` links.
- Only agents in the selected squad roster can be triggered as consultation helpers.
- Each helper task must receive the consultation request, Plan Engine context, Plan Summary, and exact lead mention markdown.
- Helper responses are saved as agent-authored chat messages in the same Plan Run.
- Helper responses should mention the lead agent; server marks consultation responded and enqueues a lead continuation.
- If helper tasks fail or time out, the lead can still be resumed and must record the gap in Plan Summary.

### Frontend Contract

- Chat composer exposes a Plan mode control and Plan Engine selector.
- Plan mode default engine is `Grill with docs`.
- An active Plan Run is visible in composer state and follow-up replies continue the run automatically.
- Plan mode can target a squad as the actor; ordinary Chat may remain agent-only.
- Transcript renders agent-authored consultation messages with the helper agent identity.
- Plan Summary appears near linked proposal issues so users can understand why the issue split was proposed.
- Existing proposal item edit/approve/dismiss behavior remains the issue creation boundary.

## Implementation Plan

### Phase 1: Schema, Queries, and Types

1. Add migrations for `chat_plan_run`, `chat_plan_consultation`, and nullable plan attribution columns.
2. Add or update sqlc queries for:
   - create/get/list active plan runs by chat session
   - update plan status and summary
   - create/update/list consultations
   - link proposals to plan runs
   - list chat messages with agent author metadata
3. Regenerate sqlc.
4. Add Go response/request structs and TypeScript types/schemas for Plan Engines, Plan Runs, Plan Summary, and consultations.

### Phase 2: Plan Engine Registry and Prompting

1. Add server Plan Engine registry with the three presets.
2. Add `GET /api/chat/plan-engines`.
3. Extend daemon task response/task context with plan-run fields.
4. Add plan prompt builder branch for lead turns and consultation turns.
5. Add structured output handling for Plan Summary, likely `.multica/plan-summary.json`.
6. Ensure issue proposal manifest behavior continues to work for plan tasks.

### Phase 3: Plan Run Lifecycle APIs

1. Extend `SendChatMessageRequest` to accept Plan mode fields.
2. On `mode=plan` without `plan_run_id`, create the run and enqueue lead task.
3. On active `plan_run_id`, append user message and enqueue lead continuation.
4. Add list/cancel endpoints for Plan Runs.
5. Broadcast query invalidations for plan run and proposal changes.
6. Preserve existing normal Chat behavior when no Plan Run is involved.

### Phase 4: Squad Consultation Routing

1. Extend assistant chat completion handling to parse plan-scoped agent mentions.
2. Gate consultation target agents to the selected squad roster.
3. Create consultation records and enqueue helper tasks.
4. Save helper results as agent-authored chat messages with `plan_run_id` and `consultation_id`.
5. Detect lead mention in helper response and enqueue lead continuation.
6. Add timeout/failure recovery path so lead can resume with missing input recorded.

### Phase 5: Frontend UX

1. Add core query/mutation hooks for Plan Engines and Plan Runs.
2. Add composer Plan mode state and engine selector.
3. Persist active Plan Run state from server, not local-only Zustand.
4. Allow squad selection as a Plan actor.
5. Render Plan Summary and consultation messages in the Chat transcript/issues area.
6. Reuse existing proposal cards and approval flow.
7. Add localized strings for Plan mode, engines, active plan state, summary, consultation status, and cancellation.

### Phase 6: Verification and Hardening

1. Backend tests for plan creation, continuation, cancellation, and normal chat regression.
2. Backend tests for prompt generation and engine version capture.
3. Backend tests for Plan Summary structured output persistence.
4. Backend tests for proposal linking to `source_plan_run_id`.
5. Backend tests for squad consultation mention parsing, squad roster gating, helper response, lead resumption, and failure/timeout recovery.
6. TypeScript schema tests for malformed/missing Plan Run API fields.
7. React tests for composer Plan mode, active Plan Run continuation, Plan Summary display, and consultation message rendering.
8. Run `make sqlc`, focused Go tests, focused TS tests, `pnpm typecheck`, and broader `make check` when feasible.

## Acceptance Criteria

- Users can start Plan mode from Chat, choose a Plan Engine, and send an initial idea.
- Follow-up user replies continue the active Plan Run without reselecting Plan mode.
- The selected Plan Engine and version are persisted on the Plan Run.
- Claimed lead plan tasks receive the correct cloud-owned Plan Engine protocol in the prompt.
- A Plan Run can produce a persisted Plan Summary and linked Chat Issue Proposal.
- Users can edit/approve proposal items through the existing approval flow, and issue creation still requires human approval.
- Selecting a squad for Plan mode routes the lead task to the squad lead.
- The squad lead can mention squad helper agents in Chat, helper tasks are enqueued, helper replies are visible in the Plan transcript, and mentioning the lead resumes lead processing.
- Mentions of agents outside the selected squad do not enqueue consultation tasks.
- Failed or timed-out helper consultations do not block Plan Run completion forever.
- Normal Chat Session behavior remains compatible for non-plan messages.
- Package boundaries remain intact: backend logic in `server`, shared types/API/hooks in `packages/core`, shared UI in `packages/views`, app wiring in app packages.

## Known Risks

- Chat message attribution changes can affect existing transcript rendering and API response compatibility.
- Plan Run state must avoid becoming a second source of truth for proposal item state.
- Agent-to-agent mention routing can loop if lead/helper prompt constraints or server gating are too loose.
- Local runtime session resume is not reliable enough to own Plan Run state; server prompt context must be complete each turn.
- Squad actor support in Chat touches permission/access checks for private agents and squads.
- Structured output ordering matters: Plan Summary and issue proposals should be processed together enough that the UI can link them coherently.

## Open Questions

- Should MVP allow a user to manually ask for another consultation round, or is one bounded round enough until users request more?
- Should Plan Summary be editable by the user before approving proposal issues, or read-only provenance in MVP?
- Should cancelled Plan Runs keep an active composer banner until dismissed, or simply return the composer to ordinary Chat?

## Definition of Done

- PRD and task context are complete enough for Trellis implement/check agents.
- Implementation commits include migrations, backend, core types/schemas/hooks, UI, locales, and tests for the touched behavior.
- Quality verification passes or any remaining gaps are documented explicitly.
