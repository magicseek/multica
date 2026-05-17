"use client";

import { useMemo, useState, useRef } from "react";
import {
  ChevronRight,
  FolderGit2,
  FolderOpen,
  Maximize2,
  Minimize2,
  Workflow,
  X as XIcon,
  UserMinus,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useCreateProject } from "@multica/core/projects/mutations";
import { useProjectDraftStore } from "@multica/core/projects";
import { repositoryListOptions, useCreateRepository } from "@multica/core/repositories";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import {
  PROJECT_STATUS_CONFIG,
  PROJECT_STATUS_ORDER,
  PROJECT_PRIORITY_ORDER,
} from "@multica/core/projects/config";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentWorkspace, useWorkspacePaths } from "@multica/core/paths";
import { memberListOptions, agentListOptions } from "@multica/core/workspace/queries";
import { useActorName } from "@multica/core/workspace/hooks";
import { runtimeListOptions } from "@multica/core/runtimes";
import { workflowListOptions } from "@multica/core/workflows";
import type {
  ProjectStatus,
  ProjectPriority,
  Repository,
  RuntimeDevice,
} from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";
import { Dialog, DialogContent, DialogTitle } from "@multica/ui/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Popover, PopoverTrigger, PopoverContent } from "@multica/ui/components/ui/popover";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { Button } from "@multica/ui/components/ui/button";
import { EmojiPicker } from "@multica/ui/components/common/emoji-picker";
import { ContentEditor, type ContentEditorRef, TitleEditor } from "../editor";
import { PriorityIcon } from "../issues/components/priority-icon";
import { ActorAvatar } from "../common/actor-avatar";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import {
  useProjectStatusLabels,
  useProjectPriorityLabels,
} from "../projects/components/labels";
import { localDirectoryPickerHealthPort, pickLocalDirectory } from "../repositories/local-directory-picker";

type LocalRepositoryDraft = {
  id: string;
  name: string;
  path: string;
  runtimeId: string;
};

function PillButton({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs",
        "hover:bg-accent/60 transition-colors cursor-pointer",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

function RepositoryText({
  label,
  detail,
  className,
}: {
  label: string;
  detail?: string | null;
  className?: string;
}) {
  const tooltip = detail ? `${label} · ${detail}` : label;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            title={tooltip}
            className={cn("truncate flex-1 text-left", className)}
          >
            {label}
          </span>
        }
      />
      <TooltipContent side="top" align="start" className="max-w-sm break-all">
        {tooltip}
      </TooltipContent>
    </Tooltip>
  );
}

function repositoryLabel(repo: Repository): string {
  return repo.name || repo.remote_url || repo.remote_key || repo.id;
}

function repositoryDetail(repo: Repository): string | null {
  return repo.remote_url ?? repo.remote_key ?? repo.source_state;
}

function runtimeLabel(runtime: RuntimeDevice): string {
  return runtime.device_info ? `${runtime.name} · ${runtime.device_info}` : runtime.name;
}

