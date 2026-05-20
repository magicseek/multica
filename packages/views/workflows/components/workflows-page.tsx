"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  Check,
  ChevronDown,
  Copy,
  Eye,
  FileText,
  FolderKanban,
  GitBranch,
  Info,
  ListChecks,
  Loader2,
  Lock,
  MessageSquareText,
  Package,
  Pencil,
  Plus,
  Save,
  Search,
  ShieldCheck,
  Trash2,
  Workflow,
  X,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects/queries";
import { useUpdateProject } from "@multica/core/projects/mutations";
import {
  useCreateWorkflow,
  useDeleteWorkflowDraft,
  useDeleteWorkflow,
  useForkWorkflow,
  usePublishWorkflow,
  useUpdateWorkflowDraft,
  workflowListOptions,
} from "@multica/core/workflows";
import type {
  Project,
  WorkflowApplicability,
  WorkflowDefinition,
  WorkflowSchema,
  WorkflowStep,
  WorkflowValidation,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Switch } from "@multica/ui/components/ui/switch";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@multica/ui/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { cn } from "@multica/ui/lib/utils";
import { Markdown } from "../../common/markdown";
import { PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { WorkflowGraphPreview } from "./workflow-graph-preview";

type FilterKey = "all" | WorkflowApplicability;

const FILTERS: FilterKey[] = ["all", "assignment", "comment"];

const SAMPLE_ISSUE_ID = "MUL-123";
const SAMPLE_COMMENT_ID = "comment-123";
const DEFAULT_WORKFLOW_TEMPLATE = `## Custom Workflow

1. Run \`multica issue get {{issue_id}} --output json\` and read the task.
2. Run \`multica issue comment list {{issue_id}} --output json\` and capture the latest discussion.
3. Complete the requested work with the smallest useful change.
4. Verify the result and post a concise issue comment.
`;

type StepGate = "none" | "human" | "quality" | "human_quality";
type InspectorSection = "basics" | "instructions" | "inputs" | "outputs" | "checks";

const EMPTY_SCHEMA: WorkflowSchema = {
  schema_version: 1,
  name: "",
  description: "",
  applicability: ["assignment"],
  source: {
    format: "markdown",
    body_template: "",
  },
  steps: [],
};

function publishedRevision(workflow: WorkflowDefinition | null | undefined) {
  return workflow?.published_revision ?? workflow?.current_revision ?? null;
}

function draftRevision(workflow: WorkflowDefinition | null | undefined) {
  return workflow?.draft_revision ?? null;
}

function authoringRevision(workflow: WorkflowDefinition | null | undefined) {
  return draftRevision(workflow) ?? publishedRevision(workflow);
}

function publishedSchema(workflow: WorkflowDefinition | null | undefined) {
  return publishedRevision(workflow)?.schema ?? null;
}

function authoringSchema(workflow: WorkflowDefinition | null | undefined) {
  return authoringRevision(workflow)?.schema ?? EMPTY_SCHEMA;
}

function workflowApplicability(
  workflow: WorkflowDefinition | null | undefined,
): WorkflowApplicability[] {
  return authoringSchema(workflow).applicability ?? ["assignment"];
}

function workflowDisplayName(
  workflow: WorkflowDefinition | null | undefined,
): string {
  return (
    authoringSchema(workflow).name?.trim() ||
    workflow?.name?.trim() ||
    "Untitled workflow"
  );
}

function workflowDisplayDescription(
  workflow: WorkflowDefinition | null | undefined,
): string {
  return (
    authoringSchema(workflow).description?.trim() ||
    workflow?.description?.trim() ||
    ""
  );
}

function buildSchema(
  workflow: WorkflowDefinition | null | undefined,
  base: WorkflowSchema,
  patch: Partial<WorkflowSchema>,
): WorkflowSchema {
  return {
    ...base,
    schema_version: base.schema_version ?? base.version ?? 1,
    version: undefined,
    name: (patch.name as string | undefined)?.trim() ?? base.name ?? workflow?.name ?? "",
    description:
      (patch.description as string | undefined)?.trim() ??
      base.description ??
      workflow?.description ??
      "",
    applicability:
      (patch.applicability as WorkflowApplicability[] | undefined) ??
      base.applicability ??
      ["assignment"],
    source: {
      ...(base.source ?? {}),
      format: "markdown",
      mode: undefined,
      body_template:
        patch.source?.body_template ?? base.source?.body_template ?? "",
    },
    steps: patch.steps ?? base.steps ?? [],
  };
}

function slugifyStepId(value: string, fallback: string): string {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || fallback;
}

function normalizeStep(
  step: WorkflowStep,
  index: number,
  previousId?: string,
): WorkflowStep {
  const id = (step.id ?? "").trim() || slugifyStepId(step.title ?? "", `step-${index + 1}`);
  const title = (step.title ?? "").trim() || step.name?.trim() || id;
  const outputDescription = step.output?.description ?? "";
  const artifactName = step.artifact?.name ?? "";
  const artifactTemplate = step.artifact?.template ?? "";
  const artifactDescription = step.artifact?.description ?? "";
  const reviewRequired = step.review?.required === true;
  const qualityEnabled = step.quality_gate?.enabled === true;
  return {
    ...step,
    id,
    title,
    name: step.name?.trim() || undefined,
    order: step.order || index + 1,
    depends_on:
      step.depends_on?.map((item) => item.trim()).filter(Boolean) ??
      (previousId ? [previousId] : undefined),
    body_template: step.body_template || undefined,
    description: step.description || undefined,
    checklist: step.checklist?.filter((item) => item.trim()),
    output: outputDescription ? { description: outputDescription } : undefined,
    artifact: step.artifact
      ? {
          name: artifactName,
          format: step.artifact.format || "markdown",
          template: artifactTemplate || undefined,
          description: artifactDescription || undefined,
        }
      : undefined,
    review: reviewRequired
      ? {
          required: true,
          reviewer_role: step.review?.reviewer_role || undefined,
          instructions: step.review?.instructions || undefined,
        }
      : undefined,
    quality_gate: qualityEnabled
      ? {
          enabled: true,
          blocking: step.quality_gate?.blocking === true,
          prompt: step.quality_gate?.prompt || undefined,
          report_mode: step.quality_gate?.report_mode || "summary",
        }
      : undefined,
  };
}

function normalizeSteps(steps: WorkflowStep[]): WorkflowStep[] {
  const next: WorkflowStep[] = [];
  for (const [index, step] of steps.entries()) {
    const normalized = normalizeStep(step, index, next[index - 1]?.id);
    next.push(normalized);
  }
  return next;
}

function formatSchema(schema: WorkflowSchema): string {
  return JSON.stringify(schema, null, 2);
}

type SchemaDiffOp =
  | { kind: "equal"; leftLine: number; rightLine: number; text: string }
  | { kind: "removed"; line: number; text: string }
  | { kind: "added"; line: number; text: string };

type SchemaDiffRow = {
  key: string;
  kind: "unchanged" | "added" | "removed" | "modified";
  leftLine: number | null;
  rightLine: number | null;
  leftText: string;
  rightText: string;
};

export type SchemaLineDiff = {
  rows: SchemaDiffRow[];
  added: number;
  removed: number;
  modified: number;
  unchanged: number;
  changed: number;
  leftLineCount: number;
  rightLineCount: number;
};

function schemaLines(value: string): string[] {
  return value ? value.split("\n") : [];
}

export function buildSchemaLineDiff(
  leftText: string,
  rightText: string,
): SchemaLineDiff {
  const left = schemaLines(leftText);
  const right = schemaLines(rightText);
  const lcs = Array.from({ length: left.length + 1 }, () =>
    Array<number>(right.length + 1).fill(0),
  );

  for (let i = left.length - 1; i >= 0; i -= 1) {
    for (let j = right.length - 1; j >= 0; j -= 1) {
      lcs[i]![j] =
        left[i] === right[j]
          ? lcs[i + 1]![j + 1]! + 1
          : Math.max(lcs[i + 1]![j]!, lcs[i]![j + 1]!);
    }
  }

  const ops: SchemaDiffOp[] = [];
  let leftIndex = 0;
  let rightIndex = 0;

  while (leftIndex < left.length && rightIndex < right.length) {
    if (left[leftIndex] === right[rightIndex]) {
      ops.push({
        kind: "equal",
        leftLine: leftIndex + 1,
        rightLine: rightIndex + 1,
        text: left[leftIndex] ?? "",
      });
      leftIndex += 1;
      rightIndex += 1;
      continue;
    }

    const removeScore = lcs[leftIndex + 1]?.[rightIndex] ?? 0;
    const addScore = lcs[leftIndex]?.[rightIndex + 1] ?? 0;
    if (removeScore >= addScore) {
      ops.push({
        kind: "removed",
        line: leftIndex + 1,
        text: left[leftIndex] ?? "",
      });
      leftIndex += 1;
    } else {
      ops.push({
        kind: "added",
        line: rightIndex + 1,
        text: right[rightIndex] ?? "",
      });
      rightIndex += 1;
    }
  }

  while (leftIndex < left.length) {
    ops.push({
      kind: "removed",
      line: leftIndex + 1,
      text: left[leftIndex] ?? "",
    });
    leftIndex += 1;
  }

  while (rightIndex < right.length) {
    ops.push({
      kind: "added",
      line: rightIndex + 1,
      text: right[rightIndex] ?? "",
    });
    rightIndex += 1;
  }

  const rows: SchemaDiffRow[] = [];
  let added = 0;
  let removed = 0;
  let modified = 0;
  let unchanged = 0;
  let opIndex = 0;

  while (opIndex < ops.length) {
    const op = ops[opIndex];
    if (op?.kind === "equal") {
      unchanged += 1;
      rows.push({
        key: `same-${rows.length}-${op.leftLine}-${op.rightLine}`,
        kind: "unchanged",
        leftLine: op.leftLine,
        rightLine: op.rightLine,
        leftText: op.text,
        rightText: op.text,
      });
      opIndex += 1;
      continue;
    }

    const removedRun: Extract<SchemaDiffOp, { kind: "removed" }>[] = [];
    const addedRun: Extract<SchemaDiffOp, { kind: "added" }>[] = [];

    while (opIndex < ops.length && ops[opIndex]?.kind !== "equal") {
      const changeOp = ops[opIndex];
      if (changeOp?.kind === "removed") removedRun.push(changeOp);
      if (changeOp?.kind === "added") addedRun.push(changeOp);
      opIndex += 1;
    }

    const paired = Math.min(removedRun.length, addedRun.length);
    for (let index = 0; index < paired; index += 1) {
      const before = removedRun[index]!;
      const after = addedRun[index]!;
      modified += 1;
      rows.push({
        key: `modified-${rows.length}-${before.line}-${after.line}`,
        kind: "modified",
        leftLine: before.line,
        rightLine: after.line,
        leftText: before.text,
        rightText: after.text,
      });
    }

    for (let index = paired; index < removedRun.length; index += 1) {
      const before = removedRun[index]!;
      removed += 1;
      rows.push({
        key: `removed-${rows.length}-${before.line}`,
        kind: "removed",
        leftLine: before.line,
        rightLine: null,
        leftText: before.text,
        rightText: "",
      });
    }

    for (let index = paired; index < addedRun.length; index += 1) {
      const after = addedRun[index]!;
      added += 1;
      rows.push({
        key: `added-${rows.length}-${after.line}`,
        kind: "added",
        leftLine: null,
        rightLine: after.line,
        leftText: "",
        rightText: after.text,
      });
    }
  }

  return {
    rows,
    added,
    removed,
    modified,
    unchanged,
    changed: added + removed + modified,
    leftLineCount: left.length,
    rightLineCount: right.length,
  };
}

function stepGate(step: WorkflowStep): StepGate {
  const review = step.review?.required === true;
  const quality = step.quality_gate?.enabled === true;
  if (review && quality) return "human_quality";
  if (review) return "human";
  if (quality) return "quality";
  return "none";
}

function gatePatchForStep(
  step: WorkflowStep,
  value: StepGate,
): Pick<WorkflowStep, "review" | "quality_gate"> {
  return {
    review:
      value === "human" || value === "human_quality"
        ? { ...step.review, required: true }
        : undefined,
    quality_gate:
      value === "quality" || value === "human_quality"
        ? {
            ...step.quality_gate,
            enabled: true,
            report_mode: step.quality_gate?.report_mode || "summary",
          }
        : undefined,
  };
}

function stepGateLabel(
  t: ReturnType<typeof useT<"workflows">>["t"],
  value: StepGate,
) {
  switch (value) {
    case "human":
      return t(($) => $.steps.gate_human);
    case "quality":
      return t(($) => $.steps.gate_quality);
    case "human_quality":
      return t(($) => $.steps.gate_human_quality);
    default:
      return t(($) => $.steps.gate_none);
  }
}

function listText(items: string[] | undefined): string {
  return (items ?? []).join("\n");
}

function parseLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .filter((item) => item.trim());
}

