import { z } from "zod";
import type {
  Agent,
  AgentAnalyticsResponse,
  AgentTemplate,
  AgentTemplateSummary,
  Attachment,
  ApproveChatIssueProposalResponse,
  ChatIssueProposal,
  ChatIssueProposalItem,
  ChatMessage,
  ChatPlanConsultation,
  ChatPlanRun,
  ChatSessionIssuesResponse,
  ChatSession,
  ChatSidebarRecentsResponse,
  ChatSidebarResponse,
  CreateAgentFromTemplateResponse,
  GroupedIssuesResponse,
  ExportWorkflowResponse,
  ImportWorkflowResponse,
  ListIssuesResponse,
  ListProjectRepositoriesResponse,
  ListRepositoriesResponse,
  ListRepositoryBindingsResponse,
  ListRepositoryOperationsResponse,
  ListTaskOutputMetadataResponse,
  ProjectRepository,
  Repository,
  RepositoryBinding,
  RepositoryOperation,
  TaskOutputMetadata,
  TimelineEntry,
  WorkflowArtifact,
  WorkflowArtifactDiff,
  WorkflowDefinition,
  WorkflowInputRequest,
  WorkflowPreviewResponse,
  WorkflowQualityGateResult,
  WorkflowReview,
  WorkflowRevision,
  WorkflowRun,
  WorkflowStepRun,
  PlanEngineListResponse,
  SendChatMessageResponse,
} from "../types";
import { DEFAULT_CHAT_PLAN_ENGINE_ID } from "../types/chat";

// ---------------------------------------------------------------------------
// Schemas for the highest-risk API endpoints — those whose responses drive
// the issue detail page (timeline, comments, subscribers) and the issues
// list. These are the surfaces that white-screened in #2143 / #2147 / #2192.
//
// These schemas are intentionally LENIENT:
//   - String enums are stored as `z.string()` rather than `z.enum([...])`.
//     A new server-side enum value should render as a generic fallback in
//     the UI, never crash a `safeParse`.
//   - Optional fields are unioned with `null` and given fallbacks where
//     existing UI code already coerces them.
//   - Arrays default to `[]` so a missing `reactions` / `attachments` /
//     `entries` field doesn't take the page down.
//   - Every object schema ends with `.loose()` so unknown server-side
//     fields pass through unchanged. zod 4's `.object()` defaults to STRIP,
//     which would silently delete fields the schema didn't explicitly list
//     — fine while the TS type doesn't claim them, but the moment a future
//     PR adds a TS field without updating the schema, the cast `as T` lies
//     and the field shows up as `undefined` at runtime. `.loose()` removes
//     that synchronisation hazard.
//
// These schemas are deliberately not typed as `z.ZodType<TimelineEntry>` /
// `z.ZodType<Issue>` etc. — the strict TS types narrow string fields to
// literal unions, which would defeat the leniency above. `parseWithFallback`
// returns the parsed value cast to the caller-supplied `T`, so the strict
// type still flows out at the call site; the schema only guards shape.
// ---------------------------------------------------------------------------

const ReactionSchema = z.object({
  id: z.string(),
  comment_id: z.string(),
  actor_type: z.string(),
  actor_id: z.string(),
  emoji: z.string(),
  created_at: z.string(),
});

// Nested attachments embedded in timeline/comment responses stay lenient on
// purpose: a single malformed attachment must not knock the whole timeline
// into the fallback `[]`.
const AttachmentSchema = z.object({
  id: z.string(),
}).loose();

// Standalone attachment lookup (`GET /api/attachments/{id}`) is the source of
// truth for click-time download URLs. The two fields the download flow opens
// in a new tab — `download_url` and `url` — must be strings, otherwise we'd
// happily `window.open(undefined)`. `filename` gates the toast/title and is
// also enforced so a missing value falls back to the empty record below.
export const AttachmentResponseSchema = z.object({
  id: z.string(),
  url: z.string(),
  download_url: z.string(),
  filename: z.string(),
  chat_session_id: z.string().nullable().optional(),
  chat_message_id: z.string().nullable().optional(),
}).loose();

export const EMPTY_ATTACHMENT: Attachment = {
  id: "",
  workspace_id: "",
  issue_id: null,
  comment_id: null,
  chat_session_id: null,
  chat_message_id: null,
  uploader_type: "",
  uploader_id: "",
  filename: "",
  url: "",
  download_url: "",
  content_type: "",
  size_bytes: 0,
  created_at: "",
};

// All object schemas use `.loose()` so unknown server-side fields pass
// through unchanged. zod 4's `.object()` defaults to STRIP, which would
// silently drop new fields and surface as a "field neither showed up in
// the UI" mystery the next time the TS type adopted them but the schema
// wasn't updated in lock-step. `.loose()` removes that synchronisation
// hazard — the schema validates the shape it knows about and leaves the
// rest alone.
const TimelineEntrySchema = z.object({
  type: z.string(),
  id: z.string(),
  actor_type: z.string(),
  actor_id: z.string(),
  created_at: z.string(),
  action: z.string().optional(),
  details: z.record(z.string(), z.unknown()).optional(),
  content: z.string().optional(),
  parent_id: z.string().nullable().optional(),
  updated_at: z.string().optional(),
  comment_type: z.string().optional(),
  reactions: z.array(ReactionSchema).optional(),
  attachments: z.array(AttachmentSchema).optional(),
  coalesced_count: z.number().optional(),
}).loose();

// /timeline returns a flat array of TimelineEntry, oldest first. The
// previously cursor-paginated wrapper was removed (#1929) — at observed data
// sizes (p99 ~30 entries per issue) paged delivery only created bugs.
export const TimelineEntriesSchema = z.array(TimelineEntrySchema);

export const EMPTY_TIMELINE_ENTRIES: TimelineEntry[] = [];

