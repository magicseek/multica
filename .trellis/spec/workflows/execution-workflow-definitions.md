# Execution Workflow Definitions

## Scenario: Editable agent execution workflows

### 1. Scope / Trigger

- Trigger: replace hardcoded assignment execution protocol templates with editable workflow definitions.
- Trigger: add workspace-level Workflows UI beside Runtimes and Skills.
- Trigger: add project workflow binding, issue workflow override, and queue-time workflow snapshots.
- Trigger: preserve current assignment execution behavior while making Standard Assignment, Trellis Task, Direct Task, Research Note, and Comment Response system-seeded workflow definitions.

This is cross-layer work. It changes database schema, sqlc queries, HTTP API contracts, daemon claim payloads, execution environment rendering, core TypeScript types, React Query hooks, and shared views.

Design records:

- `CONTEXT.md`
- `docs/adr/0001-workflow-definitions-bindings-and-overrides.md`
- `docs/agent-workflows-design.md`

### 2. Signatures

#### DB

Recommended tables and columns:

```sql
CREATE TABLE workflow_definition (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL CHECK (origin IN ('system_seeded', 'user')),
  system_key TEXT,
  forked_from_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
  current_published_revision_id UUID,
  created_by UUID REFERENCES "user"(id),
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workflow_revision (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_definition_id UUID NOT NULL REFERENCES workflow_definition(id) ON DELETE CASCADE,
  revision_number INT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'deprecated')),
  schema JSONB NOT NULL,
  created_by UUID REFERENCES "user"(id),
  published_at TIMESTAMPTZ,
  deprecated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workflow_definition_id, revision_number)
);

CREATE UNIQUE INDEX workflow_definition_system_key_unique
  ON workflow_definition(workspace_id, system_key)
  WHERE system_key IS NOT NULL;

ALTER TABLE workflow_definition
  ADD CONSTRAINT workflow_definition_current_revision_fk
  FOREIGN KEY (current_published_revision_id)
  REFERENCES workflow_revision(id)
  ON DELETE SET NULL;
```

Binding/snapshot fields:

```sql
ALTER TABLE workspace
  ADD COLUMN default_assignment_workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
  ADD COLUMN default_comment_workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE project
  ADD COLUMN workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE issue
  ADD COLUMN workflow_override_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL;

ALTER TABLE agent_task_queue
  ADD COLUMN workflow_definition_id UUID REFERENCES workflow_definition(id) ON DELETE SET NULL,
  ADD COLUMN workflow_revision_id UUID REFERENCES workflow_revision(id) ON DELETE SET NULL,
  ADD COLUMN workflow_snapshot JSONB;
```

Use nullable columns for backwards compatibility. Existing tasks and non-assignment trigger types may have null workflow fields until their trigger rules are implemented.

#### Workflow schema JSON

Canonical persisted schema lives in `workflow_revision.schema`:

```json
{
  "schema_version": 1,
  "applicability": ["assignment"],
  "source": {
    "format": "markdown",
    "body_template": "## Task Execution Protocol\n\nRun `multica issue get {{issue_id}} --output json`."
  },
  "variables": [
    { "name": "issue_id", "required": true, "source": "issue.id" },
    { "name": "project_title", "required": false, "source": "project.title" }
  ],
  "steps": [
    {
      "id": "context-first",
      "name": "Context First",
      "order": 1,
      "required": true,
      "depends_on": [],
      "body_template": "Read issue details and comments before editing.",
      "artifacts": []
    }
  ],
  "gates": [],
  "preview": {
    "default_trigger_type": "assignment"
  }
}
```

`workflow_revision.schema` is the only source of truth. Source editor, step graph editor, and preview must all read/write this schema. Do not persist an independent Markdown document beside it.

#### Workflow snapshot JSON

Queue-time resolver writes `agent_task_queue.workflow_snapshot`:

