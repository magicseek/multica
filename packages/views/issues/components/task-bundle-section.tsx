"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronRight, Loader2, PackageCheck } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { issueKeys } from "@multica/core/issues/queries";
import { agentTaskSnapshotKeys } from "@multica/core/agents/queries";
import type { Agent, Issue, Squad } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { useT } from "../../i18n";

const MAX_BUNDLE_ITEMS = 5;

interface TaskBundleSectionProps {
  workspaceId: string;
  issue: Issue;
  issues: Issue[];
  agents: Agent[];
  squads: Squad[];
}

export function TaskBundleSection({
  workspaceId,
  issue,
  issues,
  agents,
  squads,
}: TaskBundleSectionProps) {
  const { t } = useT("issues");
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const requestEfficientAgents = useMemo(
    () =>
      agents.filter(
        (agent) =>
          !agent.archived_at &&
          !!agent.runtime_id &&
          agent.request_efficient_enabled === true,
      ),
    [agents],
  );
  const eligibleAgents = useMemo(
    () =>
      requestEfficientAgents.filter((agent) =>
        issueCanRunInBundle(issue, agent, squads),
      ),
    [issue, requestEfficientAgents, squads],
  );
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [selectedIssueIds, setSelectedIssueIds] = useState<Set<string>>(
    () => new Set([issue.id]),
  );
  const [creating, setCreating] = useState(false);
  const [rerunningBundleId, setRerunningBundleId] = useState<string | null>(null);

  useEffect(() => {
    setSelectedAgentId((current) => {
      if (eligibleAgents.some((agent) => agent.id === current)) return current;
      return eligibleAgents[0]?.id ?? requestEfficientAgents[0]?.id ?? "";
    });
  }, [eligibleAgents, requestEfficientAgents]);

  useEffect(() => {
    setSelectedIssueIds(new Set([issue.id]));
  }, [issue.id, selectedAgentId]);

  const selectedAgent =
    requestEfficientAgents.find((agent) => agent.id === selectedAgentId) ?? null;

  const candidateIssues = useMemo(() => {
    if (!selectedAgent) return [issue];
    const byId = new Map<string, Issue>();
    byId.set(issue.id, issue);
    for (const candidate of issues) {
      if (candidate.status === "done" || candidate.status === "cancelled") {
        continue;
      }
      if (issueCanRunInBundle(candidate, selectedAgent, squads)) {
        byId.set(candidate.id, candidate);
      }
    }
    return Array.from(byId.values()).sort((a, b) => {
      if (a.id === issue.id) return -1;
      if (b.id === issue.id) return 1;
      return a.identifier.localeCompare(b.identifier);
    });
  }, [issue, issues, selectedAgent, squads]);

  const { data: bundles = [] } = useQuery({
    queryKey: issueKeys.taskBundles(issue.id),
    queryFn: () => api.listTaskBundlesByIssue(issue.id),
    enabled: requestEfficientAgents.length > 0,
    staleTime: 30_000,
  });

  if (requestEfficientAgents.length === 0) return null;

  const selectedIds = Array.from(selectedIssueIds).filter((id) =>
    candidateIssues.some((candidate) => candidate.id === id),
  );
  const currentIssueEligible = selectedAgent
    ? issueCanRunInBundle(issue, selectedAgent, squads)
    : false;
  const canCreate =
    !!selectedAgent &&
    currentIssueEligible &&
    selectedIds.length > 0 &&
    selectedIds.length <= MAX_BUNDLE_ITEMS &&
    !creating;

  const toggleIssue = (issueId: string) => {
    if (issueId === issue.id) return;
    setSelectedIssueIds((prev) => {
      const next = new Set(prev);
      if (next.has(issueId)) {
        next.delete(issueId);
      } else if (next.size < MAX_BUNDLE_ITEMS) {
        next.add(issueId);
      }
      return next;
    });
  };

  const createBundle = async () => {
    if (!canCreate || !selectedAgent) return;
    setCreating(true);
    try {
      await api.createTaskBundle({
        agent_id: selectedAgent.id,
        issue_ids: selectedIds,
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: issueKeys.tasks(issue.id) }),
        queryClient.invalidateQueries({ queryKey: issueKeys.taskBundles(issue.id) }),
        queryClient.invalidateQueries({
          queryKey: agentTaskSnapshotKeys.list(workspaceId),
        }),
      ]);
      toast.success(t(($) => $.task_bundle.create_success));
      setOpen(true);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.task_bundle.create_failed),
      );
    } finally {
      setCreating(false);
    }
  };

  const rerunBundle = async (bundleId: string) => {
    if (rerunningBundleId) return;
    setRerunningBundleId(bundleId);
    try {
      await api.rerunTaskBundle(bundleId);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: issueKeys.tasks(issue.id) }),
        queryClient.invalidateQueries({ queryKey: issueKeys.taskBundles(issue.id) }),
        queryClient.invalidateQueries({
          queryKey: agentTaskSnapshotKeys.list(workspaceId),
        }),
      ]);
      toast.success(t(($) => $.task_bundle.rerun_success));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.task_bundle.rerun_failed),
      );
    } finally {
      setRerunningBundleId(null);
    }
  };

  return (
    <div>
      <button
        type="button"
        className={`mb-2 flex w-full items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors hover:bg-accent/70 ${
          open ? "" : "text-muted-foreground hover:text-foreground"
        }`}
        onClick={() => setOpen(!open)}
      >
        {t(($) => $.task_bundle.section)}
        <ChevronRight
          className={`!size-3 shrink-0 stroke-[2.5] text-muted-foreground transition-transform ${
            open ? "rotate-90" : ""
          }`}
        />
        {bundles.length > 0 && (
          <span className="ml-auto rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
            {bundles.length}
          </span>
        )}
      </button>

      {open && (
        <div className="space-y-2 pl-2">
          <div className="rounded-md border bg-muted/20 p-2">
            {eligibleAgents.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                {t(($) => $.task_bundle.assign_hint)}
              </p>
            ) : (
              <div className="space-y-2">
                <Select
                  value={selectedAgentId}
                  onValueChange={(value) => {
                    if (value) {
                      setSelectedAgentId(value);
                    }
                  }}
                >
                  <SelectTrigger size="sm" className="h-7 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {eligibleAgents.map((agent) => (
                      <SelectItem key={agent.id} value={agent.id}>
                        {agent.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <div className="max-h-40 space-y-0.5 overflow-y-auto pr-1">
                  {candidateIssues.map((candidate) => {
                    const checked = selectedIssueIds.has(candidate.id);
                    const disabled =
                      candidate.id !== issue.id &&
                      !checked &&
                      selectedIssueIds.size >= MAX_BUNDLE_ITEMS;
                    return (
                      <button
                        key={candidate.id}
                        type="button"
                        disabled={disabled}
                        onClick={() => toggleIssue(candidate.id)}
                        className="flex w-full items-center gap-2 rounded px-1 py-1 text-left text-xs transition-colors hover:bg-accent/50 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <Checkbox
                          checked={checked}
                          disabled={candidate.id === issue.id || disabled}
                          className="pointer-events-none"
                        />
                        <span className="shrink-0 text-muted-foreground">
                          {candidate.identifier}
                        </span>
                        <span className="truncate">{candidate.title}</span>
                      </button>
                    );
                  })}
                </div>

                <div className="flex items-center justify-between gap-2">
                  <span className="text-[11px] text-muted-foreground">
                    {t(($) => $.task_bundle.selected_count, {
                      count: selectedIds.length,
                      max: MAX_BUNDLE_ITEMS,
                    })}
                  </span>
                  <Button
                    type="button"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    disabled={!canCreate}
                    onClick={() => void createBundle()}
                  >
                    {creating && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
                    {t(($) => $.task_bundle.create_action)}
                  </Button>
                </div>
              </div>
            )}
          </div>

          {bundles.length > 0 && (
            <div className="space-y-1">
              {bundles.slice(0, 3).map((bundle) => {
                const done = bundle.items.filter(
                  (item) =>
                    item.status === "completed" ||
                    item.status === "failed" ||
                    item.status === "blocked" ||
                    item.status === "input_needed" ||
                    item.status === "cancelled",
                ).length;
                return (
                  <div
                    key={bundle.id}
                    className="flex items-center gap-2 rounded px-1 py-1 text-xs text-muted-foreground"
                  >
                    <PackageCheck className="h-3.5 w-3.5 shrink-0" />
                    <span className="capitalize">{bundle.status}</span>
                    <span className="ml-auto tabular-nums">
                      {done}/{bundle.items.length}
                    </span>
                    {(bundle.status === "blocked" || bundle.status === "failed") && (
                      <button
                        type="button"
                        onClick={() => void rerunBundle(bundle.id)}
                        disabled={!!rerunningBundleId}
                        className="rounded px-1 py-0.5 text-[11px] text-foreground transition-colors hover:bg-accent disabled:opacity-50"
                      >
                        {rerunningBundleId === bundle.id
                          ? t(($) => $.task_bundle.rerunning)
                          : t(($) => $.task_bundle.rerun_action)}
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function issueCanRunInBundle(issue: Issue, agent: Agent, squads: Squad[]): boolean {
  if (!issue.assignee_id) return false;
  if (issue.assignee_type === "agent") return issue.assignee_id === agent.id;
  if (issue.assignee_type === "squad") {
    const squad = squads.find((candidate) => candidate.id === issue.assignee_id);
    return !!squad && !squad.archived_at && squad.leader_id === agent.id;
  }
  return false;
}