export const CommentSchema = z.object({
  id: z.string(),
  issue_id: z.string(),
  author_type: z.string(),
  author_id: z.string(),
  content: z.string(),
  type: z.string(),
  parent_id: z.string().nullable(),
  reactions: z.array(ReactionSchema).default([]),
  attachments: z.array(AttachmentSchema).default([]),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const CommentsListSchema = z.array(CommentSchema);

export const IssueSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  number: z.number(),
  identifier: z.string(),
  title: z.string(),
  description: z.string().nullable(),
  status: z.string(),
  priority: z.string(),
  assignee_type: z.string().nullable(),
  assignee_id: z.string().nullable(),
  creator_type: z.string(),
  creator_id: z.string(),
  parent_issue_id: z.string().nullable(),
  project_id: z.string().nullable(),
  position: z.number(),
  due_date: z.string().nullable(),
  reactions: z.array(z.unknown()).optional(),
  labels: z.array(z.unknown()).optional(),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const ListIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_ISSUES_RESPONSE: ListIssuesResponse = {
  issues: [],
  total: 0,
};

const IssueAssigneeGroupSchema = z.object({
  id: z.string(),
  assignee_type: z.string().nullable(),
  assignee_id: z.string().nullable(),
  issues: z.array(IssueSchema).default([]),
  total: z.number().default(0),
}).loose();

export const GroupedIssuesResponseSchema = z.object({
  groups: z.array(IssueAssigneeGroupSchema).default([]),
}).loose();

export const EMPTY_GROUPED_ISSUES_RESPONSE: GroupedIssuesResponse = {
  groups: [],
};

const SubscriberSchema = z.object({
  issue_id: z.string(),
  user_type: z.string(),
  user_id: z.string(),
  reason: z.string(),
  created_at: z.string(),
}).loose();

export const SubscribersListSchema = z.array(SubscriberSchema);

export const ChildIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
}).loose();

// ---------------------------------------------------------------------------
// Workspace dashboard schemas
//
// The dashboard hits three independent rollup endpoints. Each returns a flat
// array, and every field is consumed by chart / KPI math — a missing number
// silently degrades to NaN downstream, so we coerce missing numbers to 0.
// String fields stay lenient (no enum narrowing) to survive future model /
// agent ID drift.
// ---------------------------------------------------------------------------

const DashboardUsageDailySchema = z.object({
  date: z.string(),
  model: z.string(),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  task_count: z.number().default(0),
}).loose();

export const DashboardUsageDailyListSchema = z.array(DashboardUsageDailySchema);

const DashboardUsageByAgentSchema = z.object({
  agent_id: z.string(),
  model: z.string(),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  task_count: z.number().default(0),
}).loose();

export const DashboardUsageByAgentListSchema = z.array(DashboardUsageByAgentSchema);

const DashboardAgentRunTimeSchema = z.object({
  agent_id: z.string(),
  total_seconds: z.number().default(0),
  task_count: z.number().default(0),
  failed_count: z.number().default(0),
}).loose();

export const DashboardAgentRunTimeListSchema = z.array(DashboardAgentRunTimeSchema);

const DashboardRunTimeDailySchema = z.object({
  date: z.string(),
  total_seconds: z.number().default(0),
  task_count: z.number().default(0),
  failed_count: z.number().default(0),
}).loose();

export const DashboardRunTimeDailyListSchema = z.array(DashboardRunTimeDailySchema);

// ---------------------------------------------------------------------------
// Agent analytics schemas
//
// Project/chat analytics is an operational diagnosis surface. Most fields are
// numeric and feed gauges/tables directly, so missing numbers fall back to 0
// while arrays default to [].
// ---------------------------------------------------------------------------

const AgentAnalyticsModelUsageSchema = z.object({
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  task_count: z.number().default(0),
}).loose();

const AgentAnalyticsSummarySchema = z.object({
  task_count: z.number().default(0),
  completed_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  median_completion_ms: z.number().default(0),
  p95_completion_ms: z.number().default(0),
  prompt_cache_read_rate: z.number().default(0),
  prompt_bytes: z.number().default(0),
  median_first_text_ms: z.number().default(0),
  tool_use_count: z.number().default(0),
  tool_result_bytes: z.number().default(0),
  model_usage: z.array(AgentAnalyticsModelUsageSchema).default([]),
}).loose();

const AgentAnalyticsWindowSchema = z.object({
  days: z.number().nullable().optional(),
  since: z.string().nullable().optional(),
  before: z.string().nullable().optional(),
  has_previous: z.boolean().default(false),
}).loose();

const AgentAnalyticsDailySchema = z.object({
  date: z.string(),
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  task_count: z.number().default(0),
  completed_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
}).loose();

const AgentAnalyticsAgentSchema = z.object({
  agent_id: z.string(),
  agent_name: z.string().default(""),
  task_count: z.number().default(0),
  completed_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  median_completion_ms: z.number().default(0),
  model_usage: z.array(AgentAnalyticsModelUsageSchema).default([]),
}).loose();

const AgentAnalyticsSourceSchema = z.object({
  source_type: z.string(),
  source_id: z.string().nullable().optional(),
  source_title: z.string().nullable().optional(),
  issue_identifier: z.string().nullable().optional(),
  task_count: z.number().default(0),
  completed_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  median_completion_ms: z.number().default(0),
  model_usage: z.array(AgentAnalyticsModelUsageSchema).default([]),
}).loose();

const AgentAnalyticsTracingSchema = z.object({
  prompt_bytes: z.number().default(0),
  system_prompt_bytes: z.number().default(0),
  chat_message_bytes: z.number().default(0),
  chat_attachment_count: z.number().default(0),
  agent_instructions_bytes: z.number().default(0),
  agent_skill_count: z.number().default(0),
  agent_skill_bytes: z.number().default(0),
  repo_count: z.number().default(0),
  repository_count: z.number().default(0),
  project_resource_count: z.number().default(0),
  workflow_snapshot_bytes: z.number().default(0),
  workflow_step_snapshot_bytes: z.number().default(0),
  autopilot_description_bytes: z.number().default(0),
  autopilot_payload_bytes: z.number().default(0),
  quick_create_prompt_bytes: z.number().default(0),
  exec_env_ms: z.number().default(0),
  runtime_config_ms: z.number().default(0),
  backend_create_ms: z.number().default(0),
  agent_run_ms: z.number().default(0),
  daemon_run_ms: z.number().default(0),
  first_event_ms: z.number().default(0),
  first_text_ms: z.number().default(0),
  first_tool_use_ms: z.number().default(0),
  first_tool_result_ms: z.number().default(0),
  task_message_text_count: z.number().default(0),
  task_message_thinking_count: z.number().default(0),
  task_message_tool_use_count: z.number().default(0),
  task_message_tool_result_count: z.number().default(0),
  task_message_error_count: z.number().default(0),
  assistant_text_bytes: z.number().default(0),
  thinking_bytes: z.number().default(0),
  tool_input_bytes: z.number().default(0),
  tool_result_bytes: z.number().default(0),
  agent_result_output_bytes: z.number().default(0),
}).loose();