```json
{
  "schema_version": 1,
  "trigger_type": "assignment",
  "definition_id": "uuid",
  "revision_id": "uuid",
  "revision_number": 3,
  "workflow_name": "Trellis Task",
  "origin": "system_seeded",
  "resolved_at": "2026-05-17T00:00:00Z",
  "schema": {},
  "rendered_markdown": "## Trellis Task Protocol\n\n...",
  "capability_warnings": [
    {
      "code": "unsupported_step_graph",
      "message": "Agent has not declared step graph support."
    }
  ]
}
```

The daemon consumes `rendered_markdown`. The snapshot schema is retained for audit, preview, and future richer execution.

#### API

Use workspace-scoped routes:

```text
GET    /api/workflows?applicability=assignment&include_archived=false
POST   /api/workflows
GET    /api/workflows/{id}
PATCH  /api/workflows/{id}
POST   /api/workflows/{id}/draft
PUT    /api/workflows/{id}/draft
POST   /api/workflows/{id}/publish
POST   /api/workflows/{id}/fork
POST   /api/workflows/preview
```

Project and issue routes extend existing payloads:

```json
{
  "workflow_definition_id": "uuid-or-null"
}
```

```json
{
  "workflow_override_definition_id": "uuid-or-null"
}
```

Daemon claim payload extends `AgentTask`:

```json
{
  "workflow_definition_id": "uuid-or-null",
  "workflow_revision_id": "uuid-or-null",
  "workflow_snapshot": {
    "rendered_markdown": "..."
  }
}
```

### 3. Contracts

#### Selection precedence

For issue assignment tasks:

```text
issue.workflow_override_definition_id
> project.workflow_definition_id
> workspace.default_assignment_workflow_definition_id
> built-in fallback system workflow
```

For comment-triggered `@agent` tasks:

```text
explicit full-workflow request
> workspace.default_comment_workflow_definition_id
> Comment Response system workflow
```

Comment-triggered tasks do not silently inherit project workflow.

#### Publish lifecycle

- `draft`: editable, cannot be selected by project/issue/workspace binding.
- `published`: current selectable revision for a workflow definition.
- `deprecated`: retained for audit/history, cannot be newly selected.

Publishing a draft:

- validates schema and applicability,
- marks the new revision as published,
- updates `workflow_definition.current_published_revision_id`,
- affects only tasks queued after publish,
- never mutates existing `agent_task_queue.workflow_snapshot`.

#### System-seeded definitions

- Seed into every workspace.
- `origin = 'system_seeded'`.
- System seed rows are read-only through normal update endpoints.
- Users customize through `POST /api/workflows/{id}/fork`.
- Bindings reference definition IDs, not hardcoded slugs.
- Seed operation must be idempotent by `(workspace_id, system_key)`.

#### Rendering

- Server resolves and renders workflow snapshot when `agent_task_queue` is created.
- Daemon claim must not re-resolve workflow definitions.
- Daemon injects `workflow_snapshot.rendered_markdown` into the execution environment.
- Legacy agent `execution_protocol_enabled` / `execution_protocol_slug` can remain as compatibility input only until migration is complete; it must not be the primary workflow selector.

#### Frontend

- Add workspace route `/:workspaceSlug/workflows`.
- Add sidebar item beside Runtimes and Skills.
- Use Multica's shared UI library and design system.
- Workflows page includes list/search/filter and create/fork actions.
- Workflow detail/editor includes `Source`, `Steps`, and `Preview` tabs.
- `Preview` includes both a graph preview of the structured step dependencies
  and the rendered Markdown from the server preview contract.
- Graph preview renders nodes from `schema.steps` and edges from
  `steps[].depends_on`; it must be derived from the same schema object as the
  Steps editor and must update when steps or dependencies change.
- Graph preview provides an expand/collapse control in the header. Expanded
  mode fills the Preview workspace and can return to the normal split view.
- Graph preview must recalculate viewport scale and center the graph content
  whenever the Preview pane changes size, including expand and collapse
  transitions. Do not leave the graph top-left anchored after a size change.
- Preview renders Markdown through the shared app Markdown renderer. It must
  not display the rendered Markdown string inside a raw `<pre>` block.
