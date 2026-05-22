"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  Eye,
  FileText,
  FolderKanban,
  Lightbulb,
  MessageSquare,
  Plus,
  Sparkles,
  Users,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { cn } from "@multica/ui/lib/utils";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { DRAFT_NEW_SESSION } from "@multica/core/chat";
import { useAgentPresenceDetail } from "@multica/core/agents";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { canAssignAgentToIssue } from "@multica/core/permissions";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { BOARD_STATUSES } from "@multica/core/issues/config";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { createIssueViewStore } from "@multica/core/issues/stores/view-store";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import {
  chatIssueProposalsOptions,
  chatIssuesOptions,
  chatKeys,
  chatMessagesOptions,
  chatOutputsOptions,
  chatPlanEnginesOptions,
  chatPlanRunsOptions,
  chatSessionOptions,
  chatSessionsOptions,
  isActiveChatPlanRun,
  pendingChatTaskOptions,
} from "@multica/core/chat/queries";
import {
  workflowRunDetailOptions,
  workflowRunListOptions,
} from "@multica/core/workflows";
import {
  useApproveChatIssueProposal,
  useCancelChatPlanRun,
  useCreateChatSession,
  useSendChatMessage,
  useUpdateChatSession,
} from "@multica/core/chat/mutations";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { agentListOptions, memberListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { DEFAULT_CHAT_PLAN_ENGINE_ID } from "@multica/core/types";
import type {
  Agent,
  ChatPlanActorType,
  ChatPlanRun,
  ChatIssueProposal,
  ChatIssueProposalItem,
  ChatMessage,
  ChatSession,
  Issue,
  IssueStatus,
  TaskOutputMetadata,
  UpdateIssueRequest,
  WorkflowArtifact,
  WorkflowRun,
  PlanEngine,
  PlanSummary,
  Squad,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsContent } from "@multica/ui/components/ui/tabs";
import { AppLink, useNavigation } from "../../navigation";
import { PageHeader } from "../../layout/page-header";
import { HeaderTabs } from "../../common/header-tabs";
import { useT } from "../../i18n";
import { TitleEditor } from "../../editor";
import { ChatInput } from "./chat-input";
import { ChatMessageList, ChatMessageSkeleton } from "./chat-message-list";
import { BoardView } from "../../issues/components/board-view";
import { WorkflowArtifactReviewList } from "../../workflows";
import { AgentAnalyticsSurface } from "../../agent-analytics";

type ChatTab = "chat" | "issues" | "outputs" | "analytics";

const chatIssuesViewStore = createIssueViewStore("chat_session_issues_view");
const EMPTY_CHAT_PROPOSALS: ChatIssueProposal[] = [];
const EMPTY_CHAT_ISSUES: Issue[] = [];
const FALLBACK_PLAN_ENGINES: PlanEngine[] = [
  {
    id: "grill_with_docs",
    label: "Grill with docs",
    description: "",
    version: "",
    is_default: true,
  },
  {
    id: "brainstorming",
    label: "Brainstorming",
    description: "",
    version: "",
  },
  {
    id: "office_hours",
    label: "Office hours",
    description: "",
    version: "",
  },
];

export type ChatActorSelection = {
  type: ChatPlanActorType;
  id: string;
};

interface BuildChatPlanSendVariablesInput {
  content: string;
  attachmentIds?: string[];
  activePlanRun?: ChatPlanRun | null;
  planMode: boolean;
  selectedEngineId: string;
  selectedActor: ChatActorSelection | null;
}

export function buildChatPlanSendVariables({
  content,
  attachmentIds,
  activePlanRun,
  planMode,
  selectedEngineId,
  selectedActor,
}: BuildChatPlanSendVariablesInput) {
  const base = attachmentIds ? { content, attachmentIds } : { content };
  if (activePlanRun) {
    return { ...base, planRunId: activePlanRun.id };
  }
  if (planMode && selectedActor) {
    return {
      ...base,
      mode: "plan" as const,
      planEngine: selectedEngineId || DEFAULT_CHAT_PLAN_ENGINE_ID,
      planActorType: selectedActor.type,
      planActorId: selectedActor.id,
    };
  }
  return base;
}

export function ChatsPage() {
  const { t } = useT("chat");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const { data: sessions = [], isLoading } = useQuery(
    chatSessionsOptions(wsId, { status: "all", scope: "loose" }),
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="min-w-0">
          <h1 className="truncate text-sm font-semibold">{t(($) => $.pages.chats.title)}</h1>
        </div>
        <Button render={<AppLink href={wsPaths.chatNew()} />}>
          <Plus className="size-4" />
          {t(($) => $.pages.chats.new_chat)}
        </Button>
      </PageHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-4xl px-5 py-5">
          <SectionTitle title={t(($) => $.pages.chats.loose)} />
          {isLoading ? (
            <ChatSessionListSkeleton />
          ) : sessions.length === 0 ? (
            <EmptyState
              icon={<MessageSquare className="size-4" />}
              title={t(($) => $.pages.chats.empty_title)}
              description={t(($) => $.pages.chats.empty_description)}
            />
          ) : (
            <ChatSessionList sessions={sessions} />
          )}
        </div>
      </div>
    </div>
  );
}

export function ChatNewPage() {
  const { t } = useT("chat");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const navigation = useNavigation();
  const qc = useQueryClient();
  const projectId = navigation.searchParams.get("project_id");
  const [actor, setActor] = useState<ChatActorSelection | null>(null);
  const [planMode, setPlanMode] = useState(false);
  const [planEngineId, setPlanEngineId] = useState<string>(DEFAULT_CHAT_PLAN_ENGINE_ID);
  const sessionIdRef = useRef<string | null>(null);
  const sessionPromiseRef = useRef<Promise<string | null> | null>(null);
  const createSession = useCreateChatSession();
  const { uploadWithToast } = useFileUpload(api);
  const { visibleAgents, visibleSquads } = useVisibleChatActors();
  const selectedAgent = resolveLeadAgent(actor, visibleAgents, visibleSquads);
  const selectedActorName = resolveActorName(actor, visibleAgents, visibleSquads);
  const planEngineQuery = useQuery(chatPlanEnginesOptions(wsId));
  const planEngines = planEngineQuery.data?.engines.length
    ? planEngineQuery.data.engines
    : FALLBACK_PLAN_ENGINES;
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, projectId ?? ""),
    enabled: !!projectId,
  });

  useEffect(() => {
    if (!actor) return;
    if (!planMode && actor.type === "squad") {
      setActor(null);
      return;
    }
    if (!isActorVisible(actor, visibleAgents, visibleSquads, planMode)) {
      setActor(null);
    }
  }, [actor, planMode, visibleAgents, visibleSquads]);

  useEffect(() => {
    const defaultEngine = planEngineQuery.data?.default_engine ?? DEFAULT_CHAT_PLAN_ENGINE_ID;
    if (!planEngines.some((engine) => engine.id === planEngineId)) {
      setPlanEngineId(defaultEngine);
    }
  }, [planEngineId, planEngineQuery.data?.default_engine, planEngines]);

  const ensureSession = useCallback(
    async (titleSeed: string): Promise<string | null> => {
      if (sessionIdRef.current) return sessionIdRef.current;
      if (!selectedAgent) return null;
      if (sessionPromiseRef.current) return sessionPromiseRef.current;

      const promise = (async () => {
        try {
          const session = await createSession.mutateAsync({
            agent_id: selectedAgent.id,
            project_id: projectId,
            title: titleFromContent(titleSeed),
          });
          sessionIdRef.current = session.id;
          qc.setQueryData(chatKeys.session(wsId, session.id), session);
          qc.setQueryData<ChatMessage[]>(chatKeys.messages(session.id), (old) => old ?? []);
          return session.id;
        } finally {
          sessionPromiseRef.current = null;
        }
      })();
      sessionPromiseRef.current = promise;
      return promise;
    },
    [createSession, projectId, qc, selectedAgent, wsId],
  );

  const handleUploadFile = useCallback(
    async (file: File) => {
      const sessionId = await ensureSession("");
      if (!sessionId) return null;
      return uploadWithToast(file, { chatSessionId: sessionId });
    },
    [ensureSession, uploadWithToast],
  );

  const startChat = useSendChatMessage({
    resolveSessionId: async (content) => {
      if (!selectedAgent) throw new Error(t(($) => $.pages.new.no_actor_error));
      const sessionId = await ensureSession(content);
      if (!sessionId) throw new Error(t(($) => $.pages.new.no_actor_error));
      return sessionId;
    },
    onSuccess: ({ sessionId }) => {
      navigation.push(wsPaths.chatSession(sessionId));
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : t(($) => $.pages.new.start_failed));
    },
  });

  const title = projectQuery.data
    ? t(($) => $.pages.new.project_title, { project: projectQuery.data.title })
    : t(($) => $.pages.new.title);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex min-h-0 flex-1 items-center justify-center px-5 py-10">
        <div className="w-full max-w-4xl">
          <h1 className="mb-8 text-center text-2xl font-semibold tracking-normal">{title}</h1>
          <ChatInput
            onSend={(content, attachmentIds) =>
              startChat.mutate(buildChatPlanSendVariables({
                content,
                attachmentIds,
                activePlanRun: null,
                planMode,
                selectedEngineId: planEngineId,
                selectedActor: actor,
              }))
            }
            onUploadFile={selectedAgent ? handleUploadFile : undefined}
            disabled={startChat.isPending}
            noAgent={visibleAgents.length === 0}
            agentName={selectedActorName ?? undefined}
            draftKeyOverride={`${DRAFT_NEW_SESSION}:route:${projectId ?? "loose"}:${actor?.type ?? "none"}:${actor?.id ?? "no-agent"}`}
            editorKeyOverride={`route-new:${projectId ?? "loose"}:${actor?.type ?? "none"}:${actor?.id ?? "no-agent"}`}
            topSlot={
              <ChatComposerPlanControls
                planMode={planMode}
                onPlanModeChange={setPlanMode}
                engines={planEngines}
                selectedEngineId={planEngineId}
                onEngineChange={setPlanEngineId}
                actorPicker={
                  <ChatActorPicker
                    selectedActor={actor}
                    onSelect={setActor}
                    visibleAgents={visibleAgents}
                    visibleSquads={visibleSquads}
                    allowSquads={planMode}
                    disabled={!!sessionIdRef.current || startChat.isPending}
                  />
                }
              />
            }
          />
        </div>
      </div>
    </div>
  );
}

