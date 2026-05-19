import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { WorkflowApplicability } from "../types";

export const workflowKeys = {
  all: (wsId: string) => ["workflows", wsId] as const,
  list: (
    wsId: string,
    filter?: {
      applicability?: WorkflowApplicability;
      include_archived?: boolean;
    },
  ) => [...workflowKeys.all(wsId), "list", filter ?? {}] as const,
  detail: (wsId: string, id: string) =>
    [...workflowKeys.all(wsId), "detail", id] as const,
  runs: (
    wsId: string,
    filter: {
      issueId?: string;
      taskId?: string;
      chatSessionId?: string;
      autopilotRunId?: string;
    },
  ) => [...workflowKeys.all(wsId), "runs", filter] as const,
  runDetail: (wsId: string, id: string) =>
    [...workflowKeys.all(wsId), "run", id] as const,
};

export function workflowListOptions(
  wsId: string,
  filter?: {
    applicability?: WorkflowApplicability;
    include_archived?: boolean;
  },
) {
  return queryOptions({
    queryKey: workflowKeys.list(wsId, filter),
    queryFn: () => api.listWorkflows(filter),
  });
}

export function workflowDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: workflowKeys.detail(wsId, id),
    queryFn: () => api.getWorkflow(id),
    enabled: !!id,
  });
}

export function workflowRunListOptions(
  wsId: string,
  filter: {
    issueId?: string;
    taskId?: string;
    chatSessionId?: string;
    autopilotRunId?: string;
  },
) {
  return queryOptions({
    queryKey: workflowKeys.runs(wsId, filter),
    queryFn: () =>
      api.listWorkflowRuns({
        issue_id: filter.issueId,
        task_id: filter.taskId,
        chat_session_id: filter.chatSessionId,
        autopilot_run_id: filter.autopilotRunId,
      }),
    enabled:
      !!filter.issueId ||
      !!filter.taskId ||
      !!filter.chatSessionId ||
      !!filter.autopilotRunId,
  });
}

export function workflowRunDetailOptions(wsId: string, id: string | null | undefined) {
  return queryOptions({
    queryKey: workflowKeys.runDetail(wsId, id ?? ""),
    queryFn: () => api.getWorkflowRun(id ?? ""),
    enabled: !!id,
  });
}