const EMPTY_AGENT_ANALYTICS_TRACING = {
  prompt_bytes: 0,
  system_prompt_bytes: 0,
  chat_message_bytes: 0,
  chat_attachment_count: 0,
  agent_instructions_bytes: 0,
  agent_skill_count: 0,
  agent_skill_bytes: 0,
  repo_count: 0,
  repository_count: 0,
  project_resource_count: 0,
  workflow_snapshot_bytes: 0,
  workflow_step_snapshot_bytes: 0,
  autopilot_description_bytes: 0,
  autopilot_payload_bytes: 0,
  quick_create_prompt_bytes: 0,
  exec_env_ms: 0,
  runtime_config_ms: 0,
  backend_create_ms: 0,
  agent_run_ms: 0,
  daemon_run_ms: 0,
  first_event_ms: 0,
  first_text_ms: 0,
  first_tool_use_ms: 0,
  first_tool_result_ms: 0,
  task_message_text_count: 0,
  task_message_thinking_count: 0,
  task_message_tool_use_count: 0,
  task_message_tool_result_count: 0,
  task_message_error_count: 0,
  assistant_text_bytes: 0,
  thinking_bytes: 0,
  tool_input_bytes: 0,
  tool_result_bytes: 0,
  agent_result_output_bytes: 0,
};

const AgentAnalyticsRunSchema = z.object({
  task_id: z.string(),
  agent_id: z.string(),
  agent_name: z.string().default(""),
  status: z.string(),
  source_type: z.string(),
  issue_id: z.string().nullable().optional(),
  issue_identifier: z.string().nullable().optional(),
  issue_title: z.string().nullable().optional(),
  chat_session_id: z.string().nullable().optional(),
  chat_title: z.string().nullable().optional(),
  autopilot_run_id: z.string().nullable().optional(),
  workflow_definition_id: z.string().nullable().optional(),
  workflow_run_id: z.string().nullable().optional(),
  created_at: z.string().nullable().optional(),
  dispatched_at: z.string().nullable().optional(),
  started_at: z.string().nullable().optional(),
  completed_at: z.string().nullable().optional(),
  total_duration_ms: z.number().default(0),
  queue_ms: z.number().default(0),
  startup_ms: z.number().default(0),
  execution_ms: z.number().default(0),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  total_tokens: z.number().default(0),
  model_usage: z.array(AgentAnalyticsModelUsageSchema).default([]),
  tracing: AgentAnalyticsTracingSchema.default(EMPTY_AGENT_ANALYTICS_TRACING),
}).loose();

const AgentAnalyticsPaginationSchema = z.object({
  limit: z.number().default(50),
  offset: z.number().default(0),
  total: z.number().default(0),
}).loose();

const EMPTY_AGENT_ANALYTICS_SUMMARY = {
  task_count: 0,
  completed_count: 0,
  failed_count: 0,
  cancelled_count: 0,
  input_tokens: 0,
  output_tokens: 0,
  cache_read_tokens: 0,
  cache_write_tokens: 0,
  total_tokens: 0,
  median_completion_ms: 0,
  p95_completion_ms: 0,
  prompt_cache_read_rate: 0,
  prompt_bytes: 0,
  median_first_text_ms: 0,
  tool_use_count: 0,
  tool_result_bytes: 0,
  model_usage: [],
};

export const AgentAnalyticsResponseSchema = z.object({
  window: AgentAnalyticsWindowSchema.default({ has_previous: false }),
  summary: AgentAnalyticsSummarySchema.default(EMPTY_AGENT_ANALYTICS_SUMMARY),
  previous_summary: AgentAnalyticsSummarySchema.nullable().optional(),
  daily: z.array(AgentAnalyticsDailySchema).default([]),
  agents: z.array(AgentAnalyticsAgentSchema).default([]),
  sources: z.array(AgentAnalyticsSourceSchema).default([]),
  runs: z.array(AgentAnalyticsRunSchema).default([]),
  pagination: AgentAnalyticsPaginationSchema.default({ limit: 50, offset: 0, total: 0 }),
}).loose();

export const EMPTY_AGENT_ANALYTICS_RESPONSE: AgentAnalyticsResponse = {
  window: { has_previous: false },
  summary: EMPTY_AGENT_ANALYTICS_SUMMARY,
  previous_summary: null,
  daily: [],
  agents: [],
  sources: [],
  runs: [],
  pagination: { limit: 50, offset: 0, total: 0 },
};

// ---------------------------------------------------------------------------
// Agent template catalog — `/api/agent-templates*` and the
// create-from-template response. The desktop app's create-agent picker
// reaches these endpoints, and a future server change to the template shape
// would white-screen older installed builds (#2192 pattern) without these
// parsers. Lenient by the same rules as IssueSchema above: arrays default to
// `[]`, optional fields stay optional, `.loose()` lets unknown fields pass
// through unchanged.
// ---------------------------------------------------------------------------

const AgentTemplateSkillRefSchema = z.object({
  source_url: z.string(),
  cached_name: z.string().default(""),
  cached_description: z.string().default(""),
}).loose();

const AgentTemplateSummarySchemaBase = z.object({
  slug: z.string(),
  name: z.string(),
  description: z.string().default(""),
  category: z.string().optional(),
  icon: z.string().optional(),
  accent: z.string().optional(),
  // skills MUST default to [] — picker code reads `template.skills.length`
  // and `.map(...)`, both of which crash on `undefined`. The most common
  // future drift (field renamed / wrapped) lands here.
  skills: z.array(AgentTemplateSkillRefSchema).default([]),
}).loose();

export const AgentTemplateSummarySchema = AgentTemplateSummarySchemaBase;

// List endpoint historically returns a bare array. Server could legitimately
// migrate to `{templates: [...]}` later — we accept either shape so an old
// desktop survives the upgrade.
export const AgentTemplateSummaryListSchema = z.union([
  z.array(AgentTemplateSummarySchemaBase),
  z.object({ templates: z.array(AgentTemplateSummarySchemaBase).default([]) })
    .loose()
    .transform((v) => v.templates),
]);

