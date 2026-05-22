# Project and Chat Analytics Tabs

## Goal

Add analytics views at project and chat granularity so users can understand agent token spend, turn latency, prompt-cache health, and execution shape directly where they evaluate work. The same analytics surface should power both Project Overview and Chat Session pages, with scope-specific drilldowns where the workflow differs.

## What I Already Know

* Project detail currently has `Issues` and `Chats` tabs in `packages/views/projects/components/project-detail.tsx`.
* Chat session detail currently has `Chat`, `Issues`, and `Outputs` tabs in `packages/views/chat/components/chat-pages.tsx`.
* Existing workspace dashboard already exposes daily usage, usage by agent, agent runtime, and runtime daily query paths through `packages/core/dashboard/queries.ts` and `server/internal/handler/dashboard.go`.
* Existing dashboard helpers in `packages/views/dashboard/utils.ts` already aggregate daily cost, tokens, time, tasks, and agent-level usage.
* Recent daemon observability work records tracing metadata in `task_usage.metadata`, including prompt/context byte counts, daemon/agent timing, first event/text/tool timings, output/tool byte counts, and message event counts.
* `chat_session` has `project_id`, while `agent_task_queue` links tasks by `chat_session_id` and/or `issue_id`.
* Current dashboard project filtering is issue-centric. Project analytics must include both project issues and chats attached to the project, so backend scoping cannot rely only on `issue.project_id`.

## Product Direction

The UI should feel like an operational performance console: compact, scan-friendly, and built for repeated diagnosis. Avoid a marketing/dashboard hero treatment. The main question it answers is:

* "Why did this project or chat cost this much and take this long?"
* "Is latency dominated by queueing, model execution, tools, prompt/context size, or cache misses?"
* "Which chats, issues, agents, or turns deserve attention?"

## Information Architecture

### Project Overview: Analytics Tab

Add a third tab alongside `Issues` and `Chats`.

Recommended contents:

* Top control row:
  * Period segmented control: `7d`, `30d`, `90d`; default `30d`.
  * Source segmented control: `All`, `Issues`, `Chats`.
* KPI strip:
  * Total tasks / completed / failed.
  * Total tokens and estimated cost, using the same pricing convention as the dashboard.
  * Median and p95 turn time.
  * Prompt cache read rate.
  * Tool load: tool-use count and tool-result bytes.
* Primary gauges:
  * Total token consumption.
  * Total estimated price.
  * Average completion speed.
* Trend area:
  * Daily tokens/cost trend.
  * Daily runtime/tasks trend.
  * Cache read/write trend.
* Latency breakdown:
  * Queue time: `created_at -> dispatched_at`.
  * Startup overhead: `dispatched_at -> started_at`.
  * Agent run time: `started_at -> completed_at` and metadata `agent_run_ms`.
  * First response markers: `first_event_ms`, `first_text_ms`, `first_tool_use_ms`, `first_tool_result_ms`.
* Project source breakdown:
  * Issues vs chats contribution to tasks, tokens, cost, runtime, and failures.
  * Top chats by tokens/runtime.
  * Top issues by tokens/runtime.
* Agent/model table:
  * Agent, model, task count, failed count, tokens, cache rate, median/p95 runtime.
* Full run table:
  * Show issue-backed and chat-backed agent runs in one table by default.
  * Include source type, source title, agent/model, status, timing, tokens, cache rate, prompt bytes, tool calls, and output/tool bytes.
  * Support filtering by `All`, `Issues`, and `Chats`.
  * Link rows back to the source chat or issue when available.

### Chat Session: Analytics Tab

Add a fourth tab alongside `Chat`, `Issues`, and `Outputs`.

Recommended contents:

* Top control row:
  * Period segmented control plus an `All` option; default to all available runs.
* KPI strip:
  * Turn count, failed turns, total tokens, estimated cost.
  * Median and p95 reply time.
  * Prompt cache read rate.
  * Average prompt/context size.
* Primary gauges:
  * Total token consumption.
  * Total estimated price.
  * Average completion speed.
