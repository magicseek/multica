# Chat Plan Runs Implementation Plan

## Scope

Add a stateful **Plan mode** to page-level Chat. A user can start a **Chat Plan Run** from the chat composer, choose a **Plan Engine**, brainstorm across multiple turns, receive a **Plan Summary**, and approve generated **Chat Issue Proposals** before issues are created.

For squad-backed planning, the selected squad's lead agent coordinates bounded **Chat Plan Consultations** with squad members and synthesizes the final **Squad Plan Consensus**.

## Product Decisions

- Plan mode creates or continues a **Chat Plan Run** inside a normal **Chat Session**.
- The user does not reselect Plan mode for every reply while a plan run is active.
- MVP Plan Engines are server-owned presets:
  - `grill_with_docs` (default)
  - `brainstorming`
  - `office_hours`
- Plan Engine presets are authoritative in the cloud. Each plan task receives the selected protocol through server-generated task prompt context; supported runtimes may also receive a synthetic ephemeral skill for better local discovery.
- A Plan Run produces **Chat Issue Proposals** through the existing proposal approval flow. Approval still belongs to the human user.
- MVP stores a lightweight **Plan Summary** instead of introducing full reviewable workflow artifacts.
- Squad planning is lead-led and bounded. Helper agents provide opinions; the lead agent synthesizes the consensus.

## Non-Goals

- No separate Plan Chat Session type.
- No daemon-wide preset synchronization.
- No requirement that workspace Skills exist for the three Plan Engines.
- No unbounded roundtable mode.
- No automatic issue creation when a plan completes.
- No full Workflow Artifact dependency for MVP.

## Data Model

### `chat_plan_run`

New table for stateful planning inside a chat session.

Suggested fields:

- `id`
- `workspace_id`
- `chat_session_id`
- `creator_user_id`
- `actor_type`: `agent | squad`
- `actor_id`: selected agent or squad id
- `lead_agent_id`: executing lead agent; equals selected agent for `actor_type=agent`
- `plan_engine`: `grill_with_docs | brainstorming | office_hours`
- `engine_version`: stable preset version/hash captured at run start
- `status`: `brainstorming | consulting | ready_for_approval | completed | cancelled | failed`
- `initial_message_id`
- `latest_message_id`
- `summary`: JSONB with `confirmed_requirements`, `rejected_options`, `consensus_notes`, `open_questions`
- timestamps

### `chat_plan_consultation`

New table for bounded squad helper interactions.

Suggested fields:

- `id`
- `plan_run_id`
- `requester_agent_id`
- `target_agent_id`
- `request_message_id`
- `response_message_id`
- `task_id`
- `status`: `pending | running | responded | failed | timed_out | skipped`
- timestamps

### Existing Tables

- `chat_message`
  - Keep `role` as the UI bubble role (`user` or `assistant`) for compatibility.
  - Add `author_type`: `member | agent | system`.
  - Add nullable `author_agent_id`.
  - Add nullable `plan_run_id`.
  - Add nullable `consultation_id`.
  - Add nullable `reply_to_message_id` if the UI needs threaded grouping.
- `agent_task_queue`
  - Add nullable `chat_plan_run_id`.
  - Add nullable `chat_plan_consultation_id`.
  - Add a lightweight task kind if needed, e.g. `chat_task_kind`: `normal | plan_lead | plan_consultation`.
- `chat_issue_proposal`
  - Add nullable `source_plan_run_id` so proposals can be grouped under the Plan Summary.

## API Shape

### Plan engines

`GET /api/chat/plan-engines`

Returns server-known presets with display metadata and default engine.

### Send message

Extend `SendChatMessageRequest`:

```ts
{
  content: string;
  attachment_ids?: string[];
  mode?: "chat" | "plan";
  plan_engine?: "grill_with_docs" | "brainstorming" | "office_hours";
  plan_run_id?: string;
  plan_actor_type?: "agent" | "squad";
  plan_actor_id?: string;
}
```

Rules:

- `mode=chat`: current behavior.
- `mode=plan` with no `plan_run_id`: create a new `chat_plan_run`, persist the user message, enqueue a lead plan task.
- `mode=plan` with active `plan_run_id`: persist the user reply and enqueue a continuation task for the same lead.
- If a plan run is active, the frontend can omit `mode=plan` on follow-up sends as long as it sends `plan_run_id`.

### Plan run lifecycle

Add endpoints as needed:

- `GET /api/chat/sessions/{sessionId}/plan-runs`
- `POST /api/chat/plan-runs/{planRunId}/cancel`
- `POST /api/chat/plan-runs/{planRunId}/complete` only if user-controlled completion is needed; otherwise completion is derived from agent output.

## Runtime and Prompting

### Server-owned preset registry

Add a server-side registry for Plan Engines:

- id
- display name
- description
- protocol text
- version/hash
- optional synthetic skill content