- Graph preview and rendered Markdown preview use independent scroll containers
  inside the Preview tab. Long rendered Markdown must not resize, push, or hide
  the graph preview, and scrolling Markdown must not scroll the graph pane.
- Source, Steps, and Preview tab triggers use consistent icon treatment.
- Steps editor presents steps as a numbered, low-chrome ordered editing list;
  avoid separate card boxes around every step unless the visual system changes.
- Project and issue workflow selectors filter by applicability.
- Project and issue workflow selectors submit workflow definition IDs, but their
  collapsed trigger labels must resolve those IDs back to workflow names from
  the loaded workflow metadata. Do not show raw UUIDs to users except as a
  last-resort fallback for missing metadata.
- Capability mismatch is warning-only in v1 and must not block assignment.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
|-----------|-------------------|
| Create workflow with empty name | `400`, field error |
| Create/update schema with unsupported `schema_version` | `400`, schema error |
| Schema missing `applicability` | `400`, schema error |
| Schema has duplicate step IDs | `400`, schema error |
| Step dependency references unknown step | `400`, schema error |
| Publish workflow with invalid schema | `400`, no revision status change |
| Bind project to workflow without assignment applicability | `400` |
| Bind project to workflow without current published revision | `400` |
| Set issue override to workflow without assignment applicability | `400` |
| Edit system-seeded workflow directly | `403`, suggest fork in response detail |
| Fork workflow not visible in workspace | `404` |
| Queue assignment when selected workflow was deleted/archived | fall back by precedence and record warning |
| Capability mismatch | `200`, warning returned in UI context and stored in snapshot |
| Daemon claims task with null workflow snapshot | legacy behavior/fallback, no panic |

### 5. Good/Base/Bad Cases

- Good: A project binds to Trellis Task. Issue A is assigned to Agent 1 and snapshots Trellis revision 2. The workflow is then published to revision 3. Issue B assigned later snapshots revision 3. Issue A still runs revision 2.
- Good: A small bug inside the same project sets issue override to Direct Task. Reassigning from Agent 1 to Agent 2 keeps Direct Task because workflow belongs to the issue, not the agent.
- Good: A user forks the system Trellis Task, edits source/steps, publishes it, and binds the project to the fork.
- Good: A project binding select stores the selected workflow definition ID
  while the collapsed control displays the workflow name, for example
  `Trellis Task`, not the UUID.
- Good: A user opens Preview and sees the workflow step graph with dependency
  arrows plus the rendered Markdown generated from the same schema.
- Good: The rendered Markdown preview shows headings, lists, code blocks, and
  tables as readable Markdown output rather than raw source text.
- Good: The graph preview can expand to fill the Preview tab and then collapse
  back to the normal graph plus rendered Markdown layout.
- Good: Expanding or collapsing the graph preview keeps the workflow graph
  scaled to the available viewport and centered in the graph pane.
- Good: A long rendered Markdown preview scrolls inside the Markdown pane while
  the graph remains visible and independently scrollable.
- Base: Workspace has no project binding and no issue override. Assignment uses workspace default assignment workflow.
- Base: Existing task has null workflow snapshot after migration. Daemon falls back to current legacy prompt path or standard assignment fallback.
- Bad: A comment mention automatically runs the full project workflow. This is wrong; comments default to Comment Response unless full workflow execution is explicit.
- Bad: Frontend stores Markdown and step graph as separate independent models. This is wrong; both edit the same workflow schema.
- Bad: Preview shows only rendered Markdown and omits the graph view. This hides
  dependency structure even though the workflow schema has step graph data.

### 6. Tests Required

Backend/db:

- Migration applies and rolls back cleanly.
- sqlc queries create, list, fork, draft, publish, and fetch current published revisions.
- System seed is idempotent per workspace.
- Direct update of system-seeded workflow returns `403`.
- Publish moves current pointer and leaves prior snapshots untouched.

Resolver/service:

- Selection precedence: issue override, project binding, workspace default, fallback.
- Applicability filter rejects invalid binding/override.
- Queue-time snapshot stores definition id, revision id, schema, rendered Markdown, and warnings.
- Publishing after queue does not affect queued task snapshot.
- Capability mismatch emits warning but does not block queueing.

