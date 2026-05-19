"use client";

import { useMemo, useState } from "react";
import {
  Check,
  CheckCircle2,
  ChevronRight,
  Circle,
  CircleDashed,
  CirclePause,
  FileText,
  RefreshCw,
  ShieldCheck,
  Square,
  Workflow,
  X,
  XCircle,
} from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import type { WorkflowReview, WorkflowRun, WorkflowStepRun } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  workflowKeys,
  workflowRunDetailOptions,
  workflowRunListOptions,
} from "@multica/core/workflows";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

export function WorkflowRunViewer({
  issueId,
  taskId,
  chatSessionId,
  autopilotRunId,
}: {
  issueId?: string;
  taskId?: string;
  chatSessionId?: string;
  autopilotRunId?: string;
}) {
  const { t } = useT("workflows");
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(true);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const runFilter = { issueId, taskId, chatSessionId, autopilotRunId };
  const { data: runs = [] } = useQuery(workflowRunListOptions(wsId, runFilter));
  const latestRunId = runs[0]?.id ?? null;
  const { data: detail } = useQuery(
    workflowRunDetailOptions(wsId, latestRunId),
  );
  const run = detail ?? runs[0] ?? null;

  if (!run) return null;

  const steps = detail?.steps ?? [];
  const reviews = detail?.reviews ?? [];
  const pendingReviews = reviews.filter((review) => review.status === "requested");
  const completed = steps.filter((step) =>
    ["completed", "skipped"].includes(step.status),
  ).length;
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
  const runAction = async (
    key: string,
    action: () => Promise<unknown>,
    success: string,
  ) => {
    setPendingAction(key);
    try {
      await action();
      await invalidate();
      toast.success(success);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.runtime.action_failed));
    } finally {
      setPendingAction(null);
    }
  };

  return (
    <div>
      <button
        type="button"
        className={cn(
          "mb-2 flex w-full items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors hover:bg-accent/70",
          !open && "text-muted-foreground hover:text-foreground",
        )}
        onClick={() => setOpen((value) => !value)}
      >
        {t(($) => $.runtime.title)}
        <ChevronRight
          className={cn(
            "!size-3 shrink-0 stroke-[2.5] text-muted-foreground transition-transform",
            open && "rotate-90",
          )}
        />
      </button>
      {open && (
        <div className="space-y-2 pl-2">
          <WorkflowRunHeader
            run={run}
            completed={completed}
            total={steps.length}
            busy={pendingAction}
            onCancel={() =>
              runAction(
                "cancel-run",
                () => api.cancelWorkflowRun(run.id),
                t(($) => $.runtime.cancelled),
              )
            }
            onRerun={() =>
              runAction(
                "rerun",
                () => api.rerunWorkflowRun(run.id),
                t(($) => $.runtime.rerun_created),
              )
            }
          />
          {steps.length > 0 && (
            <div className="space-y-1">
              {steps.map((step) => (
                <WorkflowStepRow
                  key={step.id}
                  step={step}
                  busy={pendingAction}
                  onManualComplete={() =>
                    runAction(
                      `manual-${step.id}`,
                      () => api.completeManualWorkflowStepRun(step.id),
                      t(($) => $.runtime.step_completed),
                    )
                  }
                  onRetry={() =>
                    runAction(
                      `retry-${step.id}`,
                      () => api.retryWorkflowStepRun(step.id),
                      t(($) => $.runtime.step_retry_created),
                    )
                  }
                />
              ))}
            </div>
          )}
          {pendingReviews.length > 0 && (
            <WorkflowReviewControls
              reviews={pendingReviews}
              busy={pendingAction}
              onApprove={(review) =>
                runAction(
                  `approve-${review.id}`,
                  () => api.approveWorkflowReview(review.id),
                  t(($) => $.runtime.review_approved),
                )
              }
              onReject={(review) =>
                runAction(
                  `reject-${review.id}`,
                  () => api.rejectWorkflowReview(review.id),
                  t(($) => $.runtime.review_rejected),
                )
              }
            />
          )}
          {detail && (
            <WorkflowRunEvidence
              artifacts={detail.artifacts?.length ?? 0}
              reviews={detail.reviews?.length ?? 0}
              quality={detail.quality_gate_results?.length ?? 0}
            />
          )}
        </div>
      )}
    </div>
  );
}