export const EMPTY_AGENT_TEMPLATE_SUMMARY_LIST: AgentTemplateSummary[] = [];

export const AgentTemplateSchema = AgentTemplateSummarySchemaBase.extend({
  // Detail-only field. Default "" so a malformed detail still renders the
  // header + skill list; the user just sees an empty Instructions block.
  instructions: z.string().default(""),
}).loose();

// Used as the parse fallback for `GET /api/agent-templates/:slug`. Slug comes
// from the URL, so we round-trip the requested one back into the fallback
// at the call site (see `getAgentTemplate` in client.ts).
export const EMPTY_AGENT_TEMPLATE_DETAIL: AgentTemplate = {
  slug: "",
  name: "",
  description: "",
  skills: [],
  instructions: "",
};

// `agent` is a full Agent record — schematising every field would duplicate
// a 50-field interface and bit-rot fast. We keep it loose and require only
// `id`, the one field the create-from-template flow consumes (used to
// navigate to the new agent's detail page). Downstream code already
// optional-chains the rest.
const MinimalAgentSchema = z.object({
  id: z.string(),
}).loose();

export const CreateAgentFromTemplateResponseSchema = z.object({
  agent: MinimalAgentSchema,
  imported_skill_ids: z.array(z.string()).default([]),
  reused_skill_ids: z.array(z.string()).default([]),
}).loose();

// Fallback when the success response fails to parse. The agent server-side
// has likely been created already, so we can't pretend nothing happened —
// the caller (`create-agent-dialog.tsx`) is responsible for noticing
// `agent.id === ""` and skipping navigation while keeping the list
// invalidation, so the user finds their new agent in the list.
export const EMPTY_CREATE_AGENT_FROM_TEMPLATE_RESPONSE: CreateAgentFromTemplateResponse = {
  agent: { id: "" } as Agent,
  imported_skill_ids: [],
  reused_skill_ids: [],
};

// ---------------------------------------------------------------------------
// Repository workspace resources
//
// Repository responses are consumed by installed desktop builds and workspace
// settings. Keep the parser lenient: source/status strings may gain new
// server-side values, and unknown metadata must survive schema parsing.
// ---------------------------------------------------------------------------

const JsonObjectSchema = z.record(z.string(), z.unknown()).default({});

export const RepositorySchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string().default(""),
  source_state: z.string().default("remote_git"),
  remote_url: z.string().nullable().default(null),
  remote_key: z.string().nullable().default(null),
  default_branch: z.string().nullable().default(null),
  lead_agent_id: z.string().nullable().default(null),
  created_by: z.string().nullable().default(null),
  created_by_agent_id: z.string().nullable().default(null),
  status: z.string().default("ready"),
  metadata: JsonObjectSchema,
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  compatibility: z.boolean().optional(),
  compatibility_source: z.string().optional(),
}).loose();

export const EMPTY_REPOSITORY: Repository = {
  id: "",
  workspace_id: "",
  name: "",
  source_state: "remote_git",
  remote_url: null,
  remote_key: null,
  default_branch: null,
  lead_agent_id: null,
  created_by: null,
  created_by_agent_id: null,
  status: "ready",
  metadata: {},
  created_at: "",
  updated_at: "",
};