export function ChatSessionPage({ sessionId }: { sessionId: string }) {
  const { t } = useT("chat");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const navigation = useNavigation();
  const qc = useQueryClient();
  const tab = parseChatTab(navigation.searchParams.get("tab"));
  const sessionQuery = useQuery(chatSessionOptions(wsId, sessionId));
  const messagesQuery = useQuery(chatMessagesOptions(sessionId));
  const { data: proposals = [] } = useQuery(chatIssueProposalsOptions(sessionId));
  const { data: chatIssuesData } = useQuery(chatIssuesOptions(sessionId));
  const { data: chatOutputsData } = useQuery(chatOutputsOptions(sessionId));
  const { data: planRuns = [] } = useQuery(chatPlanRunsOptions(sessionId));
  const planEngineQuery = useQuery(chatPlanEnginesOptions(wsId));
  const pendingTaskQuery = useQuery(pendingChatTaskOptions(sessionId));
  const updateSession = useUpdateChatSession();
  const cancelPlanRun = useCancelChatPlanRun(sessionId);
  const { uploadWithToast } = useFileUpload(api);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { visibleAgents, visibleSquads } = useVisibleChatActors();
  const [planMode, setPlanMode] = useState(false);
  const [planEngineId, setPlanEngineId] = useState<string>(DEFAULT_CHAT_PLAN_ENGINE_ID);
  const [planActor, setPlanActor] = useState<ChatActorSelection | null>(null);
  const session = sessionQuery.data;
  const sessionAgent = session ? agents.find((agent) => agent.id === session.agent_id) ?? null : null;
  const presenceDetail = useAgentPresenceDetail(wsId, session?.agent_id);
  const availability = presenceDetail === "loading" ? undefined : presenceDetail.availability;
  const pendingTaskId = pendingTaskQuery.data?.task_id ?? null;
  const activePlanRun = useMemo(() => planRuns.find(isActiveChatPlanRun) ?? null, [planRuns]);
  const planEngines = planEngineQuery.data?.engines.length
    ? planEngineQuery.data.engines
    : FALLBACK_PLAN_ENGINES;
  const sendMessage = useSendChatMessage({
    resolveSessionId: async () => sessionId,
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.send_failed));
    },
  });
  const handleUploadFile = useCallback(
    (file: File) => uploadWithToast(file, { chatSessionId: sessionId }),
    [sessionId, uploadWithToast],
  );
  const handleStop = useCallback(() => {
    if (!pendingTaskId) return;
    qc.setQueryData(chatKeys.pendingTask(sessionId), {});
    qc.invalidateQueries({ queryKey: chatKeys.messages(sessionId) });
    api.cancelTaskById(pendingTaskId).catch((err) => {
      toast.error(err instanceof Error ? err.message : t(($) => $.input.stop_tooltip));
    });
  }, [pendingTaskId, qc, sessionId, t]);

  useEffect(() => {
    const defaultEngine = planEngineQuery.data?.default_engine ?? DEFAULT_CHAT_PLAN_ENGINE_ID;
    if (!planEngines.some((engine) => engine.id === planEngineId)) {
      setPlanEngineId(defaultEngine);
    }
  }, [planEngineId, planEngineQuery.data?.default_engine, planEngines]);

  useEffect(() => {
    if (!session?.agent_id) return;
    const fallbackActor: ChatActorSelection = { type: "agent", id: session.agent_id };
    if (!planMode) {
      if (!sameActor(planActor, fallbackActor)) setPlanActor(fallbackActor);
      return;
    }
    if (planActor && isActorVisible(planActor, visibleAgents, visibleSquads, true)) return;
    if (!sameActor(planActor, fallbackActor)) setPlanActor(fallbackActor);
  }, [planActor, planMode, session?.agent_id, visibleAgents, visibleSquads]);

  const handleTitleBlur = useCallback(
    (value: string) => {
      const title = value.trim();
      if (!session || title === "" || title === session.title) return;
      updateSession.mutate(
        { sessionId, title },
        {
          onSuccess: (updated) => {
            qc.setQueryData(chatKeys.session(wsId, sessionId), updated);
          },
        },
      );
    },
    [qc, session, sessionId, updateSession, wsId],
  );

  const running = !!pendingTaskId || sendMessage.isPending;
  const archived = session?.status === "archived";
  const issueCount = chatIssuesData?.total ?? 0;
  const outputCount = chatOutputsData?.total ?? 0;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="h-auto min-h-12 flex-wrap justify-between gap-3 px-5 py-2 sm:h-12 sm:flex-nowrap sm:py-0">
        <div className="min-w-0">
          <TitleEditor
            key={session?.id ?? sessionId}
            defaultValue={session?.title ?? ""}
            placeholder={t(($) => $.pages.session.untitled)}
            className="truncate text-sm font-semibold leading-tight"
            onBlur={handleTitleBlur}
          />
          {session?.project_snapshot && (
            <p className="truncate text-xs text-muted-foreground">{session.project_snapshot.title}</p>
          )}
        </div>
        <HeaderTabs
          ariaLabel={t(($) => $.pages.session.tabs.label)}
          value={tab}
          className="w-full sm:w-[24rem]"
          onValueChange={(value) => navigation.replace(wsPaths.chatSession(sessionId, value))}
          items={[
            { value: "chat", label: t(($) => $.pages.session.tabs.chat) },
            { value: "issues", label: t(($) => $.pages.session.tabs.issues), count: issueCount },
            { value: "outputs", label: t(($) => $.pages.session.tabs.outputs), count: outputCount },
            { value: "analytics", label: t(($) => $.pages.session.tabs.analytics) },
          ]}
        />
      </PageHeader>
      <Tabs value={tab} className="min-h-0 flex-1">
        <TabsContent value="chat" className="flex min-h-0 flex-col">
          {messagesQuery.isLoading ? (
            <ChatMessageSkeleton />
          ) : (messagesQuery.data ?? []).length === 0 ? (
            <div className="flex min-h-0 flex-1 items-center justify-center px-5">
              <EmptyState
                icon={<MessageSquare className="size-4" />}
                title={t(($) => $.pages.session.empty_chat_title)}
                description={t(($) => $.pages.session.empty_chat_description)}
              />
            </div>
          ) : (
            <ChatMessageList
              messages={messagesQuery.data ?? []}
              pendingTask={pendingTaskQuery.data}
              availability={availability}
              agents={agents}
              renderAfterMessage={(message) => (
                <InlineProposalsForMessage
                  message={message}
                  proposals={proposals}
                  planRuns={planRuns}
                  href={wsPaths.chatSession(sessionId, "issues")}
                />
              )}
            />
          )}
          <div className="shrink-0 border-t bg-background/95 py-3">
            <ChatInput
              onSend={(content, attachmentIds) =>
                sendMessage.mutate(buildChatPlanSendVariables({
                  content,
                  attachmentIds,
                  activePlanRun,
                  planMode,
                  selectedEngineId: planEngineId,
                  selectedActor: planActor,
                }))
              }
              onUploadFile={archived ? undefined : handleUploadFile}
              onStop={handleStop}
              isRunning={running}
              disabled={archived}
              agentName={sessionAgent?.name}
              draftKeyOverride={sessionId}
              editorKeyOverride={sessionId}
              topSlot={
                <ChatComposerPlanControls
                  planMode={planMode}
                  onPlanModeChange={setPlanMode}
                  engines={planEngines}
                  selectedEngineId={planEngineId}
                  onEngineChange={setPlanEngineId}
                  activePlanRun={activePlanRun}
                  onCancelActivePlan={() => {
                    if (activePlanRun) cancelPlanRun.mutate(activePlanRun.id);
                  }}
                  cancelPending={cancelPlanRun.isPending}
                  actorPicker={planMode && !activePlanRun ? (
                    <ChatActorPicker
                      selectedActor={planActor}
                      onSelect={setPlanActor}
                      visibleAgents={visibleAgents.filter((agent) => agent.id === session?.agent_id)}
                      visibleSquads={visibleSquads}
                      allowSquads
                      disabled={sendMessage.isPending || running}
                    />
                  ) : null}
                />
              }
            />
          </div>
        </TabsContent>
        <TabsContent value="issues" className="min-h-0 overflow-y-auto">
          <ChatIssuesPanel sessionId={sessionId} />
        </TabsContent>
        <TabsContent value="outputs" className="min-h-0 overflow-y-auto">
          <ChatOutputsPanel sessionId={sessionId} />
        </TabsContent>
        <TabsContent value="analytics" className="min-h-0">
          <AgentAnalyticsSurface
            scope={{ kind: "chat", sessionId }}
            active={tab === "analytics"}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export function ProjectChatsSurface({ projectId }: { projectId: string }) {
  const { t } = useT("chat");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const { data: sessions = [], isLoading } = useQuery(
    chatSessionsOptions(wsId, { status: "all", scope: "project", projectId }),
  );

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto w-full max-w-5xl px-5 py-5">
        <div className="mb-4 flex items-center justify-between gap-3">
          <SectionTitle title={t(($) => $.pages.project_chats.title)} />
          <Button size="sm" render={<AppLink href={wsPaths.chatNew(projectId)} />}>
            <Plus className="size-4" />
            {t(($) => $.pages.project_chats.new_chat)}
          </Button>
        </div>
        {isLoading ? (
          <ChatSessionListSkeleton />
        ) : sessions.length === 0 ? (
          <EmptyState
            icon={<MessageSquare className="size-4" />}
            title={t(($) => $.pages.project_chats.empty_title)}
            description={t(($) => $.pages.project_chats.empty_description)}
          />
        ) : (
          <ChatSessionList sessions={sessions} />
        )}
      </div>
    </div>
  );
}