`grill_with_docs` should ask one question at a time, prefer code/docs exploration over asking, and generate proposals only after enough shared understanding exists.

`brainstorming` should emphasize divergence before convergence.

`office_hours` should challenge assumptions and sharpen the product/strategy angle before proposing implementation work.

### Task claim payload

When a daemon claims a plan task, the server includes:

- `plan_run_id`
- `plan_engine`
- `engine_version`
- current plan status
- active Plan Summary fields
- current user message
- relevant transcript window
- proposal output path
- for squad runs: squad roster and exact mention markdown for eligible helper agents
- for consultation tasks: lead mention markdown and the requested opinion shape

### Prompt authority

The task prompt is the authoritative synchronization mechanism:

- The agent must follow the selected Plan Engine protocol.
- The agent must not create issues directly unless explicitly instructed to bypass approval.
- Issue drafts must be written to the existing proposal manifest.
- Plan Summary updates must be written to a new structured manifest, e.g. `.multica/plan-summary.json`.

Synthetic ephemeral skills are optional runtime context, not the source of truth.

## Squad Consultation Flow

1. User starts Plan mode with a squad selected.
2. Server creates `chat_plan_run` with `actor_type=squad`, resolves `lead_agent_id` from the squad.
3. Lead task prompt includes the Plan Engine protocol plus squad roster.
4. Lead may mention squad members in a chat assistant message.
5. Server parses the saved lead message for `mention://agent/<id>` links.
6. Server accepts only agents in the selected squad roster.
7. For each accepted mention, server creates `chat_plan_consultation` and enqueues a consultation task.
8. Helper agent response is saved as an assistant chat message with `author_agent_id`.
9. Helper prompt requires a lead mention in the response.
10. Server sees the lead mention, marks the consultation responded, and enqueues lead continuation.
11. Lead synthesizes the **Squad Plan Consensus**, updates Plan Summary, and may generate Chat Issue Proposals.

MVP boundedness:

- One consultation round per plan run by default.
- Multiple helper agents can be consulted in the same round.
- A helper contributes one structured opinion.
- Failed or timed-out helper tasks do not block the plan forever; the lead records missing input in the Plan Summary.

## Frontend Plan

### Chat composer

- Add a Plan mode control near the send composer.
- Default engine: `Grill with docs`.
- Engine selector options:
  - Grill with docs
  - Brainstorming
  - Office hours
- When a Plan Run is active, show a compact active-plan banner/state instead of requiring the user to reselect Plan mode.
- Allow selecting a squad as the planning actor in Plan mode. Normal chat can remain agent-only for MVP.

### Transcript

- Render plan messages in the same chat transcript.
- Show agent-authored helper messages with agent identity so the user can see the consultation path.
- Keep consultation messages visually grouped under the active Plan Run to avoid making the transcript feel like ordinary multi-user chat.

### Proposal Review

- Show Plan Summary above linked Chat Issue Proposals.
- Reuse existing proposal item editing and approval controls.
- The Approve action still creates issues through the existing Chat Issue Proposal approval endpoint.

## Implementation Phases

1. **Schema and Types**
   - Add migrations.
   - Regenerate sqlc.
   - Add core API schemas and TypeScript types.

2. **Plan Engine Registry**
   - Add server preset registry.
   - Add API endpoint for plan engine metadata.
   - Add prompt builder branch for plan tasks.

3. **Plan Run Lifecycle**
   - Extend send-message flow to create/continue plan runs.
   - Persist plan metadata and plan summary structured output.
   - Link generated proposals to plan runs.

4. **Squad Consultation**
   - Parse agent mentions on plan assistant messages.
   - Gate targets to selected squad roster.
   - Enqueue helper consultation tasks.
   - Resume lead when helpers answer or fail.

5. **Frontend**
   - Add Plan mode controls and active plan state.
   - Add squad actor support for Plan mode.
   - Render Plan Summary and consultation messages.
   - Keep proposal approval UI on the existing path.

6. **Verification**
   - Go tests for plan creation, continuation, prompt generation, structured summary persistence, proposal linking, squad mention gating, and lead resumption.
   - TypeScript schema tests with malformed API responses.
   - React tests for Plan mode composer state and proposal/summary rendering.
   - Manual browser pass for starting plan mode, multi-turn questioning, and approving generated proposals.

## Affected Areas

- `server/migrations/`
- `server/pkg/db/queries/chat.sql`
- `server/internal/handler/chat.go`
- `server/internal/handler/chat_structured_outputs.go`
- `server/internal/handler/daemon.go`
- `server/internal/daemon/prompt.go`
- `server/internal/service/task.go`
- `packages/core/api/client.ts`
- `packages/core/api/schemas.ts`
- `packages/core/types/chat.ts`
- `packages/core/chat/queries.ts`
- `packages/core/chat/mutations.ts`
- `packages/views/chat/components/`
- `packages/views/locales/*/chat.json`