Daemon/execenv:

- Claim payload with workflow snapshot injects rendered Markdown.
- Claim payload without workflow snapshot keeps legacy fallback behavior.
- Daemon does not call workflow resolution on claim.

Frontend/core/views:

- TypeScript types cover workflow definition, revision, schema, snapshot, applicability, and warnings.
- React Query keys invalidate on workflow create/update/publish/fork.
- Workflows page follows Runtimes/Skills page structure and shared components.
- Source/Steps/Preview tabs round-trip one schema object.
- Preview tab includes a graph view derived from `steps[].depends_on` and a
  rendered Markdown view; tests or visual QA must verify both are present.
- Preview UI verification covers readable Markdown rendering, graph
  expand/collapse, numbered step rows, and consistent Source/Steps/Preview tab
  icons.
- Preview UI verification covers independent graph and Markdown scrolling with
  long rendered Markdown content.
- Graph preview tests or manual QA cover both collapsed and expanded viewport
  sizes so centering/scale regressions are caught.
- Project and issue selectors filter by applicability and display capability warnings.
- Project and issue selector tests or manual QA verify that selected workflow
  IDs render as workflow names in the collapsed control after binding changes.

Regression:

- Existing agent creation/update tests pass while deprecated execution protocol fields remain.
- Existing assignment task tests pass with workflow snapshot present.
- `pnpm typecheck`, `pnpm test`, focused Go handler/service tests, and daemon execenv tests run.

### 7. Wrong vs Correct

#### Wrong

```text
agent.execution_protocol_slug = "trellis-task"
daemon claim resolves slug from hardcoded Go map
```

This makes workflow selection agent-owned and hardcoded. Reassigning the same issue to another agent can change the process.

#### Correct

```text
issue/project/workspace resolves workflow_definition_id
server snapshots current published workflow_revision at queue time
daemon injects agent_task_queue.workflow_snapshot.rendered_markdown
```

This keeps workflow ownership with the work, preserves auditability, and prevents delayed claims from observing unintended template changes.

## Scenario: Workflow input requests and reviewable planning artifacts

### 1. Scope / Trigger

- Trigger: let an agent ask bounded human clarification questions during a workflow step without marking the issue blocked or creating a new workflow run.
- Trigger: let planning, brainstorming, and requirements documents be reviewed in the cloud while preserving the local-output privacy boundary for ordinary task outputs.
- Trigger: support reviewable artifact revisions and reviewer-facing diffs without making patches the persisted source of truth.

This is cross-layer work. It changes database schema, sqlc queries, HTTP API contracts, daemon task lifecycle handling, CLI commands, workflow run responses, issue comment routing, core TypeScript types, and issue/workflow UI.

Design records:

- `CONTEXT.md`
- `docs/adr/0003-migrate-ai-desk-flows-through-multica-workflow-runs.md`
- `docs/adr/0004-workflow-input-requests-and-reviewable-planning-artifacts.md`

### 2. Signatures

#### DB

Add a task waiting lifecycle state. Waiting tasks are active but not claimable.

```sql
ALTER TABLE agent_task_queue
  DROP CONSTRAINT agent_task_queue_status_check,
  ADD CONSTRAINT agent_task_queue_status_check
    CHECK (status IN ('queued', 'dispatched', 'running', 'waiting', 'completed', 'failed', 'cancelled'));
```

Add workflow-scoped input requests:

```sql
CREATE TABLE workflow_input_request (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  workflow_run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
  workflow_step_run_id UUID NOT NULL REFERENCES workflow_step_run(id) ON DELETE CASCADE,
  issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
  chat_session_id UUID REFERENCES chat_session(id) ON DELETE SET NULL,
  question_comment_id UUID REFERENCES comment(id) ON DELETE SET NULL,
  answer_comment_id UUID REFERENCES comment(id) ON DELETE SET NULL,
  requester_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
  responder_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
  status TEXT NOT NULL CHECK (status IN ('requested', 'answered', 'cancelled', 'expired')),
  question_text TEXT NOT NULL,
  answer_text TEXT,
  round_index INT NOT NULL DEFAULT 1,
  max_rounds INT NOT NULL DEFAULT 1,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  answered_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workflow_input_request_run_status_idx
  ON workflow_input_request(workflow_run_id, status, requested_at DESC);

CREATE UNIQUE INDEX workflow_input_request_open_step_unique
  ON workflow_input_request(workflow_step_run_id)
  WHERE status = 'requested';
```