export const ListRepositoriesResponseSchema = z.object({
  repositories: z.array(RepositorySchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_REPOSITORIES_RESPONSE: ListRepositoriesResponse = {
  repositories: [],
  total: 0,
};

export const RepositoryBindingSchema = z.object({
  id: z.string(),
  repository_id: z.string(),
  workspace_id: z.string(),
  owner_user_id: z.string().nullable().default(null),
  daemon_id: z.string().default(""),
  runtime_id: z.string().nullable().default(null),
  machine_label: z.string().default(""),
  binding_kind: z.string().default("local_dir"),
  local_path: z.string().nullable().optional(),
  state: z.string().default("initializing"),
  last_seen_at: z.string().nullable().default(null),
  metadata: JsonObjectSchema,
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  local_path_visible: z.boolean().default(false),
}).loose();

export const EMPTY_REPOSITORY_BINDING: RepositoryBinding = {
  id: "",
  repository_id: "",
  workspace_id: "",
  owner_user_id: null,
  daemon_id: "",
  runtime_id: null,
  machine_label: "",
  binding_kind: "local_dir",
  local_path: null,
  state: "initializing",
  last_seen_at: null,
  metadata: {},
  created_at: "",
  updated_at: "",
  local_path_visible: false,
};

export const ListRepositoryBindingsResponseSchema = z.object({
  bindings: z.array(RepositoryBindingSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_REPOSITORY_BINDINGS_RESPONSE: ListRepositoryBindingsResponse = {
  bindings: [],
  total: 0,
};

export const ProjectRepositorySchema = z.object({
  project_id: z.string(),
  repository_id: z.string(),
  role: z.string().default("secondary"),
  position: z.number().default(0),
  created_at: z.string().default(""),
  repository: RepositorySchema,
}).loose();

export const EMPTY_PROJECT_REPOSITORY: ProjectRepository = {
  project_id: "",
  repository_id: "",
  role: "secondary",
  position: 0,
  created_at: "",
  repository: EMPTY_REPOSITORY,
};

export const ListProjectRepositoriesResponseSchema = z.object({
  repositories: z.array(ProjectRepositorySchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_PROJECT_REPOSITORIES_RESPONSE: ListProjectRepositoriesResponse = {
  repositories: [],
  total: 0,
};

export const RepositoryOperationSchema = z.object({
  id: z.string(),
  repository_id: z.string(),
  workspace_id: z.string(),
  operation_type: z.string().default("create_binding"),
  status: z.string().default("queued"),
  requested_by_type: z.string().default("member"),
  requested_by_id: z.string().nullable().default(null),
  target_daemon_id: z.string().nullable().default(null),
  target_runtime_id: z.string().nullable().default(null),
  binding_id: z.string().nullable().default(null),
  request: JsonObjectSchema,
  result: JsonObjectSchema,
  error: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  completed_at: z.string().nullable().default(null),
}).loose();

export const EMPTY_REPOSITORY_OPERATION: RepositoryOperation = {
  id: "",
  repository_id: "",
  workspace_id: "",
  operation_type: "create_binding",
  status: "queued",
  requested_by_type: "member",
  requested_by_id: null,
  target_daemon_id: null,
  target_runtime_id: null,
  binding_id: null,
  request: {},
  result: {},
  error: null,
  created_at: "",
  updated_at: "",
  completed_at: null,
};

export const ListRepositoryOperationsResponseSchema = z.object({
  operations: z.array(RepositoryOperationSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_REPOSITORY_OPERATIONS_RESPONSE: ListRepositoryOperationsResponse = {
  operations: [],
  total: 0,
};

export const TaskOutputMetadataSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  repository_id: z.string().nullable().default(null),
  task_id: z.string(),
  relative_path: z.string().default(""),
  filename: z.string().default(""),
  kind: z.string().default("unknown"),
  size_bytes: z.number().nullable().default(null),
  mime_type: z.string().nullable().default(null),
  metadata: JsonObjectSchema,
  created_at: z.string().default(""),
  source_type: z.string().nullable().optional(),
  source_issue_id: z.string().nullable().optional(),
  source_issue_identifier: z.string().nullable().optional(),
  source_issue_title: z.string().nullable().optional(),
}).loose();

export const EMPTY_TASK_OUTPUT_METADATA: TaskOutputMetadata = {
  id: "",
  workspace_id: "",
  repository_id: null,
  task_id: "",
  relative_path: "",
  filename: "",
  kind: "unknown",
  size_bytes: null,
  mime_type: null,
  metadata: {},
  created_at: "",
};

export const ListTaskOutputMetadataResponseSchema = z.object({
  outputs: z.array(TaskOutputMetadataSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_TASK_OUTPUT_METADATA_RESPONSE: ListTaskOutputMetadataResponse = {
  outputs: [],
  total: 0,
};

// ---------------------------------------------------------------------------
// Workflow definitions and runtime state
//
// Workflow data is rendered in Settings, issue detail, chat tasks, and
// autopilot runs. Desktop builds may keep running against newer servers, so
// unknown workflow fields must pass through while runtime arrays default to
// `[]`; the viewer maps steps/reviews/artifacts directly.
// ---------------------------------------------------------------------------

const WorkflowArtifactInputSchema = z.object({
  step_id: z.string().optional(),
  artifact_name: z.string().optional(),
  name: z.string().optional(),
  required: z.boolean().optional(),
}).loose();

const WorkflowStepSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
  title: z.string().default(""),
  order: z.number().optional(),
  required: z.boolean().optional(),
  depends_on: z.array(z.string()).optional(),
  execution: z.object({
    kind: z.string().optional(),
    prompt: z.string().optional(),
    rules: z.string().optional(),
  }).loose().optional(),
  artifact: z.object({
    name: z.string().optional(),
    content_kind: z.string().optional(),
    template: z.object({
      format: z.string().optional(),
      content: z.string().optional(),
      files: z.array(z.object({
        path: z.string().optional(),
        content: z.string().optional(),
      }).loose()).optional(),
    }).loose().optional(),
    inputs: z.array(WorkflowArtifactInputSchema).optional(),
  }).loose().optional(),
  input_artifacts: z.array(WorkflowArtifactInputSchema).optional(),
  input_requests: z.object({
    allowed: z.boolean().optional(),
    max_rounds: z.number().optional(),
    question_policy: z.string().optional(),
  }).loose().optional(),
  review: z.object({
    required: z.boolean().optional(),
  }).loose().optional(),
  quality_gate: z.object({
    enabled: z.boolean().optional(),
    blocking: z.boolean().optional(),
    prompt: z.string().optional(),
    report_mode: z.string().optional(),
  }).loose().optional(),
  body_template: z.string().optional(),
  description: z.string().optional(),
  checklist: z.array(z.string()).optional(),
}).loose();

export const WorkflowSchemaSchema = z.object({
  schema_version: z.number().optional(),
  version: z.number().optional(),
  name: z.string().optional(),
  description: z.string().optional(),
  applicability: z.array(z.string()).optional(),
  source: z.object({
    format: z.string().optional(),
    mode: z.string().optional(),
    body_template: z.string().optional(),
  }).loose().optional(),
  variables: z.array(z.object({
    key: z.string(),
    description: z.string().optional(),
    required: z.boolean().optional(),
  }).loose()).optional(),
  steps: z.array(WorkflowStepSchema).optional(),
  gates: z.array(z.object({
    id: z.string(),
    title: z.string(),
    description: z.string().optional(),
  }).loose()).optional(),
}).loose();

export const WorkflowRevisionSchema = z.object({
  id: z.string(),
  workflow_definition_id: z.string(),
  revision_number: z.number().default(0),
  status: z.string().default("draft"),
  schema: WorkflowSchemaSchema.default({}),
  created_by: z.string().nullable().default(null),
  published_at: z.string().nullable().default(null),
  deprecated_at: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_REVISION: WorkflowRevision = {
  id: "",
  workflow_definition_id: "",
  revision_number: 0,
  status: "draft",
  schema: {},
  created_by: null,
  published_at: null,
  deprecated_at: null,
  created_at: "",
  updated_at: "",
};

export const WorkflowDefinitionSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  name: z.string().default(""),
  description: z.string().default(""),
  origin: z.string().default("user"),
  system_key: z.string().nullable().default(null),
  forked_from_definition_id: z.string().nullable().default(null),
  current_published_revision_id: z.string().nullable().default(null),
  current_revision: WorkflowRevisionSchema.nullable().optional(),
  created_by: z.string().nullable().default(null),
  archived_at: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_DEFINITION: WorkflowDefinition = {
  id: "",
  workspace_id: "",
  name: "",
  description: "",
  origin: "user",
  system_key: null,
  forked_from_definition_id: null,
  current_published_revision_id: null,
  current_revision: null,
  created_by: null,
  archived_at: null,
  created_at: "",
  updated_at: "",
};

export const WorkflowDefinitionListSchema = z.array(WorkflowDefinitionSchema);

export const WorkflowPreviewResponseSchema = z.object({
  rendered_markdown: z.string().default(""),
  warnings: z.array(z.string()).nullable().default([]),
}).loose();

export const EMPTY_WORKFLOW_PREVIEW_RESPONSE: WorkflowPreviewResponse = {
  rendered_markdown: "",
  warnings: [],
};

export const ImportWorkflowResponseSchema = z.object({
  workflow: WorkflowDefinitionSchema,
  warnings: z.array(z.string()).nullable().default([]),
}).loose();

export const EMPTY_IMPORT_WORKFLOW_RESPONSE: ImportWorkflowResponse = {
  workflow: EMPTY_WORKFLOW_DEFINITION,
  warnings: [],
};

export const ExportWorkflowResponseSchema = z.object({
  format: z.string().default("yaml"),
  content: z.string().default(""),
  warnings: z.array(z.string()).nullable().default([]),
}).loose();

export const EMPTY_EXPORT_WORKFLOW_RESPONSE: ExportWorkflowResponse = {
  format: "yaml",
  content: "",
  warnings: [],
};

export const WorkflowStepRunSchema = z.object({
  id: z.string(),
  workflow_run_id: z.string().default(""),
  step_definition_id: z.string().default(""),
  title: z.string().default(""),
  order_index: z.number().default(0),
  required: z.boolean().default(false),
  status: z.string().default("pending"),
  execution_kind: z.string().default("agent"),
  attempt: z.number().default(1),
  depends_on_step_ids: z.unknown().optional(),
  artifact_inputs: z.unknown().optional(),
  snapshot: z.unknown().optional(),
  started_at: z.string().nullable().optional(),
  completed_at: z.string().nullable().optional(),
  error: z.string().nullable().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_STEP_RUN: WorkflowStepRun = {
  id: "",
  workflow_run_id: "",
  step_definition_id: "",
  title: "",
  order_index: 0,
  required: false,
  status: "pending",
  execution_kind: "agent",
  attempt: 1,
  created_at: "",
  updated_at: "",
};

export const WorkflowArtifactSchema = z.object({
  id: z.string(),
  workflow_run_id: z.string().default(""),
  workflow_step_run_id: z.string().default(""),
  logical_name: z.string().default(""),
  version: z.number().default(1),
  content_kind: z.string().default("text"),
  content_text: z.string().nullable().optional(),
  content_json: z.unknown().optional(),
  producer_type: z.string().default("member"),
  producer_id: z.string().nullable().optional(),
  supersedes_artifact_id: z.string().nullable().optional(),
  created_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_ARTIFACT: WorkflowArtifact = {
  id: "",
  workflow_run_id: "",
  workflow_step_run_id: "",
  logical_name: "",
  version: 1,
  content_kind: "text",
  producer_type: "member",
  created_at: "",
};

export const WorkflowReviewSchema = z.object({
  id: z.string(),
  workflow_run_id: z.string().default(""),
  workflow_step_run_id: z.string().nullable().optional(),
  workflow_artifact_id: z.string().nullable().optional(),
  status: z.string().default("requested"),
  reviewer_id: z.string().nullable().optional(),
  decision_notes: z.string().nullable().optional(),
  reviewed_at: z.string().nullable().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_REVIEW: WorkflowReview = {
  id: "",
  workflow_run_id: "",
  status: "requested",
  created_at: "",
  updated_at: "",
};

export const WorkflowQualityGateResultSchema = z.object({
  id: z.string(),
  workflow_run_id: z.string().default(""),
  workflow_step_run_id: z.string().default(""),
  workflow_artifact_id: z.string().nullable().optional(),
  status: z.string().default("pass"),
  blocking: z.boolean().default(false),
  producer_type: z.string().default("member"),
  producer_id: z.string().nullable().optional(),
  report_text: z.string().nullable().optional(),
  report_json: z.unknown().optional(),
  created_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_QUALITY_GATE_RESULT: WorkflowQualityGateResult = {
  id: "",
  workflow_run_id: "",
  workflow_step_run_id: "",
  status: "pass",
  blocking: false,
  producer_type: "member",
  created_at: "",
};

export const WorkflowInputRequestSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  workflow_run_id: z.string().default(""),
  workflow_step_run_id: z.string().default(""),
  issue_id: z.string().nullable().optional(),
  chat_session_id: z.string().nullable().optional(),
  question_comment_id: z.string().nullable().optional(),
  answer_comment_id: z.string().nullable().optional(),
  requester_agent_id: z.string().nullable().optional(),
  responder_id: z.string().nullable().optional(),
  status: z.string().default("requested"),
  question_text: z.string().default(""),
  answer_text: z.string().nullable().optional(),
  round_index: z.number().default(1),
  max_rounds: z.number().default(1),
  requested_at: z.string().default(""),
  answered_at: z.string().nullable().optional(),
  cancelled_at: z.string().nullable().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKFLOW_INPUT_REQUEST: WorkflowInputRequest = {
  id: "",
  workspace_id: "",
  workflow_run_id: "",
  workflow_step_run_id: "",
  status: "requested",
  question_text: "",
  round_index: 1,
  max_rounds: 1,
  requested_at: "",
  created_at: "",
  updated_at: "",
};

export const WorkflowArtifactDiffSchema = z.object({
  logical_name: z.string().default(""),
  base_version: z.number().default(0),
  target_version: z.number().default(0),
  content_kind: z.string().default("text"),
  unified_diff: z.string().default(""),
  summary: z.string().nullable().optional(),
}).loose();

export const EMPTY_WORKFLOW_ARTIFACT_DIFF: WorkflowArtifactDiff = {
  logical_name: "",
  base_version: 0,
  target_version: 0,
  content_kind: "text",
  unified_diff: "",
};

export const WorkflowRunSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  agent_task_queue_id: z.string().default(""),
  issue_id: z.string().nullable().optional(),
  chat_session_id: z.string().nullable().optional(),
  autopilot_run_id: z.string().nullable().optional(),
  workflow_definition_id: z.string().nullable().optional(),
  workflow_revision_id: z.string().nullable().optional(),
  trigger_type: z.string().default(""),
  snapshot: z.unknown().optional(),
  status: z.string().default("queued"),
  started_at: z.string().nullable().optional(),
  completed_at: z.string().nullable().optional(),
  cancelled_at: z.string().nullable().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  steps: z.array(WorkflowStepRunSchema).default([]),
  artifacts: z.array(WorkflowArtifactSchema).default([]),
  reviews: z.array(WorkflowReviewSchema).default([]),
  quality_gate_results: z.array(WorkflowQualityGateResultSchema).default([]),
  input_requests: z.array(WorkflowInputRequestSchema).default([]),
  current_step: WorkflowStepRunSchema.nullable().optional(),
}).loose();

export const EMPTY_WORKFLOW_RUN: WorkflowRun = {
  id: "",
  workspace_id: "",
  agent_task_queue_id: "",
  trigger_type: "",
  status: "queued",
  created_at: "",
  updated_at: "",
  steps: [],
  artifacts: [],
  reviews: [],
  quality_gate_results: [],
  input_requests: [],
};

export const WorkflowRunListSchema = z.array(WorkflowRunSchema);

// ---------------------------------------------------------------------------
// Chat sessions
//
// Route-backed chat pages consume these responses directly, so we parse the
// project/session hierarchy with the same lenient posture used for issues:
// string enums can grow server-side, unknown fields pass through, and missing
// arrays fall back to empty lists.
// ---------------------------------------------------------------------------

const ProjectContextSnapshotSchema = z.object({
  id: z.string().default(""),
  workspace_id: z.string().default(""),
  title: z.string().default(""),
  icon: z.string().nullable().default(null),
  status: z.string().default(""),
  captured_at: z.string().default(""),
}).loose();

export const ChatSessionSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  agent_id: z.string(),
  creator_id: z.string(),
  title: z.string().default(""),
  status: z.string().default("active"),
  default_repository_id: z.string().nullable().default(null),
  project_id: z.string().nullable().default(null),
  project_context_kind: z.string().default("loose"),
  project_snapshot: ProjectContextSnapshotSchema.nullable().default(null),
  title_source: z.string().default("legacy"),
  has_unread: z.boolean().default(false),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const ChatSessionListSchema = z.array(ChatSessionSchema);

export const EMPTY_CHAT_SESSION: ChatSession = {
  id: "",
  workspace_id: "",
  agent_id: "",
  creator_id: "",
  title: "",
  status: "active",
  default_repository_id: null,
  project_id: null,
  project_context_kind: "loose",
  project_snapshot: null,
  title_source: "legacy",
  has_unread: false,
  created_at: "",
  updated_at: "",
};

const ChatSidebarProjectSchema = z.object({
  id: z.string(),
  title: z.string().default(""),
  icon: z.string().nullable().default(null),
  status: z.string().default("active"),
}).loose();

const ChatSidebarProjectGroupSchema = z.object({
  project: ChatSidebarProjectSchema,
  sessions: z.array(ChatSessionSchema).default([]),
}).loose();

export const ChatSidebarResponseSchema = z.object({
  projects: z.array(ChatSidebarProjectGroupSchema).default([]),
  loose: z.array(ChatSessionSchema).default([]),
  loose_next_cursor: z.string().nullable().default(null),
  loose_has_more: z.boolean().default(false),
}).loose();

export const EMPTY_CHAT_SIDEBAR_RESPONSE: ChatSidebarResponse = {
  projects: [],
  loose: [],
  loose_next_cursor: null,
  loose_has_more: false,
};

export const ChatSidebarRecentsResponseSchema = z.object({
  sessions: z.array(ChatSessionSchema).default([]),
  next_cursor: z.string().nullable().default(null),
  has_more: z.boolean().default(false),
}).loose();

export const EMPTY_CHAT_SIDEBAR_RECENTS_RESPONSE: ChatSidebarRecentsResponse = {
  sessions: [],
  next_cursor: null,
  has_more: false,
};

const PlanEngineSchema = z.object({
  id: z.string(),
  label: z.string().optional(),
  display_label: z.string().optional(),
  description: z.string().default(""),
  version: z.string().default(""),
  is_default: z.boolean().optional(),
}).loose().transform((engine) => ({
  ...engine,
  label: engine.label ?? engine.display_label ?? engine.id,
  description: engine.description ?? "",
  version: engine.version ?? "",
}));

const PlanEngineEnvelopeSchema = z.object({
  engines: z.array(PlanEngineSchema).default([]),
  default_engine: z.string().nullable().optional(),
}).loose().transform((response) => ({
  engines: response.engines,
  default_engine:
    response.default_engine ??
    response.engines.find((engine) => engine.is_default === true)?.id ??
    DEFAULT_CHAT_PLAN_ENGINE_ID,
}));

export const PlanEngineListResponseSchema = z.union([
  z.array(PlanEngineSchema).transform((engines) => ({
    engines,
    default_engine: engines.find((engine) => engine.is_default === true)?.id ?? DEFAULT_CHAT_PLAN_ENGINE_ID,
  })),
  PlanEngineEnvelopeSchema,
]);

export const EMPTY_PLAN_ENGINE_LIST_RESPONSE: PlanEngineListResponse = {
  engines: [],
  default_engine: DEFAULT_CHAT_PLAN_ENGINE_ID,
};

export const PlanSummarySchema = z.object({
  confirmed_requirements: z.array(z.string()).default([]),
  rejected_options: z.array(z.string()).default([]),
  consensus_notes: z.array(z.string()).default([]),
  open_questions: z.array(z.string()).default([]),
}).loose();

export const EMPTY_PLAN_SUMMARY = {
  confirmed_requirements: [],
  rejected_options: [],
  consensus_notes: [],
  open_questions: [],
};

export const ChatPlanRunSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  chat_session_id: z.string().default(""),
  creator_user_id: z.string().default(""),
  actor_type: z.string().default("agent"),
  actor_id: z.string().default(""),
  lead_agent_id: z.string().default(""),
  plan_engine: z.string().default(DEFAULT_CHAT_PLAN_ENGINE_ID),
  engine_version: z.string().default(""),
  status: z.string().default("brainstorming"),
  initial_message_id: z.string().nullable().default(null),
  latest_message_id: z.string().nullable().default(null),
  summary: PlanSummarySchema.nullable().default(null),
  consultation_wave_count: z.number().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  completed_at: z.string().nullable().optional(),
  cancelled_at: z.string().nullable().optional(),
  failed_at: z.string().nullable().optional(),
}).loose();

const ChatPlanRunListResponseSchema = z.object({
  plan_runs: z.array(ChatPlanRunSchema).default([]),
  total: z.number().default(0),
}).loose();

export const ChatPlanRunListSchema = z.union([
  z.array(ChatPlanRunSchema),
  ChatPlanRunListResponseSchema.transform((response) => response.plan_runs),
]);

export const EMPTY_CHAT_PLAN_RUN: ChatPlanRun = {
  id: "",
  workspace_id: "",
  chat_session_id: "",
  creator_user_id: "",
  actor_type: "agent",
  actor_id: "",
  lead_agent_id: "",
  plan_engine: DEFAULT_CHAT_PLAN_ENGINE_ID,
  engine_version: "",
  status: "cancelled",
  initial_message_id: null,
  latest_message_id: null,
  summary: null,
  consultation_wave_count: 0,
  created_at: "",
  updated_at: "",
};

export const EMPTY_CHAT_PLAN_RUNS: ChatPlanRun[] = [];

export const ChatPlanConsultationSchema = z.object({
  id: z.string(),
  plan_run_id: z.string().default(""),
  requester_agent_id: z.string().default(""),
  target_agent_id: z.string().default(""),
  request_message_id: z.string().nullable().default(null),
  response_message_id: z.string().nullable().default(null),
  task_id: z.string().nullable().default(null),
  status: z.string().default("pending"),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const ChatPlanConsultationListSchema = z.array(ChatPlanConsultationSchema);
export const EMPTY_CHAT_PLAN_CONSULTATIONS: ChatPlanConsultation[] = [];

export const ChatMessageActorSchema = z.object({
  type: z.string().default("member"),
  id: z.string().nullable().optional(),
}).loose();

export const ChatMessageRecipientSchema = z.object({
  id: z.string(),
  message_id: z.string().default(""),
  recipient_type: z.string().default("agent"),
  recipient_id: z.string().default(""),
  resolved_agent_id: z.string().nullable().default(null),
  source: z.string().default("explicit_mention"),
  status: z.string().default("pending"),
  task_id: z.string().nullable().default(null),
  warning_code: z.string().default(""),
  warning_message: z.string().default(""),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const ChatRoutingWarningSchema = z.object({
  recipient_id: z.string().default(""),
  code: z.string().default(""),
  message: z.string().default(""),
}).loose();

export const ChatMessageSchema = z.object({
  id: z.string(),
  chat_session_id: z.string().default(""),
  role: z.string().default("assistant"),
  content: z.string().default(""),
  task_id: z.string().nullable().default(null),
  author_type: z.string().nullable().optional(),
  author_member_id: z.string().nullable().optional(),
  author_agent_id: z.string().nullable().optional(),
  plan_run_id: z.string().nullable().optional(),
  consultation_id: z.string().nullable().optional(),
  reply_to_message_id: z.string().nullable().optional(),
  sender: ChatMessageActorSchema.optional(),
  recipients: z.array(ChatMessageRecipientSchema).default([]),
  routing_warnings: z.array(ChatRoutingWarningSchema).default([]),
  created_at: z.string().default(""),
  attachments: z.array(AttachmentSchema).optional(),
  failure_reason: z.string().nullable().optional(),
  elapsed_ms: z.number().nullable().optional(),
}).loose();

export const ChatMessageListSchema = z.array(ChatMessageSchema);
export const EMPTY_CHAT_MESSAGES: ChatMessage[] = [];

export const ChatIssueProposalItemSchema = z.object({
  id: z.string(),
  proposal_id: z.string(),
  position: z.number().default(0),
  title: z.string().default(""),
  description: z.string().default(""),
  priority: z.string().nullable().default(null),
  labels: z.array(z.unknown()).default([]),
  assignee_type: z.string().nullable().default(null),
  assignee_id: z.string().nullable().default(null),
  status: z.string().default("pending"),
  issue_id: z.string().nullable().default(null),
  approved_snapshot: JsonObjectSchema.nullable().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_CHAT_ISSUE_PROPOSAL_ITEM: ChatIssueProposalItem = {
  id: "",
  proposal_id: "",
  position: 0,
  title: "",
  description: "",
  priority: null,
  labels: [],
  assignee_type: null,
  assignee_id: null,
  status: "pending",
  issue_id: null,
  approved_snapshot: null,
  created_at: "",
  updated_at: "",
};

export const ChatIssueProposalSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  chat_session_id: z.string(),
  source_chat_message_id: z.string().nullable().default(null),
  source_task_id: z.string().nullable().default(null),
  proposer_agent_id: z.string().nullable().default(null),
  source_plan_run_id: z.string().nullable().default(null),
  title: z.string().default(""),
  summary: z.string().nullable().default(null),
  status: z.string().default("pending"),
  items: z.array(ChatIssueProposalItemSchema).default([]),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

const ChatIssueProposalListResponseSchema = z.object({
  proposals: z.array(ChatIssueProposalSchema).default([]),
  total: z.number().default(0),
}).loose();

export const ChatIssueProposalListSchema = z.union([
  z.array(ChatIssueProposalSchema),
  ChatIssueProposalListResponseSchema.transform((response) => response.proposals),
]);

export const EMPTY_CHAT_ISSUE_PROPOSALS: ChatIssueProposal[] = [];

export const ChatIssueProposalApprovalResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
  proposal: ChatIssueProposalSchema,
}).loose();

export const EMPTY_CHAT_ISSUE_PROPOSAL_APPROVAL_RESPONSE: ApproveChatIssueProposalResponse = {
  issues: [],
  proposal: {
    id: "",
    workspace_id: "",
    chat_session_id: "",
    source_chat_message_id: null,
    source_task_id: null,
    proposer_agent_id: null,
    source_plan_run_id: null,
    title: "",
    summary: null,
    status: "pending",
    items: [],
    created_at: "",
    updated_at: "",
  },
};

export const ChatSessionIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_CHAT_SESSION_ISSUES_RESPONSE: ChatSessionIssuesResponse = {
  issues: [],
  total: 0,
};

export const SendChatMessageResponseSchema = z.object({
  message_id: z.string().default(""),
  task_id: z.string().default(""),
  agent_id: z.string().nullable().optional(),
  plan_run_id: z.string().nullable().optional(),
  created_at: z.string().default(""),
}).loose();

export const EMPTY_SEND_CHAT_MESSAGE_RESPONSE: SendChatMessageResponse = {
  message_id: "",
  task_id: "",
  agent_id: null,
  plan_run_id: null,
  created_at: "",
};
