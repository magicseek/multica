export type AgentAnalyticsScopeKind = "project" | "chat";
export type AgentAnalyticsSourceFilter = "all" | "issues" | "chats";
export type AgentAnalyticsSort = "newest" | "duration_desc" | "tokens_desc";

export interface GetAgentAnalyticsParams {
  scope: AgentAnalyticsScopeKind;
  scope_id: string;
  days?: number | "all";
  source?: AgentAnalyticsSourceFilter;
  limit?: number;
  offset?: number;
  sort?: AgentAnalyticsSort;
}

export interface AgentAnalyticsResponse {
  window: AgentAnalyticsWindow;
  summary: AgentAnalyticsSummary;
  previous_summary?: AgentAnalyticsSummary | null;
  daily: AgentAnalyticsDailyRow[];
  agents: AgentAnalyticsAgentRow[];
  sources?: AgentAnalyticsSourceRow[];
  runs: AgentAnalyticsRunRow[];
  pagination: AgentAnalyticsPagination;
}

export interface AgentAnalyticsWindow {
  days?: number | null;
  since?: string | null;
  before?: string | null;
  has_previous: boolean;
}

export interface AgentAnalyticsSummary {
  task_count: number;
  completed_count: number;
  failed_count: number;
  cancelled_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  median_completion_ms: number;
  p95_completion_ms: number;
  prompt_cache_read_rate: number;
  prompt_bytes: number;
  median_first_text_ms: number;
  tool_use_count: number;
  tool_result_bytes: number;
  model_usage: AgentAnalyticsModelUsage[];
}

export interface AgentAnalyticsModelUsage {
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  task_count: number;
}

export interface AgentAnalyticsDailyRow {
  date: string;
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  task_count: number;
  completed_count: number;
  failed_count: number;
  cancelled_count: number;
}

export interface AgentAnalyticsAgentRow {
  agent_id: string;
  agent_name: string;
  task_count: number;
  completed_count: number;
  failed_count: number;
  cancelled_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  median_completion_ms: number;
  model_usage: AgentAnalyticsModelUsage[];
}

export interface AgentAnalyticsSourceRow {
  source_type: string;
  source_id?: string | null;
  source_title?: string | null;
  issue_identifier?: string | null;
  task_count: number;
  completed_count: number;
  failed_count: number;
  cancelled_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  median_completion_ms: number;
  model_usage: AgentAnalyticsModelUsage[];
}

export interface AgentAnalyticsRunRow {
  task_id: string;
  agent_id: string;
  agent_name: string;
  status: string;
  source_type: string;
  issue_id?: string | null;
  issue_identifier?: string | null;
  issue_title?: string | null;
  chat_session_id?: string | null;
  chat_title?: string | null;
  autopilot_run_id?: string | null;
  workflow_definition_id?: string | null;
  workflow_run_id?: string | null;
  created_at?: string | null;
  dispatched_at?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
  total_duration_ms: number;
  queue_ms: number;
  startup_ms: number;
  execution_ms: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_tokens: number;
  model_usage: AgentAnalyticsModelUsage[];
  tracing: AgentAnalyticsTracing;
}

export interface AgentAnalyticsTracing {
  prompt_bytes: number;
  system_prompt_bytes: number;
  chat_message_bytes: number;
  chat_attachment_count: number;
  agent_instructions_bytes: number;
  agent_skill_count: number;
  agent_skill_bytes: number;
  repo_count: number;
  repository_count: number;
  project_resource_count: number;
  workflow_snapshot_bytes: number;
  workflow_step_snapshot_bytes: number;
  autopilot_description_bytes: number;
  autopilot_payload_bytes: number;
  quick_create_prompt_bytes: number;
  exec_env_ms: number;
  runtime_config_ms: number;
  backend_create_ms: number;
  agent_run_ms: number;
  daemon_run_ms: number;
  first_event_ms: number;
  first_text_ms: number;
  first_tool_use_ms: number;
  first_tool_result_ms: number;
  task_message_text_count: number;
  task_message_thinking_count: number;
  task_message_tool_use_count: number;
  task_message_tool_result_count: number;
  task_message_error_count: number;
  assistant_text_bytes: number;
  thinking_bytes: number;
  tool_input_bytes: number;
  tool_result_bytes: number;
  agent_result_output_bytes: number;
}

export interface AgentAnalyticsPagination {
  limit: number;
  offset: number;
  total: number;
}