* Per-turn timeline:
  * One row per agent task/turn.
  * Columns: started time, status, total duration, first text latency, input/output/cache tokens, cache rate, prompt bytes, tool calls, tool-result bytes, assistant output bytes.
* Context growth:
  * Prompt bytes over turns.
  * Runtime brief stable/dynamic bytes over turns.
  * Agent instructions / skills / workflow snapshot bytes where present.
* Cache health:
  * Cache read/write tokens over turns.
  * Highlight turns where prompt grew but cache read rate fell.
* Event composition:
  * Assistant text bytes, thinking bytes, tool input bytes, tool result bytes.
  * Message event counts: text, thinking, tool use, tool result, error.

## Reusable UI Design

Create one reusable analytics surface with scope-specific configuration:

```ts
type AgentAnalyticsScope =
  | { kind: "project"; projectId: string }
  | { kind: "chat"; sessionId: string };
```

Suggested component split:

* `AgentAnalyticsSurface`
  * Owns query selection, loading/empty/error states, period/source controls, and layout.
* `AnalyticsKpiStrip`
  * Compact bordered grid, not separate floating cards.
* `AnalyticsPrimaryGauges`
  * Reusable gauge row shown at the top of both project and chat analytics.
  * Uses compact numeric metric tiles with delta/progress accents.
  * Avoids radial or semi-circular gauges because these imply fixed thresholds the MVP does not have.
  * Always includes total token consumption, total estimated price, and average completion speed.
  * Average completion speed is defined as median completed-task duration.
  * Total estimated price gauge shows the aggregate price only.
  * Input/output/cache-read/cache-write cost segments are available in expandable details or secondary breakdown sections.
  * Total estimated price excludes tokens from unknown/unpriced models.
  * Unpriced model usage is shown as a separate "unpriced tokens" indicator, not silently folded into `$0.00`.
  * Unpriced-token indicator includes an action to open the existing custom pricing dialog.
  * Gauge deltas compare the selected window with the previous equivalent window when such a window exists.
  * Chat `All` mode has no natural previous equivalent window, so it displays absolute values until the user selects a bounded period.
  * Uses the same data contract for both scopes so comparison across project/chat pages stays consistent.
* `AnalyticsTrendPanel`
  * Shared chart shell for token, cost, runtime, task, and cache trends.
* `AnalyticsLatencyBreakdown`
  * Shared latency waterfall/stacked bars.
* `AnalyticsRunTable`
  * Reusable table used by both project and chat scopes.
  * Project scope adds source columns for issue/chat attribution.
  * Chat scope emphasizes turn ordering and context growth.
  * Default view shows the highest-signal columns only.
  * Collapsed rows prioritize operational diagnosis: source/turn, status, agent/model, duration, first text latency, tokens, estimated cost, cache rate, and tool calls.
  * Expanded rows expose the full tracing detail set.
* `ProjectAnalyticsBreakdown`
  * Project-only source and top source rows.
* `ChatContextGrowthPanel`
  * Chat-only prompt/context/cache-over-turns panel.

Visual style:

* Dense, quiet, utilitarian layout with constrained width matching the existing project/chat surfaces.
* Neutral surfaces, restrained borders, compact type.
* Accent colors by metric family:
  * token/cost: blue/cyan
  * cache: emerald
  * latency/wait: amber
  * failures/errors: rose
  * tool activity: slate/indigo only as secondary accent
* Use lucide icons where helpful: `BarChart3`, `Timer`, `Gauge`, `Database`, `Wrench`, `MessageSquare`, `FolderKanban`.
* Avoid nested cards, decorative gradients, oversized hero headings, and one-color palette dominance.

## Data Contract Direction

Existing dashboard endpoints are useful for workspace/project rollups, but the new tracing metadata needs scoped raw-task analytics. Recommended MVP endpoint shape:

```http
GET /api/analytics/agent-runs?scope=project&scope_id=<project_id>&days=30&source=all
GET /api/analytics/agent-runs?scope=chat&scope_id=<chat_session_id>&days=30
```

Recommended response shape:

