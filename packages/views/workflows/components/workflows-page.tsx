"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  Check,
  ChevronDown,
  Copy,
  Eye,
  FileText,
  FolderKanban,
  GitBranch,
  ListChecks,
  Loader2,
  Lock,
  Plus,
  Save,
  Search,
  Trash2,
  Workflow,
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
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
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
  const outputDescription = step.output?.description?.trim() ?? "";
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
    body_template: step.body_template?.trim() || undefined,
    description: step.description?.trim() || undefined,
    checklist: step.checklist?.map((item) => item.trim()).filter(Boolean),
    output: outputDescription ? { description: outputDescription } : undefined,
    review: reviewRequired ? { required: true } : undefined,
    quality_gate: qualityEnabled ? { enabled: true } : undefined,
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

function stepGate(step: WorkflowStep): StepGate {
  const review = step.review?.required === true;
  const quality = step.quality_gate?.enabled === true;
  if (review && quality) return "human_quality";
  if (review) return "human";
  if (quality) return "quality";
  return "none";
}

function gatePatch(value: StepGate): Pick<WorkflowStep, "review" | "quality_gate"> {
  return {
    review:
      value === "human" || value === "human_quality"
        ? { required: true }
        : undefined,
    quality_gate:
      value === "quality" || value === "human_quality"
        ? { enabled: true }
        : undefined,
  };
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
      <DialogContent className="flex max-h-[86vh] max-w-3xl grid-rows-none flex-col overflow-hidden">
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
            className="min-h-0 flex-1 overflow-hidden rounded-md border bg-background"
          >
            <div className="grid h-full min-h-[320px] md:grid-cols-2">
              <div className="min-w-0 border-b md:border-b-0 md:border-r">
                <div className="border-b px-3 py-2 text-xs font-medium">
                  {t(($) => $.review.published_schema)}
                </div>
                <pre className="h-[280px] overflow-auto p-3 text-xs leading-5 text-muted-foreground">
                  {published ? formatSchema(published) : t(($) => $.review.none)}
                </pre>
              </div>
              <div className="min-w-0">
                <div className="border-b px-3 py-2 text-xs font-medium">
                  {t(($) => $.review.draft_schema)}
                </div>
                <pre className="h-[280px] overflow-auto p-3 text-xs leading-5">
                  {formatSchema(draft)}
                </pre>
              </div>
            </div>
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
  const [reviewOpen, setReviewOpen] = useState(false);
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
    setReviewOpen(false);
    setLocalDraftValidation(draftRevision(workflow)?.validation ?? null);
  }, [workflow]);

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

  const removeStep = (index: number) => {
    patchSchema({
      steps: normalizeSteps(
        (schema.steps ?? [])
          .filter((_, i) => i !== index)
          .map((step, i) => ({ ...step, order: i + 1 })),
      ),
    });
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

        <TabsContent value="steps" className="h-full min-h-0 overflow-y-auto p-4">
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

          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-sm font-medium">{t(($) => $.steps.title)}</h2>
              <p className="text-xs text-muted-foreground">
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

          {(schema.steps ?? []).length === 0 ? (
            <div className="mt-4 rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t(($) => $.steps.empty)}
            </div>
          ) : (
            <div className="mt-4 overflow-hidden rounded-md border bg-background">
              {(schema.steps ?? []).map((step, index) => (
                <div
                  key={`${step.id}-${index}`}
                  className="grid gap-3 border-b p-3 last:border-b-0 md:grid-cols-[2.25rem_minmax(0,1fr)_2.25rem]"
                >
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-muted font-mono text-xs text-muted-foreground">
                    {index + 1}
                  </div>

                  <div className="min-w-0 space-y-3">
                    <div className="grid gap-3 md:grid-cols-[minmax(220px,0.8fr)_minmax(180px,0.45fr)_minmax(120px,0.3fr)]">
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-title-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.title_label)}
                        </Label>
                        <Input
                          id={`workflow-step-title-${index}`}
                          value={step.title ?? ""}
                          onChange={(e) =>
                            updateStep(index, { title: e.target.value })
                          }
                          disabled={isSystem}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-gate-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.gate_label)}
                        </Label>
                        <Select
                          value={stepGate(step)}
                          onValueChange={(value) =>
                            updateStep(index, gatePatch(value as StepGate))
                          }
                          disabled={isSystem}
                        >
                          <SelectTrigger
                            id={`workflow-step-gate-${index}`}
                            size="sm"
                          >
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
                      <Button
                        type="button"
                        variant={step.required === false ? "outline" : "secondary"}
                        size="sm"
                        className="self-end"
                        disabled={isSystem}
                        onClick={() =>
                          updateStep(index, { required: step.required === false })
                        }
                      >
                        {step.required === false ? (
                          <ChevronDown className="h-3 w-3" />
                        ) : (
                          <Check className="h-3 w-3" />
                        )}
                        {step.required === false
                          ? t(($) => $.steps.optional)
                          : t(($) => $.steps.required)}
                      </Button>
                    </div>

                    <div className="grid gap-3 md:grid-cols-2">
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-description-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.purpose_label)}
                        </Label>
                        <Textarea
                          id={`workflow-step-description-${index}`}
                          value={step.description ?? ""}
                          onChange={(e) =>
                            updateStep(index, { description: e.target.value })
                          }
                          disabled={isSystem}
                          className="min-h-20 resize-y text-xs leading-5"
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-output-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.output_label)}
                        </Label>
                        <Textarea
                          id={`workflow-step-output-${index}`}
                          value={step.output?.description ?? ""}
                          onChange={(e) =>
                            updateStep(index, {
                              output: { description: e.target.value },
                            })
                          }
                          disabled={isSystem}
                          className="min-h-20 resize-y text-xs leading-5"
                        />
                      </div>
                    </div>
                  </div>

                  <Button
                    type="button"
                    size="icon"
                    variant="ghost"
                    className="h-8 w-8 self-start"
                    onClick={() => removeStep(index)}
                    disabled={isSystem}
                    aria-label={t(($) => $.steps.remove)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}
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