function dependencyText(step: WorkflowStep): string {
  return (step.depends_on ?? []).join(", ");
}

function isEditableTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement
    ? Boolean(target.closest("input, textarea, select, button, [contenteditable='true']"))
    : false;
}

function dependencyOptions(steps: WorkflowStep[], index: number): WorkflowStep[] {
  return steps.slice(0, index);
}

function dependencySummary(steps: WorkflowStep[], step: WorkflowStep): string {
  const dependencies = step.depends_on ?? [];
  if (dependencies.length === 0) return "";
  return dependencies
    .map((id) => steps.find((candidate) => candidate.id === id)?.title ?? id)
    .join(", ");
}

function toggleDependency(
  steps: WorkflowStep[],
  step: WorkflowStep,
  dependencyId: string,
): string[] {
  const selected = new Set(step.depends_on ?? []);
  if (selected.has(dependencyId)) {
    selected.delete(dependencyId);
  } else {
    selected.add(dependencyId);
  }
  const knownIds = new Set(steps.map((candidate) => candidate.id));
  const orderedKnown = steps
    .filter((candidate) => candidate.id !== step.id && selected.has(candidate.id))
    .map((candidate) => candidate.id);
  const unknown = Array.from(selected).filter((id) => !knownIds.has(id));
  return [...orderedKnown, ...unknown];
}

function hasCustomDependencies(steps: WorkflowStep[], index: number): boolean {
  const step = steps[index];
  if (!step) return false;
  const dependencies = step.depends_on ?? [];
  if (index === 0) return dependencies.length > 0;
  const previousId = steps[index - 1]?.id;
  return dependencies.length !== 1 || dependencies[0] !== previousId;
}

function defaultArtifactName(step: WorkflowStep): string {
  return `${slugifyStepId(step.title || step.id, step.id || "artifact")}.md`;
}

function applicabilityLabel(
  t: ReturnType<typeof useT<"workflows">>["t"],
  value: WorkflowApplicability,
) {
  return value === "comment"
    ? t(($) => $.applicability.comment)
    : t(($) => $.applicability.assignment);
}

function PageHeaderBar({
  totalCount,
  onCreate,
}: {
  totalCount: number;
  onCreate: () => void;
}) {
  const { t } = useT("workflows");
  return (
    <PageHeader className="justify-between px-5">
      <div className="flex items-center gap-2">
        <Workflow className="h-4 w-4 text-muted-foreground" />
        <h1 className="text-sm font-medium">{t(($) => $.page.title)}</h1>
        {totalCount > 0 && (
          <span className="font-mono text-xs tabular-nums text-muted-foreground/70">
            {totalCount}
          </span>
        )}
        <p className="ml-2 hidden text-xs text-muted-foreground md:block">
          {t(($) => $.page.tagline)}
        </p>
      </div>
      <Button type="button" size="sm" onClick={onCreate}>
        <Plus className="h-3 w-3" />
        {t(($) => $.page.new_workflow)}
      </Button>
    </PageHeader>
  );
}