```ts
type AgentAnalyticsResponse = {
  summary: AgentAnalyticsSummary;
  daily: AgentAnalyticsDailyRow[];
  agents: AgentAnalyticsAgentRow[];
  runs: AgentAnalyticsRunRow[];
  sources?: AgentAnalyticsSourceRow[];
  pagination: {
    limit: number;
    offset: number;
    total: number;
  };
};
```

Server should return raw token counts and timing fields. Cost should stay client-side to match the existing dashboard convention.
Summary, daily, agent, source, and gauge data should be computed across the full filtered window; pagination applies only to `runs`.

Endpoint and access-control decisions:

* Use one generic scoped endpoint: `GET /api/analytics/agent-runs`.
* Accepted scopes: `project` and `chat`.
* Project scope follows the same resource visibility as Project detail: resolve the requested project inside the current workspace before returning analytics.
* Chat scope follows the same visibility as Chat detail: the requester must own the chat session and pass the private-agent access gate.
* Workflow/autopilot execution context should appear as row badges or expanded metadata, not as top-level Project filters. Project filters stay `All`, `Issues`, and `Chats`.

Important backend detail:

* Project scope should resolve project ownership through both issue and chat joins, for example `COALESCE(issue.project_id, chat_session.project_id)`.
* Rollup table paths may not contain new metadata fields. For tracing metrics, MVP should query `task_usage` plus `agent_task_queue` directly for the requested window.
* JSON metadata fields should be normalized into typed numeric fields before sending to the client, instead of forcing the UI to parse arbitrary metadata.

## Metrics To Expose

Token/cache metrics:

* `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`.
* Cache read rate using the existing dashboard convention, with zero-safe division.
* Estimated cost from existing model pricing logic.

Timing metrics:

* Queue duration.
* Startup/dispatch duration.
* Total task duration.
* `exec_env_ms`, `runtime_config_ms`, `backend_create_ms`.
* `agent_run_ms`, `daemon_run_ms`.
* `first_event_ms`, `first_text_ms`, `first_tool_use_ms`, `first_tool_result_ms`.

Prompt/context metrics:

* `prompt_bytes`, `system_prompt_bytes`.
* `chat_message_bytes`, `chat_attachment_count`.
* `agent_instructions_bytes`, `agent_skill_count`, `agent_skill_bytes`.
* `repo_count`, `repository_count`, `project_resource_count`.
* `workflow_snapshot_bytes`, `workflow_step_snapshot_bytes`.
* `autopilot_description_bytes`, `autopilot_payload_bytes`.
* `quick_create_prompt_bytes`.

Output/tool metrics:

* `task_message_text_count`, `task_message_thinking_count`, `task_message_tool_use_count`, `task_message_tool_result_count`, `task_message_error_count`.
* `assistant_text_bytes`, `thinking_bytes`, `tool_input_bytes`, `tool_result_bytes`, `agent_result_output_bytes`.

## Requirements

* Add `Analytics` tab to Project Overview next to `Issues` and `Chats`.
* Add `Analytics` tab to Chat Session next to `Chat`, `Issues`, and `Outputs`.
* Use one reusable analytics surface for both pages.
* Both Project Analytics and Chat Analytics must expose a full run table by default.
* Both Project Analytics and Chat Analytics must show primary gauges for total token consumption, total estimated price, and average completion speed.
* Primary gauges must use compact numeric metric tiles with delta/progress accents.
* Average completion speed gauge must use median completed-task duration, not arithmetic mean.
* Total price gauge must stay aggregate-first; cost segments must live in details/breakdowns rather than the primary gauge row.
* Token and price gauges must include every run that actually recorded token usage, including failed or cancelled runs.
* Completion speed gauges must exclude non-completed runs.
* Total estimated price must exclude unknown/unpriced models and surface their token volume separately.
* Unpriced-token indicators must offer a direct action to configure custom pricing through the existing custom pricing dialog.
* Primary gauge deltas must compare the current selected time window against the previous equivalent window.
* Chat `All` mode must suppress period delta copy and show absolute gauge values.
* Analytics run tables must show only terminal runs with recorded usage/tracing data in the MVP.
* Running/in-progress task visibility remains owned by the Chat view, not the Analytics tab.
* Full run tables must use server-side pagination with a default page size of 50.
* Summary/gauge/trend aggregates must cover the full filtered result set, not only the current page.
* Run tables must default to newest-first sorting.
* Run tables should allow user sorting by duration and token count from the table header.
* Collapsed run table columns must prioritize operational diagnosis over raw tracing completeness.
* Analytics queries must fetch only after the user opens the Analytics tab.
* Analytics loading states must use Multica's existing Skeleton/loading patterns.
* Empty states must explain that metrics appear after agent runs finish and usage is recorded.
* Analytics must use one generic scoped endpoint for project and chat scopes.
* Generic analytics endpoint must enforce the existing Project detail and Chat detail visibility rules for each scope.
* Workflow/autopilot context should be shown as badges/metadata inside rows, while source filters remain `All`, `Issues`, and `Chats`.
* Ship backend contract, core schema/query options, Project/Chat UI tabs, reusable analytics surface, and tests as one end-to-end implementation change.
* Use React Query for analytics server state.
* Keep package boundaries:
  * `packages/core` owns API client, schemas/types, query options.
  * `packages/views` owns visual components and aggregation/presentation helpers.
  * `packages/ui` remains business-logic free.