Do not add a `waiting_for_clarification` issue status. Use workflow runtime data to derive issue attention state.

Do not require a persisted `workflow_artifact_diff` table in the first implementation. Diffs may be generated on demand from complete artifact versions. A cache table can be introduced later if artifact size or traffic makes on-demand diffing too expensive.

#### Workflow schema JSON

Agent-executed steps may opt into bounded input requests:

```json
{
  "id": "contract",
  "execution": { "kind": "agent" },
  "input_requests": {
    "allowed": true,
    "max_rounds": 3,
    "question_policy": "one_at_a_time"
  }
}
```

Planning artifacts that need review are declared as workflow artifacts with review:

```json
{
  "id": "plan",
  "execution": { "kind": "agent" },
  "artifact": {
    "name": "implementation-plan",
    "content_kind": "markdown",
    "required": true
  },
  "review": {
    "required": true
  }
}
```

Default policy:

- `input_requests.allowed = false` unless the step or system seed enables it.
- Execution steps default to `max_rounds = 1`.
- Planning, contract, or clarity steps may opt into a small higher limit, normally `3`.
- Only one requested input request may be open for a step run at a time.

#### API

Workflow run responses include open and historical input requests:

```json
{
  "input_requests": [
    {
      "id": "uuid",
      "workflow_run_id": "uuid",
      "workflow_step_run_id": "uuid",
      "status": "requested",
      "question_text": "...",
      "answer_text": null,
      "question_comment_id": "uuid",
      "answer_comment_id": null,
      "round_index": 1,
      "max_rounds": 3,
      "requested_at": "..."
    }
  ]
}
```

Input request routes:

```text
POST /api/workflow-step-runs/{id}/input-requests
POST /api/workflow-input-requests/{id}/answer
POST /api/workflow-input-requests/{id}/cancel
```

Request body for creating an input request:

```json
{
  "question_text": "Which repository providers should this support first?",
  "max_rounds": 3
}
```

Request body for answering:

```json
{
  "answer_text": "Start with GitHub only.",
  "continue": true
}
```

Artifact diff route:

```text
GET /api/workflow-artifacts/{id}/diff?base_version=1&target_version=2
```

Response body:

```json
{
  "logical_name": "implementation-plan",
  "base_version": 1,
  "target_version": 2,
  "content_kind": "markdown",
  "unified_diff": "...",
  "summary": null
}
```

#### CLI

Agent-facing input request command:

```text
multica workflow input request <step-run-id> --question-file <path|-> [--max-rounds N]
```

Reviewer-facing or debugging commands may be added as wrappers over API routes:

```text
multica workflow input answer <input-request-id> --file <path|-> [--no-continue]
multica workflow artifact diff <artifact-id> --base-version N --target-version N
```

Existing explicit artifact save remains the publication boundary:

```text
multica workflow artifact save <step-run-id> --name implementation-plan --file plan.md --format markdown
```

### 3. Contracts

#### Workflow input requests

- An input request belongs to exactly one workflow step run.
- Creating an input request is allowed only for an agent step whose snapshot allows input requests and whose round limit has not been exceeded.
- Creating an input request creates or links an issue-visible question comment when the run has an issue context.
- Creating an input request sets the step run and workflow run to a waiting state and suspends the backing task instead of failing or completing it.
- Waiting tasks are not claimable by daemon pollers.
- Waiting tasks still count as active for duplicate task guards, issue live banners, and cancellation.
- Answering through the explicit input request answer route creates or links an answer comment.
- Answer-bound comments must not trigger the ordinary comment response workflow, even if they mention an agent.
- Answering with `continue = true` changes the backing task from `waiting` to `queued`, makes the waiting step ready or running as appropriate, and lets the daemon resume with the same workflow run, work directory, and provider session when available.
- Ordinary issue comments do not answer input requests unless routed through the explicit answer action.
- If the round limit is exhausted, the agent must report the remaining decision gap instead of opening another input request.