function WorkflowRunHeader({
  run,
  completed,
  total,
  busy,
  onCancel,
  onRerun,
}: {
  run: WorkflowRun;
  completed: number;
  total: number;
  busy: string | null;
  onCancel: () => void;
  onRerun: () => void;
}) {
  const { t } = useT("workflows");
  const percent = total > 0 ? Math.round((completed / total) * 100) : 0;
  return (
    <div className="rounded-md border bg-background px-2.5 py-2">
      <div className="flex items-center gap-2">
        <Workflow className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="min-w-0 flex-1 truncate text-xs font-medium">
          {workflowName(run)}
        </span>
        <WorkflowStatusBadge status={run.status} />
      </div>
      <div className="mt-2 flex items-center gap-1">
        {run.status !== "completed" && run.status !== "cancelled" && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-[11px]"
            disabled={busy === "cancel-run"}
            onClick={onCancel}
          >
            <Square className="h-3 w-3" />
            {t(($) => $.runtime.cancel)}
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-[11px]"
          disabled={busy === "rerun"}
          onClick={onRerun}
        >
          <RefreshCw className={cn("h-3 w-3", busy === "rerun" && "animate-spin")} />
          {t(($) => $.runtime.rerun)}
        </Button>
      </div>
      {total > 0 && (
        <div className="mt-2">
          <div className="mb-1 flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t(($) => $.runtime.steps_progress, { completed, total })}</span>
            <span className="font-mono tabular-nums">{percent}%</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-foreground/70 transition-[width]"
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>
      )}
    </div>
  );
}

function WorkflowStepRow({
  step,
  busy,
  onManualComplete,
  onRetry,
}: {
  step: WorkflowStepRun;
  busy: string | null;
  onManualComplete: () => void;
  onRetry: () => void;
}) {
  const { t } = useT("workflows");
  const canManualComplete = step.status === "waiting_manual";
  const canRetry = ["failed", "blocked", "paused"].includes(step.status);
  return (
    <div className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent/40">
      <StepStatusIcon status={step.status} />
      <span className="min-w-0 flex-1 truncate">{step.title || step.step_definition_id}</span>
      <span className="shrink-0 text-[11px] text-muted-foreground">
        {step.execution_kind}
      </span>
      <WorkflowStatusBadge status={step.status} />
      {canManualComplete && (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-6 px-2 text-[11px]"
          disabled={busy === `manual-${step.id}`}
          onClick={onManualComplete}
        >
          <Check className="h-3 w-3" />
          {t(($) => $.runtime.complete)}
        </Button>
      )}
      {canRetry && (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-6 px-2 text-[11px]"
          disabled={busy === `retry-${step.id}`}
          onClick={onRetry}
        >
          <RefreshCw
            className={cn("h-3 w-3", busy === `retry-${step.id}` && "animate-spin")}
          />
          {t(($) => $.runtime.retry)}
        </Button>
      )}
    </div>
  );
}

function WorkflowReviewControls({
  reviews,
  busy,
  onApprove,
  onReject,
}: {
  reviews: WorkflowReview[];
  busy: string | null;
  onApprove: (review: WorkflowReview) => void;
  onReject: (review: WorkflowReview) => void;
}) {
  const { t } = useT("workflows");
  return (
    <div className="rounded-md border bg-background px-2 py-2">
      <div className="mb-1 flex items-center gap-1.5 text-xs font-medium">
        <ShieldCheck className="h-3.5 w-3.5 text-muted-foreground" />
        {t(($) => $.runtime.pending_reviews)}
      </div>
      <div className="space-y-1">
        {reviews.map((review) => (
          <div key={review.id} className="flex items-center gap-2 text-xs">
            <span className="min-w-0 flex-1 truncate">
              {review.workflow_step_run_id ?? review.workflow_artifact_id ?? review.id}
            </span>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="h-6 w-6"
              disabled={busy === `approve-${review.id}`}
              aria-label={t(($) => $.runtime.approve)}
              onClick={() => onApprove(review)}
            >
              <Check className="h-3.5 w-3.5" />
            </Button>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="h-6 w-6"
              disabled={busy === `reject-${review.id}`}
              aria-label={t(($) => $.runtime.reject)}
              onClick={() => onReject(review)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
}

function WorkflowRunEvidence({
  artifacts,
  reviews,
  quality,
}: {
  artifacts: number;
  reviews: number;
  quality: number;
}) {
  const { t } = useT("workflows");
  const items = useMemo(
    () => [
      { icon: FileText, label: t(($) => $.runtime.artifacts), value: artifacts },
      { icon: Workflow, label: t(($) => $.runtime.reviews), value: reviews },
      { icon: ShieldCheck, label: t(($) => $.runtime.quality), value: quality },
    ],
    [artifacts, quality, reviews, t],
  );
  return (
    <div className="grid grid-cols-3 gap-1">
      {items.map((item) => (
        <div
          key={item.label}
          className="rounded-md border bg-background px-2 py-1.5"
        >
          <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
            <item.icon className="h-3 w-3" />
            <span className="truncate">{item.label}</span>
          </div>
          <div className="mt-0.5 font-mono text-xs tabular-nums">
            {item.value}
          </div>
        </div>
      ))}
    </div>
  );
}

function WorkflowStatusBadge({ status }: { status: string }) {
  const tone =
    status === "completed"
      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
      : status === "failed" || status === "blocked" || status === "cancelled"
        ? "border-destructive/30 bg-destructive/10 text-destructive"
        : status.startsWith("waiting") || status === "paused"
          ? "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300"
          : "border-border bg-muted/50 text-muted-foreground";
  return (
    <Badge variant="outline" className={cn("h-4 rounded-md px-1.5 text-[10px]", tone)}>
      {status.replaceAll("_", " ")}
    </Badge>
  );
}

function StepStatusIcon({ status }: { status: string }) {
  const className = "h-3.5 w-3.5 shrink-0";
  if (status === "completed") {
    return <CheckCircle2 className={cn(className, "text-emerald-600")} />;
  }
  if (status === "failed" || status === "blocked") {
    return <XCircle className={cn(className, "text-destructive")} />;
  }
  if (status === "paused" || status.startsWith("waiting")) {
    return <CirclePause className={cn(className, "text-amber-600")} />;
  }
  if (status === "running") {
    return <CircleDashed className={cn(className, "animate-spin text-muted-foreground")} />;
  }
  return <Circle className={cn(className, "text-muted-foreground")} />;
}

function workflowName(run: WorkflowRun) {
  const snapshot = run.snapshot;
  if (snapshot && typeof snapshot === "object") {
    const data = snapshot as { workflow_name?: unknown };
    if (typeof data.workflow_name === "string" && data.workflow_name.trim()) {
      return data.workflow_name;
    }
  }
  return run.trigger_type || run.id;
}
