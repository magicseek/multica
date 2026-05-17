# Agent Execution Protocol Contracts

## Domain Terms

- **Task Agent**: an agent runtime invocation that executes a Multica task from `agent_task_queue`.
- **Execution Protocol**: task-level working contract injected into a Task Agent for context gathering, planning, implementation, verification, and reporting.
- **Execution Protocol Template**: first-party static protocol content selected by slug. The current built-ins are `standard-assignment` and `trellis-task`.
- **Protocol-Enabled Agent**: an agent whose settings opt it into the Execution Protocol path.
- **Plan Gate**: the protocol rule requiring a concise implementation plan before code edits when work is complex enough.
- **Checkpoint**: stored execution metadata used by later work to decide whether a run may resume a prior provider session.
- **Work Item**: a structured slice of work derived from a larger issue or squad task.
- **Assignment Lease**: a time-bounded claim of a Work Item by a specific agent runtime.

## Scenario: Protocol-Gated Task Agent Execution

### 1. Scope / Trigger

- Trigger: any change that controls task-agent prompt behavior through agent settings.
- Applies when a feature adds or changes a database field, API request/response field, daemon claim payload, `TaskContextForEnv`, or runtime prompt injection predicate.
- Goal: legacy behavior remains default; only explicitly enabled ordinary assignment tasks receive the protocol.

### 2. Signatures

- DB:
  - `agent.execution_protocol_enabled BOOLEAN NOT NULL DEFAULT FALSE`
  - `agent.execution_protocol_slug TEXT NOT NULL DEFAULT ''`
  - Migration pair: `NNN_agent_execution_protocol.up.sql` / `.down.sql`
- Backend API:
  - `CreateAgentRequest.ExecutionProtocolEnabled bool json:"execution_protocol_enabled"`
  - `CreateAgentRequest.ExecutionProtocolSlug string json:"execution_protocol_slug"`
  - `UpdateAgentRequest.ExecutionProtocolEnabled *bool json:"execution_protocol_enabled"`
  - `UpdateAgentRequest.ExecutionProtocolSlug *string json:"execution_protocol_slug"`
  - `AgentResponse.ExecutionProtocolEnabled bool json:"execution_protocol_enabled"`
  - `AgentResponse.ExecutionProtocolSlug string json:"execution_protocol_slug"`
- Daemon claim payload:
  - `TaskAgentData.ExecutionProtocolEnabled bool json:"execution_protocol_enabled,omitempty"`
  - `TaskAgentData.ExecutionProtocolSlug string json:"execution_protocol_slug,omitempty"`
  - `daemon.AgentData.ExecutionProtocolEnabled bool json:"execution_protocol_enabled,omitempty"`
  - `daemon.AgentData.ExecutionProtocolSlug string json:"execution_protocol_slug,omitempty"`
- Exec environment:
  - `TaskContextForEnv.ExecutionProtocolEnabled bool`
  - `TaskContextForEnv.ExecutionProtocolSlug string`
  - Predicate owner: `shouldUseTaskExecutionProtocol(ctx TaskContextForEnv) bool`
- Frontend:
  - `Agent.execution_protocol_enabled?: boolean`
  - `Agent.execution_protocol_slug?: ExecutionProtocolSlug`
  - `CreateAgentRequest.execution_protocol_enabled?: boolean`
  - `CreateAgentRequest.execution_protocol_slug?: ExecutionProtocolSlug`
  - `UpdateAgentRequest.execution_protocol_enabled?: boolean`
  - `UpdateAgentRequest.execution_protocol_slug?: ExecutionProtocolSlug`

### 3. Contracts

- Create default: omitted or `false` creates a legacy agent.
- Update default: omitted preserves the existing DB value; explicit `false` must disable the setting.
- Template compatibility:
  - `execution_protocol_enabled=false` always renders the legacy assignment workflow, regardless of stored slug.
  - `execution_protocol_enabled=true` with empty slug resolves to `standard-assignment`.
  - `execution_protocol_enabled=true` with `trellis-task` renders the Trellis template.