#### Reviewable workflow artifacts

- Ordinary local outputs remain Output Metadata unless the agent or human explicitly saves a workflow artifact.
- Planning, brainstorming, and requirements documents that need cloud approval are saved as workflow artifacts with `content_kind = markdown`, `text`, or `json`.
- Output Metadata is never enough for a review/approval decision. If the user is expected to read, approve, reject, or compare a design/plan, the server must receive complete artifact content through `workflow artifact save`; a local path in `.multica/outputs.json` or an issue comment is only discoverability metadata.
- Workflow run surfaces must render saved artifact content inline for review. Listing only artifact names, relative paths, sizes, or versions is not a valid review UI.
- Approval of a reviewable artifact unblocks workflow dependencies; it does not directly create issues or other workspace side effects.
- A later explicit workflow step may create Chat Issue Proposals or issues after approval.
- Each artifact version is stored as complete content.
- Diff output is a reviewer aid generated from complete versions; it is not the source of truth.
- Agents may use patch-style or section-level edits locally to reduce model output tokens, but the saved artifact version must be the complete revised document.

#### Daemon behavior

- The daemon releases its execution slot when a task is suspended for input.
- A suspended task must preserve `session_id` and `work_dir` when the provider returned them.
- Resume claim payloads include the workflow run, step runs, input requests, and answer comment context.
- The daemon must not call `CompleteTask` for a workflow run that is waiting on an open input request.
- The daemon must not call `FailTask` for expected user input waits.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
|-----------|-------------------|
| Create input request for non-agent step | `400`, input requests are agent-step waiting points |
| Create input request when step snapshot disallows it | `400`, policy error |
| Create second open input request for same step run | `409`, existing requested input returned |
| Create input request after `max_rounds` exhausted | `400`, round limit exceeded |
| Answer input request outside workspace | `404` or `403` according to existing workspace auth convention |
| Answer already answered/cancelled/expired request | `409`, terminal input request |
| Ordinary comment on issue with open input request | Creates ordinary comment; does not answer request unless explicit answer route is used |
| Answer-bound comment contains `@agent` | Does not enqueue comment response workflow |
| Suspend task without open input request | `400` or no-op guard; do not hide failures as waiting |
| Claim poll sees waiting task | Waiting task is not returned as claim candidate |
| New task for same issue/agent while prior task waiting | Duplicate guard treats waiting as active |
| Diff requested for missing version | `404` |
| Diff requested for unsupported binary content | `400`; first version supports text/markdown/json only |
| Artifact save uses invalid JSON for json kind | `400`, no artifact row |

### 5. Good/Base/Bad Cases

- Good: During a contract step, the agent asks one question, the task moves to waiting, the issue shows `Needs input`, the user answers through `Answer & continue`, and the same workflow run resumes in a later execution batch.
- Good: A user posts a separate discussion comment while an input request is open. The comment remains ordinary discussion and does not resume the workflow.
- Good: An answer-bound comment mentions the agent by name, but only the input request resumes. No Comment Response workflow is queued.
- Good: A planning workflow in a chat session saves `implementation-plan` v1 as a reviewable workflow artifact. A reviewer requests changes, the agent saves v2, and the UI shows a diff from v1 to v2.
- Good: The workflow run evidence area renders the full latest `implementation-plan` Markdown so the reviewer can approve it without opening a daemon-local file.
- Good: A reviewer approves a plan artifact. The workflow proceeds to a separate step that creates Chat Issue Proposals, which the user can still accept or edit.
- Base: A workflow step has no `input_requests` policy. The agent cannot open an input request and must use the existing blocked path if it cannot proceed.
- Base: Artifact diff is generated on demand and is not persisted.
- Bad: A final issue comment says `Artifacts: deliverables/plan.md` but no workflow artifact was saved. This is wrong because cloud reviewers cannot preview, approve, or diff a daemon-local path.
- Bad: Adding `waiting_for_clarification` to issue status. This is wrong because issue status is a coarse lifecycle and should not encode workflow attention state.
- Bad: Completing the backing task when an input request is open. This is wrong because completion currently auto-completes unfinished step runs and would erase the wait state.
- Bad: Saving only a patch as artifact v2. This is wrong because review and downstream execution require complete artifact content.