export function CreateProjectModal({ onClose }: { onClose: () => void }) {
  const { t } = useT("modals");
  const router = useNavigation();
  const user = useAuthStore((s) => s.user);
  const workspace = useCurrentWorkspace();
  const workspaceName = workspace?.name;
  const wsPaths = useWorkspacePaths();
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: repositories = [] } = useQuery(repositoryListOptions(wsId));
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const { data: workflows = [] } = useQuery(
    workflowListOptions(wsId, { applicability: "assignment" }),
  );
  const { getActorName } = useActorName();
  const projectStatusLabels = useProjectStatusLabels();
  const projectPriorityLabels = useProjectPriorityLabels();

  const draft = useProjectDraftStore((s) => s.draft);
  const setDraft = useProjectDraftStore((s) => s.setDraft);
  const clearDraft = useProjectDraftStore((s) => s.clearDraft);

  const [title, setTitle] = useState(draft.title);
  const descEditorRef = useRef<ContentEditorRef>(null);
  const [status, setStatus] = useState<ProjectStatus>(draft.status);
  const [priority, setPriority] = useState<ProjectPriority>(draft.priority);
  const [leadType, setLeadType] = useState<"member" | "agent" | undefined>(draft.leadType);
  const [leadId, setLeadId] = useState<string | undefined>(draft.leadId);
  const [icon, setIcon] = useState<string | undefined>(draft.icon);
  const [iconPickerOpen, setIconPickerOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const [selectedRepositoryIds, setSelectedRepositoryIds] = useState<string[]>([]);
  const [repoPopoverOpen, setRepoPopoverOpen] = useState(false);
  const [customRepoUrl, setCustomRepoUrl] = useState("");
  const [customRepoUrls, setCustomRepoUrls] = useState<string[]>([]);
  const [localRepoDrafts, setLocalRepoDrafts] = useState<LocalRepositoryDraft[]>([]);
  const [selectedLocalRuntimeId, setSelectedLocalRuntimeId] = useState("");
  const [workflowDefinitionId, setWorkflowDefinitionId] = useState<string | null>(null);
  const selectableRepositories = repositories.filter(
    (repo) => repo.status !== "archived" && repo.compatibility !== true,
  );
  const localRuntimes = useMemo(
    () =>
      runtimes.filter(
        (runtime) =>
          runtime.runtime_mode === "local" &&
          runtime.status === "online" &&
          runtime.owner_id === user?.id &&
          Boolean(runtime.daemon_id),
      ),
    [runtimes, user?.id],
  );
  const currentWorkflow = workflows.find((workflow) => workflow.id === workflowDefinitionId);
  const workflowLabel = currentWorkflow?.name ?? t(($) => $.create_project.workflow_default);

  // Sync field changes to draft store
  const updateTitle = (v: string) => { setTitle(v); setDraft({ title: v }); };
  const updateStatus = (v: ProjectStatus) => { setStatus(v); setDraft({ status: v }); };
  const updatePriority = (v: ProjectPriority) => { setPriority(v); setDraft({ priority: v }); };
  const updateLead = (type?: "member" | "agent", id?: string) => {
    setLeadType(type); setLeadId(id);
    setDraft({ leadType: type, leadId: id });
  };
  const updateIcon = (v: string | undefined) => { setIcon(v); setDraft({ icon: v }); };

  const [leadOpen, setLeadOpen] = useState(false);
  const [leadFilter, setLeadFilter] = useState("");

  const leadQuery = leadFilter.toLowerCase();
  const filteredMembers = members.filter((m) => m.name.toLowerCase().includes(leadQuery) || matchesPinyin(m.name, leadQuery));
  const filteredAgents = agents.filter(
    (a) => !a.archived_at && (a.name.toLowerCase().includes(leadQuery) || matchesPinyin(a.name, leadQuery)),
  );

  const leadLabel =
    leadType && leadId ? getActorName(leadType, leadId) : t(($) => $.create_project.lead);

  const createProject = useCreateProject();
  const createRepository = useCreateRepository(wsId);

  const handleSubmit = async () => {
    if (!title.trim() || submitting) return;
    setSubmitting(true);
    try {
      const project = await createProject.mutateAsync({
        title: title.trim(),
        description: descEditorRef.current?.getMarkdown()?.trim() || undefined,
        icon,
        status,
        priority,
        lead_type: leadType,
        lead_id: leadId,
        ...(workflowDefinitionId ? { workflow_definition_id: workflowDefinitionId } : {}),
      });
      const repositoryIds = [...selectedRepositoryIds];
      for (const url of customRepoUrls) {
        const existing = selectableRepositories.find((repo) => repo.remote_url === url);
        if (existing) {
          if (!repositoryIds.includes(existing.id)) repositoryIds.push(existing.id);
          continue;
        }
        const repo = await createRepository.mutateAsync({
          source_state: "remote_git",
          remote_url: url,
        });
        repositoryIds.push(repo.id);
      }
      for (const localRepo of localRepoDrafts) {
        const selectedRuntime =
          localRuntimes.find((runtime) => runtime.id === localRepo.runtimeId) ?? null;
        if (!selectedRuntime?.daemon_id) {
          throw new Error(t(($) => $.create_project.repos_local_runtime_required));
        }
        const repo = await createRepository.mutateAsync({
          name: localRepo.name || undefined,
          source_state: "local_dir",
          binding: {
            daemon_id: selectedRuntime.daemon_id,
            runtime_id: selectedRuntime.id,
            machine_label: runtimeLabel(selectedRuntime),
            binding_kind: "local_dir",
            local_path: localRepo.path,
            state: "ready",
          },
        });
        repositoryIds.push(repo.id);
      }
      if (repositoryIds.length === 0 && leadType === "agent" && leadId) {
        const repo = await createRepository.mutateAsync({
          name: title.trim(),
          source_state: "agent_managed",
          lead_agent_id: leadId,
        });
        repositoryIds.push(repo.id);
      }
      if (repositoryIds.length > 0) {
        await api.setProjectRepositories(project.id, {
          repositories: repositoryIds.map((repository_id, index) => ({
            repository_id,
            role: index === 0 ? "primary" : "secondary",
            position: index,
          })),
        });
      }
      clearDraft();
      onClose();
      toast.success(t(($) => $.create_project.toast_created));
      router.push(wsPaths.projectDetail(project.id));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.create_project.toast_failed));
    } finally {
      setSubmitting(false);
    }
  };

  const toggleRepository = (repositoryId: string) => {
    setSelectedRepositoryIds((prev) =>
      prev.includes(repositoryId)
        ? prev.filter((id) => id !== repositoryId)
        : [...prev, repositoryId],
    );
  };

  const addCustomRepo = () => {
    const url = customRepoUrl.trim();
    if (!url) return;
    setCustomRepoUrls((prev) => (prev.includes(url) ? prev : [...prev, url]));
    setCustomRepoUrl("");
  };

  const addLocalRepo = async () => {
    const runtimeId = selectedLocalRuntimeId || localRuntimes[0]?.id || "";
    if (!runtimeId) {
      toast.error(t(($) => $.create_project.repos_local_runtime_required));
      return;
    }
    try {
      const selectedRuntime = localRuntimes.find((runtime) => runtime.id === runtimeId) ?? null;
      const result = await pickLocalDirectory({
        daemonId: selectedRuntime?.daemon_id,
        healthPort: localDirectoryPickerHealthPort(selectedRuntime?.metadata),
      });
      if (!result) return;
      if (!result.path) {
        toast.error(t(($) => $.create_project.repos_browser_path_unavailable, { name: result.name }));
        return;
      }
      setLocalRepoDrafts((prev) => [
        ...prev,
        {
          id: `local-${Date.now()}`,
          name: result.name,
          path: result.path,
          runtimeId,
        },
      ]);
    } catch {
      toast.error(t(($) => $.create_project.repos_pick_directory_failed));
    }
  };

  const selectedCount =
    selectedRepositoryIds.length + customRepoUrls.length + localRepoDrafts.length;

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className={cn(
          "p-0 gap-0 flex flex-col overflow-hidden",
          "!top-1/2 !left-1/2 !-translate-x-1/2",
          "!transition-all !duration-300 !ease-out",
          isExpanded
            ? "!max-w-4xl !w-full !h-5/6 !-translate-y-1/2"
            : "!max-w-2xl !w-full !h-96 !-translate-y-1/2",
        )}
      >
        <DialogTitle className="sr-only">{t(($) => $.create_project.title)}</DialogTitle>

        <div className="flex items-center justify-between px-5 pt-3 pb-2 shrink-0">
          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-muted-foreground">{workspaceName}</span>
            <ChevronRight className="size-3 text-muted-foreground/50" />
            <span className="font-medium">{t(($) => $.create_project.title_breadcrumb)}</span>
          </div>
          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger
                render={
                  <button
                    onClick={() => setIsExpanded(!isExpanded)}
                    className="rounded-sm p-1.5 opacity-70 hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
                  >
                    {isExpanded ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
                  </button>
                }
              />
              <TooltipContent side="bottom">
                {isExpanded
                  ? t(($) => $.common.collapse_tooltip)
                  : t(($) => $.common.expand_tooltip)}
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <button
                    onClick={onClose}
                    className="rounded-sm p-1.5 opacity-70 hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
                  >
                    <XIcon className="size-4" />
                  </button>
                }
              />
              <TooltipContent side="bottom">{t(($) => $.common.close)}</TooltipContent>
            </Tooltip>
          </div>
        </div>

        <div className="px-5 pb-2 shrink-0">
          <Popover open={iconPickerOpen} onOpenChange={setIconPickerOpen}>
            <PopoverTrigger
              render={
                <button
                  type="button"
                  className="text-2xl cursor-pointer rounded-lg p-1 -ml-1 hover:bg-accent/60 transition-colors"
                  title={t(($) => $.create_project.icon_tooltip)}
                >
                  {icon || "📁"}
                </button>
              }
            />
            <PopoverContent align="start" className="w-auto p-0">
              <EmojiPicker
                onSelect={(emoji) => {
                  updateIcon(emoji);
                  setIconPickerOpen(false);
                }}
              />
            </PopoverContent>
          </Popover>
          <TitleEditor
            autoFocus
            defaultValue={draft.title}
            placeholder={t(($) => $.create_project.title_placeholder)}
            className="text-lg font-semibold"
            onChange={(v) => updateTitle(v)}
            onSubmit={handleSubmit}
          />
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto px-5">
          <ContentEditor
            ref={descEditorRef}
            defaultValue={draft.description}
            placeholder={t(($) => $.create_project.description_placeholder)}
            onUpdate={(md) => setDraft({ description: md })}
            debounceMs={500}
          />
        </div>

        {/* Footer: properties (left, wrap) + Create button (right). Single row
            so the modal stays compact — Linear-style.
            Repos lives here alongside the property pills for now. Once we
            support more resource types (Linear / Notion / Figma / Slack), pull
            them out into a dedicated Resources strip above this footer — a
            single Repos pill on its own row looked too sparse. */}
        <div className="flex items-center justify-between gap-2 px-4 py-3 border-t shrink-0">
          <div className="flex items-center gap-1.5 flex-wrap min-w-0">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <PillButton>
                  <span className={cn("size-2 rounded-full", PROJECT_STATUS_CONFIG[status].dotColor)} />
                  <span>{projectStatusLabels[status]}</span>
                </PillButton>
              }
            />
            <DropdownMenuContent align="start" className="w-44">
              {PROJECT_STATUS_ORDER.map((s) => (
                <DropdownMenuItem key={s} onClick={() => updateStatus(s)}>
                  <span className={cn("size-2 rounded-full", PROJECT_STATUS_CONFIG[s].dotColor)} />
                  <span>{projectStatusLabels[s]}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <PillButton>
                  <PriorityIcon priority={priority} />
                  <span>{projectPriorityLabels[priority]}</span>
                </PillButton>
              }
            />
            <DropdownMenuContent align="start" className="w-44">
              {PROJECT_PRIORITY_ORDER.map((pr) => (
                <DropdownMenuItem key={pr} onClick={() => updatePriority(pr)}>
                  <PriorityIcon priority={pr} />
                  <span>{projectPriorityLabels[pr]}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>

          <Popover
            open={leadOpen}
            onOpenChange={(v) => {
              setLeadOpen(v);
              if (!v) setLeadFilter("");
            }}
          >
            <PopoverTrigger
              render={
                <PillButton>
                  {leadType && leadId ? (
                    <>
                      <ActorAvatar actorType={leadType} actorId={leadId} size={16} showStatusDot />
                      <span>{leadLabel}</span>
                    </>
                  ) : (
                    <span className="text-muted-foreground">{t(($) => $.create_project.lead)}</span>
                  )}
                </PillButton>
              }
            />
            <PopoverContent align="start" className="w-52 p-0">
              <div className="px-2 py-1.5 border-b">
                <input
                  type="text"
                  value={leadFilter}
                  onChange={(e) => setLeadFilter(e.target.value)}
                  placeholder={t(($) => $.create_project.lead_placeholder)}
                  className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
                />
              </div>
              <div className="p-1 max-h-60 overflow-y-auto">
                <button
                  type="button"
                  onClick={() => {
                    updateLead(undefined, undefined);
                    setLeadOpen(false);
                  }}
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                >
                  <UserMinus className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-muted-foreground">{t(($) => $.create_project.no_lead)}</span>
                </button>
                {filteredMembers.length > 0 && (
                  <>
                    <div className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground uppercase tracking-wider">
                      {t(($) => $.create_project.members_group)}
                    </div>
                    {filteredMembers.map((m) => (
                      <button
                        type="button"
                        key={m.user_id}
                        onClick={() => {
                          updateLead("member", m.user_id);
                          setLeadOpen(false);
                        }}
                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                      >
                        <ActorAvatar actorType="member" actorId={m.user_id} size={16} />
                        <span>{m.name}</span>
                      </button>
                    ))}
                  </>
                )}
                {filteredAgents.length > 0 && (
                  <>
                    <div className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground uppercase tracking-wider">
                      {t(($) => $.create_project.agents_group)}
                    </div>
                    {filteredAgents.map((a) => (
                      <button
                        type="button"
                        key={a.id}
                        onClick={() => {
                          updateLead("agent", a.id);
                          setLeadOpen(false);
                        }}
                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                      >
                        <ActorAvatar actorType="agent" actorId={a.id} size={16} showStatusDot />
                        <span>{a.name}</span>
                      </button>
                    ))}
                  </>
                )}
                {filteredMembers.length === 0 &&
                  filteredAgents.length === 0 &&
                  leadFilter && (
                    <div className="px-2 py-3 text-center text-sm text-muted-foreground">
                      {t(($) => $.create_project.no_results)}
                    </div>
                  )}
              </div>
            </PopoverContent>
          </Popover>

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <PillButton>
                  <Workflow className="size-3" />
                  <span className="truncate max-w-40">{workflowLabel}</span>
                </PillButton>
              }
            />
            <DropdownMenuContent align="start" className="w-56">
              <DropdownMenuItem onClick={() => setWorkflowDefinitionId(null)}>
                <Workflow className="h-3.5 w-3.5 text-muted-foreground" />
                <span>{t(($) => $.create_project.workflow_default)}</span>
              </DropdownMenuItem>
              {workflows.map((workflow) => (
                <DropdownMenuItem
                  key={workflow.id}
                  onClick={() => setWorkflowDefinitionId(workflow.id)}
                >
                  <Workflow className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="truncate">{workflow.name}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>

          <Popover open={repoPopoverOpen} onOpenChange={setRepoPopoverOpen}>
            <PopoverTrigger
              render={
                <PillButton>
                  <FolderGit2 className="size-3" />
                  <span>
                    {selectedCount === 0
                      ? t(($) => $.create_project.repos_pill)
                      : t(($) => $.create_project.repos_pill_count, { count: selectedCount })}
                  </span>
                </PillButton>
              }
            />
            <PopoverContent align="start" className="w-72 p-2 space-y-2">
              <div className="text-xs font-medium text-muted-foreground">
                {t(($) => $.create_project.repos_heading)}
              </div>
              {selectableRepositories.length > 0 ? (
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  {selectableRepositories.map((repo) => {
                    const checked = selectedRepositoryIds.includes(repo.id);
                    return (
                      <button
                        type="button"
                        key={repo.id}
                        onClick={() => toggleRepository(repo.id)}
                        className={cn(
                          "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent transition-colors",
                          checked && "bg-accent",
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          readOnly
                          className="size-3.5"
                        />
                        <FolderGit2 className="size-3.5" />
                        <RepositoryText
                          label={repositoryLabel(repo)}
                          detail={repositoryDetail(repo)}
                        />
                      </button>
                    );
                  })}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">
                  {t(($) => $.create_project.repos_empty)}
                </p>
              )}
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  addCustomRepo();
                }}
                className="flex items-center gap-1.5 pt-1 border-t"
              >
                <input
                  type="text"
                  value={customRepoUrl}
                  onChange={(e) => setCustomRepoUrl(e.target.value)}
                  placeholder={t(($) => $.create_project.repos_url_placeholder)}
                  className="flex-1 bg-transparent text-xs px-2 py-1 outline-none placeholder:text-muted-foreground"
                />
                <Button
                  type="submit"
                  size="sm"
                  variant="ghost"
                  className="h-6 px-2 text-xs"
                  disabled={!customRepoUrl.trim()}
                >
                  {t(($) => $.create_project.repos_add)}
                </Button>
              </form>
              <div className="space-y-1.5 pt-1 border-t">
                <div className="text-xs font-medium text-muted-foreground">
                  {t(($) => $.create_project.repos_local_heading)}
                </div>
                {localRuntimes.length === 0 ? (
                  <p className="text-xs text-muted-foreground">
                    {t(($) => $.create_project.repos_local_empty)}
                  </p>
                ) : (
                  <div className="flex items-center gap-1.5">
                    <select
                      aria-label={t(($) => $.create_project.repos_local_runtime_aria)}
                      value={selectedLocalRuntimeId || localRuntimes[0]?.id || ""}
                      onChange={(e) => setSelectedLocalRuntimeId(e.target.value)}
                      className="min-w-0 flex-1 rounded-md border bg-background px-2 py-1 text-xs text-foreground"
                    >
                      {localRuntimes.map((runtime) => (
                        <option key={runtime.id} value={runtime.id}>
                          {runtimeLabel(runtime)}
                        </option>
                      ))}
                    </select>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="h-6 px-2 text-xs"
                      onClick={addLocalRepo}
                    >
                      <FolderOpen className="size-3" />
                      {t(($) => $.create_project.repos_choose_folder)}
                    </Button>
                  </div>
                )}
              </div>
              {selectedCount > 0 && (
                <div className="space-y-1 pt-1 border-t">
                  <div className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                    {t(($) => $.create_project.repos_selected)}
                  </div>
                  {selectedRepositoryIds.map((id) => {
                    const repo = selectableRepositories.find((candidate) => candidate.id === id);
                    if (!repo) return null;
                    return (
                      <div
                        key={id}
                        className="flex items-center gap-2 text-xs"
                      >
                        <FolderGit2 className="size-3 text-muted-foreground" />
                        <RepositoryText
                          label={repositoryLabel(repo)}
                          detail={repositoryDetail(repo)}
                        />
                        <button
                          type="button"
                          onClick={() => toggleRepository(id)}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <XIcon className="size-3" />
                        </button>
                      </div>
                    );
                  })}
                  {customRepoUrls.map((url) => (
                    <div
                      key={url}
                      className="flex items-center gap-2 text-xs"
                    >
                      <FolderGit2 className="size-3 text-muted-foreground" />
                      <RepositoryText label={url} detail={t(($) => $.create_project.repos_new_remote)} />
                      <button
                        type="button"
                        onClick={() =>
                          setCustomRepoUrls((prev) => prev.filter((item) => item !== url))
                        }
                        className="text-muted-foreground hover:text-foreground"
                      >
                        <XIcon className="size-3" />
                      </button>
                    </div>
                  ))}
                  {localRepoDrafts.map((draft) => {
                    const runtime = localRuntimes.find((candidate) => candidate.id === draft.runtimeId);
                    return (
                      <div
                        key={draft.id}
                        className="flex items-center gap-2 text-xs"
                      >
                        <FolderOpen className="size-3 text-muted-foreground" />
                        <RepositoryText
                          label={draft.name}
                          detail={
                            runtime
                              ? `${draft.path} · ${runtimeLabel(runtime)}`
                              : draft.path
                          }
                        />
                        <button
                          type="button"
                          onClick={() =>
                            setLocalRepoDrafts((prev) => prev.filter((item) => item.id !== draft.id))
                          }
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <XIcon className="size-3" />
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}
            </PopoverContent>
          </Popover>
          </div>

          <Button
            size="sm"
            onClick={handleSubmit}
            disabled={!title.trim() || submitting}
            className="shrink-0"
          >
            {submitting ? t(($) => $.create_project.submitting) : t(($) => $.create_project.submit)}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
