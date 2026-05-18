import type { Issue } from "./issue";
import type { ListTaskOutputMetadataResponse } from "./repository";

export type ChatSessionStatus = "active" | "archived";
export type ChatProjectContextKind = "loose" | "project";
export type ChatSessionTitleSource = "legacy" | "first_message" | "agent_summary" | "user";

export interface ProjectContextSnapshot {
  id: string;
  workspace_id: string;
  title: string;
  icon: string | null;
  status: string;
  captured_at: string;
}

export interface ChatSession {
  id: string;
  workspace_id: string;
  agent_id: string;
  creator_id: string;
  title: string;
  status: ChatSessionStatus;
  default_repository_id: string | null;
  project_id: string | null;
  project_context_kind: ChatProjectContextKind;
  project_snapshot: ProjectContextSnapshot | null;
  title_source: ChatSessionTitleSource;
  /** True when the session has any unread assistant replies. List-only. */
  has_unread: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChatSessionListParams {
  status?: "active" | "archived" | "all";
  scope?: "loose" | "project";
  projectId?: string;
}

export interface ChatSidebarProject {
  id: string;
  title: string;
  icon: string | null;
  status: string;
}

export interface ChatSidebarProjectGroup {
  project: ChatSidebarProject;
  sessions: ChatSession[];
}

export interface ChatSidebarResponse {
  projects: ChatSidebarProjectGroup[];
  loose: ChatSession[];
}

export type ChatIssueProposalStatus = "pending" | "accepted" | "partially_accepted" | "dismissed";
export type ChatIssueProposalItemStatus = "pending" | "created" | "skipped";

export interface ChatIssueProposalItem {
  id: string;
  proposal_id: string;
  position: number;
  title: string;
  description: string;
  priority: string | null;
  labels: unknown[];
  assignee_type: string | null;
  assignee_id: string | null;
  status: ChatIssueProposalItemStatus;
  issue_id: string | null;
  approved_snapshot?: Record<string, unknown> | null;
  created_at: string;
  updated_at: string;
}

export interface ChatIssueProposal {
  id: string;
  workspace_id: string;
  chat_session_id: string;
  source_chat_message_id: string | null;
  source_task_id: string | null;
  proposer_agent_id: string | null;
  title: string;
  summary: string | null;
  status: ChatIssueProposalStatus;
  items: ChatIssueProposalItem[];
  created_at: string;
  updated_at: string;
}

export interface UpdateChatIssueProposalItemRequest {
  title: string;
  description: string;
  priority: string | null;
  labels: string[];
  assignee_type: "member" | "agent" | null;
  assignee_id: string | null;
}

export interface ApproveChatIssueProposalResponse {
  issues: Issue[];
  proposal: ChatIssueProposal;
}

export interface ChatSessionIssuesResponse {
  issues: Issue[];
  total: number;
}

export type ChatSessionOutputsResponse = ListTaskOutputMetadataResponse;

export interface PendingChatTaskItem {
  task_id: string;
  status: string;
  chat_session_id: string;
}

export interface PendingChatTasksResponse {
  tasks: PendingChatTaskItem[];
}

export interface ChatMessage {
  id: string;
  chat_session_id: string;
  role: "user" | "assistant";
  content: string;
  task_id: string | null;
  created_at: string;
  /**
   * Attachments linked to this message via the attachment table's
   * chat_message_id FK. Populated by ListChatMessages. UI renders these
   * as file/image cards inside the bubble; the markdown URL inline in
   * `content` may have an expiring signature, while attachment metadata
   * here is stable and the source of truth for click-time download.
   */
  attachments?: import("./attachment").Attachment[];
  /**
   * When set, this is an assistant message synthesized by the server's
   * FailTask fallback (mirrors the issue path's failure system comment).
   * `content` carries the raw daemon-reported errMsg; the front-end maps
   * `failure_reason` (an enum like "agent_error" / "connection_error" /
   * "timeout") to a user-facing label and renders a destructive bubble.
   * Null on success messages and on user messages.
   */
  failure_reason?: string | null;
  /**
   * Wall-clock duration from `task.created_at` (user hit send) to terminal
   * state (completed/failed). Set by the server on assistant messages
   * synthesized by CompleteTask/FailTask. UI renders it as "Replied in
   * 38s" / "Failed after 12s" beneath the bubble. Null on user messages
   * and on legacy assistant messages predating migration 063.
   */
  elapsed_ms?: number | null;
}

export interface SendChatMessageResponse {
  message_id: string;
  task_id: string;
  /**
   * Server-authoritative task creation time. Optimistic StatusPill seed
   * uses this as its anchor so the timer starts from the real `0s` —
   * without it the front-end falls back to its local clock and the
   * timer "snaps backwards" later when WS events update the cache.
   */
  created_at: string;
}

/**
 * Response from GET /api/chat/sessions/{id}/pending-task.
 * All fields are absent when the session has no in-flight task.
 *
 * `created_at` is the server-authoritative anchor for the chat StatusPill's
 * elapsed-seconds timer — the optimistic seed in chat-window.tsx fills in
 * task_id/status only, then this query catches up with the real created_at
 * so the timer survives refresh / reopen without "resetting to 0s".
 */
export interface ChatPendingTask {
  task_id?: string;
  status?: string;
  created_at?: string;
}
