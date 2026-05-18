"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  ExternalLink,
  FileText,
  FolderKanban,
  MessageSquare,
  Plus,
  RotateCcw,
  Save,
  Sparkles,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { DRAFT_NEW_SESSION } from "@multica/core/chat";
import { useAgentPresenceDetail } from "@multica/core/agents";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { canAssignAgentToIssue } from "@multica/core/permissions";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
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
  useApproveChatIssueProposal,
  useCreateChatSession,
  useDismissChatIssueProposal,
  useRestoreChatIssueProposalItem,
  useUpdateChatIssueProposalItem,
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
  MemberWithUser,
  TaskOutputMetadata,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { AppLink, useNavigation } from "../../navigation";
import { PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { TitleEditor } from "../../editor";
import { ChatInput } from "./chat-input";
import { ChatMessageList, ChatMessageSkeleton } from "./chat-message-list";

type ChatTab = "chat" | "issues" | "outputs";

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
  const wsId = useWorkspaceId();
  const { data: proposals = [], isLoading: proposalsLoading } = useQuery(chatIssueProposalsOptions(sessionId));
  const { data: issueData, isLoading: issuesLoading } = useQuery(chatIssuesOptions(sessionId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const issues = issueData?.issues ?? [];
  const isLoading = proposalsLoading || issuesLoading;

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-5 py-5">
      <section>
        <SectionTitle title={t(($) => $.pages.session.proposals_title)} />
        {isLoading ? (
          <ChatSessionListSkeleton />
        ) : proposals.length === 0 && issues.length === 0 ? (
          <EmptyState
            icon={<FolderKanban className="size-4" />}
            title={t(($) => $.pages.session.empty_issues_title)}
            description={t(($) => $.pages.session.empty_issues_description)}
          />
        ) : proposals.length === 0 ? (
          <p className="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground">
            {t(($) => $.pages.session.empty_proposals_description)}
          </p>
        ) : (
          <div className="space-y-3">
            {proposals.map((proposal) => (
              <ProposalCard
                key={proposal.id}
                sessionId={sessionId}
                proposal={proposal}
                agents={agents}
                members={members}
              />
            ))}
          </div>
        )}
      </section>

      {(issues.length > 0 || proposals.length > 0) && (
        <section>
          <SectionTitle title={t(($) => $.pages.session.created_issues_title)} />
          {issues.length === 0 ? (
            <p className="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground">
              {t(($) => $.pages.session.empty_created_issues_description)}
            </p>
          ) : (
            <div className="divide-y rounded-lg border">
              {issues.map((issue) => (
                <CreatedIssueRow key={issue.id} issue={issue} />
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function ChatOutputsPanel({ sessionId }: { sessionId: string }) {
  const { t } = useT("chat");
  const { data, isLoading } = useQuery(chatOutputsOptions(sessionId));
  const outputs = data?.outputs ?? [];

  return (
    <div className="mx-auto w-full max-w-4xl px-5 py-5">
      <SectionTitle title={t(($) => $.pages.session.outputs_title)} />
      {isLoading ? (
        <ChatSessionListSkeleton />
      ) : outputs.length === 0 ? (
        <EmptyState
          icon={<FileText className="size-4" />}
          title={t(($) => $.pages.session.empty_outputs_title)}
          description={t(($) => $.pages.session.empty_outputs_description)}
        />
      ) : (
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

function OutputSourceLine({ output }: { output: TaskOutputMetadata }) {
  const { t } = useT("chat");
  if (!output.source_type) return null;
  const sourceLabel = output.source_type === "issue_task"
    ? t(($) => $.pages.session.output_source.issue_task)
    : t(($) => $.pages.session.output_source.chat_task);
  const issueLabel = output.source_issue_identifier
    ? output.source_issue_title
      ? `${output.source_issue_identifier} ${output.source_issue_title}`
      : output.source_issue_identifier
    : null;
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

function ProposalCard({
  sessionId,
  proposal,
  agents,
  members,
}: {
  sessionId: string;
  proposal: ChatIssueProposal;
  agents: Agent[];
  members: MemberWithUser[];
}) {
  const { t } = useT("chat");
  const pendingItemIds = useMemo(
    () => proposal.items.filter((item) => item.status === "pending").map((item) => item.id),
    [proposal.items],
  );
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set(pendingItemIds));
  const approveProposal = useApproveChatIssueProposal(sessionId);
  const dismissProposal = useDismissChatIssueProposal(sessionId);
  const hasPendingItems = pendingItemIds.length > 0;
  const selectedCount = pendingItemIds.filter((id) => selectedIds.has(id)).length;

  useEffect(() => {
    setSelectedIds(new Set(pendingItemIds));
  }, [pendingItemIds]);

  const toggleItem = (itemId: string, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(itemId);
      else next.delete(itemId);
      return next;
    });
  };

  const handleApprove = () => {
    const itemIds = pendingItemIds.filter((id) => selectedIds.has(id));
    if (itemIds.length === 0) {
      toast.error(t(($) => $.pages.session.no_selected_items));
      return;
    }
    approveProposal.mutate(
      { proposalId: proposal.id, itemIds },
      {
        onSuccess: (result) => {
          toast.success(t(($) => $.pages.session.approve_success, { count: result.issues.length }));
        },
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.approve_failed));
        },
      },
    );
  };

  const handleDismiss = () => {
    dismissProposal.mutate(proposal.id, {
      onSuccess: () => toast.success(t(($) => $.pages.session.dismiss_success)),
      onError: (err) => {
        toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.dismiss_failed));
      },
    });
  };

  return (
    <div className="rounded-lg border bg-card text-sm">
      <div className="flex flex-col gap-3 border-b px-3 py-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <Sparkles className="size-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0 flex-1 truncate font-medium">{proposal.title}</div>
            <ProposalStatusBadge status={proposal.status} />
          </div>
          {proposal.summary && <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{proposal.summary}</p>}
          <p className="mt-2 text-xs text-muted-foreground">
            {t(($) => $.pages.session.proposal_items, { count: proposal.items.length })}
          </p>
        </div>
        {hasPendingItems && (
          <div className="flex shrink-0 items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleDismiss}
              disabled={dismissProposal.isPending || approveProposal.isPending}
            >
              <X className="size-3.5" />
              {t(($) => $.pages.session.dismiss)}
            </Button>
            <Button
              type="button"
              size="sm"
              onClick={handleApprove}
              disabled={approveProposal.isPending || selectedCount === 0}
            >
              <Check className="size-3.5" />
              {t(($) => $.pages.session.approve_selected, { count: selectedCount })}
            </Button>
          </div>
        )}
      </div>
      <div className="divide-y">
        {proposal.items.map((item) => (
          <ProposalItemEditor
            key={item.id}
            sessionId={sessionId}
            proposalId={proposal.id}
            item={item}
            agents={agents}
            members={members}
            selected={selectedIds.has(item.id)}
            onSelectedChange={(checked) => toggleItem(item.id, checked)}
          />
        ))}
      </div>
    </div>
  );
}

function ProposalItemEditor({
  sessionId,
  proposalId,
  item,
  agents,
  members,
  selected,
  onSelectedChange,
}: {
  sessionId: string;
  proposalId: string;
  item: ChatIssueProposalItem;
  agents: Agent[];
  members: MemberWithUser[];
  selected: boolean;
  onSelectedChange: (checked: boolean) => void;
}) {
  const { t } = useT("chat");
  const wsPaths = useWorkspacePaths();
  const updateItem = useUpdateChatIssueProposalItem(sessionId);
  const restoreItem = useRestoreChatIssueProposalItem(sessionId);
  const [title, setTitle] = useState(item.title);
  const [description, setDescription] = useState(item.description);
  const [priority, setPriority] = useState(item.priority ?? "none");
  const [labels, setLabels] = useState(proposalLabelsToString(item.labels));
  const [assigneeType, setAssigneeType] = useState<"none" | "member" | "agent">(
    item.assignee_type === "member" || item.assignee_type === "agent" ? item.assignee_type : "none",
  );
  const [assigneeId, setAssigneeId] = useState(item.assignee_id ?? "");
  const canEdit = item.status === "pending";
  const assigneeOptions = assigneeType === "member"
    ? members.map((member) => ({ id: member.user_id, label: member.name || member.email }))
    : assigneeType === "agent"
      ? agents.filter((agent) => !agent.archived_at).map((agent) => ({ id: agent.id, label: agent.name }))
      : [];
  const dirty =
    title !== item.title ||
    description !== item.description ||
    priority !== (item.priority ?? "none") ||
    labels !== proposalLabelsToString(item.labels) ||
    assigneeType !== (item.assignee_type === "member" || item.assignee_type === "agent" ? item.assignee_type : "none") ||
    assigneeId !== (item.assignee_id ?? "");

  useEffect(() => {
    setTitle(item.title);
    setDescription(item.description);
    setPriority(item.priority ?? "none");
    setLabels(proposalLabelsToString(item.labels));
    setAssigneeType(item.assignee_type === "member" || item.assignee_type === "agent" ? item.assignee_type : "none");
    setAssigneeId(item.assignee_id ?? "");
  }, [item]);

  const handleAssigneeTypeChange = (value: string | null) => {
    const nextType = value === "member" || value === "agent" ? value : "none";
    setAssigneeType(nextType);
    if (nextType === "none") {
      setAssigneeId("");
      return;
    }
    const options = nextType === "member"
      ? members.map((member) => member.user_id)
      : agents.filter((agent) => !agent.archived_at).map((agent) => agent.id);
    setAssigneeId(options[0] ?? "");
  };

  const handleSave = () => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      toast.error(t(($) => $.pages.session.title_required));
      return;
    }
    const normalizedAssigneeType = assigneeType === "none" ? null : assigneeType;
    const normalizedAssigneeId = normalizedAssigneeType ? assigneeId : null;
    if (normalizedAssigneeType && !normalizedAssigneeId) {
      toast.error(t(($) => $.pages.session.assignee_required));
      return;
    }
    updateItem.mutate(
      {
        proposalId,
        itemId: item.id,
        patch: {
          title: trimmedTitle,
          description: description.trim(),
          priority: priority || null,
          labels: splitProposalLabels(labels),
          assignee_type: normalizedAssigneeType,
          assignee_id: normalizedAssigneeId,
        },
      },
      {
        onSuccess: () => toast.success(t(($) => $.pages.session.save_success)),
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.save_failed));
        },
      },
    );
  };

  const handleRestore = () => {
    restoreItem.mutate(
      { proposalId, itemId: item.id },
      {
        onSuccess: () => toast.success(t(($) => $.pages.session.restore_success)),
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : t(($) => $.pages.session.restore_failed));
        },
      },
    );
  };

  return (
    <div className={cn("px-3 py-3", item.status !== "pending" && "bg-muted/20")}>
      <div className="flex items-start gap-3">
        <Checkbox
          checked={selected}
          disabled={!canEdit}
          onCheckedChange={(checked) => onSelectedChange(checked === true)}
          className="mt-2"
        />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              disabled={!canEdit}
              aria-label={t(($) => $.pages.session.issue_title_label)}
              className="font-medium"
            />
            <div className="flex shrink-0 items-center gap-2">
              <ProposalItemStatusBadge status={item.status} />
              {item.issue_id && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  render={<AppLink href={wsPaths.issueDetail(item.issue_id)} />}
                >
                  <ExternalLink className="size-3.5" />
                </Button>
              )}
            </div>
          </div>
          <Textarea
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            disabled={!canEdit}
            aria-label={t(($) => $.pages.session.issue_description_label)}
            className="min-h-20 resize-y text-sm"
          />
          <div className="grid gap-2 sm:grid-cols-[160px_160px_minmax(0,1fr)]">
            <Select value={priority} onValueChange={(value) => value && setPriority(value)} disabled={!canEdit}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PRIORITY_OPTIONS.map((option) => (
                  <SelectItem key={option} value={option}>
                    {t(($) => $.pages.session.priority[option])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={assigneeType} onValueChange={handleAssigneeTypeChange} disabled={!canEdit}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{t(($) => $.pages.session.assignee_type.none)}</SelectItem>
                <SelectItem value="member">{t(($) => $.pages.session.assignee_type.member)}</SelectItem>
                <SelectItem value="agent">{t(($) => $.pages.session.assignee_type.agent)}</SelectItem>
              </SelectContent>
            </Select>
            {assigneeType === "none" ? (
              <Input value="" disabled placeholder={t(($) => $.pages.session.no_assignee)} />
            ) : (
              <Select value={assigneeId} onValueChange={(value) => setAssigneeId(value ?? "")} disabled={!canEdit}>
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {assigneeOptions.find((option) => option.id === assigneeId)?.label ??
                      t(($) => $.pages.session.select_assignee)}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {assigneeOptions.length === 0 ? (
                    <SelectItem value="__none" disabled>
                      {t(($) => $.pages.session.no_assignee_options)}
                    </SelectItem>
                  ) : (
                    assigneeOptions.map((option) => (
                      <SelectItem key={option.id} value={option.id}>
                        {option.label}
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
            )}
          </div>
          <Input
            value={labels}
            onChange={(event) => setLabels(event.target.value)}
            disabled={!canEdit}
            placeholder={t(($) => $.pages.session.labels_placeholder)}
          />
          <div className="flex justify-end gap-2">
            {item.status === "skipped" && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleRestore}
                disabled={restoreItem.isPending}
              >
                <RotateCcw className="size-3.5" />
                {t(($) => $.pages.session.restore)}
              </Button>
            )}
            {canEdit && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleSave}
                disabled={!dirty || updateItem.isPending}
              >
                <Save className="size-3.5" />
                {t(($) => $.pages.session.save_item)}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
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

function ProposalItemStatusBadge({ status }: { status: ChatIssueProposalItem["status"] }) {
  const { t } = useT("chat");
  const variant = status === "created" ? "default" : status === "skipped" ? "secondary" : "outline";
  return (
    <Badge variant={variant} className={status === "pending" ? "text-muted-foreground" : undefined}>
      {status === "created"
        ? t(($) => $.pages.session.item_status.created)
        : status === "skipped"
          ? t(($) => $.pages.session.item_status.skipped)
          : t(($) => $.pages.session.item_status.pending)}
    </Badge>
  );
}

function CreatedIssueRow({ issue }: { issue: Issue }) {
  const wsPaths = useWorkspacePaths();
  return (
    <AppLink
      href={wsPaths.issueDetail(issue.id)}
      className="flex min-h-12 items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-accent/40"
    >
      <FolderKanban className="size-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <div className="truncate font-medium">{issue.title}</div>
        <div className="truncate text-xs text-muted-foreground">
          {issue.identifier} · {issue.status} · {issue.priority}
        </div>
      </div>
      <ExternalLink className="size-3.5 shrink-0 text-muted-foreground" />
    </AppLink>
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

const PRIORITY_OPTIONS = ["none", "low", "medium", "high", "urgent"] as const;

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

function splitProposalLabels(value: string): string[] {
  return value
    .split(",")
    .map((label) => label.trim())
    .filter(Boolean);
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