* Project analytics must include tasks from both project issues and project-associated chats.
* Project analytics must show issue/chat attribution for each run.
* Chat analytics must show per-turn details and prompt/cache changes over turns.
* UI must include loading, empty, error, and no-metrics-yet states.
* Add i18n strings for tab labels and analytics copy.

## Acceptance Criteria

* [ ] Project detail shows a third `Analytics` tab.
* [ ] Chat session detail shows a fourth `Analytics` tab.
* [ ] Both tabs render through the same reusable analytics surface.
* [ ] Both tabs show primary gauges for total token consumption, total estimated price, and average completion speed.
* [ ] Primary gauges use compact numeric tiles with delta/progress accents, not radial gauges.
* [ ] Unpriced-token indicators can open the existing custom pricing dialog.
* [ ] Project analytics can filter/break down `All`, `Issues`, and `Chats`.
* [ ] Project analytics shows a full run table with issue/chat source attribution.
* [ ] Chat analytics shows a per-turn table with token, timing, cache, and tool metrics.
* [ ] Run tables default to high-signal columns and reveal full tracing metrics through expandable rows.
* [ ] Collapsed run table rows show source/turn, status, agent/model, duration, first text latency, tokens, estimated cost, cache rate, and tool calls.
* [ ] Run tables use server-side pagination with a default page size of 50.
* [ ] Primary gauges and trend charts remain stable when changing table pages because they aggregate across the full filtered window.
* [ ] Analytics queries are not issued until the Analytics tab is active.
* [ ] Analytics tab shows a skeleton/loading state consistent with existing Multica views while data is loading.
* [ ] Empty state copy says metrics appear after agent runs finish and usage is recorded.
* [ ] Run tables default to newest-first sorting and support duration/token sorting.
* [ ] Analytics uses a generic scoped endpoint for project/chat scopes.
* [ ] Generic analytics endpoint enforces project workspace visibility and chat ownership/private-agent access.
* [ ] Workflow/autopilot execution context is visible in run rows without adding extra top-level source filters.
* [ ] Backend contract, core schema/query options, Project/Chat UI tabs, reusable analytics surface, and tests ship together in one end-to-end change.
* [ ] Prompt/cache/context metrics from `task_usage.metadata` are surfaced in the UI.
* [ ] Project-scoped backend analytics include chat tasks whose `chat_session.project_id` matches the project.
* [ ] Analytics query code has typed core schemas/query options.
* [ ] Empty states explain that metrics appear after agent runs complete.
* [ ] Server tests cover project scoping and chat scoping.
* [ ] Frontend tests cover tab routing/rendering and key aggregation helpers.

## Out Of Scope

* Changing prompt construction or pruning behavior.
* Real-time streaming analytics while a task is still running.
* Organization-level analytics.
* Server-side model pricing/cost calculation.
* New analytics rollup tables unless raw scoped queries are too slow in verification.
* Exporting analytics to CSV/PDF.

## Technical Notes

