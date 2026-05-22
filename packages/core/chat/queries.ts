import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { ChatPlanRun, ChatSessionListParams } from "../types";

export interface ChatSidebarRecentsParams {
  limit?: number;
  cursor?: string | null;
}

// NOTE on workspace scoping:
// `wsId` is used only as part of queryKey for cache isolation per workspace.
// The actual workspace context comes from ApiClient's X-Workspace-Slug header,
// which is set by the URL-driven [workspaceSlug] layout. Callers must ensure
// the header is in sync with the wsId they pass here — otherwise cache writes
// will be misattributed during a workspace switch race window.

export const chatKeys = {
  all: (wsId: string) => ["chat", wsId] as const,
  /** Full sessions list (active + archived); the dropdown splits locally. */
  sessions: (wsId: string, params?: ChatSessionListParams) =>
    params
      ? [...chatKeys.all(wsId), "sessions", normalizeChatSessionListParams(params)] as const
      : [...chatKeys.all(wsId), "sessions"] as const,
  sidebar: (wsId: string) => [...chatKeys.all(wsId), "sidebar"] as const,
  sidebarRecents: (wsId: string, params?: ChatSidebarRecentsParams) =>
    params
      ? [...chatKeys.all(wsId), "sidebar-recents", normalizeChatSidebarRecentsParams(params)] as const
      : [...chatKeys.all(wsId), "sidebar-recents"] as const,
  session: (wsId: string, id: string) => [...chatKeys.all(wsId), "session", id] as const,
  planEngines: (wsId: string) => [...chatKeys.all(wsId), "plan-engines"] as const,
  planRuns: (sessionId: string) => ["chat", "plan-runs", sessionId] as const,
  messages: (sessionId: string) => ["chat", "messages", sessionId] as const,
  issueProposals: (sessionId: string) => ["chat", "issue-proposals", sessionId] as const,
  issues: (sessionId: string) => ["chat", "issues", sessionId] as const,
  outputs: (sessionId: string) => ["chat", "outputs", sessionId] as const,
  pendingTask: (sessionId: string) => ["chat", "pending-task", sessionId] as const,
  /** Aggregate of in-flight chat tasks for the current user — FAB reads this. */
  pendingTasks: (wsId: string) => [...chatKeys.all(wsId), "pending-tasks"] as const,
  /** Per-task execution messages — shared with issue agent cards. */
  taskMessages: (taskId: string) => ["task-messages", taskId] as const,
};

function normalizeChatSessionListParams(params: ChatSessionListParams) {
  return {
    status: params.status ?? "active",
    scope: params.scope ?? "all",
    projectId: params.projectId ?? null,
  };
}

function normalizeChatSidebarRecentsParams(params: ChatSidebarRecentsParams) {
  return {
    limit: params.limit ?? 10,
    cursor: params.cursor ?? null,
  };
}

export function chatSessionsOptions(wsId: string, params?: ChatSessionListParams) {
  const resolvedParams = params ?? { status: "all" as const };
  return queryOptions({
    queryKey: chatKeys.sessions(wsId, params),
    queryFn: () => api.listChatSessions(resolvedParams),
    enabled: !!wsId,
    staleTime: Infinity,
  });
}

export function chatSidebarOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.sidebar(wsId),
    queryFn: () => api.listChatSidebar(),
    enabled: !!wsId,
    staleTime: Infinity,
  });
}

export function chatSidebarRecentsOptions(wsId: string, params?: ChatSidebarRecentsParams) {
  return queryOptions({
    queryKey: chatKeys.sidebarRecents(wsId, params),
    queryFn: () => api.listChatSidebarRecents(params),
    enabled: !!wsId,
    staleTime: Infinity,
  });
}

export function chatSessionOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: chatKeys.session(wsId, id),
    queryFn: () => api.getChatSession(id),
    enabled: !!id,
    staleTime: Infinity,
  });
}

export function chatPlanEnginesOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.planEngines(wsId),
    queryFn: () => api.listPlanEngines(),
    enabled: !!wsId,
    staleTime: Infinity,
  });
}

export function chatPlanRunsOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.planRuns(sessionId),
    queryFn: () => api.listChatPlanRuns(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

export function isActiveChatPlanRun(run: ChatPlanRun | null | undefined): run is ChatPlanRun {
  return !!run && run.status !== "completed" && run.status !== "cancelled" && run.status !== "failed";
}

export function chatMessagesOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.messages(sessionId),
    queryFn: () => api.listChatMessages(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

export function chatIssueProposalsOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.issueProposals(sessionId),
    queryFn: () => api.listChatIssueProposals(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

export function chatIssuesOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.issues(sessionId),
    queryFn: () => api.listChatSessionIssues(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

export function chatOutputsOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.outputs(sessionId),
    queryFn: () => api.listChatSessionOutputs(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

/**
 * Pending task for a chat session — the "is something still running?" signal.
 * Refetched via WS invalidation in useRealtimeSync when chat:message / chat:done
 * / task:completed / task:failed arrive.
 */
export function pendingChatTaskOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.pendingTask(sessionId),
    queryFn: () => api.getPendingChatTask(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

/**
 * Timeline for a single task — rendered by both the live chat view (while a
 * task is running) and AssistantMessage (for completed tasks). WS
 * `task:message` events seed this cache in real time via useRealtimeSync.
 */
export function taskMessagesOptions(taskId: string) {
  return queryOptions({
    queryKey: chatKeys.taskMessages(taskId),
    queryFn: () => api.listTaskMessages(taskId),
    enabled: !!taskId,
    staleTime: Infinity,
  });
}

/**
 * Aggregate of in-flight chat tasks for the current user in this workspace.
 * Drives the FAB "running" indicator while the chat window is minimised —
 * no per-session query is active then, so we need this roll-up.
 */
export function pendingChatTasksOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.pendingTasks(wsId),
    queryFn: () => api.listPendingChatTasks(),
    staleTime: Infinity,
  });
}