function WorkflowList({
  workflows,
  selectedId,
  onSelect,
  search,
  setSearch,
  filter,
  setFilter,
}: {
  workflows: WorkflowDefinition[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  search: string;
  setSearch: (value: string) => void;
  filter: FilterKey;
  setFilter: (value: FilterKey) => void;
}) {
  const { t } = useT("workflows");
  return (
    <div className="flex min-h-0 flex-col border-r bg-muted/15 md:w-80">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-3">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t(($) => $.page.search_placeholder)}
            className="h-8 pl-8 text-sm"
          />
        </div>
      </div>
      <div className="flex shrink-0 gap-1 border-b px-3 py-2">
        {FILTERS.map((item) => (
          <Button
            key={item}
            type="button"
            variant="outline"
            size="sm"
            className={cn(
              "h-7 px-2 text-xs",
              filter === item &&
                "bg-accent text-accent-foreground hover:bg-accent/80",
            )}
            onClick={() => setFilter(item)}
          >
            {item === "all"
              ? t(($) => $.filters.all)
              : applicabilityLabel(t, item)}
          </Button>
        ))}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {workflows.length === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-muted-foreground">
            {t(($) => $.page.no_matches)}
          </div>
        ) : (
          <div className="space-y-1">
            {workflows.map((workflow) => {
              const isSelected = workflow.id === selectedId;
              const apps = workflowApplicability(workflow);
              const hasDraft = !!draftRevision(workflow);
              const hasPublished = !!publishedRevision(workflow);
              return (
                <button
                  key={workflow.id}
                  type="button"
                  onClick={() => onSelect(workflow.id)}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-left transition-colors",
                    isSelected
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-accent/60",
                  )}
                >
                  <div className="flex items-center gap-2">
                    <span className="min-w-0 flex-1 truncate text-sm font-medium">
                      {workflowDisplayName(workflow)}
                    </span>
                    {workflow.origin === "system_seeded" && (
                      <Lock className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    )}
                  </div>
                  <div className="mt-1 flex items-center gap-1.5">
                    <Badge
                      variant="outline"
                      className="h-4 rounded-md px-1.5 text-[10px]"
                    >
                      {workflow.origin === "system_seeded"
                        ? t(($) => $.origin.system)
                        : t(($) => $.origin.user)}
                    </Badge>
                    {hasDraft && (
                      <Badge
                        variant="secondary"
                        className="h-4 rounded-md px-1.5 text-[10px]"
                      >
                        {hasPublished
                          ? t(($) => $.status.unpublished_changes)
                          : t(($) => $.status.draft)}
                      </Badge>
                    )}
                    {!hasDraft && hasPublished && (
                      <Badge
                        variant="outline"
                        className="h-4 rounded-md px-1.5 text-[10px]"
                      >
                        {t(($) => $.status.published)}
                      </Badge>
                    )}
                    <span className="truncate text-xs text-muted-foreground">
                      {apps.map((app) => applicabilityLabel(t, app)).join(", ")}
                    </span>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function ProjectWorkflowBindings({
  projects,
  workflows,
}: {
  projects: Project[];
  workflows: WorkflowDefinition[];
}) {
  const { t } = useT("workflows");
  const updateProject = useUpdateProject();
  const workflowNameById = useMemo(
    () =>
      new Map(
        workflows.map((workflow) => [
          workflow.id,
          workflow.name || publishedSchema(workflow)?.name || workflow.id,
        ]),
      ),
    [workflows],
  );

  return (
    <div className="flex min-h-0 flex-col border-t">
      <div className="flex h-10 shrink-0 items-center gap-2 px-4">
        <FolderKanban className="h-3.5 w-3.5 text-muted-foreground" />
        <h2 className="text-xs font-medium">
          {t(($) => $.project_bindings.title)}
        </h2>
        <span className="text-xs text-muted-foreground">
          {t(($) => $.project_bindings.description)}
        </span>
      </div>
      <div className="grid max-h-56 gap-1 overflow-y-auto px-4 pb-4">
        {projects.length === 0 ? (
          <div className="rounded-md border border-dashed px-3 py-3 text-xs text-muted-foreground">
            {t(($) => $.project_bindings.empty)}
          </div>
        ) : (
          projects.map((project) => (
            <div
              key={project.id}
              className="grid grid-cols-[minmax(0,1fr)_220px] items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent/40"
            >
              <span className="truncate text-sm">{project.title}</span>
              <Select
                value={project.workflow_definition_id ?? "__default"}
                onValueChange={(value) => {
                  updateProject.mutate(
                    {
                      id: project.id,
                      workflow_definition_id:
                        value === "__default" ? null : value,
                    },
                    {
                      onError: () =>
                        toast.error(t(($) => $.project_bindings.update_failed)),
                    },
                  );
                }}
              >
                <SelectTrigger
                  size="sm"
                  className="w-full justify-between"
                  aria-label={t(($) => $.project_bindings.select_aria, {
                    project: project.title,
                  })}
                >
                  <SelectValue>
                    {project.workflow_definition_id
                      ? workflowNameById.get(project.workflow_definition_id) ??
                        project.workflow_definition_id
                      : t(($) => $.project_bindings.workspace_default)}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent align="end">
                  <SelectItem value="__default">
                    {t(($) => $.project_bindings.workspace_default)}
                  </SelectItem>
                  {workflows.map((workflow) => (
                    <SelectItem key={workflow.id} value={workflow.id}>
                      {workflow.name || publishedSchema(workflow)?.name || workflow.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function EmptyEditor() {
  const { t } = useT("workflows");
  return (
    <div className="flex flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">
      {t(($) => $.editor.empty)}
    </div>
  );
}

function sameStepContract(left: WorkflowStep, right: WorkflowStep): boolean {
  return (
    (left.title ?? "") === (right.title ?? "") &&
    (left.description ?? "") === (right.description ?? "") &&
    (left.output?.description ?? "") === (right.output?.description ?? "") &&
    stepGate(left) === stepGate(right) &&
    left.required === right.required
  );
}

function reviewSummary(
  t: ReturnType<typeof useT<"workflows">>["t"],
  published: WorkflowSchema | null,
  draft: WorkflowSchema,
): string[] {
  const draftSteps = draft.steps ?? [];
  if (!published) {
    return [
      t(($) => $.review.initial_publish, {
        count: draftSteps.length,
      }),
    ];
  }
  const items: string[] = [];
  const publishedSteps = published.steps ?? [];
  const publishedById = new Map(
    publishedSteps.map((step, index) => [step.id, { step, index }]),
  );
  const draftById = new Map(
    draftSteps.map((step, index) => [step.id, { step, index }]),
  );
  for (const [id, { step, index }] of draftById) {
    const before = publishedById.get(id);
    if (!before) {
      items.push(
        t(($) => $.review.step_added, {
          title: step.title || id,
          number: String(index + 1),
        }),
      );
      continue;
    }
    if (before.index !== index) {
      items.push(
        t(($) => $.review.step_moved, {
          title: step.title || id,
          from: String(before.index + 1),
          to: String(index + 1),
        }),
      );
    }
    if (!sameStepContract(before.step, step)) {
      items.push(
        t(($) => $.review.step_changed, {
          title: step.title || id,
        }),
      );
    }
  }
  for (const [id, { step }] of publishedById) {
    if (!draftById.has(id)) {
      items.push(
        t(($) => $.review.step_removed, {
          title: step.title || id,
        }),
      );
    }
  }
  if ((published.name ?? "") !== (draft.name ?? "")) {
    items.push(t(($) => $.review.name_changed));
  }
  if ((published.description ?? "") !== (draft.description ?? "")) {
    items.push(t(($) => $.review.description_changed));
  }
  if (
    JSON.stringify(published.applicability ?? []) !==
    JSON.stringify(draft.applicability ?? [])
  ) {
    items.push(t(($) => $.review.applicability_changed));
  }
  return items.length ? items : [t(($) => $.review.no_changes)];
}

function diffCellTone(row: SchemaDiffRow, side: "left" | "right"): string {
  if (row.kind === "removed" && side === "left") {
    return "bg-destructive/10 text-destructive";
  }
  if (row.kind === "removed" && side === "right") {
    return "bg-destructive/5";
  }
  if (row.kind === "added" && side === "right") {
    return "bg-success/10 text-success";
  }
  if (row.kind === "added" && side === "left") {
    return "bg-success/5";
  }
  if (row.kind === "modified") {
    return "bg-warning/10";
  }
  return "bg-background";
}

function diffMarker(row: SchemaDiffRow, side: "left" | "right"): string {
  if (row.kind === "removed" && side === "left") return "-";
  if (row.kind === "added" && side === "right") return "+";
  if (row.kind === "modified") return "~";
  return "";
}

function SchemaDiffCell({
  row,
  side,
}: {
  row: SchemaDiffRow;
  side: "left" | "right";
}) {
  const line = side === "left" ? row.leftLine : row.rightLine;
  const text = side === "left" ? row.leftText : row.rightText;

  return (
    <div
      className={cn(
        "grid min-h-6 w-full grid-cols-[3.5rem_minmax(0,1fr)] border-b border-border/50 font-mono text-xs leading-5",
        side === "left" && "md:border-r",
        diffCellTone(row, side),
      )}
    >
      <span className="select-none border-r border-border/50 bg-muted/30 px-2 py-0.5 text-right tabular-nums text-muted-foreground/70">
        {line ?? ""}
      </span>
      <span className="grid grid-cols-[1.25rem_minmax(0,1fr)] px-2 py-0.5">
        <span className="select-none text-center tabular-nums text-muted-foreground/70">
          {diffMarker(row, side)}
        </span>
        <code
          data-testid="schema-diff-code"
          className="min-w-0 whitespace-pre-wrap break-words [overflow-wrap:anywhere]"
        >
          {text}
        </code>
      </span>
    </div>
  );
}

function SchemaDiffRowView({ row }: { row: SchemaDiffRow }) {
  return (
    <div className="grid min-w-0 grid-cols-1 md:grid-cols-2">
      <SchemaDiffCell row={row} side="left" />
      <SchemaDiffCell row={row} side="right" />
    </div>
  );
}

function SchemaDiffViewer({
  published,
  draft,
}: {
  published: WorkflowSchema | null;
  draft: WorkflowSchema;
}) {
  const { t } = useT("workflows");
  const publishedText = published ? formatSchema(published) : "";
  const draftText = formatSchema(draft);
  const diff = useMemo(
    () => buildSchemaLineDiff(publishedText, draftText),
    [publishedText, draftText],
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b bg-muted/20 px-3 py-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge
            variant="outline"
            className="h-5 rounded-md border-success/30 bg-success/10 px-1.5 font-mono text-[10px] text-success"
          >
            {t(($) => $.review.schema_diff_added, { count: diff.added })}
          </Badge>
          <Badge
            variant="outline"
            className="h-5 rounded-md border-destructive/30 bg-destructive/10 px-1.5 font-mono text-[10px] text-destructive"
          >
            {t(($) => $.review.schema_diff_removed, { count: diff.removed })}
          </Badge>
          <Badge
            variant="outline"
            className="h-5 rounded-md border-warning/30 bg-warning/10 px-1.5 font-mono text-[10px] text-warning"
          >
            {t(($) => $.review.schema_diff_modified, {
              count: diff.modified,
            })}
          </Badge>
        </div>
        <span className="font-mono text-xs tabular-nums text-muted-foreground">
          {t(($) => $.review.schema_diff_changed_lines, {
            count: diff.changed,
          })}
        </span>
      </div>

      <div className="grid border-b bg-muted/30 text-xs font-medium md:grid-cols-2">
        <div className="flex min-w-0 items-center justify-between gap-3 border-b px-3 py-2 md:border-b-0 md:border-r">
          <span>{t(($) => $.review.published_schema)}</span>
          <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {published
              ? t(($) => $.review.schema_diff_line_count, {
                  count: diff.leftLineCount,
                })
              : t(($) => $.review.none)}
          </span>
        </div>
        <div className="flex min-w-0 items-center justify-between gap-3 px-3 py-2">
          <span>{t(($) => $.review.draft_schema)}</span>
          <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {t(($) => $.review.schema_diff_line_count, {
              count: diff.rightLineCount,
            })}
          </span>
        </div>
      </div>

      <div
        data-testid="schema-diff-scroll"
        className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto"
      >
        {diff.rows.map((row) => (
          <SchemaDiffRowView key={row.key} row={row} />
        ))}
      </div>
    </div>
  );
}

function ReviewChangesDialog({
  open,
  onOpenChange,
  workflow,
  draft,
  published,
  onPublish,
  onDiscard,
  isPublishing,
  isDiscarding,
  validation,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workflow: WorkflowDefinition;
  draft: WorkflowSchema;
  published: WorkflowSchema | null;
  onPublish: () => void;
  onDiscard: () => void;
  isPublishing: boolean;
  isDiscarding: boolean;
  validation?: WorkflowValidation | null;
}) {
  const { t } = useT("workflows");
  const issues = validation?.issues ?? [];
  const publishable = validation?.publishable !== false;
  const summary = reviewSummary(t, published, draft);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[86vh] w-[min(96vw,80rem)] max-w-none grid-rows-none flex-col overflow-hidden sm:max-w-none">
        <DialogHeader>
          <DialogTitle>{t(($) => $.review.title)}</DialogTitle>
          <DialogDescription>{t(($) => $.review.description)}</DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="summary" className="min-h-0 flex-1 gap-3">
          <TabsList
            variant="line"
            className="h-8 !flex-row !items-center !justify-start"
          >
            <TabsTrigger
              value="summary"
              className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
            >
              {t(($) => $.review.summary_tab)}
            </TabsTrigger>
            <TabsTrigger
              value="schema"
              className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
            >
              {t(($) => $.review.schema_tab)}
            </TabsTrigger>
          </TabsList>

          <TabsContent
            value="summary"
            className="min-h-0 flex-1 overflow-y-auto rounded-md border bg-background p-3"
          >
            <div className="space-y-3">
              <div className="space-y-2">
                {summary.map((item) => (
                  <div
                    key={item}
                    className="flex items-start gap-2 rounded-md bg-muted/45 px-3 py-2 text-sm"
                  >
                    <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-success" />
                    <span>{item}</span>
                  </div>
                ))}
              </div>
              {issues.length > 0 && (
                <div className="space-y-2">
                  {issues.map((issue) => (
                    <div
                      key={`${issue.code}-${issue.message}`}
                      className={cn(
                        "flex items-start gap-2 rounded-md px-3 py-2 text-sm",
                        issue.severity === "blocking"
                          ? "bg-destructive/10 text-destructive"
                          : "bg-warning/10 text-warning",
                      )}
                    >
                      <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                      <span>{issue.message}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent
            value="schema"
            className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border bg-background"
          >
            <SchemaDiffViewer published={published} draft={draft} />
          </TabsContent>
        </Tabs>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onDiscard}
            disabled={isDiscarding || isPublishing || !draftRevision(workflow)}
          >
            {isDiscarding ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <Trash2 className="h-3 w-3" />
            )}
            {t(($) => $.review.discard)}
          </Button>
          <Button
            type="button"
            onClick={onPublish}
            disabled={!publishable || isPublishing || isDiscarding}
          >
            {isPublishing ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <GitBranch className="h-3 w-3" />
            )}
            {t(($) => $.review.publish)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function WorkflowEditor({
  workflow,
  assignmentWorkflows,
  projects,
  onSelect,
}: {
  workflow: WorkflowDefinition | null;
  assignmentWorkflows: WorkflowDefinition[];
  projects: Project[];
  onSelect: (id: string) => void;
}) {
  const { t } = useT("workflows");
  const [schema, setSchema] = useState<WorkflowSchema>(EMPTY_SCHEMA);
  const [schemaText, setSchemaText] = useState(formatSchema(EMPTY_SCHEMA));
  const [schemaError, setSchemaError] = useState("");
  const [preview, setPreview] = useState("");
  const [previewWarnings, setPreviewWarnings] = useState<string[]>([]);
  const [previewError, setPreviewError] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [graphExpanded, setGraphExpanded] = useState(false);
  const [selectedStepIndex, setSelectedStepIndex] = useState(0);
  const [inspectorSection, setInspectorSection] =
    useState<InspectorSection>("basics");
  const [editingTitleIndex, setEditingTitleIndex] = useState<number | null>(
    null,
  );
  const [titleDraft, setTitleDraft] = useState("");
  const [promptEditorOpen, setPromptEditorOpen] = useState(false);
  const [promptEditorStepIndex, setPromptEditorStepIndex] = useState(0);
  const [promptDraft, setPromptDraft] = useState("");
  const [checklistDraft, setChecklistDraft] = useState("");
  const [reviewOpen, setReviewOpen] = useState(false);
  const [outputHelpOpen, setOutputHelpOpen] = useState(false);
  const [localDraftValidation, setLocalDraftValidation] =
    useState<WorkflowValidation | null>(null);
  const lastSavedSchemaRef = useRef("");
  const updateDraftRef = useRef<ReturnType<typeof useUpdateWorkflowDraft> | null>(
    null,
  );

  const updateWorkflowDraft = useUpdateWorkflowDraft();
  const publishWorkflow = usePublishWorkflow();
  const forkWorkflow = useForkWorkflow();
  const deleteWorkflow = useDeleteWorkflow();
  const deleteWorkflowDraft = useDeleteWorkflowDraft();

  const isSystem = workflow?.origin === "system_seeded";
  const hasPublished = !!publishedRevision(workflow);
  const hasDraft = !!draftRevision(workflow);
  const isSavingDraft = updateWorkflowDraft.isPending;

  useEffect(() => {
    updateDraftRef.current = updateWorkflowDraft;
  }, [updateWorkflowDraft]);

  useEffect(() => {
    if (!workflow) return;
    const next = authoringSchema(workflow);
    const formatted = formatSchema(next);
    setSchema(next);
    setSchemaText(formatted);
    setSchemaError("");
    lastSavedSchemaRef.current = formatted;
    setPreview("");
    setPreviewWarnings([]);
    setPreviewError("");
    setGraphExpanded(false);
    setSelectedStepIndex(0);
    setInspectorSection("basics");
    setEditingTitleIndex(null);
    setTitleDraft("");
    setPromptEditorOpen(false);
    setReviewOpen(false);
    setOutputHelpOpen(false);
    setLocalDraftValidation(draftRevision(workflow)?.validation ?? null);
  }, [workflow]);

  useEffect(() => {
    const stepCount = schema.steps?.length ?? 0;
    if (stepCount === 0) {
      setSelectedStepIndex(0);
      setEditingTitleIndex(null);
      return;
    }
    if (selectedStepIndex >= stepCount) {
      setSelectedStepIndex(stepCount - 1);
      setInspectorSection("basics");
      setEditingTitleIndex(null);
    }
  }, [schema.steps?.length, selectedStepIndex]);

  useEffect(() => {
    if (!workflow) return;
    const timer = window.setTimeout(async () => {
      setPreviewLoading(true);
      setPreviewError("");
      try {
        const result = await api.previewWorkflow({
          schema,
          issue_id: SAMPLE_ISSUE_ID,
          trigger_comment_id: SAMPLE_COMMENT_ID,
        });
        setPreview(result.rendered_markdown);
        setPreviewWarnings(result.warnings ?? []);
      } catch (err) {
        setPreviewError(
          err instanceof Error ? err.message : t(($) => $.editor.preview_failed),
        );
      } finally {
        setPreviewLoading(false);
      }
    }, 300);
    return () => window.clearTimeout(timer);
  }, [schema, t, workflow]);

  const serializedSchema = formatSchema(schema);
  const isDirty = !isSystem && serializedSchema !== lastSavedSchemaRef.current;

  useEffect(() => {
    if (!workflow || isSystem || schemaError) return;
    if (serializedSchema === lastSavedSchemaRef.current) return;
    const timer = window.setTimeout(async () => {
      try {
        const saved = await updateDraftRef.current?.mutateAsync({
          id: workflow.id,
          schema,
        });
        setLocalDraftValidation(saved?.validation ?? null);
        lastSavedSchemaRef.current = serializedSchema;
      } catch (err) {
        toast.error(
          err instanceof Error ? err.message : t(($) => $.editor.save_failed),
        );
      }
    }, 700);
    return () => window.clearTimeout(timer);
  }, [isSystem, schema, schemaError, serializedSchema, t, workflow]);

  if (!workflow) return <EmptyEditor />;

  const applySchema = (next: WorkflowSchema) => {
    const normalized = buildSchema(workflow, next, {
      steps: normalizeSteps(next.steps ?? []),
    });
    setSchema(normalized);
    setSchemaText(formatSchema(normalized));
    setSchemaError("");
  };

  const patchSchema = (patch: Partial<WorkflowSchema>) => {
    applySchema(buildSchema(workflow, schema, patch));
  };

  const saveDraftNow = async (): Promise<boolean> => {
    if (!workflow || isSystem || schemaError) return false;
    const serialized = formatSchema(schema);
    if (serialized === lastSavedSchemaRef.current) return true;
    try {
      const saved = await updateDraftRef.current?.mutateAsync({
        id: workflow.id,
        schema,
      });
      setLocalDraftValidation(saved?.validation ?? null);
      lastSavedSchemaRef.current = serialized;
      return true;
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.save_failed),
      );
      return false;
    }
  };

  const toggleApplicability = (value: WorkflowApplicability) => {
    const current = schema.applicability ?? ["assignment"];
    const next = current.includes(value)
      ? current.filter((item) => item !== value)
      : [...current, value];
    patchSchema({ applicability: next.length > 0 ? next : current });
  };

  const addStep = () => {
    const prev = schema.steps ?? [];
    patchSchema({
      steps: normalizeSteps([
        ...prev,
        {
          id: `step-${prev.length + 1}`,
          title: t(($) => $.steps.new_step_title, { number: prev.length + 1 }),
          order: prev.length + 1,
        },
      ]),
    });
  };

  const updateStep = (index: number, patch: Partial<WorkflowStep>) => {
    patchSchema({
      steps: normalizeSteps(
        (schema.steps ?? []).map((step, i) =>
          i === index ? { ...step, ...patch } : step,
        ),
      ),
    });
  };

  const selectStep = (index: number, section: InspectorSection = "basics") => {
    setSelectedStepIndex(index);
    setInspectorSection(section);
    setEditingTitleIndex(null);
  };

  const beginTitleEdit = (index: number) => {
    const step = schema.steps?.[index];
    if (!step || isSystem) return;
    setSelectedStepIndex(index);
    setEditingTitleIndex(index);
    setTitleDraft(step.title ?? "");
  };

  const commitTitleEdit = () => {
    if (editingTitleIndex === null) return;
    const title = titleDraft.trim();
    if (title) {
      updateStep(editingTitleIndex, { title });
    }
    setEditingTitleIndex(null);
    setTitleDraft("");
  };

  const moveStep = (index: number, direction: -1 | 1) => {
    const steps = schema.steps ?? [];
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= steps.length || isSystem) return;
    const next = steps.slice();
    const [step] = next.splice(index, 1);
    if (!step) return;
    next.splice(targetIndex, 0, step);
    patchSchema({
      steps: normalizeSteps(next.map((item, i) => ({ ...item, order: i + 1 }))),
    });
    setSelectedStepIndex(targetIndex);
    setInspectorSection("basics");
    setEditingTitleIndex(null);
  };

  const openPromptEditor = (index: number) => {
    const step = schema.steps?.[index];
    if (!step) return;
    setSelectedStepIndex(index);
    setPromptEditorStepIndex(index);
    setPromptDraft(step.body_template ?? "");
    setChecklistDraft(listText(step.checklist));
    setPromptEditorOpen(true);
  };

  const applyPromptEditor = () => {
    updateStep(promptEditorStepIndex, {
      body_template: promptDraft || undefined,
      checklist: parseLines(checklistDraft),
    });
    setPromptEditorOpen(false);
  };

  const removeStep = (index: number) => {
    patchSchema({
      steps: normalizeSteps(
        (schema.steps ?? [])
          .filter((_, i) => i !== index)
          .map((step, i) => ({ ...step, order: i + 1 })),
      ),
    });
    setSelectedStepIndex(Math.max(0, Math.min(index, (schema.steps?.length ?? 1) - 2)));
    setInspectorSection("basics");
    setEditingTitleIndex(null);
  };

  const fork = async () => {
    try {
      const forked = await forkWorkflow.mutateAsync({
        id: workflow.id,
        name: `${workflowDisplayName(workflow)} copy`,
        description: workflowDisplayDescription(workflow),
      });
      toast.success(t(($) => $.editor.forked));
      onSelect(forked.id);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.fork_failed),
      );
    }
  };

  const openReview = async () => {
    if (!workflow || isSystem || schemaError) return;
    const saved = await saveDraftNow();
    if (saved) setReviewOpen(true);
  };

  const publish = async () => {
    if (!workflow || isSystem || !schema.name?.trim()) return;
    const saved = await saveDraftNow();
    if (!saved) return;
    try {
      const updated = await publishWorkflow.mutateAsync({ id: workflow.id });
      toast.success(t(($) => $.editor.published));
      setReviewOpen(false);
      onSelect(updated.id);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.publish_failed),
      );
    }
  };

  const discardDraft = async () => {
    if (!workflow || isSystem || !hasDraft) return;
    try {
      if (!hasPublished) {
        await deleteWorkflow.mutateAsync(workflow.id);
        toast.success(t(($) => $.editor.deleted));
      } else {
        await deleteWorkflowDraft.mutateAsync(workflow.id);
        toast.success(t(($) => $.editor.draft_discarded));
      }
      setReviewOpen(false);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.archive_failed),
      );
    }
  };

  const archive = async () => {
    if (!workflow || isSystem) return;
    try {
      await deleteWorkflow.mutateAsync(workflow.id);
      toast.success(
        hasPublished ? t(($) => $.editor.archived) : t(($) => $.editor.deleted),
      );
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.archive_failed),
      );
    }
  };

  const steps = schema.steps ?? [];
  const selectedStep = steps[selectedStepIndex] ?? null;
  const promptEditorStep = steps[promptEditorStepIndex] ?? null;
  const selectedDependencyOptions = selectedStep
    ? dependencyOptions(steps, selectedStepIndex)
    : [];
  const selectedDependencySummary = selectedStep
    ? dependencySummary(steps, selectedStep)
    : "";

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center justify-between gap-3 border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <Workflow className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-medium">
            {schema.name || workflowDisplayName(workflow)}
          </span>
          {isSystem && (
            <Badge variant="outline" className="h-5 rounded-md">
              <Lock className="h-3 w-3" />
              {t(($) => $.origin.system)}
            </Badge>
          )}
          {!isSystem && hasDraft && (
            <Badge variant="secondary" className="h-5 rounded-md">
              {hasPublished
                ? t(($) => $.status.unpublished_changes)
                : t(($) => $.status.draft)}
            </Badge>
          )}
          {!isSystem && isSavingDraft && (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              {t(($) => $.editor.saving)}
            </span>
          )}
          {!isSystem && !isSavingDraft && isDirty && (
            <span className="text-xs text-muted-foreground">
              {t(($) => $.editor.unsaved)}
            </span>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {isSystem ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={fork}
              disabled={forkWorkflow.isPending}
            >
              {forkWorkflow.isPending ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <Copy className="h-3 w-3" />
              )}
              {t(($) => $.editor.fork_to_edit)}
            </Button>
          ) : (
            <>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={archive}
                disabled={deleteWorkflow.isPending}
              >
                <Trash2 className="h-3 w-3" />
                {hasPublished
                  ? t(($) => $.editor.archive)
                  : t(($) => $.editor.delete)}
              </Button>
              <Button
                type="button"
                size="sm"
                onClick={openReview}
                disabled={
                  !schema.name?.trim() ||
                  !!schemaError ||
                  (!hasDraft && !isDirty) ||
                  isSavingDraft ||
                  publishWorkflow.isPending
                }
              >
                {isSavingDraft || publishWorkflow.isPending ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <GitBranch className="h-3 w-3" />
                )}
                {t(($) => $.review.open)}
              </Button>
            </>
          )}
        </div>
      </div>

      <Tabs
        defaultValue="steps"
        data-testid="workflow-editor-tabs"
        className="min-h-0 flex-1 flex-col gap-0"
      >
        <div className="flex h-10 shrink-0 items-center border-b px-4">
          <TabsList
            variant="line"
            data-testid="workflow-editor-tabs-list"
            className="h-8 !flex-row !items-center !justify-start"
          >
            <TabsTrigger
              value="steps"
              className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
            >
              <ListChecks className="h-3.5 w-3.5" />
              {t(($) => $.tabs.steps)}
            </TabsTrigger>
            <TabsTrigger
              value="schema"
              className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
            >
              <FileText className="h-3.5 w-3.5" />
              {t(($) => $.tabs.schema)}
            </TabsTrigger>
            <TabsTrigger
              value="preview"
              className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
            >
              <Eye className="h-3.5 w-3.5" />
              {t(($) => $.tabs.preview)}
            </TabsTrigger>
          </TabsList>
          {previewLoading && (
            <Loader2 className="ml-auto h-3.5 w-3.5 animate-spin text-muted-foreground" />
          )}
        </div>

        <TabsContent value="schema" className="h-full min-h-0 overflow-y-auto p-4">
          <div className="grid gap-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-medium">
                  {t(($) => $.schema.title)}
                </h2>
                <p className="text-xs text-muted-foreground">
                  {t(($) => $.schema.description)}
                </p>
              </div>
              {!schemaError && (
                <Badge variant="outline" className="h-6 rounded-md">
                  <Save className="h-3 w-3" />
                  {isSavingDraft
                    ? t(($) => $.editor.saving)
                    : t(($) => $.editor.autosaved)}
                </Badge>
              )}
            </div>
            {schemaError && (
              <div className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
                <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                <span>{schemaError}</span>
              </div>
            )}
            <Textarea
              value={schemaText}
              onChange={(e) => {
                const nextText = e.target.value;
                setSchemaText(nextText);
                try {
                  const parsed = JSON.parse(nextText) as WorkflowSchema;
                  const normalized = buildSchema(workflow, parsed, {
                    steps: normalizeSteps(parsed.steps ?? []),
                  });
                  setSchemaError("");
                  setSchema(normalized);
                } catch (err) {
                  setSchemaError(
                    err instanceof Error
                      ? err.message
                      : t(($) => $.schema.parse_failed),
                  );
                }
              }}
              disabled={isSystem}
              spellCheck={false}
              className="min-h-[520px] resize-y font-mono text-xs leading-5"
            />
          </div>
        </TabsContent>

        <TabsContent value="steps" className="h-full min-h-0 overflow-hidden p-4">
          <div className="mb-4 grid gap-3 rounded-md border bg-muted/15 p-3 md:grid-cols-[minmax(180px,0.35fr)_minmax(220px,1fr)_auto]">
            <div className="space-y-1.5">
              <Label htmlFor="workflow-name" className="text-xs">
                {t(($) => $.editor.name_label)}
              </Label>
              <Input
                id="workflow-name"
                value={schema.name ?? ""}
                onChange={(e) => patchSchema({ name: e.target.value })}
                disabled={isSystem}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="workflow-description" className="text-xs">
                {t(($) => $.editor.description_label)}
              </Label>
              <Input
                id="workflow-description"
                value={schema.description ?? ""}
                onChange={(e) => patchSchema({ description: e.target.value })}
                disabled={isSystem}
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">
                {t(($) => $.editor.applicability_label)}
              </Label>
              <div className="flex flex-wrap gap-2">
                {(["assignment", "comment"] as WorkflowApplicability[]).map(
                  (item) => (
                    <Button
                      key={item}
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={isSystem}
                      className={cn(
                        "h-8 px-2 text-xs",
                        (schema.applicability ?? ["assignment"]).includes(
                          item,
                        ) &&
                          "bg-accent text-accent-foreground hover:bg-accent/80",
                      )}
                      onClick={() => toggleApplicability(item)}
                    >
                      {(schema.applicability ?? ["assignment"]).includes(
                        item,
                      ) && <Check className="h-3 w-3" />}
                      {applicabilityLabel(t, item)}
                    </Button>
                  ),
                )}
              </div>
            </div>
          </div>

          <div className="grid h-[calc(100%-5.75rem)] min-h-[460px] gap-4 lg:grid-cols-[minmax(280px,0.42fr)_minmax(420px,0.58fr)]">
            <div className="flex min-h-0 flex-col rounded-md border bg-background">
              <div className="flex h-11 shrink-0 items-center justify-between gap-3 border-b px-3">
                <div className="min-w-0">
                  <h2 className="text-sm font-medium">{t(($) => $.steps.title)}</h2>
                  <p className="truncate text-xs text-muted-foreground">
                    {t(($) => $.steps.description)}
                  </p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={addStep}
                  disabled={isSystem}
                >
                  <Plus className="h-3 w-3" />
                  {t(($) => $.steps.add)}
                </Button>
              </div>

              {steps.length === 0 ? (
                <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
                  {t(($) => $.steps.empty)}
                </div>
              ) : (
                <div className="min-h-0 flex-1 overflow-y-auto p-2">
                  <div className="space-y-1.5">
                    {steps.map((step, index) => {
                      const gate = stepGate(step);
                      const selected = index === selectedStepIndex;
                      const customDependencies = hasCustomDependencies(steps, index);
                      return (
                        <div
                          key={`${step.id}-${index}`}
                          role="button"
                          tabIndex={0}
                          onClick={() => selectStep(index)}
                          onKeyDown={(event) => {
                            if (isEditableTarget(event.target)) return;
                            if (event.key === "Enter" || event.key === " ") {
                              event.preventDefault();
                              selectStep(index);
                            }
                          }}
                          className={cn(
                            "group grid w-full grid-cols-[2rem_minmax(0,1fr)] gap-2 rounded-md border px-2.5 py-2 text-left transition-colors",
                            selected
                              ? "border-primary/40 bg-accent text-accent-foreground"
                              : "border-transparent bg-transparent hover:border-border hover:bg-muted/45",
                          )}
                        >
                          <span className="flex h-7 w-7 items-center justify-center rounded-md bg-muted font-mono text-xs text-muted-foreground">
                            {index + 1}
                          </span>
                          <span className="min-w-0">
                            <span className="flex min-w-0 items-center gap-1.5">
                              {editingTitleIndex === index ? (
                                <span
                                  className="flex min-w-0 flex-1 items-center gap-1"
                                  onClick={(event) => event.stopPropagation()}
                                >
                                  <Input
                                    value={titleDraft}
                                    onChange={(event) =>
                                      setTitleDraft(event.target.value)
                                    }
                                    onKeyDown={(event) => {
                                      if (event.key === "Enter") {
                                        event.preventDefault();
                                        commitTitleEdit();
                                      }
                                      if (event.key === "Escape") {
                                        setEditingTitleIndex(null);
                                        setTitleDraft("");
                                      }
                                    }}
                                    autoFocus
                                    disabled={isSystem}
                                    className="h-7 min-w-0 flex-1 text-sm"
                                  />
                                  <Button
                                    type="button"
                                    size="icon"
                                    variant="ghost"
                                    className="h-7 w-7"
                                    onClick={commitTitleEdit}
                                    disabled={isSystem}
                                    aria-label={t(($) => $.steps.save_title)}
                                  >
                                    <Check className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button
                                    type="button"
                                    size="icon"
                                    variant="ghost"
                                    className="h-7 w-7"
                                    onClick={() => {
                                      setEditingTitleIndex(null);
                                      setTitleDraft("");
                                    }}
                                    aria-label={t(($) => $.steps.cancel_title)}
                                  >
                                    <X className="h-3.5 w-3.5" />
                                  </Button>
                                </span>
                              ) : (
                                <>
                                  <span className="min-w-0 flex-1 truncate text-sm font-medium">
                                    {step.title}
                                  </span>
                                  <Button
                                    type="button"
                                    size="icon"
                                    variant="ghost"
                                    className="h-6 w-6 opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      beginTitleEdit(index);
                                    }}
                                    disabled={isSystem}
                                    aria-label={t(($) => $.steps.edit_title)}
                                  >
                                    <Pencil className="h-3.5 w-3.5" />
                                  </Button>
                                </>
                              )}
                            </span>

                            <span className="mt-1 block truncate text-xs text-muted-foreground">
                              {step.description || t(($) => $.steps.no_purpose)}
                            </span>

                            <span className="mt-2 flex flex-wrap items-center gap-1.5">
                              <Badge
                                variant="outline"
                                className="h-5 rounded-md px-1.5 text-[10px]"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  selectStep(index, "checks");
                                }}
                              >
                                <ShieldCheck className="h-3 w-3" />
                                {stepGateLabel(t, gate)}
                              </Badge>
                              <Badge
                                variant={step.required === false ? "outline" : "secondary"}
                                className="h-5 rounded-md px-1.5 text-[10px]"
                              >
                                {step.required === false
                                  ? t(($) => $.steps.optional)
                                  : t(($) => $.steps.required)}
                              </Badge>
                              {step.artifact && (
                                <Badge
                                  variant="outline"
                                  className="h-5 rounded-md px-1.5 text-[10px]"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    selectStep(index, "outputs");
                                  }}
                                >
                                  <Package className="h-3 w-3" />
                                  {t(($) => $.steps.artifact_chip)}
                                </Badge>
                              )}
                              {customDependencies && (
                                <Badge
                                  variant="outline"
                                  className="h-5 rounded-md px-1.5 text-[10px]"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    selectStep(index, "inputs");
                                  }}
                                >
                                  {t(($) => $.steps.custom_inputs)}
                                </Badge>
                              )}
                            </span>
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            <div className="flex min-h-0 flex-col rounded-md border bg-background">
              {selectedStep ? (
                <>
                  <div className="flex h-11 shrink-0 items-center justify-between gap-3 border-b px-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="flex h-6 min-w-6 items-center justify-center rounded-md bg-muted font-mono text-[11px] text-muted-foreground">
                          {selectedStepIndex + 1}
                        </span>
                        <h3 className="truncate text-sm font-medium">
                          {selectedStep.title}
                        </h3>
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        onClick={() => moveStep(selectedStepIndex, -1)}
                        disabled={isSystem || selectedStepIndex === 0}
                        aria-label={t(($) => $.steps.move_up)}
                      >
                        <ArrowUp className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        onClick={() => moveStep(selectedStepIndex, 1)}
                        disabled={isSystem || selectedStepIndex === steps.length - 1}
                        aria-label={t(($) => $.steps.move_down)}
                      >
                        <ArrowDown className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        onClick={() => removeStep(selectedStepIndex)}
                        disabled={isSystem}
                        aria-label={t(($) => $.steps.remove)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>

                  <Tabs
                    value={inspectorSection}
                    onValueChange={(value) =>
                      setInspectorSection(value as InspectorSection)
                    }
                    className="min-h-0 flex-1 gap-0"
                  >
                    <div className="flex h-10 shrink-0 items-center border-b px-3">
                      <TabsList
                        variant="line"
                        className="h-8 !flex-row !items-center !justify-start"
                      >
                        {[
                          ["basics", t(($) => $.steps.section_basics)],
                          ["instructions", t(($) => $.steps.section_instructions)],
                          ["inputs", t(($) => $.steps.section_inputs)],
                          ["outputs", t(($) => $.steps.section_outputs)],
                          ["checks", t(($) => $.steps.section_checks)],
                        ].map(([value, label]) => (
                          <TabsTrigger
                            key={value}
                            value={value}
                            className="!w-auto !justify-center after:!inset-x-0 after:!bottom-[-5px] after:!top-auto after:!right-auto after:!h-0.5 after:!w-auto"
                          >
                            {label}
                          </TabsTrigger>
                        ))}
                      </TabsList>
                    </div>

                    <TabsContent
                      value="basics"
                      className="min-h-0 flex-1 overflow-y-auto p-4"
                    >
                      <div className="grid gap-4">
                        <div className="grid gap-3 md:grid-cols-2">
                          <div className="space-y-1.5">
                            <Label
                              htmlFor={`workflow-step-description-${selectedStepIndex}`}
                              className="text-xs"
                            >
                              {t(($) => $.steps.purpose_label)}
                            </Label>
                            <Textarea
                              id={`workflow-step-description-${selectedStepIndex}`}
                              value={selectedStep.description ?? ""}
                              onChange={(event) =>
                                updateStep(selectedStepIndex, {
                                  description: event.target.value,
                                })
                              }
                              disabled={isSystem}
                              className="min-h-28 resize-y text-sm leading-5"
                            />
                          </div>
                          <div className="space-y-1.5">
                            <div className="flex items-center gap-1.5">
                              <Label
                                htmlFor={`workflow-step-output-${selectedStepIndex}`}
                                className="text-xs"
                              >
                                {t(($) => $.steps.output_label)}
                              </Label>
                              <Tooltip
                                open={outputHelpOpen}
                                onOpenChange={setOutputHelpOpen}
                              >
                                <TooltipTrigger
                                  render={
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="icon-xs"
                                      className="h-5 w-5 text-muted-foreground hover:text-foreground"
                                      aria-label={t(($) => $.steps.output_help_label)}
                                      onClick={() => setOutputHelpOpen((open) => !open)}
                                      onMouseEnter={() => setOutputHelpOpen(true)}
                                      onMouseLeave={() => setOutputHelpOpen(false)}
                                      onFocus={() => setOutputHelpOpen(true)}
                                      onBlur={() => setOutputHelpOpen(false)}
                                    >
                                      <Info className="h-3.5 w-3.5" />
                                    </Button>
                                  }
                                />
                                <TooltipContent side="top" className="max-w-72">
                                  {t(($) => $.steps.output_help)}
                                </TooltipContent>
                              </Tooltip>
                            </div>
                            <Textarea
                              id={`workflow-step-output-${selectedStepIndex}`}
                              value={selectedStep.output?.description ?? ""}
                              onChange={(event) =>
                                updateStep(selectedStepIndex, {
                                  output: { description: event.target.value },
                                })
                              }
                              disabled={isSystem}
                              className="min-h-28 resize-y text-sm leading-5"
                            />
                          </div>
                        </div>

                        <div className="grid gap-3 md:grid-cols-[minmax(180px,0.4fr)_minmax(220px,0.6fr)]">
                          <div className="space-y-1.5">
                            <Label className="text-xs">
                              {t(($) => $.steps.gate_label)}
                            </Label>
                            <Select
                              value={stepGate(selectedStep)}
                              onValueChange={(value) =>
                                updateStep(
                                  selectedStepIndex,
                                  gatePatchForStep(selectedStep, value as StepGate),
                                )
                              }
                              disabled={isSystem}
                            >
                              <SelectTrigger size="sm">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="none">
                                  {t(($) => $.steps.gate_none)}
                                </SelectItem>
                                <SelectItem value="human">
                                  {t(($) => $.steps.gate_human)}
                                </SelectItem>
                                <SelectItem value="quality">
                                  {t(($) => $.steps.gate_quality)}
                                </SelectItem>
                                <SelectItem value="human_quality">
                                  {t(($) => $.steps.gate_human_quality)}
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="flex items-end justify-between gap-3 rounded-md border bg-muted/20 px-3 py-2">
                            <div>
                              <div className="text-xs font-medium">
                                {t(($) => $.steps.required_label)}
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {selectedStep.required === false
                                  ? t(($) => $.steps.optional_hint)
                                  : t(($) => $.steps.required_hint)}
                              </div>
                            </div>
                            <Switch
                              size="sm"
                              checked={selectedStep.required !== false}
                              onCheckedChange={(checked) =>
                                updateStep(selectedStepIndex, {
                                  required: checked ? undefined : false,
                                })
                              }
                              disabled={isSystem}
                              aria-label={t(($) => $.steps.required_label)}
                            />
                          </div>
                        </div>

                        <div className="rounded-md border bg-muted/15 p-3">
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="flex items-center gap-2 text-xs font-medium">
                                <MessageSquareText className="h-3.5 w-3.5 text-muted-foreground" />
                                {t(($) => $.steps.instructions_summary)}
                              </div>
                              <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                                {selectedStep.body_template ||
                                  t(($) => $.steps.instructions_empty)}
                              </p>
                            </div>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => openPromptEditor(selectedStepIndex)}
                            >
                              <Pencil className="h-3 w-3" />
                              {t(($) => $.steps.edit_prompt)}
                            </Button>
                          </div>
                        </div>
                      </div>
                    </TabsContent>

                    <TabsContent
                      value="instructions"
                      className="min-h-0 flex-1 overflow-y-auto p-4"
                    >
                      <div className="grid gap-4">
                        <div className="flex items-center justify-between gap-3">
                          <div>
                            <h4 className="text-sm font-medium">
                              {t(($) => $.steps.instructions_title)}
                            </h4>
                            <p className="text-xs text-muted-foreground">
                              {t(($) => $.steps.instructions_description)}
                            </p>
                          </div>
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            onClick={() => openPromptEditor(selectedStepIndex)}
                          >
                            <Pencil className="h-3 w-3" />
                            {t(($) => $.steps.focus_editor)}
                          </Button>
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">
                            {t(($) => $.steps.body_template_label)}
                          </Label>
                          <Textarea
                            value={selectedStep.body_template ?? ""}
                            onChange={(event) =>
                              updateStep(selectedStepIndex, {
                                body_template: event.target.value,
                              })
                            }
                            disabled={isSystem}
                            className="min-h-48 resize-y font-mono text-xs leading-5"
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">
                            {t(($) => $.steps.checklist_label)}
                          </Label>
                          <Textarea
                            value={listText(selectedStep.checklist)}
                            onChange={(event) =>
                              updateStep(selectedStepIndex, {
                                checklist: parseLines(event.target.value),
                              })
                            }
                            disabled={isSystem}
                            placeholder={t(($) => $.steps.checklist_placeholder)}
                            className="min-h-28 resize-y text-sm leading-5"
                          />
                        </div>
                      </div>
                    </TabsContent>

                    <TabsContent
                      value="inputs"
                      className="min-h-0 flex-1 overflow-y-auto p-4"
                    >
                      <div className="grid gap-4">
                        <div className="rounded-md border bg-muted/15 p-3">
                          <div className="text-xs font-medium">
                            {t(($) => $.steps.default_inputs)}
                          </div>
                          <p className="mt-1 text-sm">
                            {selectedStepIndex === 0
                              ? t(($) => $.steps.first_step_inputs)
                              : t(($) => $.steps.previous_step_input, {
                                  title: steps[selectedStepIndex - 1]?.title ?? "",
                                })}
                          </p>
                        </div>
                        <div className="space-y-1.5 rounded-md border p-3">
                          <Label className="text-xs">
                            {t(($) => $.steps.advanced_dependencies)}
                          </Label>
                          <Popover>
                            <PopoverTrigger
                              render={
                                <Button
                                  type="button"
                                  variant="outline"
                                  className="h-auto min-h-9 w-full justify-between gap-3 px-3 py-2 text-left"
                                  disabled={isSystem || selectedDependencyOptions.length === 0}
                                >
                                  <span className="flex min-w-0 items-center gap-2">
                                    <GitBranch className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                    <span className="min-w-0 truncate text-xs font-normal text-muted-foreground">
                                      {selectedDependencySummary ||
                                        t(($) => $.steps.dependencies_none)}
                                    </span>
                                  </span>
                                  <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                </Button>
                              }
                            />
                            <PopoverContent
                              align="start"
                              className="w-[min(28rem,calc(100vw-2rem))] p-2"
                            >
                              <div className="px-1 pb-1">
                                <div className="text-xs font-medium">
                                  {t(($) => $.steps.depends_on_label)}
                                </div>
                                <p className="mt-0.5 text-xs text-muted-foreground">
                                  {t(($) => $.steps.advanced_dependencies_help)}
                                </p>
                              </div>
                              <div className="max-h-64 space-y-1 overflow-y-auto">
                                {selectedDependencyOptions.map((dependency, optionIndex) => {
                                  const selected = (selectedStep.depends_on ?? []).includes(
                                    dependency.id,
                                  );
                                  return (
                                    <button
                                      key={dependency.id}
                                      type="button"
                                      onClick={() =>
                                        updateStep(selectedStepIndex, {
                                          depends_on: toggleDependency(
                                            steps,
                                            selectedStep,
                                            dependency.id,
                                          ),
                                        })
                                      }
                                      aria-pressed={selected}
                                      className={cn(
                                        "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors",
                                        selected ? "bg-accent" : "hover:bg-accent/50",
                                      )}
                                    >
                                      <Checkbox
                                        checked={selected}
                                        tabIndex={-1}
                                        className="pointer-events-none"
                                      />
                                      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-[11px] text-muted-foreground">
                                        {optionIndex + 1}
                                      </span>
                                      <span className="min-w-0 flex-1">
                                        <span className="block truncate text-sm font-medium">
                                          {dependency.title}
                                        </span>
                                        <span className="block truncate font-mono text-[11px] text-muted-foreground">
                                          {dependency.id}
                                        </span>
                                      </span>
                                    </button>
                                  );
                                })}
                              </div>
                            </PopoverContent>
                          </Popover>
                          {selectedDependencyOptions.length === 0 && (
                            <p className="text-xs text-muted-foreground">
                              {t(($) => $.steps.dependencies_empty)}
                            </p>
                          )}
                          {!!dependencyText(selectedStep) && (
                            <div className="flex flex-wrap gap-1.5">
                              {(selectedStep.depends_on ?? []).map((id) => (
                                <Badge
                                  key={id}
                                  variant="outline"
                                  className="h-5 rounded-md px-1.5 font-mono text-[10px]"
                                >
                                  {id}
                                </Badge>
                              ))}
                            </div>
                          )}
                          <p className="text-xs text-muted-foreground">
                            {t(($) => $.steps.depends_on_placeholder)}
                          </p>
                        </div>
                      </div>
                    </TabsContent>

                    <TabsContent
                      value="outputs"
                      className="min-h-0 flex-1 overflow-y-auto p-4"
                    >
                      <div className="grid gap-4">
                        <div className="flex items-center justify-between gap-3 rounded-md border bg-muted/15 px-3 py-2">
                          <div>
                            <div className="text-xs font-medium">
                              {t(($) => $.steps.artifact_title)}
                            </div>
                            <p className="text-xs text-muted-foreground">
                              {t(($) => $.steps.artifact_description)}
                            </p>
                          </div>
                          <Switch
                            size="sm"
                            checked={!!selectedStep.artifact}
                            onCheckedChange={(checked) =>
                              updateStep(selectedStepIndex, {
                                artifact: checked
                                  ? {
                                      name:
                                        selectedStep.artifact?.name ||
                                        defaultArtifactName(selectedStep),
                                      format:
                                        selectedStep.artifact?.format || "markdown",
                                      template:
                                        selectedStep.artifact?.template || "",
                                    }
                                  : undefined,
                              })
                            }
                            disabled={isSystem}
                            aria-label={t(($) => $.steps.artifact_title)}
                          />
                        </div>

                        {selectedStep.artifact ? (
                          <div className="grid gap-3">
                            <div className="grid gap-3 md:grid-cols-[minmax(180px,1fr)_150px]">
                              <div className="space-y-1.5">
                                <Label className="text-xs">
                                  {t(($) => $.steps.artifact_name)}
                                </Label>
                                <Input
                                  value={selectedStep.artifact.name ?? ""}
                                  onChange={(event) =>
                                    updateStep(selectedStepIndex, {
                                      artifact: {
                                        ...selectedStep.artifact,
                                        name: event.target.value,
                                      },
                                    })
                                  }
                                  disabled={isSystem}
                                />
                              </div>
                              <div className="space-y-1.5">
                                <Label className="text-xs">
                                  {t(($) => $.steps.artifact_format)}
                                </Label>
                                <Select
                                  value={selectedStep.artifact.format ?? "markdown"}
                                  onValueChange={(value) =>
                                    updateStep(selectedStepIndex, {
                                      artifact: {
                                        ...selectedStep.artifact,
                                        format: value ?? "markdown",
                                      },
                                    })
                                  }
                                  disabled={isSystem}
                                >
                                  <SelectTrigger size="sm">
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="markdown">
                                      {t(($) => $.steps.artifact_format_markdown)}
                                    </SelectItem>
                                    <SelectItem value="json">
                                      {t(($) => $.steps.artifact_format_json)}
                                    </SelectItem>
                                    <SelectItem value="text">
                                      {t(($) => $.steps.artifact_format_text)}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>
                            </div>
                            <div className="space-y-1.5">
                              <Label className="text-xs">
                                {t(($) => $.steps.artifact_template)}
                              </Label>
                              <Textarea
                                value={selectedStep.artifact.template ?? ""}
                                onChange={(event) =>
                                  updateStep(selectedStepIndex, {
                                    artifact: {
                                      ...selectedStep.artifact,
                                      template: event.target.value,
                                    },
                                  })
                                }
                                disabled={isSystem}
                                className="min-h-40 resize-y font-mono text-xs leading-5"
                              />
                            </div>
                          </div>
                        ) : (
                          <div className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                            {t(($) => $.steps.artifact_empty)}
                          </div>
                        )}
                      </div>
                    </TabsContent>

                    <TabsContent
                      value="checks"
                      className="min-h-0 flex-1 overflow-y-auto p-4"
                    >
                      <div className="grid gap-4">
                        <div className="space-y-1.5">
                          <Label className="text-xs">
                            {t(($) => $.steps.gate_label)}
                          </Label>
                          <Select
                            value={stepGate(selectedStep)}
                            onValueChange={(value) =>
                              updateStep(
                                selectedStepIndex,
                                gatePatchForStep(selectedStep, value as StepGate),
                              )
                            }
                            disabled={isSystem}
                          >
                            <SelectTrigger size="sm">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="none">
                                {t(($) => $.steps.gate_none)}
                              </SelectItem>
                              <SelectItem value="human">
                                {t(($) => $.steps.gate_human)}
                              </SelectItem>
                              <SelectItem value="quality">
                                {t(($) => $.steps.gate_quality)}
                              </SelectItem>
                              <SelectItem value="human_quality">
                                {t(($) => $.steps.gate_human_quality)}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>

                        {selectedStep.review?.required && (
                          <div className="grid gap-3 rounded-md border p-3">
                            <div className="text-xs font-medium">
                              {t(($) => $.steps.review_details)}
                            </div>
                            <div className="space-y-1.5">
                              <Label className="text-xs">
                                {t(($) => $.steps.reviewer_role)}
                              </Label>
                              <Input
                                value={selectedStep.review.reviewer_role ?? ""}
                                onChange={(event) =>
                                  updateStep(selectedStepIndex, {
                                    review: {
                                      ...selectedStep.review,
                                      required: true,
                                      reviewer_role: event.target.value,
                                    },
                                  })
                                }
                                disabled={isSystem}
                              />
                            </div>
                            <div className="space-y-1.5">
                              <Label className="text-xs">
                                {t(($) => $.steps.review_instructions)}
                              </Label>
                              <Textarea
                                value={selectedStep.review.instructions ?? ""}
                                onChange={(event) =>
                                  updateStep(selectedStepIndex, {
                                    review: {
                                      ...selectedStep.review,
                                      required: true,
                                      instructions: event.target.value,
                                    },
                                  })
                                }
                                disabled={isSystem}
                                className="min-h-24 resize-y text-sm leading-5"
                              />
                            </div>
                          </div>
                        )}

                        {selectedStep.quality_gate?.enabled && (
                          <div className="grid gap-3 rounded-md border p-3">
                            <div className="text-xs font-medium">
                              {t(($) => $.steps.quality_details)}
                            </div>
                            <div className="flex items-center justify-between gap-3 rounded-md bg-muted/20 px-3 py-2">
                              <div>
                                <div className="text-xs font-medium">
                                  {t(($) => $.steps.quality_blocking)}
                                </div>
                                <p className="text-xs text-muted-foreground">
                                  {t(($) => $.steps.quality_blocking_help)}
                                </p>
                              </div>
                              <Switch
                                size="sm"
                                checked={selectedStep.quality_gate.blocking === true}
                                onCheckedChange={(checked) =>
                                  updateStep(selectedStepIndex, {
                                    quality_gate: {
                                      ...selectedStep.quality_gate,
                                      enabled: true,
                                      blocking: checked,
                                    },
                                  })
                                }
                                disabled={isSystem}
                                aria-label={t(($) => $.steps.quality_blocking)}
                              />
                            </div>
                            <div className="space-y-1.5">
                              <Label className="text-xs">
                                {t(($) => $.steps.quality_prompt)}
                              </Label>
                              <Textarea
                                value={selectedStep.quality_gate.prompt ?? ""}
                                onChange={(event) =>
                                  updateStep(selectedStepIndex, {
                                    quality_gate: {
                                      ...selectedStep.quality_gate,
                                      enabled: true,
                                      prompt: event.target.value,
                                    },
                                  })
                                }
                                disabled={isSystem}
                                className="min-h-32 resize-y text-sm leading-5"
                              />
                            </div>
                          </div>
                        )}

                        {stepGate(selectedStep) === "none" && (
                          <div className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                            {t(($) => $.steps.no_checks)}
                          </div>
                        )}
                      </div>
                    </TabsContent>
                  </Tabs>
                </>
              ) : (
                <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
                  {t(($) => $.steps.empty)}
                </div>
              )}
            </div>
          </div>
        </TabsContent>

        <TabsContent
          value="preview"
          className="h-full min-h-0 overflow-hidden bg-muted/10 p-4"
        >
          <div
            className={cn(
              "grid h-full min-h-0 gap-4",
              graphExpanded
                ? "xl:grid-cols-1"
                : "xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]",
            )}
          >
            <WorkflowGraphPreview
              steps={schema.steps ?? []}
              expanded={graphExpanded}
              onExpandedChange={setGraphExpanded}
              className="h-full min-h-0"
            />

            {!graphExpanded && (
              <div className="flex h-full min-h-0 flex-col rounded-md border bg-background">
                <div className="flex h-10 shrink-0 items-center border-b px-3">
                  <h2 className="text-xs font-medium">
                    {t(($) => $.preview.instructions_title)}
                  </h2>
                </div>
                {previewError ? (
                  <div className="m-4 flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
                    <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <span>{previewError}</span>
                  </div>
                ) : (
                  <div className="min-h-0 flex-1 overflow-auto px-4 py-3">
                    {preview ? (
                      <Markdown mode="full" className="text-sm leading-6">
                        {preview}
                      </Markdown>
                    ) : (
                      <div className="text-sm text-muted-foreground">
                        {t(($) => $.preview.empty)}
                      </div>
                    )}
                  </div>
                )}
                {previewWarnings.length > 0 && (
                  <div className="border-t px-4 py-2 text-xs text-warning">
                    {previewWarnings.join(" ")}
                  </div>
                )}
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>

      <ProjectWorkflowBindings
        projects={projects}
        workflows={assignmentWorkflows}
      />
      <Dialog open={promptEditorOpen} onOpenChange={setPromptEditorOpen}>
        <DialogContent className="flex max-h-[86vh] w-[min(96vw,72rem)] max-w-none flex-col overflow-hidden sm:max-w-none">
          <DialogHeader>
            <DialogTitle>
              {t(($) => $.steps.prompt_editor_title, {
                number: promptEditorStepIndex + 1,
                title: promptEditorStep?.title ?? "",
              })}
            </DialogTitle>
            <DialogDescription>
              {t(($) => $.steps.prompt_editor_description)}
            </DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 flex-1 gap-4 overflow-y-auto py-1">
            <div className="space-y-1.5">
              <Label className="text-xs">
                {t(($) => $.steps.body_template_label)}
              </Label>
              <Textarea
                value={promptDraft}
                onChange={(event) => setPromptDraft(event.target.value)}
                disabled={isSystem}
                spellCheck={false}
                className="min-h-[320px] resize-y font-mono text-xs leading-5"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">
                {t(($) => $.steps.checklist_label)}
              </Label>
              <Textarea
                value={checklistDraft}
                onChange={(event) => setChecklistDraft(event.target.value)}
                disabled={isSystem}
                placeholder={t(($) => $.steps.checklist_placeholder)}
                className="min-h-28 resize-y text-sm leading-5"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setPromptEditorOpen(false)}
            >
              {t(($) => $.steps.prompt_editor_cancel)}
            </Button>
            <Button
              type="button"
              onClick={applyPromptEditor}
              disabled={isSystem}
            >
              {t(($) => $.steps.prompt_editor_apply)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {!isSystem && (
        <ReviewChangesDialog
          open={reviewOpen}
          onOpenChange={setReviewOpen}
          workflow={workflow}
          draft={schema}
          published={publishedSchema(workflow)}
          onPublish={publish}
          onDiscard={discardDraft}
          isPublishing={publishWorkflow.isPending}
          isDiscarding={deleteWorkflowDraft.isPending || deleteWorkflow.isPending}
          validation={localDraftValidation}
        />
      )}
    </div>
  );
}

export function WorkflowsPage() {
  const { t } = useT("workflows");
  const wsId = useWorkspaceId();
  const {
    data: workflows = [],
    isLoading,
    error,
  } = useQuery(workflowListOptions(wsId));
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: assignmentWorkflows = [] } = useQuery(
    workflowListOptions(wsId, { applicability: "assignment" }),
  );
  const createWorkflow = useCreateWorkflow();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<FilterKey>("all");

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return workflows.filter((workflow) => {
      if (filter !== "all" && !workflowApplicability(workflow).includes(filter)) {
        return false;
      }
      if (!q) return true;
      return (
        workflowDisplayName(workflow).toLowerCase().includes(q) ||
        workflowDisplayDescription(workflow).toLowerCase().includes(q)
      );
    });
  }, [workflows, search, filter]);

  useEffect(() => {
    if (selectedId && workflows.some((workflow) => workflow.id === selectedId)) {
      return;
    }
    setSelectedId(filtered[0]?.id ?? workflows[0]?.id ?? null);
  }, [filtered, selectedId, workflows]);

  const selected =
    workflows.find((workflow) => workflow.id === selectedId) ?? null;

  const create = async () => {
    try {
      const workflow = await createWorkflow.mutateAsync({
        name: t(($) => $.new_workflow.name),
        description: t(($) => $.new_workflow.description),
        schema: {
          schema_version: 1,
          name: t(($) => $.new_workflow.name),
          description: t(($) => $.new_workflow.description),
          applicability: ["assignment"],
          source: {
            format: "markdown",
            body_template: DEFAULT_WORKFLOW_TEMPLATE,
          },
          steps: [
            {
              id: "context",
              title: "Read current task",
              order: 1,
              description: "Load the issue details and current task state.",
              output: { description: "The agent knows the requested outcome." },
            },
            {
              id: "discussion",
              title: "Read latest discussion",
              order: 2,
              depends_on: ["context"],
              description: "Incorporate the newest comments before acting.",
              output: { description: "Recent owner input is reflected." },
            },
            {
              id: "execute",
              title: "Execute work",
              order: 3,
              depends_on: ["discussion"],
              description: "Complete the requested change.",
              output: { description: "The requested deliverable is ready." },
            },
            {
              id: "verify",
              title: "Verify and report",
              order: 4,
              depends_on: ["execute"],
              description: "Run checks and report the outcome.",
              output: { description: "Verification evidence is posted." },
              quality_gate: { enabled: true },
            },
          ],
        },
      });
      setSelectedId(workflow.id);
      toast.success(t(($) => $.new_workflow.created));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.new_workflow.failed),
      );
    }
  };

  if (isLoading) return <WorkflowsSkeleton />;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeaderBar totalCount={workflows.length} onCreate={create} />
      <div className="flex min-h-0 flex-1 p-6">
        <div className="flex min-h-0 flex-1 overflow-hidden rounded-lg border bg-background">
          {error ? (
            <div className="flex flex-1 items-center justify-center text-sm text-destructive">
              {t(($) => $.page.load_failed)}
            </div>
          ) : (
            <>
              <WorkflowList
                workflows={filtered}
                selectedId={selectedId}
                onSelect={setSelectedId}
                search={search}
                setSearch={setSearch}
                filter={filter}
                setFilter={setFilter}
              />
              <WorkflowEditor
                workflow={selected}
                assignmentWorkflows={assignmentWorkflows}
                projects={projects}
                onSelect={setSelectedId}
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function WorkflowsSkeleton() {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader className="justify-between px-5">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-8 w-28" />
      </PageHeader>
      <div className="flex min-h-0 flex-1 p-6">
        <div className="flex min-h-0 flex-1 overflow-hidden rounded-lg border bg-background">
          <div className="w-80 border-r p-3">
            <Skeleton className="h-8 w-full" />
            <div className="mt-4 space-y-2">
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton key={index} className="h-14 w-full" />
              ))}
            </div>
          </div>
          <div className="flex-1 p-4">
            <Skeleton className="h-8 w-1/2" />
            <Skeleton className="mt-4 h-96 w-full" />
          </div>
        </div>
      </div>
    </div>
  );
}
