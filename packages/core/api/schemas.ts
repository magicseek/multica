import { z } from "zod";
import type {
  Agent,
  AgentTemplate,
  AgentTemplateSummary,
  Attachment,
  CreateAgentFromTemplateResponse,
  GroupedIssuesResponse,
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
} from "../types";

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

const IssueSchema = z.object({
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