* `packages/views/projects/components/project-detail.tsx` currently stores project tab state as `"issues" | "chats"` and branches between `ProjectIssuesSurface` and `ProjectChatsSurface`.
* `packages/views/chat/components/chat-pages.tsx` currently parses `ChatTab = "chat" | "issues" | "outputs"` from the `tab` query param.
* `ProjectChatsSurface` already queries project-scoped chat sessions with `chatSessionsOptions(wsId, { status: "all", scope: "project", projectId })`.
* Dashboard queries in `packages/core/dashboard/queries.ts` use stable keys with 60s stale time; analytics can mirror this cache behavior.
* Dashboard handlers currently accept optional `project_id`, but the existing rollup/query path is not enough for chat-level tracing metadata.
* Existing issue detail already has an issue usage pattern via `issueUsageOptions(issueId)`; this can inform small scoped usage UI but should not drive the new reusable analytics surface.
* Existing pricing utilities and custom pricing flow live under `packages/views/runtimes/utils.ts`, `packages/core/runtimes/custom-pricing-store.ts`, and `packages/views/runtimes/components/custom-pricing-dialog.tsx`.
* `GetProject` resolves projects through `GetProjectInWorkspace`; analytics project scope should use the same guard.
* Chat detail uses `gateChatSessionForUser`, combining chat ownership with private-agent visibility; analytics chat scope should use the same guard.
* `agent_task_queue` carries `issue_id`, `chat_session_id`, and `autopilot_run_id`; workflow runs can also point at issue/chat/autopilot context. Analytics should treat issue/chat as the primary source dimension and expose workflow/autopilot as execution context.

## Decisions

* MVP drilldown depth: use full run tables in both Project Analytics and Chat Analytics. Project rows include issue/chat attribution; chat rows focus on turn-level diagnosis.
* Run table density: default to high-signal columns, with expandable detail rows for full tracing fields.
* Primary gauges: both Project Analytics and Chat Analytics must display total token consumption, total estimated price, and average completion speed.
* Average completion speed: use median completed-task duration so the gauge reflects typical interaction speed instead of outliers.
* Default time window: Project Analytics defaults to `30d`; Chat Analytics defaults to all available runs.
* Failed/cancelled runs: token and price gauges include all runs with recorded token usage; speed gauges only include completed runs.
* Price display: unknown/unpriced models are excluded from total price and shown through a separate unpriced-token indicator.
* Gauge baseline: finite time windows compare against the previous equivalent period; Chat `All` mode shows absolute values because it has no previous equivalent period.
* Cost breakdown: the primary price gauge shows aggregate total only; input/output/cache-read/cache-write segments are shown in expandable or secondary detail areas.
* Live runs: Analytics shows only terminal runs with recorded usage/tracing data; in-progress rows are out of MVP.
* Run table scale: use server-side pagination with default page size 50; only the run table is paginated, not the aggregate gauges/trends.
* Default run table sorting: newest first; slowest and highest-token sorting are available as table sort modes.
* Default high-signal columns: collapsed run tables prioritize operational diagnosis columns: source/turn, status, agent/model, duration, first text latency, tokens, estimated cost, cache rate, and tool calls.
* Query activation: analytics data fetches only after the user opens the Analytics tab.
* Loading state: use Multica's existing `Skeleton`/loading design rather than introducing a new loading style.
* Empty state copy: use "Metrics appear after agent runs finish and usage is recorded."
* Unpriced model action: show unpriced tokens and provide a direct action to open the existing custom pricing dialog.
* Endpoint shape: use one generic scoped endpoint, `/api/analytics/agent-runs?scope=project|chat&scope_id=...`.
* Access control: project analytics mirrors Project detail visibility; chat analytics mirrors Chat detail ownership and private-agent access.
* Source taxonomy: Project source filters stay `All`, `Issues`, and `Chats`; workflow/autopilot context appears as badges or expanded row metadata.
* Implementation split: ship as one end-to-end change covering backend, core, Project/Chat UI, reusable surface, and tests.
* Primary gauge visual form: use compact numeric metric tiles with delta/progress accents; avoid radial/semi-circular gauges.

## Implementation Ready State

* No remaining product-shape blockers identified before implementation.