### 6. Tests Required

Backend/db:

- Migration adds `agent_task_queue.status = waiting` and `workflow_input_request`; rollback restores prior constraints safely.
- sqlc queries create, fetch, answer, cancel, and list input requests by workflow run.
- Partial unique index prevents two requested input requests for one step run.
- Waiting tasks are excluded from claim candidates but included in active duplicate guards and issue live task queries.

Workflow service:

- Creating input request validates step kind, snapshot policy, open-request uniqueness, and round limit.
- Creating input request moves workflow run to `waiting` and suspends the backing task without completing unfinished steps.
- Answering input request records answer text/comment, marks request answered, returns the task to `queued`, and makes the step resumable.
- Answering does not create a new workflow run.
- Cancelling a run with an open input request cancels or terminalizes the request.
- Artifact diff returns deterministic output for markdown/text versions.

Comment routing:

- Answer-bound comments do not enqueue comment-triggered agent tasks.
- Ordinary comments still follow existing mention/comment workflow behavior.
- Ordinary comments on issues with open input requests remain ordinary unless routed through the explicit answer action.

Daemon/execenv:

- Agent request-input path causes task suspension, not complete/fail.
- Resume claim includes workflow run, step runs, input requests, answer context, `session_id`, and `work_dir`.
- Timeout/orphan recovery does not fail tasks already suspended as waiting.

Frontend/core/views:

- TypeScript types cover workflow input requests and artifact diff responses.
- Workflow run viewer shows open input requests with `Answer & continue`, `Comment only`, and cancel affordances.
- Workflow run viewer exposes which step runs can open workflow input requests, so users can distinguish "this step may ask" from ordinary comment discussion even before a request is open.
- Issue cards/details derive `Needs input` attention from open workflow input requests without changing issue status.
- Comment composer can target an input request and submit through the answer route.
- Artifact viewer shows complete latest version content and reviewer-facing diff between versions. Tests must fail if artifacts degrade to names/paths only.
- Review approval unblocks workflow but does not directly create issues.

Verification commands:

- `make test` for Go service/handler/db paths touched.
- Focused Go tests for workflow runtime, comment routing, daemon task lifecycle, and artifact diff.
- `pnpm typecheck`.
- `pnpm test` for core/view changes.
- Manual or browser QA for issue attention badge, answer flow, artifact review, and artifact diff rendering.

### 7. Wrong vs Correct

#### Wrong

```text
agent posts "I need input" as an ordinary comment
daemon completes the task
user replies with @agent
server queues Comment Response workflow
```

This loses the waiting step state, can auto-complete unfinished workflow steps, and starts the wrong workflow.

#### Correct

```text
agent calls multica workflow input request <step-run-id>
server creates Workflow Input Request and suspends the task
user submits Answer & continue
server records answer and requeues the same task
daemon resumes the same Workflow Run in a new Execution Batch
```

This preserves workflow history, releases daemon capacity while waiting, and keeps ordinary comments separate from workflow-resume inputs.

#### Wrong

```text
reviewer requests changes
agent saves only plan.patch as artifact v2
downstream implementation step reads latest artifact
```

This makes review and execution depend on replaying previous versions.

#### Correct

```text
agent applies patch locally or rewrites changed sections
agent saves complete implementation-plan v2
UI displays Workflow Artifact Diff from v1 to v2
downstream implementation step reads complete v2
```

This reduces model output cost while keeping complete artifacts as the durable source of truth.