- Built-in templates are static server code, not rows in the agent template catalog.
- Unknown non-empty protocol slugs must be rejected by create/update handlers before DB writes.
- API responses include the field so React Query remains the source of truth.
- Frontend controls send explicit enabled + slug values through create/update requests; do not copy server state into Zustand.
- Duplicate/create flows preserve an explicit enabled value and known slug from the source agent.
- Claim responses copy the fresh DB values into `task.agent.execution_protocol_enabled` and `task.agent.execution_protocol_slug`; the daemon must not do an extra query to decide prompt behavior.
- Prompt injection is allowed only when all are true:
  - `ExecutionProtocolEnabled == true`
  - `IssueID != ""`
  - `TriggerCommentID == ""`
  - `ChatSessionID == ""`
  - `AutopilotRunID == ""`
  - `QuickCreatePrompt == ""`
  - `IsSquadLeader == false`
- Comment, chat, autopilot, quick-create, and squad leader tasks keep their specialized workflows even when the agent setting is enabled.

### 4. Validation & Error Matrix

- Old rows after migration -> `execution_protocol_enabled=false`.
- Old rows after slug migration -> `execution_protocol_slug=''`.
- Missing SQL regeneration after adding the DB field -> generated model/query mismatch; run `make sqlc`.
- Create request omits field -> DB stores `false`; response returns `false`.
- Update request omits field -> DB value is unchanged.
- Update request sends `false` -> DB value becomes `false`.
- Create/update request sends `execution_protocol_slug='unknown'` -> HTTP 400.
- Enabled comment/chat/autopilot/quick-create/squad task -> no `## Task Execution Protocol` in rendered runtime config.
- Enabled ordinary assignment task -> rendered runtime config includes protocol commands scoped to that issue ID.
- Enabled ordinary assignment task with `trellis-task` -> rendered runtime config includes `## Trellis Task Protocol` and no AETHER instructions.

### 5. Good/Base/Bad Cases

- Good: create agent with `execution_protocol_enabled=true` and `execution_protocol_slug="trellis-task"`, claim an assignment task, and assert daemon context receives both values.
- Base: existing agent with omitted field runs legacy assignment workflow.
- Base: enabled agent handles a comment-triggered task and renders the comment workflow only.
- Bad: adding a frontend protocol picker without backend round-trip tests.
- Bad: gating only on the agent setting and injecting protocol before specialized task branches.
- Bad: storing the flag in a client-side Zustand store instead of API state.
- Bad: introducing AETHER as a built-in option when the product requirement is Trellis plus standard assignment only.

### 6. Tests Required

- Migration applies and rolls back cleanly.
- `make sqlc` produces generated `Agent` and agent query params with the new field.
- Handler tests:
  - create with `true` and `trellis-task` returns and stores both values
  - create/update reject unknown non-empty slugs
  - update with `true` + slug, omitted, then `false` + empty slug preserves/toggles correctly
- Claim handler test:
  - queued task response includes `task.agent.execution_protocol_enabled=true` and the selected slug for enabled agents
- Execenv tests:
  - disabled ordinary assignment keeps legacy workflow
  - enabled ordinary assignment renders `## Task Execution Protocol`
  - enabled `trellis-task` ordinary assignment renders `## Trellis Task Protocol`
  - enabled comment/chat/autopilot/quick-create/squad paths do not render the protocol
- Frontend tests:
  - create dialog submits enabled + slug from the protocol picker
  - duplicate mode initializes from the source agent value
- Project checks:
  - `pnpm typecheck`
  - `pnpm lint`
  - `pnpm test`
  - focused Go packages for daemon/handler/execenv

### 7. Wrong vs Correct

#### Wrong

```go
if ctx.ExecutionProtocolEnabled {
    b.WriteString(renderTaskExecutionProtocol(ctx))
}
```

This injects the protocol into comment, chat, autopilot, quick-create, and squad leader tasks.

#### Correct

```go
if shouldUseTaskExecutionProtocol(ctx) {
    b.WriteString(renderTaskExecutionProtocol(ctx))
}
```

The predicate owns every exclusion and should be covered by prompt-rendering tests.

#### Wrong

```typescript
const enabled = useAgentSettingsStore((s) => s.executionProtocolEnabled);
```

This creates a second source of truth for server state.

#### Correct

```typescript
<Select
  value={agent.execution_protocol_enabled ? agent.execution_protocol_slug || "standard-assignment" : "off"}
  onValueChange={(value) => update({
    execution_protocol_enabled: value !== "off",
    execution_protocol_slug: value === "off" ? "" : value,
  })}
/>
```

The UI reads the React Query-backed `Agent` response and writes through the API mutation.
