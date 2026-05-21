"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  Eye,
  FileText,
  FolderKanban,
  MessageSquare,
  Plus,
  Sparkles,
} from "lucide-react";
import { toast } from "sonner";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
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
  chatSessionOptions,
  chatSessionsOptions,
  pendingChatTaskOptions,
} from "@multica/core/chat/queries";
import {
  workflowRunDetailOptions,
  workflowRunListOptions,
} from "@multica/core/workflows";
import {
  useApproveChatIssueProposal,
  useCreateChatSession,
  useUpdateChatSession,
} from "@multica/core/chat/mutations";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { agentListOptions, memberListOptions } from "@multica/core/workspace/queries";
import type {
  Agent,
  ChatIssueProposal,
  ChatIssueProposalItem,
  ChatMessage,
  ChatPendingTask,
  ChatSession,
  Issue,
  IssueStatus,
  TaskOutputMetadata,
  UpdateIssueRequest,
  WorkflowArtifact,
  WorkflowRun,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { AppLink, useNavigation } from "../../navigation";
import { PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { TitleEditor } from "../../editor";
import { ChatInput } from "./chat-input";
import { ChatMessageList, ChatMessageSkeleton } from "./chat-message-list";
import { BoardView } from "../../issues/components/board-view";
import { WorkflowArtifactReviewList } from "../../workflows";

type ChatTab = "chat" | "issues" | "outputs";

const chatIssuesViewStore = createIssueViewStore("chat_session_issues_view");
const EMPTY_CHAT_PROPOSALS: ChatIssueProposal[] = [];
const EMPTY_CHAT_ISSUES: Issue[] = [];

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
  const [agentId, setAgentId] = useState<string | null>(null);
  const sessionIdRef = useRef<string | null>(null);
  const sessionPromiseRef = useRef<Promise<string | null> | null>(null);
  const createSession = useCreateChatSession();
  const { uploadWithToast } = useFileUpload(api);
  const visibleAgents = useVisibleChatAgents();
  const selectedAgent = visibleAgents.find((agent) => agent.id === agentId) ?? null;
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, projectId ?? ""),
    enabled: !!projectId,
  });

  const ensureSession = useCallback(
    async (titleSeed: string): Promise<string | null> => {
      if (sessionIdRef.current) return sessionIdRef.current;
      if (!agentId) return null;
      if (sessionPromiseRef.current) return sessionPromiseRef.current;

      const promise = (async () => {
        try {
          const session = await createSession.mutateAsync({
            agent_id: agentId,
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
    [agentId, createSession, projectId, qc, wsId],
  );

  const handleUploadFile = useCallback(
    async (file: File) => {
      const sessionId = await ensureSession("");
      if (!sessionId) return null;
      return uploadWithToast(file, { chatSessionId: sessionId });
    },
    [ensureSession, uploadWithToast],
  );

  const startChat = useMutation({
    mutationFn: async ({ content, attachmentIds }: { content: string; attachmentIds?: string[] }) => {
      if (!agentId) throw new Error(t(($) => $.pages.new.no_agent_error));
      const sessionId = await ensureSession(content);
      if (!sessionId) throw new Error(t(($) => $.pages.new.no_agent_error));
      const result = await api.sendChatMessage(sessionId, content, attachmentIds);
      return { sessionId, result };
    },
    onSuccess: async ({ sessionId, result }) => {
      qc.setQueryData<ChatPendingTask>(chatKeys.pendingTask(sessionId), {
        task_id: result.task_id,
        status: "queued",
        created_at: result.created_at,
      });
      await Promise.all([
        qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) }),
        qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) }),
        qc.invalidateQueries({ queryKey: chatKeys.session(wsId, sessionId) }),
        qc.invalidateQueries({ queryKey: chatKeys.messages(sessionId) }),
        qc.invalidateQueries({ queryKey: chatKeys.pendingTask(sessionId) }),
        qc.invalidateQueries({ queryKey: chatKeys.pendingTasks(wsId) }),
      ]);
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
            onSend={(content, attachmentIds) => startChat.mutate({ content, attachmentIds })}
            onUploadFile={agentId ? handleUploadFile : undefined}
            disabled={startChat.isPending}
            noAgent={visibleAgents.length === 0}
            agentName={selectedAgent?.name}
            draftKeyOverride={`${DRAFT_NEW_SESSION}:route:${projectId ?? "loose"}:${agentId ?? "no-agent"}`}
            editorKeyOverride={`route-new:${projectId ?? "loose"}:${agentId ?? "no-agent"}`}
            leftAdornment={
              <AgentPicker
                selectedAgentId={agentId}
                onSelect={setAgentId}
                disabled={!!sessionIdRef.current || startChat.isPending}
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
  const pendingTaskQuery = useQuery(pendingChatTaskOptions(sessionId));
  const updateSession = useUpdateChatSession();
  const { uploadWithToast } = useFileUpload(api);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const session = sessionQuery.data;
  const sessionAgent = session ? agents.find((agent) => agent.id === session.agent_id) ?? null : null;
  const presenceDetail = useAgentPresenceDetail(wsId, session?.agent_id);
  const availability = presenceDetail === "loading" ? undefined : presenceDetail.availability;
  const pendingTaskId = pendingTaskQuery.data?.task_id ?? null;
  const sendMessage = useMutation({
    mutationFn: ({ content, attachmentIds }: { content: string; attachmentIds?: string[] }) =>
      api.sendChatMessage(sessionId, content, attachmentIds),
    onSuccess: async (result) => {
      qc.setQueryData<ChatPendingTask>(chatKeys.pendingTask(sessionId), {
        task_id: result.task_id,
        status: "queued",
        created_at: result.created_at,
      });
      await Promise.all([
        qc.invalidateQueries({ queryKey: chatKeys.messages(sessionId) }),
        qc.invalidateQueries({ queryKey: chatKeys.pendingTask(sessionId) }),
        qc.invalidateQueries({ queryKey: chatKeys.pendingTasks(wsId) }),
        qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) }),
        qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) }),
        qc.invalidateQueries({ queryKey: chatKeys.session(wsId, sessionId) }),
      ]);
    },
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
      <PageHeader className="justify-between gap-3 px-5">
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
        <Tabs value={tab} onValueChange={(value) => navigation.replace(wsPaths.chatSession(sessionId, parseChatTab(value)))}>
          <TabsList>
            <TabsTrigger value="chat">{t(($) => $.pages.session.tabs.chat)}</TabsTrigger>
            <TabsTrigger value="issues">
              <TabLabel label={t(($) => $.pages.session.tabs.issues)} count={issueCount} />
            </TabsTrigger>
            <TabsTrigger value="outputs">
              <TabLabel label={t(($) => $.pages.session.tabs.outputs)} count={outputCount} />
            </TabsTrigger>
          </TabsList>
        </Tabs>
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
              renderAfterMessage={(message) => (
                <InlineProposalsForMessage
                  message={message}
                  proposals={proposals}
                  href={wsPaths.chatSession(sessionId, "issues")}
                />
              )}
            />
          )}
          <div className="shrink-0 border-t bg-background/95 py-3">
            <ChatInput
              onSend={(content, attachmentIds) => sendMessage.mutate({ content, attachmentIds })}
              onUploadFile={archived ? undefined : handleUploadFile}
              onStop={handleStop}
              isRunning={running}
              disabled={archived}
              agentName={sessionAgent?.name}
              draftKeyOverride={sessionId}
              editorKeyOverride={sessionId}
            />
          </div>
        </TabsContent>
        <TabsContent value="issues" className="min-h-0 overflow-y-auto">
          <ChatIssuesPanel sessionId={sessionId} />
        </TabsContent>
        <TabsContent value="outputs" className="min-h-0 overflow-y-auto">
          <ChatOutputsPanel sessionId={sessionId} />
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
  const { data: issueData, isLoading: issuesLoading } = useQuery(chatIssuesOptions(sessionId));
  const queryClient = useQueryClient();
  const updateIssue = useUpdateIssue();
  const approveProposal = useApproveChatIssueProposal(sessionId);
  const issues = issueData?.issues ?? EMPTY_CHAT_ISSUES;
  const isLoading = proposalsLoading || issuesLoading;
  const pendingProposalItems = useMemo(
    () =>
      proposals.flatMap((proposal) =>
        proposal.items
          .filter((item) => item.status === "pending")
          .map((item) => ({ proposal, item })),
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
  const outputs = data?.outputs ?? [];
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

function TabLabel({ label, count }: { label: string; count: number }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span>{label}</span>
      {count > 0 && (
        <span className="rounded-full bg-muted px-1.5 py-0.5 text-[0.65rem] leading-none text-muted-foreground">
          {count}
        </span>
      )}
    </span>
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

function AgentPicker({
  selectedAgentId,
  onSelect,
  disabled = false,
}: {
  selectedAgentId: string | null;
  onSelect: (agentId: string) => void;
  disabled?: boolean;
}) {
  const { t } = useT("chat");
  const visibleAgents = useVisibleChatAgents();
  const selected = visibleAgents.find((agent) => agent.id === selectedAgentId) ?? null;

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
            <Bot className="size-4 shrink-0" />
            <span className="truncate">{selected ? selected.name : t(($) => $.pages.new.select_agent)}</span>
            <ChevronDown className="ml-auto size-3.5 shrink-0" />
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="w-64">
        {visibleAgents.length === 0 ? (
          <DropdownMenuItem disabled>{t(($) => $.pages.new.no_agents)}</DropdownMenuItem>
        ) : (
          visibleAgents.map((agent) => (
            <DropdownMenuItem key={agent.id} onClick={() => onSelect(agent.id)}>
              <Bot className="size-4" />
              <span className="truncate">{agent.name}</span>
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
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
  selectedIds,
  isApproving,
  onSelectItem,
  onApproveSelected,
}: {
  items: PendingProposalItem[];
  selectedIds: Set<string>;
  isApproving: boolean;
  onSelectItem: (itemId: string, checked: boolean) => void;
  onApproveSelected: () => void;
}) {
  const { t } = useT("chat");
  const selectedCount = items.filter(({ item }) => selectedIds.has(item.id)).length;
  const [preview, setPreview] = useState<PendingProposalItem | null>(null);

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

function useVisibleChatAgents(): Agent[] {
  const wsId = useWorkspaceId();
  const userId = useAuthStore((s) => s.user?.id);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const role = members.find((member) => member.user_id === userId)?.role ?? null;

  return useMemo(
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
  href,
}: {
  message: ChatMessage;
  proposals: ChatIssueProposal[];
  href: string;
}) {
  if (message.role !== "assistant") return null;
  const linked = proposals.filter((proposal) => proposal.source_chat_message_id === message.id);
  if (linked.length === 0) return null;
  return (
    <div className="space-y-2">
      {linked.map((proposal) => (
        <InlineProposalCard key={proposal.id} proposal={proposal} href={href} />
      ))}
    </div>
  );
}

function InlineProposalCard({ proposal, href }: { proposal: ChatIssueProposal; href: string }) {
  const { t } = useT("chat");
  return (
    <AppLink
      href={href}
      className="ml-0 flex max-w-2xl items-center gap-3 rounded-lg border bg-muted/25 px-3 py-2 text-sm transition-colors hover:bg-accent/40"
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
  return value === "issues" || value === "outputs" ? value : "chat";
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