function ChatIssuesPanel({ sessionId }: { sessionId: string }) {
  const { t } = useT("chat");
  const { data: proposals = EMPTY_CHAT_PROPOSALS, isLoading: proposalsLoading } = useQuery(chatIssueProposalsOptions(sessionId));
  const { data: planRuns = [] } = useQuery(chatPlanRunsOptions(sessionId));
  const { data: issueData, isLoading: issuesLoading } = useQuery(chatIssuesOptions(sessionId));
  const queryClient = useQueryClient();
  const updateIssue = useUpdateIssue();
  const approveProposal = useApproveChatIssueProposal(sessionId);
  const issues = issueData?.issues ?? EMPTY_CHAT_ISSUES;
  const isLoading = proposalsLoading || issuesLoading;
  const pendingProposalItems = useMemo(
    () =>
      proposals.flatMap((proposal) =>
        proposal.status === "pending"
          ? proposal.items
              .filter((item) => item.status === "pending")
              .map((item) => ({ proposal, item }))
          : [],
      ),
    [proposals],
  );
  const [selectedProposalItemIds, setSelectedProposalItemIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    setSelectedProposalItemIds(new Set(pendingProposalItems.map(({ item }) => item.id)));
  }, [pendingProposalItems]);

  const handleSelectProposalItem = useCallback((itemId: string, checked: boolean) => {
    setSelectedProposalItemIds((current) => {
      const next = new Set(current);
      if (checked) next.add(itemId);
      else next.delete(itemId);
      return next;
    });
  }, []);

  const handleApproveSelected = useCallback(async () => {
    const grouped = new Map<string, string[]>();
    for (const { proposal, item } of pendingProposalItems) {
      if (!selectedProposalItemIds.has(item.id)) continue;
      const ids = grouped.get(proposal.id) ?? [];
      ids.push(item.id);
      grouped.set(proposal.id, ids);
    }
    if (grouped.size === 0) return;

    try {
      const selectedCount = [...grouped.values()].reduce((sum, itemIds) => sum + itemIds.length, 0);
      await Promise.all(
        [...grouped.entries()].map(([proposalId, itemIds]) =>
          approveProposal.mutateAsync({ proposalId, itemIds }),
        ),
      );
      toast.success(t(($) => $.pages.session.approve_success, { count: selectedCount }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.approve_failed));
    }
  }, [approveProposal, pendingProposalItems, selectedProposalItemIds, t]);

  const handleMoveIssue = useCallback(
    (issueId: string, updates: Pick<UpdateIssueRequest, "status" | "assignee_type" | "assignee_id" | "position">) => {
      updateIssue.mutate(
        { id: issueId, ...updates },
        {
          onSettled: () => {
            queryClient.invalidateQueries({ queryKey: chatKeys.issues(sessionId) });
          },
        },
      );
    },
    [queryClient, sessionId, updateIssue],
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      {isLoading ? (
        <div className="mx-auto w-full max-w-4xl px-5 py-5">
          <ChatSessionListSkeleton />
        </div>
      ) : proposals.length === 0 && issues.length === 0 ? (
        <div className="mx-auto w-full max-w-4xl px-5 py-5">
          <EmptyState
            icon={<FolderKanban className="size-4" />}
            title={t(($) => $.pages.session.empty_issues_title)}
            description={t(($) => $.pages.session.empty_issues_description)}
          />
        </div>
      ) : (
        <ViewStoreProvider store={chatIssuesViewStore}>
          <BoardView
            issues={issues}
            visibleStatuses={BOARD_STATUSES as IssueStatus[]}
            hiddenStatuses={[]}
            onMoveIssue={handleMoveIssue}
            leadingColumn={
              <ProposedIssuesColumn
                items={pendingProposalItems}
                planRuns={planRuns}
                selectedIds={selectedProposalItemIds}
                isApproving={approveProposal.isPending}
                onSelectItem={handleSelectProposalItem}
                onApproveSelected={handleApproveSelected}
              />
            }
          />
        </ViewStoreProvider>
      )}
    </div>
  );
}

