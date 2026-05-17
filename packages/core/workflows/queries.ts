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
