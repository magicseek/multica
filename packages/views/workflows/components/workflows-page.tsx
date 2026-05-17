"use client";

import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  Check,
  Copy,
  Eye,
  FileText,
  FolderKanban,
  GitBranch,
  ListChecks,
  Loader2,
  Lock,
  Plus,
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
  useDeleteWorkflow,
  useForkWorkflow,
  usePublishWorkflow,
  useUpdateWorkflow,
  workflowListOptions,
} from "@multica/core/workflows";
import type {
  Project,
  WorkflowApplicability,
  WorkflowDefinition,
  WorkflowSchema,
  WorkflowStep,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
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

function workflowApplicability(
  workflow: WorkflowDefinition | null | undefined,
): WorkflowApplicability[] {
  return workflow?.current_revision?.schema.applicability ?? ["assignment"];
}

function workflowBody(workflow: WorkflowDefinition | null | undefined): string {
  return workflow?.current_revision?.schema.source?.body_template ?? "";
}

function bodyToSchema(
  workflow: WorkflowDefinition | null | undefined,
  draft: {
    name: string;
    description: string;
    body: string;
    applicability: WorkflowApplicability[];
    steps: WorkflowStep[];
  },
): WorkflowSchema {
  const base = workflow?.current_revision?.schema ?? {};
  return {
    ...base,
    schema_version: base.schema_version ?? base.version ?? 1,
    version: undefined,
    name: draft.name.trim(),
    description: draft.description.trim(),
    applicability: draft.applicability,
    source: {
      ...(base.source ?? {}),
      format: "markdown",
      mode: undefined,
      body_template: draft.body,
    },
    steps: draft.steps,
  };
}

function workflowSteps(
  workflow: WorkflowDefinition | null | undefined,
): WorkflowStep[] {
  return workflow?.current_revision?.schema.steps ?? [];
}

function normalizeStep(step: WorkflowStep, index: number): WorkflowStep {
  const id = (step.id ?? "").trim() || `step-${index + 1}`;
  const title = (step.title ?? "").trim() || step.name?.trim() || id;
  return {
    ...step,
    id,
    title,
    name: step.name?.trim() || undefined,
    order: step.order || index + 1,
    depends_on: step.depends_on?.map((item) => item.trim()).filter(Boolean),
    body_template: step.body_template?.trim() || undefined,
    description: step.description?.trim() || undefined,
    checklist: step.checklist?.map((item) => item.trim()).filter(Boolean),
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
                      {workflow.name}
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
                aria-label={t(($) => $.project_bindings.select_aria, {
                  project: project.title,
                })}
              >
                <SelectTrigger size="sm" className="w-full justify-between">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent align="end">
                  <SelectItem value="__default">
                    {t(($) => $.project_bindings.workspace_default)}
                  </SelectItem>
                  {workflows.map((workflow) => (
                    <SelectItem key={workflow.id} value={workflow.id}>
                      {workflow.name}
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
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [body, setBody] = useState("");
  const [applicability, setApplicability] = useState<WorkflowApplicability[]>([
    "assignment",
  ]);
  const [steps, setSteps] = useState<WorkflowStep[]>([]);
  const [preview, setPreview] = useState("");
  const [previewWarnings, setPreviewWarnings] = useState<string[]>([]);
  const [previewError, setPreviewError] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [graphExpanded, setGraphExpanded] = useState(false);

  const updateWorkflow = useUpdateWorkflow();
  const publishWorkflow = usePublishWorkflow();
  const forkWorkflow = useForkWorkflow();
  const deleteWorkflow = useDeleteWorkflow();

  const isSystem = workflow?.origin === "system_seeded";
  const isSaving = updateWorkflow.isPending || publishWorkflow.isPending;

  useEffect(() => {
    if (!workflow) return;
    setName(workflow.name);
    setDescription(workflow.description);
    setBody(workflowBody(workflow));
    setApplicability(workflowApplicability(workflow));
    setSteps(workflowSteps(workflow));
    setPreview("");
    setPreviewWarnings([]);
    setPreviewError("");
    setGraphExpanded(false);
  }, [workflow]);

  const schema = useMemo(
    () =>
      bodyToSchema(workflow, {
        name,
        description,
        body,
        applicability,
        steps: steps.map(normalizeStep),
      }),
    [workflow, name, description, body, applicability, steps],
  );

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

  if (!workflow) return <EmptyEditor />;

  const toggleApplicability = (value: WorkflowApplicability) => {
    setApplicability((prev) => {
      if (prev.includes(value)) {
        const next = prev.filter((item) => item !== value);
        return next.length > 0 ? next : prev;
      }
      return [...prev, value];
    });
  };

  const addStep = () => {
    setSteps((prev) => [
      ...prev,
      {
        id: `step-${prev.length + 1}`,
        title: t(($) => $.steps.new_step_title, { number: prev.length + 1 }),
        order: prev.length + 1,
      },
    ]);
  };

  const updateStep = (index: number, patch: Partial<WorkflowStep>) => {
    setSteps((prev) =>
      prev.map((step, i) => (i === index ? { ...step, ...patch } : step)),
    );
  };

  const removeStep = (index: number) => {
    setSteps((prev) =>
      prev
        .filter((_, i) => i !== index)
        .map((step, i) => ({ ...step, order: i + 1 })),
    );
  };

  const fork = async () => {
    try {
      const forked = await forkWorkflow.mutateAsync({
        id: workflow.id,
        name: `${workflow.name} copy`,
        description: workflow.description,
      });
      toast.success(t(($) => $.editor.forked));
      onSelect(forked.id);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.fork_failed),
      );
    }
  };

  const publish = async () => {
    if (!workflow || isSystem || !name.trim()) return;
    try {
      await updateWorkflow.mutateAsync({
        id: workflow.id,
        name: name.trim(),
        description: description.trim(),
      });
      const updated = await publishWorkflow.mutateAsync({
        id: workflow.id,
        schema,
      });
      toast.success(t(($) => $.editor.published));
      onSelect(updated.id);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.editor.publish_failed),
      );
    }
  };

  const archive = async () => {
    if (!workflow || isSystem) return;
    try {
      await deleteWorkflow.mutateAsync(workflow.id);
      toast.success(t(($) => $.editor.archived));
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
          <span className="truncate text-sm font-medium">{workflow.name}</span>
          {isSystem && (
            <Badge variant="outline" className="h-5 rounded-md">
              <Lock className="h-3 w-3" />
              {t(($) => $.origin.system)}
            </Badge>
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
                {t(($) => $.editor.archive)}
              </Button>
              <Button
                type="button"
                size="sm"
                onClick={publish}
                disabled={!name.trim() || isSaving}
              >
                {isSaving ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <GitBranch className="h-3 w-3" />
                )}
                {t(($) => $.editor.publish)}
              </Button>
            </>
          )}
        </div>
      </div>

      <Tabs defaultValue="source" className="min-h-0 flex-1 gap-0">
        <div className="flex h-10 shrink-0 items-center border-b px-4">
          <TabsList variant="line" className="h-8">
            <TabsTrigger value="source">
              <FileText className="h-3.5 w-3.5" />
              {t(($) => $.tabs.source)}
            </TabsTrigger>
            <TabsTrigger value="steps">
              <ListChecks className="h-3.5 w-3.5" />
              {t(($) => $.tabs.steps)}
            </TabsTrigger>
            <TabsTrigger value="preview">
              <Eye className="h-3.5 w-3.5" />
              {t(($) => $.tabs.preview)}
            </TabsTrigger>
          </TabsList>
          {previewLoading && (
            <Loader2 className="ml-auto h-3.5 w-3.5 animate-spin text-muted-foreground" />
          )}
        </div>

        <TabsContent value="source" className="min-h-0 overflow-y-auto p-4">
          <div className="grid gap-4">
            <div className="grid gap-2 md:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="workflow-name" className="text-xs">
                  {t(($) => $.editor.name_label)}
                </Label>
                <Input
                  id="workflow-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={isSystem}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="workflow-description" className="text-xs">
                  {t(($) => $.editor.description_label)}
                </Label>
                <Input
                  id="workflow-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  disabled={isSystem}
                />
              </div>
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
                        "h-7 px-2 text-xs",
                        applicability.includes(item) &&
                          "bg-accent text-accent-foreground hover:bg-accent/80",
                      )}
                      onClick={() => toggleApplicability(item)}
                    >
                      {applicability.includes(item) && (
                        <Check className="h-3 w-3" />
                      )}
                      {applicabilityLabel(t, item)}
                    </Button>
                  ),
                )}
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="workflow-body" className="text-xs">
                {t(($) => $.editor.body_label)}
              </Label>
              <Textarea
                id="workflow-body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                disabled={isSystem}
                spellCheck={false}
                className="min-h-[460px] resize-y font-mono text-xs leading-5"
              />
              <p className="text-xs text-muted-foreground">
                {t(($) => $.editor.variables_hint)}
              </p>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="steps" className="min-h-0 overflow-y-auto p-4">
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

          {steps.length === 0 ? (
            <div className="mt-4 rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t(($) => $.steps.empty)}
            </div>
          ) : (
            <div className="mt-4 overflow-hidden rounded-md border bg-background">
              {steps.map((step, index) => (
                <div
                  key={`${step.id}-${index}`}
                  className="grid gap-3 border-b p-3 last:border-b-0 md:grid-cols-[2.25rem_minmax(0,1fr)_2.25rem]"
                >
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-muted font-mono text-xs text-muted-foreground">
                    {index + 1}
                  </div>

                  <div className="min-w-0 space-y-3">
                    <div className="grid gap-3 md:grid-cols-[minmax(140px,0.45fr)_minmax(220px,1fr)_minmax(160px,0.55fr)]">
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-id-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.id_label)}
                        </Label>
                        <Input
                          id={`workflow-step-id-${index}`}
                          value={step.id ?? ""}
                          onChange={(e) =>
                            updateStep(index, { id: e.target.value })
                          }
                          disabled={isSystem}
                        />
                      </div>
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
                          htmlFor={`workflow-step-depends-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.depends_on_label)}
                        </Label>
                        <Input
                          id={`workflow-step-depends-${index}`}
                          value={(step.depends_on ?? []).join(", ")}
                          onChange={(e) =>
                            updateStep(index, {
                              depends_on: e.target.value
                                .split(",")
                                .map((item) => item.trim())
                                .filter(Boolean),
                            })
                          }
                          placeholder={t(($) => $.steps.depends_on_placeholder)}
                          disabled={isSystem}
                        />
                      </div>
                    </div>

                    <div className="grid gap-3 md:grid-cols-3">
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-description-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.description_label)}
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
                          htmlFor={`workflow-step-body-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.body_template_label)}
                        </Label>
                        <Textarea
                          id={`workflow-step-body-${index}`}
                          value={step.body_template ?? ""}
                          onChange={(e) =>
                            updateStep(index, { body_template: e.target.value })
                          }
                          disabled={isSystem}
                          spellCheck={false}
                          className="min-h-20 resize-y font-mono text-xs leading-5"
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label
                          htmlFor={`workflow-step-checklist-${index}`}
                          className="text-xs"
                        >
                          {t(($) => $.steps.checklist_label)}
                        </Label>
                        <Textarea
                          id={`workflow-step-checklist-${index}`}
                          value={(step.checklist ?? []).join("\n")}
                          onChange={(e) =>
                            updateStep(index, {
                              checklist: e.target.value
                                .split("\n")
                                .map((item) => item.trim())
                                .filter(Boolean),
                            })
                          }
                          placeholder={t(($) => $.steps.checklist_placeholder)}
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
          className="min-h-0 overflow-hidden bg-muted/10 p-4"
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
                    {t(($) => $.preview.markdown_title)}
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
        workflow.name.toLowerCase().includes(q) ||
        workflow.description.toLowerCase().includes(q)
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
