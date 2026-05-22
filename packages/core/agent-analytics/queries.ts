import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { GetAgentAnalyticsParams } from "../types";

const STALE_TIME = 60 * 1000;

export const agentAnalyticsKeys = {
  all: (wsId: string) => ["agent-analytics", wsId] as const,
  runs: (wsId: string, params: GetAgentAnalyticsParams) =>
    [
      ...agentAnalyticsKeys.all(wsId),
      params.scope,
      params.scope_id,
      params.days ?? "default",
      params.source ?? "all",
      params.limit ?? 50,
      params.offset ?? 0,
      params.sort ?? "newest",
    ] as const,
};

export function agentAnalyticsRunsOptions(
  wsId: string,
  params: GetAgentAnalyticsParams,
  enabled = true,
) {
  return queryOptions({
    queryKey: agentAnalyticsKeys.runs(wsId, params),
    queryFn: () => api.getAgentAnalyticsRuns(params),
    enabled: enabled && !!wsId && !!params.scope_id,
    staleTime: STALE_TIME,
  });
}
