"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronRight, Loader2, PackageCheck } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { issueKeys } from "@multica/core/issues/queries";
import { agentTaskSnapshotKeys } from "@multica/core/agents/queries";
import type {
  Agent,
  Issue,
  Squad,
  TaskBundle,
  TaskBundleItemStatus,
} from "@multica/core/types";
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
const STARTABLE_ISSUE_STATUSES = new Set<Issue["status"]>(["todo", "blocked"]);
const FINAL_BUNDLE_ITEM_STATUSES = new Set<TaskBundleItemStatus>([
  "completed",
  "failed",
  "blocked",
  "input_needed",
  "cancelled",
]);

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
    () => new Set(isIssueStartableForNewBundle(issue) ? [issue.id] : []),
  );
  const [creating, setCreating] = useState(false);
  const [rerunningBundleId, setRerunningBundleId] = useState<string | null>(null);
  const currentIssueStartable = isIssueStartableForNewBundle(issue);
  const issueById = useMemo(() => {
    const byId = new Map<string, Issue>();
    byId.set(issue.id, issue);
    for (const candidate of issues) byId.set(candidate.id, candidate);
    return byId;
  }, [issue, issues]);

  useEffect(() => {
    setSelectedAgentId((current) => {
      if (eligibleAgents.some((agent) => agent.id === current)) return current;
      return eligibleAgents[0]?.id ?? requestEfficientAgents[0]?.id ?? "";
    });
  }, [eligibleAgents, requestEfficientAgents]);

  useEffect(() => {
    setSelectedIssueIds(new Set(currentIssueStartable ? [issue.id] : []));
  }, [currentIssueStartable, issue.id, selectedAgentId]);

  const selectedAgent =
    requestEfficientAgents.find((agent) => agent.id === selectedAgentId) ?? null;

  const candidateIssues = useMemo(() => {
    if (!selectedAgent || !currentIssueStartable) return [];
    const byId = new Map<string, Issue>();
    byId.set(issue.id, issue);
    for (const candidate of issues) {
      if (!isIssueStartableForNewBundle(candidate)) {
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
  }, [currentIssueStartable, issue, issues, selectedAgent, squads]);

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
  const creationAvailable =
    eligibleAgents.length > 0 && currentIssueEligible && currentIssueStartable;
  const shouldRender =
    bundles.length > 0 || creationAvailable || eligibleAgents.length === 0;
  if (!shouldRender) return null;

  const canCreate =
    !!selectedAgent &&
    creationAvailable &&
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

  const bundleStatusLabel = (status: TaskBundle["status"]): string => {
    switch (status) {
      case "queued":
        return t(($) => $.task_bundle.status_queued);
      case "running":
        return t(($) => $.task_bundle.status_running);
      case "completed":
        return t(($) => $.task_bundle.status_completed);
      case "failed":
        return t(($) => $.task_bundle.status_failed);
      case "blocked":
        return t(($) => $.task_bundle.status_blocked);
      case "cancelled":
        return t(($) => $.task_bundle.status_cancelled);
    }
  };

  const bundleItemStatusLabel = (status: TaskBundleItemStatus): string => {
    switch (status) {
      case "queued":
        return t(($) => $.task_bundle.item_status_queued);
      case "in_progress":
        return t(($) => $.task_bundle.item_status_running);
      case "completed":
        return t(($) => $.task_bundle.item_status_completed);
      case "failed":
        return t(($) => $.task_bundle.item_status_failed);
      case "blocked":
        return t(($) => $.task_bundle.item_status_blocked);
      case "input_needed":
        return t(($) => $.task_bundle.item_status_input_needed);
      case "cancelled":
        return t(($) => $.task_bundle.item_status_cancelled);
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
          {bundles.length > 0 && (
            <div className="space-y-2">
              {bundles.slice(0, 3).map((bundle) => {
                const done = bundle.items.filter((item) =>
                  FINAL_BUNDLE_ITEM_STATUSES.has(item.status),
                ).length;
                return (
                  <div key={bundle.id} className="rounded-md border bg-muted/20 p-2">
                    <div className="flex min-w-0 items-center gap-2 text-xs">
                      <PackageCheck className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                      <span className="min-w-0 truncate font-medium text-foreground">
                        {bundleStatusLabel(bundle.status)}
                      </span>
                      <span className="shrink-0 tabular-nums text-muted-foreground">
                        {done}/{bundle.items.length}
                      </span>
                      {(bundle.status === "blocked" || bundle.status === "failed") && (
                        <button
                          type="button"
                          onClick={() => void rerunBundle(bundle.id)}
                          disabled={!!rerunningBundleId}
                          className="ml-auto shrink-0 rounded px-1 py-0.5 text-[11px] text-foreground transition-colors hover:bg-accent disabled:opacity-50"
                        >
                          {rerunningBundleId === bundle.id
                            ? t(($) => $.task_bundle.rerunning)
                            : t(($) => $.task_bundle.rerun_action)}
                        </button>
                      )}
                    </div>

                    <div className="mt-2 space-y-1">
                      {[...bundle.items]
                        .sort((a, b) => a.position - b.position)
                        .map((item) => {
                          const itemIssue = issueById.get(item.issue_id);
                          const label = itemIssue
                            ? `${itemIssue.identifier} ${itemIssue.title}`
                            : t(($) => $.task_bundle.transcript_item, {
                                position: item.position,
                              });
                          return (
                            <div
                              key={item.id}
                              className="flex min-w-0 items-center gap-2 text-[11px] text-muted-foreground"
                            >
                              <span
                                className={`h-1.5 w-1.5 shrink-0 rounded-full ${bundleItemDotClass(
                                  item.status,
                                )}`}
                              />
                              <span className="min-w-0 flex-1 truncate">{label}</span>
                              {item.issue_id === issue.id && (
                                <span className="shrink-0 rounded bg-background px-1 py-0.5 text-[10px]">
                                  {t(($) => $.task_bundle.current_issue)}
                                </span>
                              )}
                              <span className="shrink-0">
                                {bundleItemStatusLabel(item.status)}
                              </span>
                            </div>
                          );
                        })}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {eligibleAgents.length === 0 && bundles.length === 0 && (
            <div className="rounded-md border bg-muted/20 p-2">
              <p className="text-xs text-muted-foreground">
                {t(($) => $.task_bundle.assign_hint)}
              </p>
            </div>
          )}

          {creationAvailable && (
            <div className="rounded-md border bg-muted/20 p-2">
              <div className="space-y-2">
                <div className="space-y-0.5">
                  <div className="text-xs font-medium text-foreground">
                    {t(($) => $.task_bundle.create_title)}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    {t(($) => $.task_bundle.create_description)}
                  </p>
                </div>

                {eligibleAgents.length > 1 ? (
                  <Select
                    value={selectedAgentId}
                    onValueChange={(value) => {
                      if (value) {
                        setSelectedAgentId(value);
                      }
                    }}
                  >
                    <SelectTrigger size="sm" className="h-7 text-xs">
                      <SelectValue
                        placeholder={t(($) => $.task_bundle.agent_placeholder)}
                      >
                        {selectedAgent?.name ?? ""}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {eligibleAgents.map((agent) => (
                        <SelectItem key={agent.id} value={agent.id}>
                          {agent.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : selectedAgent ? (
                  <div className="flex min-w-0 items-center gap-2 rounded-md border bg-background px-2 py-1.5 text-xs">
                    <span className="shrink-0 text-muted-foreground">
                      {t(($) => $.task_bundle.agent_label)}
                    </span>
                    <span className="min-w-0 truncate font-medium">
                      {selectedAgent.name}
                    </span>
                  </div>
                ) : null}

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
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function isIssueStartableForNewBundle(issue: Issue): boolean {
  return STARTABLE_ISSUE_STATUSES.has(issue.status);
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

function bundleItemDotClass(status: TaskBundleItemStatus): string {
  switch (status) {
    case "completed":
      return "bg-emerald-500";
    case "failed":
    case "blocked":
    case "input_needed":
      return "bg-amber-500";
    case "cancelled":
      return "bg-muted-foreground/50";
    case "in_progress":
      return "bg-blue-500";
    case "queued":
      return "bg-muted-foreground/35";
  }
}