function ChatOutputsPanel({ sessionId }: { sessionId: string }) {
  const { t } = useT("chat");
  const { t: tWorkflows } = useT("workflows");
  const wsId = useWorkspaceId();
  const { data, isLoading } = useQuery(chatOutputsOptions(sessionId));
  const workflowRunsQuery = useQuery(
    workflowRunListOptions(wsId, { chatSessionId: sessionId }),
  );
  const latestWorkflowRunId = workflowRunsQuery.data?.[0]?.id ?? null;
  const workflowRunDetailQuery = useQuery(
    workflowRunDetailOptions(wsId, latestWorkflowRunId),
  );
  const outputs = useMemo(() => data?.outputs ?? [], [data?.outputs]);
  const outputTaskIds = useMemo(
    () =>
      Array.from(
        new Set(
          outputs
            .map((output) => output.task_id)
            .filter((taskId): taskId is string => typeof taskId === "string" && taskId.length > 0),
        ),
      ),
    [outputs],
  );
  const outputWorkflowRunQueries = useQueries({
    queries: outputTaskIds.map((taskId) => workflowRunListOptions(wsId, { taskId })),
  });
  const outputWorkflowRunIds = useMemo(
    () =>
      Array.from(
        new Set(
          outputWorkflowRunQueries
            .map((query) => query.data?.[0]?.id)
            .filter((runId): runId is string => typeof runId === "string" && runId.length > 0),
        ),
      ).filter((runId) => runId !== latestWorkflowRunId),
    [latestWorkflowRunId, outputWorkflowRunQueries],
  );
  const outputWorkflowRunDetailQueries = useQueries({
    queries: outputWorkflowRunIds.map((runId) => workflowRunDetailOptions(wsId, runId)),
  });
  const outputIssueLabels = useMemo(() => {
    const labels = new Map<string, string>();
    for (const output of outputs) {
      if (!output.source_issue_id) continue;
      const label = outputSourceLabel(output);
      if (label) labels.set(output.source_issue_id, label);
    }
    return labels;
  }, [outputs]);
  const workflowArtifacts = useMemo(() => {
    const byId = new Map<string, WorkflowArtifact>();
    for (const artifact of workflowRunDetailQuery.data?.artifacts ?? []) {
      byId.set(artifact.id, artifact);
    }
    for (const query of outputWorkflowRunDetailQueries) {
      for (const artifact of query.data?.artifacts ?? []) {
        byId.set(artifact.id, artifact);
      }
    }
    return Array.from(byId.values());
  }, [outputWorkflowRunDetailQueries, workflowRunDetailQuery.data]);
  const artifactContextById = useMemo(() => {
    const byId = new Map<string, string>();
    const addRunContexts = (run: WorkflowRun | undefined) => {
      if (!run?.artifacts?.length) return;
      const issueLabel = run.issue_id ? outputIssueLabels.get(run.issue_id) : undefined;
      const stepTitleById = new Map((run.steps ?? []).map((step) => [step.id, step.title]));
      for (const artifact of run.artifacts) {
        const stepTitle = stepTitleById.get(artifact.workflow_step_run_id);
        const context = [issueLabel, stepTitle].filter(Boolean).join(" · ");
        if (context) byId.set(artifact.id, context);
      }
    };
    addRunContexts(workflowRunDetailQuery.data);
    for (const query of outputWorkflowRunDetailQueries) {
      addRunContexts(query.data);
    }
    return byId;
  }, [outputIssueLabels, outputWorkflowRunDetailQueries, workflowRunDetailQuery.data]);
  const artifactContextLabel = useCallback(
    (artifact: WorkflowArtifact) => artifactContextById.get(artifact.id),
    [artifactContextById],
  );
  const loading =
    isLoading ||
    workflowRunsQuery.isLoading ||
    workflowRunDetailQuery.isLoading ||
    outputWorkflowRunQueries.some((query) => query.isLoading) ||
    outputWorkflowRunDetailQueries.some((query) => query.isLoading);
  const hasOutputs = outputs.length > 0 || workflowArtifacts.length > 0;

  return (
    <div className="mx-auto w-full max-w-4xl px-5 py-5">
      <SectionTitle title={t(($) => $.pages.session.outputs_title)} />
      {loading ? (
        <ChatSessionListSkeleton />
      ) : !hasOutputs ? (
        <EmptyState
          icon={<FileText className="size-4" />}
          title={t(($) => $.pages.session.empty_outputs_title)}
          description={t(($) => $.pages.session.empty_outputs_description)}
        />
      ) : (
        <div className="space-y-5">
          {workflowArtifacts.length > 0 && (
            <section className="space-y-2">
              <div className="text-xs font-medium text-muted-foreground">
                {tWorkflows(($) => $.runtime.latest_artifacts)}
              </div>
              <WorkflowArtifactReviewList
                artifacts={workflowArtifacts}
                contextLabelForArtifact={artifactContextLabel}
              />
            </section>
          )}
          {outputs.length > 0 && (
            <section className="space-y-2">
              <div className="text-xs font-medium text-muted-foreground">
                {t(($) => $.pages.session.outputs_title)}
              </div>
              <div className="divide-y rounded-lg border">
                {outputs.map((output) => (
                  <div key={output.id} className="flex min-h-12 items-center gap-3 px-3 py-2 text-sm">
                    <FileText className="size-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-medium">{output.filename || output.relative_path}</div>
                      <div className="truncate text-xs text-muted-foreground">
                        {output.kind} · {output.relative_path}
                      </div>
                      <OutputSourceLine output={output} />
                    </div>
                    <span className="shrink-0 text-xs text-muted-foreground">{formatBytes(output.size_bytes)}</span>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

function outputSourceLabel(output: TaskOutputMetadata) {
  if (output.source_issue_identifier) {
    return output.source_issue_title
      ? `${output.source_issue_identifier} ${output.source_issue_title}`
      : output.source_issue_identifier;
  }
  return output.source_issue_title ?? null;
}

function OutputSourceLine({ output }: { output: TaskOutputMetadata }) {
  const { t } = useT("chat");
  if (!output.source_type) return null;
  const sourceLabel = output.source_type === "issue_task"
    ? t(($) => $.pages.session.output_source.issue_task)
    : t(($) => $.pages.session.output_source.chat_task);
  const issueLabel = outputSourceLabel(output);
  return (
    <div className="truncate text-xs text-muted-foreground">
      {issueLabel ? `${sourceLabel} · ${issueLabel}` : sourceLabel}
    </div>
  );
}

function ChatSessionList({ sessions }: { sessions: ChatSession[] }) {
  const wsPaths = useWorkspacePaths();
  const { t } = useT("chat");

  return (
    <div className="divide-y rounded-lg border">
      {sessions.map((session) => (
        <AppLink
          key={session.id}
          href={wsPaths.chatSession(session.id)}
          className="flex min-h-12 items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-accent/40"
        >
          <MessageSquare className="size-4 shrink-0 text-muted-foreground" />
          <div className="min-w-0 flex-1">
            <div className="truncate font-medium">{session.title || t(($) => $.pages.session.untitled)}</div>
            <div className="truncate text-xs text-muted-foreground">
              {session.project_snapshot?.title ?? t(($) => $.pages.chats.loose)} · {formatRelativeTime(session.updated_at)}
            </div>
          </div>
          {session.status === "archived" && (
            <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
              {t(($) => $.pages.session.archived)}
            </span>
          )}
        </AppLink>
      ))}
    </div>
  );
}

function ChatActorPicker({
  selectedActor,
  onSelect,
  visibleAgents,
  visibleSquads,
  allowSquads,
  disabled = false,
}: {
  selectedActor: ChatActorSelection | null;
  onSelect: (actor: ChatActorSelection) => void;
  visibleAgents: Agent[];
  visibleSquads: Squad[];
  allowSquads: boolean;
  disabled?: boolean;
}) {
  const { t } = useT("chat");
  const selectedAgent = selectedActor?.type === "agent"
    ? visibleAgents.find((agent) => agent.id === selectedActor.id) ?? null
    : null;
  const selectedSquad = selectedActor?.type === "squad"
    ? visibleSquads.find((squad) => squad.id === selectedActor.id) ?? null
    : null;
  const selectedLabel = selectedSquad?.name ?? selectedAgent?.name ?? null;
  const squads = allowSquads ? visibleSquads : [];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={disabled}
            className="h-8 max-w-56 justify-start gap-2 px-2"
          >
            {selectedActor?.type === "squad" ? (
              <Users className="size-4 shrink-0" />
            ) : (
              <Bot className="size-4 shrink-0" />
            )}
            <span className="truncate">{selectedLabel ?? t(($) => $.pages.new.select_agent)}</span>
            <ChevronDown className="ml-auto size-3.5 shrink-0" />
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="w-64">
        {visibleAgents.length === 0 && squads.length === 0 ? (
          <DropdownMenuItem disabled>{t(($) => $.pages.new.no_agents)}</DropdownMenuItem>
        ) : (
          <>
            {visibleAgents.map((agent) => (
              <DropdownMenuItem key={agent.id} onClick={() => onSelect({ type: "agent", id: agent.id })}>
                <Bot className="size-4" />
                <span className="truncate">{agent.name}</span>
              </DropdownMenuItem>
            ))}
            {squads.map((squad) => (
              <DropdownMenuItem key={squad.id} onClick={() => onSelect({ type: "squad", id: squad.id })}>
                <Users className="size-4" />
                <span className="truncate">{squad.name}</span>
              </DropdownMenuItem>
            ))}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ChatComposerPlanControls({
  planMode,
  onPlanModeChange,
  engines,
  selectedEngineId,
  onEngineChange,
  activePlanRun,
  onCancelActivePlan,
  cancelPending = false,
  actorPicker,
}: {
  planMode: boolean;
  onPlanModeChange: (enabled: boolean) => void;
  engines: PlanEngine[];
  selectedEngineId: string;
  onEngineChange: (engineId: string) => void;
  activePlanRun?: ChatPlanRun | null;
  onCancelActivePlan?: () => void;
  cancelPending?: boolean;
  actorPicker?: ReactNode;
}) {
  const { t } = useT("chat");
  const selectedEngine = engines.find((engine) => engine.id === selectedEngineId) ?? engines[0] ?? FALLBACK_PLAN_ENGINES[0];
  const activeEngine = activePlanRun
    ? engines.find((engine) => engine.id === activePlanRun.plan_engine) ?? selectedEngine
    : selectedEngine;

  if (activePlanRun) {
    return (
      <div className="flex min-h-9 items-center gap-2 border-b px-2 py-1.5 text-xs">
        <Lightbulb className="size-3.5 shrink-0 text-brand" />
        <div className="min-w-0 flex-1 truncate text-muted-foreground">
          <span className="font-medium text-foreground">
            {t(($) => $.plan.active_title)}
          </span>
          <span className="mx-1">·</span>
          <span>{activeEngine?.label ?? activePlanRun.plan_engine}</span>
          <span className="mx-1">·</span>
          <span>{planRunStatusLabel(t, activePlanRun.status)}</span>
        </div>
        {onCancelActivePlan && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 shrink-0 px-2 text-xs"
            disabled={cancelPending}
            onClick={onCancelActivePlan}
          >
            <X className="size-3.5" />
            {t(($) => $.plan.cancel)}
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className="flex min-h-9 flex-wrap items-center gap-1.5 border-b px-2 py-1.5">
      <Button
        type="button"
        variant={planMode ? "secondary" : "ghost"}
        size="sm"
        className="h-7 gap-1.5 px-2 text-xs"
        onClick={() => onPlanModeChange(!planMode)}
        aria-pressed={planMode}
      >
        <Lightbulb className="size-3.5" />
        {t(($) => $.plan.mode)}
      </Button>
      {planMode && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button type="button" variant="ghost" size="sm" className="h-7 max-w-48 gap-1.5 px-2 text-xs">
                <span className="truncate">{selectedEngine?.label ?? t(($) => $.plan.engine_fallback)}</span>
                <ChevronDown className="size-3.5 shrink-0" />
              </Button>
            }
          />
          <DropdownMenuContent align="start" className="w-72">
            {engines.map((engine) => (
              <DropdownMenuItem key={engine.id} onClick={() => onEngineChange(engine.id)}>
                <div className="min-w-0">
                  <div className="truncate text-sm">{engine.label}</div>
                  {engine.description && (
                    <div className="line-clamp-2 text-xs text-muted-foreground">{engine.description}</div>
                  )}
                </div>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      {actorPicker}
    </div>
  );
}

type PendingProposalItem = {
  proposal: ChatIssueProposal;
  item: ChatIssueProposalItem;
};

function proposalPriorityLabel(
  t: ReturnType<typeof useT<"chat">>["t"],
  priority: string | null,
): string {
  switch (priority) {
    case "urgent":
      return t(($) => $.pages.session.priority.urgent);
    case "high":
      return t(($) => $.pages.session.priority.high);
    case "medium":
      return t(($) => $.pages.session.priority.medium);
    case "low":
      return t(($) => $.pages.session.priority.low);
    default:
      return t(($) => $.pages.session.priority.none);
  }
}

export function ProposedIssuesColumn({
  items,
  planRuns = [],
  selectedIds,
  isApproving,
  onSelectItem,
  onApproveSelected,
}: {
  items: PendingProposalItem[];
  planRuns?: ChatPlanRun[];
  selectedIds: Set<string>;
  isApproving: boolean;
  onSelectItem: (itemId: string, checked: boolean) => void;
  onApproveSelected: () => void;
}) {
  const { t } = useT("chat");
  const selectedCount = items.filter(({ item }) => selectedIds.has(item.id)).length;
  const [preview, setPreview] = useState<PendingProposalItem | null>(null);
  const linkedPlanRuns = useMemo(
    () => planRunsForProposals(items.map(({ proposal }) => proposal), planRuns),
    [items, planRuns],
  );

  return (
    <div className="flex w-[280px] shrink-0 flex-col rounded-xl bg-brand/5 p-2 ring-1 ring-brand/15">
      <div className="mb-2 flex items-center justify-between px-1.5">
        <div className="flex items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-xs font-semibold">
            <Sparkles className="h-3 w-3 text-brand" />
            {t(($) => $.pages.session.proposed_lane_title)}
          </span>
          <span className="text-xs text-muted-foreground">{items.length}</span>
        </div>
      </div>
      <div className="min-h-[200px] flex-1 space-y-2 overflow-y-auto rounded-lg p-1">
        {linkedPlanRuns.map((run) => (
          <PlanSummaryBlock key={run.id} summary={run.summary} compact />
        ))}
        {items.length === 0 ? (
          <p className="py-8 text-center text-xs text-muted-foreground">
            {t(($) => $.pages.session.empty_proposed_lane)}
          </p>
        ) : (
          items.map(({ proposal, item }) => (
            <div
              key={item.id}
              onClick={() => setPreview({ proposal, item })}
              className="group/proposal cursor-pointer rounded-lg border bg-background p-3 text-left text-sm shadow-xs transition-colors hover:border-brand/35 hover:bg-brand/5"
            >
              <div className="flex items-start gap-2">
                <div
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={(event) => event.stopPropagation()}
                >
                  <Checkbox
                    checked={selectedIds.has(item.id)}
                    onCheckedChange={(checked) => onSelectItem(item.id, checked === true)}
                    className="mt-0.5"
                    aria-label={t(($) => $.pages.session.select_proposed_issue, { title: item.title })}
                  />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="line-clamp-2 font-medium">{item.title}</div>
                  <div className="mt-1 truncate text-xs text-muted-foreground">{proposal.title}</div>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  className="mt-[-0.125rem] size-6 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover/proposal:opacity-100 focus-visible:opacity-100"
                  aria-label={t(($) => $.pages.session.preview_issue, { title: item.title })}
                  onClick={(event) => {
                    event.stopPropagation();
                    setPreview({ proposal, item });
                  }}
                >
                  <Eye className="size-3.5" />
                </Button>
              </div>
              {item.description && (
                <p className="mt-2 line-clamp-3 text-xs text-muted-foreground">{item.description}</p>
              )}
              <div className="mt-2 flex flex-wrap gap-1">
                {item.priority && (
                  <Badge variant="outline" className="text-[10px]">
                    {item.priority}
                  </Badge>
                )}
                {proposalLabelsToString(item.labels)
                  .split(",")
                  .map((label) => label.trim())
                  .filter(Boolean)
                  .slice(0, 3)
                  .map((label) => (
                    <Badge key={label} variant="secondary" className="max-w-full truncate text-[10px]">
                      {label}
                    </Badge>
                  ))}
              </div>
            </div>
          ))
        )}
      </div>
      {items.length > 0 && (
        <div className="mt-2 px-1">
          <Button
            type="button"
            className="w-full"
            size="sm"
            disabled={selectedCount === 0 || isApproving}
            onClick={onApproveSelected}
          >
            <Check className="size-3.5" />
            {t(($) => $.pages.session.approve_selected, { count: selectedCount })}
          </Button>
        </div>
      )}
      <ProposedIssuePreviewDialog
        preview={preview}
        selected={preview ? selectedIds.has(preview.item.id) : false}
        onOpenChange={(open) => {
          if (!open) setPreview(null);
        }}
        onSelectItem={(checked) => {
          if (!preview) return;
          onSelectItem(preview.item.id, checked);
        }}
      />
    </div>
  );
}

function ProposedIssuePreviewDialog({
  preview,
  selected,
  onOpenChange,
  onSelectItem,
}: {
  preview: PendingProposalItem | null;
  selected: boolean;
  onOpenChange: (open: boolean) => void;
  onSelectItem: (checked: boolean) => void;
}) {
  const { t } = useT("chat");
  const labels = preview
    ? proposalLabelsToString(preview.item.labels)
      .split(",")
      .map((label) => label.trim())
      .filter(Boolean)
    : [];

  return (
    <Dialog open={!!preview} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,42rem)] max-w-none sm:max-w-none">
        <DialogHeader>
          <DialogTitle>{preview?.item.title}</DialogTitle>
          <DialogDescription>
            {preview
              ? t(($) => $.pages.session.preview_source, {
                  proposal: preview.proposal.title,
                })
              : null}
          </DialogDescription>
        </DialogHeader>

        {preview && (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline" className="text-xs">
                {proposalPriorityLabel(t, preview.item.priority)}
              </Badge>
              {labels.map((label) => (
                <Badge key={label} variant="secondary" className="max-w-full truncate text-xs">
                  {label}
                </Badge>
              ))}
            </div>

            <div className="rounded-lg border bg-muted/20 p-3">
              <div className="mb-1 text-xs font-medium text-muted-foreground">
                {t(($) => $.pages.session.issue_description_label)}
              </div>
              {preview.item.description ? (
                <p className="whitespace-pre-wrap text-sm leading-6">
                  {preview.item.description}
                </p>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {t(($) => $.pages.session.preview_no_description)}
                </p>
              )}
            </div>

            {preview.proposal.summary && (
              <div className="rounded-lg border bg-background p-3">
                <div className="mb-1 text-xs font-medium text-muted-foreground">
                  {t(($) => $.pages.session.preview_proposal_summary)}
                </div>
                <p className="text-sm leading-6 text-muted-foreground">
                  {preview.proposal.summary}
                </p>
              </div>
            )}

            <label className="flex items-center gap-2 rounded-lg border bg-background px-3 py-2 text-sm">
              <Checkbox
                checked={selected}
                onCheckedChange={(checked) => onSelectItem(checked === true)}
              />
              <span>{t(($) => $.pages.session.preview_include_in_create)}</span>
            </label>
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t(($) => $.pages.session.preview_close)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function useVisibleChatActors(): { visibleAgents: Agent[]; visibleSquads: Squad[] } {
  const wsId = useWorkspaceId();
  const userId = useAuthStore((s) => s.user?.id);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const role = members.find((member) => member.user_id === userId)?.role ?? null;

  const visibleAgents = useMemo(
    () =>
      agents.filter(
        (agent) =>
          !agent.archived_at &&
          canAssignAgentToIssue(agent, {
            userId: userId ?? null,
            role: role === "owner" || role === "admin" || role === "member" ? role : null,
          }).allowed,
      ),
    [agents, role, userId],
  );
  const visibleAgentIds = useMemo(
    () => new Set(visibleAgents.map((agent) => agent.id)),
    [visibleAgents],
  );
  const visibleSquads = useMemo(
    () => squads.filter((squad) => !squad.archived_at && visibleAgentIds.has(squad.leader_id)),
    [squads, visibleAgentIds],
  );

  return { visibleAgents, visibleSquads };
}

function ProposalStatusBadge({ status }: { status: ChatIssueProposal["status"] }) {
  const { t } = useT("chat");
  const label =
    status === "accepted"
      ? t(($) => $.pages.session.proposal_status.accepted)
      : status === "dismissed"
        ? t(($) => $.pages.session.proposal_status.dismissed)
        : status === "partially_accepted"
          ? t(($) => $.pages.session.proposal_status.partially_accepted)
          : status === "superseded"
            ? t(($) => $.pages.session.proposal_status.superseded)
            : t(($) => $.pages.session.proposal_status.pending);
  return (
    <Badge variant="outline" className="text-muted-foreground">
      {label}
    </Badge>
  );
}

function InlineProposalsForMessage({
  message,
  proposals,
  planRuns,
  href,
}: {
  message: ChatMessage;
  proposals: ChatIssueProposal[];
  planRuns: ChatPlanRun[];
  href: string;
}) {
  if (message.role !== "assistant") return null;
  const linked = proposals.filter((proposal) => proposal.source_chat_message_id === message.id);
  if (linked.length === 0) return null;
  return (
    <div className="space-y-2">
      {linked.map((proposal) => (
        <InlineProposalCard
          key={proposal.id}
          proposal={proposal}
          planRun={planRuns.find((run) => run.id === proposal.source_plan_run_id) ?? null}
          href={href}
        />
      ))}
    </div>
  );
}

function InlineProposalCard({
  proposal,
  planRun,
  href,
}: {
  proposal: ChatIssueProposal;
  planRun: ChatPlanRun | null;
  href: string;
}) {
  const { t } = useT("chat");
  return (
    <div className="max-w-2xl space-y-2">
      <PlanSummaryBlock summary={planRun?.summary ?? null} compact />
      <AppLink
        href={href}
        className="ml-0 flex items-center gap-3 rounded-lg border bg-muted/25 px-3 py-2 text-sm transition-colors hover:bg-accent/40"
      >
        <Sparkles className="size-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium">{proposal.title}</div>
          <div className="truncate text-xs text-muted-foreground">
            {t(($) => $.pages.session.inline_proposal_summary, { count: proposal.items.length })}
          </div>
        </div>
        <ProposalStatusBadge status={proposal.status} />
      </AppLink>
    </div>
  );
}

export function PlanSummaryBlock({
  summary,
  compact = false,
}: {
  summary: PlanSummary | null | undefined;
  compact?: boolean;
}) {
  const { t } = useT("chat");
  if (!summary || !hasPlanSummaryContent(summary)) return null;
  const sections = [
    {
      key: "confirmed",
      label: t(($) => $.plan.summary.confirmed),
      items: summary.confirmed_requirements,
    },
    {
      key: "rejected",
      label: t(($) => $.plan.summary.rejected),
      items: summary.rejected_options,
    },
    {
      key: "consensus",
      label: t(($) => $.plan.summary.consensus),
      items: summary.consensus_notes,
    },
    {
      key: "open",
      label: t(($) => $.plan.summary.open),
      items: summary.open_questions,
    },
  ].filter((section) => section.items.length > 0);

  return (
    <div className={cn(
      "rounded-lg border bg-background/80 text-xs",
      compact ? "p-2" : "p-3",
    )}>
      <div className="mb-1.5 flex items-center gap-1.5 font-medium">
        <Lightbulb className="size-3.5 text-brand" />
        {t(($) => $.plan.summary.title)}
      </div>
      <div className="space-y-1.5 text-muted-foreground">
        {sections.map((section) => (
          <div key={section.key}>
            <span className="font-medium text-foreground">{section.label}</span>
            <span className="mx-1">·</span>
            <span>{section.items.join("; ")}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function proposalLabelsToString(labels: unknown[]): string {
  return labels
    .map((label) => {
      if (typeof label === "string") return label;
      if (label && typeof label === "object" && "name" in label) {
        const value = (label as { name?: unknown }).name;
        return typeof value === "string" ? value : "";
      }
      return "";
    })
    .filter(Boolean)
    .join(", ");
}

function SectionTitle({ title }: { title: string }) {
  return <h2 className="mb-3 text-xs font-semibold uppercase tracking-normal text-muted-foreground">{title}</h2>;
}

function EmptyState({
  icon,
  title,
  description,
}: {
  icon: ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-40 flex-col items-center justify-center rounded-lg border border-dashed px-5 py-8 text-center">
      <div className="mb-2 flex size-8 items-center justify-center rounded-md bg-muted text-muted-foreground">{icon}</div>
      <div className="text-sm font-medium">{title}</div>
      <div className="mt-1 max-w-sm text-sm text-muted-foreground">{description}</div>
    </div>
  );
}

function ChatSessionListSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-12 w-full rounded-lg" />
      <Skeleton className="h-12 w-full rounded-lg" />
      <Skeleton className="h-12 w-full rounded-lg" />
    </div>
  );
}

function parseChatTab(value: string | null | undefined): ChatTab {
  return value === "issues" || value === "outputs" || value === "analytics" ? value : "chat";
}

function sameActor(a: ChatActorSelection | null, b: ChatActorSelection | null): boolean {
  return a?.type === b?.type && a?.id === b?.id;
}

function isActorVisible(
  actor: ChatActorSelection,
  visibleAgents: Agent[],
  visibleSquads: Squad[],
  allowSquads: boolean,
): boolean {
  if (actor.type === "agent") return visibleAgents.some((agent) => agent.id === actor.id);
  return allowSquads && visibleSquads.some((squad) => squad.id === actor.id);
}

function resolveLeadAgent(
  actor: ChatActorSelection | null,
  visibleAgents: Agent[],
  visibleSquads: Squad[],
): Agent | null {
  if (!actor) return null;
  if (actor.type === "agent") return visibleAgents.find((agent) => agent.id === actor.id) ?? null;
  const squad = visibleSquads.find((candidate) => candidate.id === actor.id);
  if (!squad) return null;
  return visibleAgents.find((agent) => agent.id === squad.leader_id) ?? null;
}

function resolveActorName(
  actor: ChatActorSelection | null,
  visibleAgents: Agent[],
  visibleSquads: Squad[],
): string | null {
  if (!actor) return null;
  if (actor.type === "agent") {
    return visibleAgents.find((agent) => agent.id === actor.id)?.name ?? null;
  }
  return visibleSquads.find((squad) => squad.id === actor.id)?.name ?? null;
}

function planRunStatusLabel(
  t: ReturnType<typeof useT<"chat">>["t"],
  status: string,
): string {
  switch (status) {
    case "consulting":
      return t(($) => $.plan.status.consulting);
    case "ready_for_approval":
      return t(($) => $.plan.status.ready_for_approval);
    case "completed":
      return t(($) => $.plan.status.completed);
    case "cancelled":
      return t(($) => $.plan.status.cancelled);
    case "failed":
      return t(($) => $.plan.status.failed);
    case "brainstorming":
      return t(($) => $.plan.status.brainstorming);
    default:
      return t(($) => $.plan.status.active);
  }
}

function planRunsForProposals(
  proposals: ChatIssueProposal[],
  planRuns: ChatPlanRun[],
): ChatPlanRun[] {
  const ids = new Set(
    proposals
      .map((proposal) => proposal.source_plan_run_id)
      .filter((id): id is string => !!id),
  );
  return planRuns.filter((run) => ids.has(run.id) && hasPlanSummaryContent(run.summary));
}

function hasPlanSummaryContent(summary: PlanSummary | null | undefined): summary is PlanSummary {
  if (!summary) return false;
  return (
    summary.confirmed_requirements.length > 0 ||
    summary.rejected_options.length > 0 ||
    summary.consensus_notes.length > 0 ||
    summary.open_questions.length > 0
  );
}

function titleFromContent(content: string): string {
  return content.replace(/\s+/g, " ").trim().slice(0, 80);
}

function formatRelativeTime(value: string): string {
  if (!value) return "";
  const time = new Date(value).getTime();
  if (!Number.isFinite(time)) return "";
  const diff = Date.now() - time;
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return "now";
  if (diff < hour) return `${Math.floor(diff / minute)}m`;
  if (diff < day) return `${Math.floor(diff / hour)}h`;
  if (diff < 7 * day) return `${Math.floor(diff / day)}d`;
  return new Date(value).toLocaleDateString();
}

function formatBytes(size: number | null): string {
  if (size == null) return "";
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${Math.round(size / 1024)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}
