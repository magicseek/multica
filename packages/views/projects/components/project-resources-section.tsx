"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BookOpen, ChevronRight, FolderGit, GitBranch, Plus, Ticket, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  projectResourcesOptions,
  useCreateProjectResource,
  useDeleteProjectResource,
} from "@multica/core/projects";
import { connectorProvidersOptions } from "@multica/core/connectors/queries";
import {
  projectRepositoriesOptions,
  repositoryListOptions,
  useCreateRepository,
  useSetProjectRepositories,
} from "@multica/core/repositories";
import { useWorkspaceId } from "@multica/core/hooks";
import type {
  GithubRepoResourceRef,
  ProjectRepository,
  ProjectResource,
  ProjectResourceType,
  Repository,
  RingCentralGitLabRepoResourceRef,
  RingCentralJiraIssueResourceRef,
  RingCentralJiraProjectResourceRef,
  RingCentralWikiPageResourceRef,
  RingCentralWikiSpaceResourceRef,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@multica/ui/components/ui/tooltip";
import { useT } from "../../i18n";

// Project Resources sidebar section.
//
// Today only renders github_repo, but the rendering layer is type-dispatched
// so adding a new type means: (1) extend the API validator, (2) add a render
// case here. No changes to the schema or query layer.
export function ProjectResourcesSection({ projectId }: { projectId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const [open, setOpen] = useState(true);
  const [addOpen, setAddOpen] = useState(false);

  const { data: resources = [] } = useQuery(
    projectResourcesOptions(wsId, projectId),
  );
  const { data: connectorProviders } = useQuery(connectorProvidersOptions(wsId));
  const ringCentralResourceTypes = new Set(
    (connectorProviders?.providers ?? [])
      .filter((provider) => provider.profile === "ringcentral")
      .flatMap((provider) => provider.resource_types),
  );
  const hasRingCentralResources = ringCentralResourceTypes.size > 0;
  const deleteResource = useDeleteProjectResource(wsId, projectId);
  const createResource = useCreateProjectResource(wsId, projectId);
  const { data: projectRepositories = [] } = useQuery(
    projectRepositoriesOptions(wsId, projectId),
  );
  const { data: workspaceRepositories = [] } = useQuery(repositoryListOptions(wsId));
  const createRepository = useCreateRepository(wsId);
  const setProjectRepositories = useSetProjectRepositories(wsId, projectId);

  const attachedRepositoryIds = new Set(
    projectRepositories.map((repo) => repo.repository_id),
  );
  const selectableRepositories = workspaceRepositories.filter(
    (repo) => repo.status !== "archived" && repo.compatibility !== true,
  );
  const hasResources = projectRepositories.length > 0 || resources.length > 0;

  const setRepositories = async (repositories: ProjectRepository[]) => {
    await setProjectRepositories.mutateAsync({
      repositories: repositories.map((repo, index) => ({
        repository_id: repo.repository_id,
        role: index === 0 ? "primary" : repo.role,
        position: index,
      })),
    });
  };

  const handleAttachRepository = async (repository: Repository) => {
    try {
      if (attachedRepositoryIds.has(repository.id)) return;
      await setRepositories([
        ...projectRepositories,
        {
          project_id: projectId,
          repository_id: repository.id,
          role: projectRepositories.length === 0 ? "primary" : "secondary",
          position: projectRepositories.length,
          created_at: "",
          repository,
        },
      ]);
      toast.success(t(($) => $.resources.toast_attached));
    } catch (err) {
      const msg = err instanceof Error ? err.message : t(($) => $.resources.toast_attach_failed);
      toast.error(msg);
    }
  };

  const handleCreateAndAttachRepository = async (url: string) => {
    try {
      const existing = selectableRepositories.find((repo) => repo.remote_url === url);
      if (existing) {
        await handleAttachRepository(existing);
        return;
      }
      const repository = await createRepository.mutateAsync({
        source_state: "remote_git",
        remote_url: url,
      });
      await handleAttachRepository(repository);
    } catch (err) {
      const msg = err instanceof Error ? err.message : t(($) => $.resources.toast_attach_failed);
      toast.error(msg);
    }
  };

  const handleRemoveRepository = async (repositoryID: string) => {
    try {
      await setRepositories(
        projectRepositories.filter((repo) => repo.repository_id !== repositoryID),
      );
      toast.success(t(($) => $.resources.toast_removed));
    } catch {
      toast.error(t(($) => $.resources.toast_remove_failed));
    }
  };

  const handleRemove = async (resource: ProjectResource) => {
    try {
      await deleteResource.mutateAsync(resource.id);
      toast.success(t(($) => $.resources.toast_removed));
    } catch {
      toast.error(t(($) => $.resources.toast_remove_failed));
    }
  };

  return (
    <div>
      <button
        className={`flex w-full items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors mb-2 hover:bg-accent/70 ${open ? "" : "text-muted-foreground hover:text-foreground"}`}
        onClick={() => setOpen(!open)}
      >
        {t(($) => $.resources.section_header)}
        <ChevronRight
          className={`!size-3 shrink-0 stroke-[2.5] text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`}
        />
      </button>
      {open && (
        <div className="pl-2 space-y-1.5">
          {!hasResources && (
            <p className="text-xs text-muted-foreground">
              {t(($) => $.resources.empty)}
            </p>
          )}
          {projectRepositories.map((projectRepository) => (
            <ProjectRepositoryRow
              key={projectRepository.repository_id}
              projectRepository={projectRepository}
              onRemove={() => handleRemoveRepository(projectRepository.repository_id)}
            />
          ))}
          {resources.map((resource) => (
            <ResourceRow
              key={resource.id}
              resource={resource}
              onRemove={() => handleRemove(resource)}
            />
          ))}
          <Popover open={addOpen} onOpenChange={setAddOpen}>
            <PopoverTrigger
              render={
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
                >
                  <Plus className="size-3" />
                  {t(($) => $.resources.add_button)}
                </Button>
              }
            />
            <PopoverContent align="start" className="w-72 p-2 space-y-2">
              <div className="text-xs font-medium text-muted-foreground">
                {t(($) => $.resources.popover_title)}
              </div>
              {selectableRepositories.length > 0 && (
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  {selectableRepositories.map((repo) => {
                    const isAttached = attachedRepositoryIds.has(repo.id);
                    const isDisabled = isAttached || setProjectRepositories.isPending;
                    return (
                      // Use aria-disabled instead of the native `disabled` attribute so
                      // hover events still reach the tooltip trigger on attached rows
                      // (browsers suppress pointer events on disabled form controls).
                      <button
                        key={repo.id}
                        type="button"
                        aria-disabled={isDisabled}
                        onClick={async () => {
                          if (isDisabled) return;
                          await handleAttachRepository(repo);
                          setAddOpen(false);
                        }}
                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs text-left hover:bg-accent transition-colors aria-disabled:opacity-50 aria-disabled:cursor-not-allowed aria-disabled:hover:bg-transparent"
                      >
                        <FolderGit className="size-3.5" />
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <span className="truncate flex-1">
                                {repositoryLabel(repo)}
                              </span>
                            }
                          />
                          <TooltipContent side="top">
                            {repositoryTooltip(repo)}
                          </TooltipContent>
                        </Tooltip>
                        {isAttached && (
                          <span className="text-[10px] text-muted-foreground">
                            {t(($) => $.resources.attached_badge)}
                          </span>
                        )}
                      </button>
                    );
                  })}
                </div>
              )}
              <CustomRepoForm
                onSubmit={async (url) => {
                  await handleCreateAndAttachRepository(url);
                  setAddOpen(false);
                }}
              />
              {hasRingCentralResources && (
                <RingCentralResourceForm
                  enabledTypes={ringCentralResourceTypes}
                  disabled={createResource.isPending}
                  onSubmit={async (resource) => {
                    await createResource.mutateAsync(resource);
                    toast.success(t(($) => $.resources.toast_attached));
                    setAddOpen(false);
                  }}
                />
              )}
            </PopoverContent>
          </Popover>
        </div>
      )}
    </div>
  );
}

function repositoryLabel(repo: Repository): string {
  return repo.name || repo.remote_url || repo.remote_key || repo.id;
}

function repositoryTooltip(repo: Repository): string {
  return repo.remote_url ?? repo.remote_key ?? repo.source_state;
}

function ProjectRepositoryRow({
  projectRepository,
  onRemove,
}: {
  projectRepository: ProjectRepository;
  onRemove: () => void;
}) {
  const { t } = useT("projects");
  const repo = projectRepository.repository;
  return (
    <div className="flex items-center gap-2 text-xs group">
      <FolderGit className="size-3.5 text-muted-foreground shrink-0" />
      <Tooltip>
        <TooltipTrigger
          render={
            <span className="truncate flex-1">
              {repositoryLabel(repo)}
            </span>
          }
        />
        <TooltipContent side="top">
          {repositoryTooltip(repo)}
        </TooltipContent>
      </Tooltip>
      {projectRepository.role === "primary" && (
        <span className="text-[10px] text-muted-foreground">
          {t(($) => $.resources.primary_badge)}
        </span>
      )}
      <button
        type="button"
        onClick={onRemove}
        className="opacity-0 group-hover:opacity-100 transition-opacity rounded-sm p-0.5 hover:bg-accent"
        title={t(($) => $.resources.remove_tooltip)}
      >
        <Trash2 className="size-3 text-muted-foreground" />
      </button>
    </div>
  );
}

function ResourceRow({
  resource,
  onRemove,
}: {
  resource: ProjectResource;
  onRemove: () => void;
}) {
  const { t } = useT("projects");
  if (resource.resource_type === "github_repo") {
    const ref = resource.resource_ref as GithubRepoResourceRef;
    return (
      <div className="flex items-center gap-2 text-xs group">
        <FolderGit className="size-3.5 text-muted-foreground shrink-0" />
        <Tooltip>
          <TooltipTrigger
            render={
              <a
                href={ref.url}
                target="_blank"
                rel="noopener noreferrer"
                className="truncate flex-1 hover:underline"
              >
                {resource.label || ref.url}
              </a>
            }
          />
          <TooltipContent side="top">{ref.url}</TooltipContent>
        </Tooltip>
        <button
          type="button"
          onClick={onRemove}
          className="opacity-0 group-hover:opacity-100 transition-opacity rounded-sm p-0.5 hover:bg-accent"
          title={t(($) => $.resources.remove_tooltip)}
        >
          <Trash2 className="size-3 text-muted-foreground" />
        </button>
      </div>
    );
  }
  if (resource.resource_type.startsWith("ringcentral_")) {
    return <RingCentralResourceRow resource={resource} onRemove={onRemove} />;
  }
  return (
    <div className="flex items-center gap-2 text-xs text-muted-foreground">
      <span className="truncate flex-1">
        {resource.label || resource.resource_type}
      </span>
      <button
        type="button"
        onClick={onRemove}
        className="rounded-sm p-0.5 hover:bg-accent"
        title={t(($) => $.resources.remove_tooltip)}
      >
        <Trash2 className="size-3" />
      </button>
    </div>
  );
}

function RingCentralResourceRow({
  resource,
  onRemove,
}: {
  resource: ProjectResource;
  onRemove: () => void;
}) {
  const { t } = useT("projects");
  const { icon, label, detail } = ringCentralResourceDisplay(resource);
  return (
    <div className="flex items-center gap-2 text-xs group">
      {icon}
      <Tooltip>
        <TooltipTrigger
          render={<span className="truncate flex-1">{resource.label || label}</span>}
        />
        <TooltipContent side="top">{detail}</TooltipContent>
      </Tooltip>
      <button
        type="button"
        onClick={onRemove}
        className="opacity-0 group-hover:opacity-100 transition-opacity rounded-sm p-0.5 hover:bg-accent"
        title={t(($) => $.resources.remove_tooltip)}
      >
        <Trash2 className="size-3 text-muted-foreground" />
      </button>
    </div>
  );
}

function ringCentralResourceDisplay(resource: ProjectResource) {
  const iconClass = "size-3.5 text-muted-foreground shrink-0";
  switch (resource.resource_type) {
    case "ringcentral_gitlab_repo": {
      const ref = resource.resource_ref as RingCentralGitLabRepoResourceRef;
      return {
        icon: <GitBranch className={iconClass} />,
        label: ref.project_id,
        detail: ref.web_url || ref.project_id,
      };
    }
    case "ringcentral_jira_project": {
      const ref = resource.resource_ref as RingCentralJiraProjectResourceRef;
      return {
        icon: <Ticket className={iconClass} />,
        label: ref.name || ref.project_key,
        detail: ref.project_key,
      };
    }
    case "ringcentral_jira_issue": {
      const ref = resource.resource_ref as RingCentralJiraIssueResourceRef;
      return {
        icon: <Ticket className={iconClass} />,
        label: ref.summary || ref.issue_key,
        detail: ref.issue_key,
      };
    }
    case "ringcentral_wiki_space": {
      const ref = resource.resource_ref as RingCentralWikiSpaceResourceRef;
      return {
        icon: <BookOpen className={iconClass} />,
        label: ref.name || ref.space_key,
        detail: ref.space_key,
      };
    }
    case "ringcentral_wiki_page": {
      const ref = resource.resource_ref as RingCentralWikiPageResourceRef;
      return {
        icon: <BookOpen className={iconClass} />,
        label: ref.title || ref.page_id,
        detail: ref.space_key ? `${ref.space_key}: ${ref.page_id}` : ref.page_id,
      };
    }
    default:
      return {
        icon: <BookOpen className={iconClass} />,
        label: resource.resource_type,
        detail: resource.resource_type,
      };
  }
}

function CustomRepoForm({
  onSubmit,
}: {
  onSubmit: (url: string) => Promise<void> | void;
}) {
  const { t } = useT("projects");
  const [url, setUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const handle = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = url.trim();
    if (!trimmed) return;
    setSubmitting(true);
    try {
      await onSubmit(trimmed);
      setUrl("");
    } finally {
      setSubmitting(false);
    }
  };
  return (
    <form onSubmit={handle} className="flex items-center gap-1.5 pt-1 border-t">
      <input
        type="text"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        placeholder={t(($) => $.resources.url_placeholder)}
        className="flex-1 bg-transparent text-xs px-2 py-1 outline-none placeholder:text-muted-foreground"
      />
      <Button
        type="submit"
        size="sm"
        variant="ghost"
        className="h-6 px-2 text-xs"
        disabled={!url.trim() || submitting}
      >
        {t(($) => $.resources.url_submit)}
      </Button>
    </form>
  );
}

function RingCentralResourceForm({
  enabledTypes,
  disabled,
  onSubmit,
}: {
  enabledTypes: Set<string>;
  disabled: boolean;
  onSubmit: (resource: {
    resource_type: ProjectResourceType;
    resource_ref: Record<string, string>;
    label?: string;
  }) => Promise<void> | void;
}) {
  const { t } = useT("projects");
  const options = ringCentralResourceOptions.filter((option) => enabledTypes.has(option.type));
  const [type, setType] = useState<ProjectResourceType>(
    (options[0]?.type ?? "ringcentral_gitlab_repo") as ProjectResourceType,
  );
  const [value, setValue] = useState("");
  const [label, setLabel] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const selected = options.find((option) => option.type === type) ?? options[0];
  if (!selected) return null;

  const handle = async (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = value.trim();
    if (!trimmed) return;
    setSubmitting(true);
    try {
      await onSubmit({
        resource_type: selected.type,
        resource_ref: { [selected.refKey]: trimmed },
        label: label.trim() || undefined,
      });
      setValue("");
      setLabel("");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handle} className="space-y-2 pt-2 border-t">
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-1.5">
        <select
          value={type}
          onChange={(event) => setType(event.target.value as ProjectResourceType)}
          className="min-w-0 rounded-md border bg-background px-2 py-1 text-xs outline-none"
          disabled={disabled || submitting}
        >
          {options.map((option) => (
            <option key={option.type} value={option.type}>
              {option.label}
            </option>
          ))}
        </select>
        <input
          type="text"
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          placeholder={t(($) => $.resources.ringcentral_label_placeholder)}
          className="min-w-0 rounded-md border bg-background px-2 py-1 text-xs outline-none placeholder:text-muted-foreground"
        />
      </div>
      <div className="flex items-center gap-1.5">
        <input
          type="text"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder={selected.placeholder}
          className="flex-1 bg-transparent text-xs px-2 py-1 outline-none placeholder:text-muted-foreground"
        />
        <Button
          type="submit"
          size="sm"
          variant="ghost"
          className="h-6 px-2 text-xs"
          disabled={disabled || submitting || !value.trim()}
        >
          {t(($) => $.resources.url_submit)}
        </Button>
      </div>
    </form>
  );
}

const ringCentralResourceOptions: Array<{
  type: ProjectResourceType;
  label: string;
  refKey: string;
  placeholder: string;
}> = [
  {
    type: "ringcentral_gitlab_repo",
    label: "GitLab",
    refKey: "project_id",
    placeholder: "group/project",
  },
  {
    type: "ringcentral_jira_project",
    label: "Jira project",
    refKey: "project_key",
    placeholder: "ABC",
  },
  {
    type: "ringcentral_jira_issue",
    label: "Jira issue",
    refKey: "issue_key",
    placeholder: "ABC-123",
  },
  {
    type: "ringcentral_wiki_space",
    label: "Wiki space",
    refKey: "space_key",
    placeholder: "ENG",
  },
  {
    type: "ringcentral_wiki_page",
    label: "Wiki page",
    refKey: "page_id",
    placeholder: "123456",
  },
];
