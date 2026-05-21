"use client";

import { useMemo, useState } from "react";
import {
  Eye,
  FileDiff,
  FileText,
  Maximize2,
  Rows3,
} from "lucide-react";
import type { WorkflowArtifact } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { Markdown } from "../../common/markdown";
import { useT } from "../../i18n";

interface ArtifactGroup {
  key: string;
  logicalName: string;
  contextLabel?: string;
  latest: WorkflowArtifact;
  versions: WorkflowArtifact[];
}

export function WorkflowArtifactReviewList({
  artifacts,
  contextLabelForArtifact,
  className,
}: {
  artifacts: WorkflowArtifact[];
  contextLabelForArtifact?: (artifact: WorkflowArtifact) => string | undefined;
  className?: string;
}) {
  const { t } = useT("workflows");
  const groups = useMemo(
    () => groupWorkflowArtifacts(artifacts, contextLabelForArtifact),
    [artifacts, contextLabelForArtifact],
  );
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [modalKey, setModalKey] = useState<string | null>(null);
  const modalGroup = groups.find((group) => group.key === modalKey) ?? null;

  if (groups.length === 0) return null;

  const togglePreview = (key: string) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <>
      <div className={cn("divide-y overflow-hidden rounded-md border bg-background", className)}>
        {groups.map((group) => {
          const open = expanded.has(group.key);
          return (
            <div key={group.key} className="bg-background">
              <div className="flex min-h-12 items-center gap-2 px-2.5 py-2 text-xs">
                <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{group.logicalName}</div>
                  {group.contextLabel && (
                    <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
                      {group.contextLabel}
                    </div>
                  )}
                  <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] text-muted-foreground">
                    <span className="truncate">{group.latest.content_kind}</span>
                    <span aria-hidden="true">·</span>
                    <span className="font-mono">
                      {t(($) => $.runtime.artifact_version, { version: group.latest.version })}
                    </span>
                    {group.versions.length > 1 && (
                      <Badge variant="outline" className="h-4 rounded-md px-1.5 text-[10px]">
                        {t(($) => $.runtime.artifact_versions, { count: group.versions.length })}
                      </Badge>
                    )}
                  </div>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant={open ? "secondary" : "ghost"}
                  className="h-7 px-2 text-[11px]"
                  aria-expanded={open}
                  aria-label={t(($) => $.runtime.preview_artifact_named, { name: group.logicalName })}
                  onClick={() => togglePreview(group.key)}
                >
                  <Eye className="h-3 w-3" />
                  {open ? t(($) => $.runtime.hide_preview) : t(($) => $.runtime.preview)}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 px-2 text-[11px]"
                  aria-label={t(($) => $.runtime.open_artifact_named, { name: group.logicalName })}
                  onClick={() => setModalKey(group.key)}
                >
                  <Maximize2 className="h-3 w-3" />
                  {t(($) => $.runtime.open_preview)}
                </Button>
              </div>
              {open && (
                <div className="border-t border-border/60 bg-muted/20 px-2.5 py-2">
                  <WorkflowArtifactContent
                    artifact={group.latest}
                    className="max-h-72 rounded-md border bg-background"
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
      <WorkflowArtifactDialog
        group={modalGroup}
        open={!!modalGroup}
        onOpenChange={(open) => {
          if (!open) setModalKey(null);
        }}
      />
    </>
  );
}

function WorkflowArtifactDialog({
  group,
  open,
  onOpenChange,
}: {
  group: ArtifactGroup | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useT("workflows");
  if (!group) return null;
  const canCompare = group.versions.length > 1;
  const base = canCompare ? group.versions[group.versions.length - 2] : null;
  const target = group.latest;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex !h-[min(820px,calc(100vh-3rem))] !w-full !max-w-[min(1120px,calc(100vw-2rem))] flex-col gap-0 overflow-hidden p-0">
        <DialogHeader className="shrink-0 border-b px-4 py-3 pr-12">
          <DialogTitle className="truncate">{group.logicalName}</DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-1.5 text-xs">
            <span>{target.content_kind}</span>
            <span aria-hidden="true">·</span>
            <span className="font-mono">
              {t(($) => $.runtime.artifact_version, { version: target.version })}
            </span>
            {canCompare && base && (
              <>
                <span aria-hidden="true">·</span>
                <span>
                  {t(($) => $.runtime.compare_pair, {
                    base: base.version,
                    target: target.version,
                  })}
                </span>
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        <Tabs
          defaultValue={canCompare ? "compare" : "preview"}
          className="min-h-0 flex-1 gap-0"
        >
          <div className="flex h-10 shrink-0 items-center justify-between border-b bg-muted/30 px-3">
            <TabsList variant="line" className="h-8">
              <TabsTrigger value="preview" className="text-xs">
                <Rows3 className="h-3.5 w-3.5" />
                {t(($) => $.runtime.preview)}
              </TabsTrigger>
              {canCompare && (
                <TabsTrigger value="compare" className="text-xs">
                  <FileDiff className="h-3.5 w-3.5" />
                  {t(($) => $.runtime.compare_versions)}
                </TabsTrigger>
              )}
            </TabsList>
          </div>
          <TabsContent value="preview" className="min-h-0 overflow-auto p-3">
            <WorkflowArtifactContent artifact={target} className="min-h-full rounded-md border bg-background" />
          </TabsContent>
          {canCompare && base && (
            <TabsContent value="compare" className="min-h-0 overflow-hidden p-3">
              <SideBySideArtifactDiff base={base} target={target} />
            </TabsContent>
          )}
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function WorkflowArtifactContent({
  artifact,
  className,
}: {
  artifact: WorkflowArtifact;
  className?: string;
}) {
  const { t } = useT("workflows");
  const content = workflowArtifactContent(artifact);
  if (!content) {
    return (
      <div className={cn("px-3 py-2 text-xs text-muted-foreground", className)}>
        {t(($) => $.runtime.no_artifact_preview)}
      </div>
    );
  }
  if (artifact.content_kind === "markdown") {
    return (
      <div className={cn("overflow-auto px-3 py-2", className)}>
        <Markdown mode="full" className="text-xs leading-5">
          {content}
        </Markdown>
      </div>
    );
  }
  return (
    <pre className={cn("overflow-auto px-3 py-2 font-mono text-[11px] leading-relaxed text-foreground", className)}>
      {content}
    </pre>
  );
}

function SideBySideArtifactDiff({
  base,
  target,
}: {
  base: WorkflowArtifact;
  target: WorkflowArtifact;
}) {
  const { t } = useT("workflows");
  const rows = useMemo(
    () => buildSideBySideDiff(workflowArtifactContent(base), workflowArtifactContent(target)),
    [base, target],
  );
  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_1fr] overflow-hidden rounded-md border bg-background">
      <div className="grid grid-cols-2 border-b bg-muted/40 text-xs font-medium">
        <div className="border-r px-3 py-2">
          {t(($) => $.runtime.artifact_version, { version: base.version })}
        </div>
        <div className="px-3 py-2">
          {t(($) => $.runtime.artifact_version, { version: target.version })}
        </div>
      </div>
      <div className="min-h-0 overflow-auto">
        {rows.map((row, index) => (
          <div key={index} className="grid grid-cols-2 font-mono text-[11px] leading-relaxed">
            <DiffCell side="left" lineNumber={row.leftNumber} content={row.left} tone={row.leftTone} />
            <DiffCell side="right" lineNumber={row.rightNumber} content={row.right} tone={row.rightTone} />
          </div>
        ))}
      </div>
    </div>
  );
}

function DiffCell({
  side,
  lineNumber,
  content,
  tone,
}: {
  side: "left" | "right";
  lineNumber?: number;
  content?: string;
  tone?: "added" | "removed";
}) {
  return (
    <div
      className={cn(
        "grid min-w-0 grid-cols-[3rem_minmax(0,1fr)] border-b border-border/50",
        side === "left" && "border-r",
        tone === "added" && "bg-emerald-500/10",
        tone === "removed" && "bg-destructive/10",
        !content && "bg-muted/20",
      )}
    >
      <div className="select-none border-r border-border/40 px-2 py-0.5 text-right text-muted-foreground">
        {lineNumber ?? ""}
      </div>
      <div className="min-w-0 whitespace-pre-wrap break-words px-2 py-0.5">
        {content ?? ""}
      </div>
    </div>
  );
}

function groupWorkflowArtifacts(
  artifacts: WorkflowArtifact[],
  contextLabelForArtifact?: (artifact: WorkflowArtifact) => string | undefined,
) {
  const groups = new Map<string, ArtifactGroup>();
  for (const artifact of artifacts) {
    const key = `${artifact.workflow_run_id}:${artifact.workflow_step_run_id}:${artifact.logical_name}`;
    const existing = groups.get(key);
    if (existing) {
      existing.versions.push(artifact);
      existing.contextLabel ||= contextLabelForArtifact?.(artifact);
    } else {
      groups.set(key, {
        key,
        logicalName: artifact.logical_name,
        contextLabel: contextLabelForArtifact?.(artifact),
        latest: artifact,
        versions: [artifact],
      });
    }
  }
  for (const group of groups.values()) {
    group.versions.sort((a, b) => a.version - b.version || a.created_at.localeCompare(b.created_at));
    group.latest = group.versions[group.versions.length - 1] ?? group.latest;
  }
  return Array.from(groups.values()).sort((a, b) => {
    const byName = a.logicalName.localeCompare(b.logicalName);
    if (byName !== 0) return byName;
    return b.latest.version - a.latest.version;
  });
}

function workflowArtifactContent(artifact: WorkflowArtifact) {
  if (typeof artifact.content_text === "string" && artifact.content_text.trim()) {
    return artifact.content_text;
  }
  if (artifact.content_json !== undefined && artifact.content_json !== null) {
    try {
      return JSON.stringify(artifact.content_json, null, 2);
    } catch {
      return String(artifact.content_json);
    }
  }
  return "";
}

interface SideBySideDiffRow {
  left?: string;
  right?: string;
  leftNumber?: number;
  rightNumber?: number;
  leftTone?: "removed";
  rightTone?: "added";
}

function buildSideBySideDiff(baseContent: string, targetContent: string): SideBySideDiffRow[] {
  const baseLines = splitLines(baseContent);
  const targetLines = splitLines(targetContent);
  if (baseLines.length * targetLines.length > 60_000) {
    return buildIndexedDiff(baseLines, targetLines);
  }

  const dp = Array.from({ length: baseLines.length + 1 }, () =>
    new Array<number>(targetLines.length + 1).fill(0),
  );
  const score = (row: number, col: number) => dp[row]?.[col] ?? 0;
  for (let i = baseLines.length - 1; i >= 0; i -= 1) {
    for (let j = targetLines.length - 1; j >= 0; j -= 1) {
      const row = dp[i];
      if (!row) continue;
      row[j] = baseLines[i] === targetLines[j]
        ? score(i + 1, j + 1) + 1
        : Math.max(score(i + 1, j), score(i, j + 1));
    }
  }

  const rows: SideBySideDiffRow[] = [];
  let i = 0;
  let j = 0;
  while (i < baseLines.length || j < targetLines.length) {
    if (i < baseLines.length && j < targetLines.length && baseLines[i] === targetLines[j]) {
      rows.push({
        left: baseLines[i] ?? "",
        right: targetLines[j] ?? "",
        leftNumber: i + 1,
        rightNumber: j + 1,
      });
      i += 1;
      j += 1;
    } else if (i < baseLines.length && (j >= targetLines.length || score(i + 1, j) >= score(i, j + 1))) {
      rows.push({
        left: baseLines[i] ?? "",
        leftNumber: i + 1,
        leftTone: "removed",
      });
      i += 1;
    } else if (j < targetLines.length) {
      rows.push({
        right: targetLines[j] ?? "",
        rightNumber: j + 1,
        rightTone: "added",
      });
      j += 1;
    }
  }
  return rows;
}

function buildIndexedDiff(baseLines: string[], targetLines: string[]): SideBySideDiffRow[] {
  const length = Math.max(baseLines.length, targetLines.length);
  const rows: SideBySideDiffRow[] = [];
  for (let i = 0; i < length; i += 1) {
    const left = baseLines[i];
    const right = targetLines[i];
    const changed = left !== right;
    rows.push({
      left,
      right,
      leftNumber: left === undefined ? undefined : i + 1,
      rightNumber: right === undefined ? undefined : i + 1,
      leftTone: changed && left !== undefined ? "removed" : undefined,
      rightTone: changed && right !== undefined ? "added" : undefined,
    });
  }
  return rows;
}

function splitLines(content: string) {
  if (!content) return [];
  return content.replace(/\r\n/g, "\n").split("\n");
}
