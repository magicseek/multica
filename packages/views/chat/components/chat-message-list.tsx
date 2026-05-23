"use client";

import { useMemo, useState, useRef, type ReactNode } from "react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@multica/ui/lib/utils";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Button } from "@multica/ui/components/ui/button";
import { ActorAvatar as ActorAvatarBase } from "@multica/ui/components/common/actor-avatar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@multica/ui/components/ui/collapsible";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@multica/ui/components/ui/tooltip";
import { ChevronRight, ChevronDown, Brain, AlertCircle, AlertTriangle, Copy, Bot } from "lucide-react";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import { useAutoScroll } from "@multica/ui/hooks/use-auto-scroll";
import { taskMessagesOptions } from "@multica/core/chat/queries";
import { Markdown } from "@multica/views/common/markdown";
import { copyMarkdown } from "../../editor";
import { WorkflowRunViewer } from "../../workflows";
import type { AgentAvailability } from "@multica/core/agents";
import type { Agent, ChatMessage, ChatPendingTask, Squad, TaskMessagePayload, TaskFailureReason, User } from "@multica/core/types";
import type { ChatTimelineItem } from "@multica/core/chat";
import { failureReasonLabel } from "../../agents/components/tabs/task-failure";
import { TaskStatusPill } from "./task-status-pill";
import { formatElapsedMs } from "../lib/format";
import { splitTimeline, extractCopyText } from "../lib/copy-text";
import { useT } from "../../i18n";

// ─── Public component ────────────────────────────────────────────────────

interface ChatMessageListProps {
  messages: ChatMessage[];
  /**
   * Server-authoritative pending-task snapshot. `null` / undefined means
   * no in-flight task — list renders without StatusPill.
   */
  pendingTask: ChatPendingTask | null | undefined;
  /** Resolved presence; pass `undefined` while loading to keep the pill copy neutral. */
  availability: AgentAvailability | undefined;
  agents?: (Pick<Agent, "id" | "name"> & { avatar_url?: string | null })[];
  squads?: Pick<Squad, "id" | "name" | "avatar_url">[];
  currentUser?: Pick<User, "id" | "name" | "avatar_url"> | null;
  sessionAgentId?: string | null;
  /** Optional per-message extension point rendered inside the message body. */
  renderAfterMessage?: (message: ChatMessage) => ReactNode;
}

export function ChatMessageList({
  messages,
  pendingTask,
  availability,
  agents,
  squads,
  currentUser,
  sessionAgentId,
  renderAfterMessage,
}: ChatMessageListProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const fadeStyle = useScrollFade(scrollRef);
  useAutoScroll(scrollRef);
  const agentById = useMemo(
    () => new Map((agents ?? []).map((agent) => [agent.id, agent])),
    [agents],
  );
  const squadById = useMemo(
    () => new Map((squads ?? []).map((squad) => [squad.id, squad])),
    [squads],
  );

  const pendingTaskId = pendingTask?.task_id ?? null;

  // Once the assistant message for this pending task has landed in the
  // messages list, AssistantMessage owns its rendering — suppress the live
  // timeline (and pill) to avoid rendering the same content in two places
  // during the invalidate → refetch window.
  const pendingAlreadyPersisted = !!pendingTaskId && messages.some(
    (m) => m.role === "assistant" && m.task_id === pendingTaskId,
  );

  // Live timeline for the in-flight task. useRealtimeSync keeps this cache
  // current via setQueryData on task:message events.
  const showLiveTimeline = !!pendingTaskId && !pendingAlreadyPersisted;
  const { data: liveTaskMessages } = useQuery({
    ...taskMessagesOptions(pendingTaskId ?? ""),
    enabled: showLiveTimeline,
  });
  const liveTimeline: ChatTimelineItem[] = (liveTaskMessages ?? []).map(toTimelineItem);
  const showLiveTaskRow = showLiveTimeline && !!pendingTask;

  return (
    <div ref={scrollRef} style={fadeStyle} className="flex-1 overflow-y-auto">
      {/* Inner container matches issue / project detail width convention
       *  (max-w-4xl + mx-auto) so switching between chat and content
       *  views doesn't jolt the reading width. px-5 is a touch tighter
       *  than issue-detail's px-8 because the chat window can be narrow. */}
      <div className="mx-auto w-full max-w-4xl px-5 py-4 space-y-4">
        {messages.map((msg) => (
          <MessageBubble
            key={msg.id}
            message={msg}
            isPending={!!pendingTaskId && msg.task_id === pendingTaskId}
            agentById={agentById}
            squadById={squadById}
            currentUser={currentUser}
            addon={renderAfterMessage?.(msg)}
          />
        ))}
        {showLiveTaskRow && pendingTask && (
          <LiveTaskMessage
            items={liveTimeline}
            pendingTask={pendingTask}
            taskMessages={liveTaskMessages ?? []}
            availability={availability}
            fallbackAgentId={sessionAgentId}
            agentById={agentById}
            squadById={squadById}
          />
        )}
      </div>
    </div>
  );
}

function LiveTaskMessage({
  items,
  pendingTask,
  taskMessages,
  availability,
  fallbackAgentId,
  agentById,
  squadById,
}: {
  items: ChatTimelineItem[];
  pendingTask: ChatPendingTask;
  taskMessages: readonly TaskMessagePayload[];
  availability: AgentAvailability | undefined;
  fallbackAgentId?: string | null;
  agentById: Map<string, Pick<Agent, "id" | "name"> & { avatar_url?: string | null }>;
  squadById: Map<string, Pick<Squad, "id" | "name" | "avatar_url">>;
}) {
  const agentId = pendingTask?.agent_id ?? fallbackAgentId ?? null;
  const agent = agentId ? agentById.get(agentId) : null;
  const sender: ResolvedMessageActor = {
    type: "agent",
    id: agentId ?? "agent",
    name: agent?.name ?? "Agent",
    avatarUrl: agent?.avatar_url ?? null,
  };
  return (
    <div className="flex w-full items-start gap-3" data-testid="chat-live-task-row">
      <ActorAvatarBase
        name={sender.name}
        initials={initialsForName(sender.name)}
        avatarUrl={sender.avatarUrl}
        isAgent
        size={28}
      />
      <div className="min-w-0 flex-1 space-y-1">
        <MessageHeader
          sender={sender}
          recipients={[]}
          agentById={agentById}
          squadById={squadById}
        />
        <div className="w-full space-y-1.5" data-testid="chat-live-task-body">
          <TaskStatusPill
            pendingTask={pendingTask}
            taskMessages={taskMessages}
            availability={availability}
            className="px-0"
          />
          {items.length > 0 && (
            <TimelineView items={items} isStreaming />
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * Placeholder shown while `chat_message` for a session is being fetched
 * (initial refresh, or switching to an un-cached session). Shape roughly
 * mirrors an assistant → user → assistant exchange so the window doesn't
 * shift under the user when real messages arrive.
 */
export function ChatMessageSkeleton() {
  return (
    <div className="flex-1 overflow-hidden">
      <div className="mx-auto w-full max-w-4xl px-5 py-4 space-y-5">
        <div className="space-y-2">
          <Skeleton className="h-3.5 w-3/4" />
          <Skeleton className="h-3.5 w-1/2" />
        </div>
        <div className="flex justify-end">
          <Skeleton className="h-8 w-48 rounded-2xl" />
        </div>
        <div className="space-y-2">
          <Skeleton className="h-3.5 w-2/3" />
          <Skeleton className="h-3.5 w-5/6" />
          <Skeleton className="h-3.5 w-1/3" />
        </div>
      </div>
    </div>
  );
}

function toTimelineItem(m: TaskMessagePayload): ChatTimelineItem {
  return {
    seq: m.seq,
    type: m.type,
    tool: m.tool,
    content: m.content,
    input: m.input,
    output: m.output,
  };
}

// ─── Message bubbles ─────────────────────────────────────────────────────

function MessageBubble({
  message,
  isPending,
  agentById,
  squadById,
  currentUser,
  addon,
}: {
  message: ChatMessage;
  isPending: boolean;
  agentById: Map<string, Pick<Agent, "id" | "name"> & { avatar_url?: string | null }>;
  squadById: Map<string, Pick<Squad, "id" | "name" | "avatar_url">>;
  currentUser?: Pick<User, "id" | "name" | "avatar_url"> | null;
  addon?: ReactNode;
}) {
  const sender = resolveMessageSender(message, currentUser, agentById);
  const recipients = message.recipients ?? [];
  const helperAgentName = message.author_agent_id
    ? agentById.get(message.author_agent_id)?.name
    : undefined;

  return (
    <div className="flex w-full items-start gap-3" data-testid="chat-message-row" data-message-id={message.id}>
      <ActorAvatarBase
        name={sender.name}
        initials={initialsForName(sender.name)}
        avatarUrl={sender.avatarUrl}
        isAgent={sender.type === "agent"}
        isSystem={sender.type === "system"}
        isSquad={sender.type === "squad"}
        size={28}
      />
      <div className="min-w-0 flex-1 space-y-1">
        <MessageHeader
          sender={sender}
          recipients={recipients}
          agentById={agentById}
          squadById={squadById}
        />
        {message.role === "user" ? (
          <div className="text-sm leading-relaxed prose prose-sm dark:prose-invert max-w-none [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
            <Markdown>{message.content}</Markdown>
          </div>
        ) : (
          <AssistantMessage
            message={message}
            isPending={isPending}
            helperAgentName={helperAgentName}
          />
        )}
        <RoutingWarnings warnings={message.routing_warnings ?? []} />
        {addon}
      </div>
    </div>
  );
}

type ResolvedMessageActor = {
  type: string;
  id: string;
  name: string;
  avatarUrl?: string | null;
};

function resolveMessageSender(
  message: ChatMessage,
  currentUser: Pick<User, "id" | "name" | "avatar_url"> | null | undefined,
  agentById: Map<string, Pick<Agent, "id" | "name"> & { avatar_url?: string | null }>,
): ResolvedMessageActor {
  const sender = message.sender;
  if (sender?.type === "agent" && sender.id) {
    const agent = agentById.get(sender.id);
    return {
      type: "agent",
      id: sender.id,
      name: agent?.name ?? "Agent",
      avatarUrl: agent?.avatar_url ?? null,
    };
  }
  if (message.author_agent_id) {
    const agent = agentById.get(message.author_agent_id);
    return {
      type: "agent",
      id: message.author_agent_id,
      name: agent?.name ?? "Agent",
      avatarUrl: agent?.avatar_url ?? null,
    };
  }
  if (sender?.type === "system" || message.author_type === "system") {
    return { type: "system", id: "system", name: "Multica" };
  }
  const memberId = sender?.type === "member" && sender.id
    ? sender.id
    : message.author_member_id ?? currentUser?.id ?? "member";
  return {
    type: "member",
    id: memberId,
    name: currentUser?.id === memberId ? currentUser.name : currentUser?.name ?? "You",
    avatarUrl: currentUser?.id === memberId ? currentUser.avatar_url : null,
  };
}

function MessageHeader({
  sender,
  recipients,
  agentById,
  squadById,
}: {
  sender: ResolvedMessageActor;
  recipients: NonNullable<ChatMessage["recipients"]>;
  agentById: Map<string, Pick<Agent, "id" | "name"> & { avatar_url?: string | null }>;
  squadById: Map<string, Pick<Squad, "id" | "name" | "avatar_url">>;
}) {
  const routedRecipients = recipients.filter((recipient) => recipient.status !== "blocked");
  return (
    <div className="flex min-h-5 flex-wrap items-center gap-1.5 text-xs">
      <span className="font-medium text-foreground">{sender.name}</span>
      {routedRecipients.length > 0 && (
        <>
          <span className="text-muted-foreground">→</span>
          <span className="truncate text-muted-foreground">
            {routedRecipients
              .map((recipient) => recipientLabel(recipient, agentById, squadById))
              .join(", ")}
          </span>
        </>
      )}
    </div>
  );
}

function recipientLabel(
  recipient: NonNullable<ChatMessage["recipients"]>[number],
  agentById: Map<string, Pick<Agent, "id" | "name"> & { avatar_url?: string | null }>,
  squadById: Map<string, Pick<Squad, "id" | "name" | "avatar_url">>,
): string {
  if (recipient.recipient_type === "agent") {
    return agentById.get(recipient.recipient_id)?.name ?? "Agent";
  }
  if (recipient.recipient_type === "squad") {
    return squadById.get(recipient.recipient_id)?.name ?? "Squad";
  }
  if (recipient.recipient_type === "member") {
    return "Member";
  }
  return "Recipient";
}

function RoutingWarnings({ warnings }: { warnings: NonNullable<ChatMessage["routing_warnings"]> }) {
  if (warnings.length === 0) return null;
  return (
    <div className="space-y-1">
      {warnings.map((warning) => (
        <div
          key={`${warning.recipient_id}:${warning.code}:${warning.message}`}
          className="inline-flex max-w-full items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground"
        >
          <AlertCircle className="size-3 shrink-0" />
          <span className="truncate">{warning.message || warning.code}</span>
        </div>
      ))}
    </div>
  );
}

function initialsForName(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((part) => part[0])
    .join("")
    .toUpperCase()
    .slice(0, 2) || "U";
}

function AssistantMessage({
  message,
  isPending,
  helperAgentName,
}: {
  message: ChatMessage;
  isPending: boolean;
  helperAgentName?: string;
}) {
  const { t } = useT("chat");
  const taskId = message.task_id;
  const isConsultation = !!message.consultation_id && !!message.author_agent_id;

  // Use the shared taskMessagesOptions so this cache entry is the same one
  // seeded by useRealtimeSync during task execution — zero refetch when the
  // task finishes, since WS already populated it.
  const { data: taskMessages } = useQuery({
    ...taskMessagesOptions(taskId ?? ""),
    enabled: !!taskId,
  });

  const timeline: ChatTimelineItem[] = (taskMessages ?? []).map(toTimelineItem);

  // Failure bubble path: when the server's FailTask wrote a failure
  // chat_message (failure_reason set), render a destructive bubble with the
  // human-readable reason label + collapsible raw errMsg + the same timeline
  // so the user can see exactly where the run broke.
  if (message.failure_reason) {
    return (
      <FailureBubble
        reason={message.failure_reason}
        rawError={message.content}
        timeline={timeline}
        elapsedMs={message.elapsed_ms}
      />
    );
  }

  return (
    <div className={cn(
      "w-full space-y-1.5",
      isConsultation && "rounded-lg border-l-2 border-brand/30 bg-muted/20 py-2 pl-3 pr-2",
    )}>
      {isConsultation && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Bot className="size-3.5" />
          {t(($) => $.plan.consultation.from_helper, {
            name: helperAgentName ?? t(($) => $.plan.consultation.helper_fallback),
          })}
        </div>
      )}
      {timeline.length > 0 ? (
        <TimelineView items={timeline} />
      ) : (
        <div className="text-sm leading-relaxed prose prose-sm dark:prose-invert max-w-none">
          <Markdown>{message.content}</Markdown>
        </div>
      )}
      <MessageFooter
        message={message}
        timeline={timeline}
        isPending={isPending}
      />
      {taskId && <WorkflowRunViewer taskId={taskId} />}
    </div>
  );
}

// Inline footer row beneath the assistant reply: "Replied in 38s · [Copy]".
// Action icons live here (not as a hover-floating overlay) so they're
// discoverable on first read and don't shift content. Buttons stay quiet
// (muted) until hover. Copy is suppressed during streaming because the
// final text is still being appended.
function MessageFooter({
  message,
  timeline,
  isPending,
}: {
  message: ChatMessage;
  timeline: ChatTimelineItem[];
  isPending: boolean;
}) {
  const showCopy = !isPending;
  if (message.elapsed_ms == null && !showCopy) return null;
  return (
    <div className="flex items-center gap-1.5">
      {message.elapsed_ms != null && (
        <ElapsedCaption variant="replied" elapsedMs={message.elapsed_ms} />
      )}
      {showCopy && <MessageCopyButton message={message} timeline={timeline} />}
    </div>
  );
}

function MessageCopyButton({
  message,
  timeline,
}: {
  message: ChatMessage;
  timeline: ChatTimelineItem[];
}) {
  const { t } = useT("chat");
  const handleCopy = async () => {
    try {
      await copyMarkdown(extractCopyText(message, timeline));
      toast.success(t(($) => $.message_list.copied_toast));
    } catch {
      toast.error(t(($) => $.message_list.copy_failed_toast));
    }
  };
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-xs"
            className="text-muted-foreground/70 hover:text-foreground"
            onClick={handleCopy}
            aria-label={t(($) => $.message_list.copy_action)}
          />
        }
      >
        <Copy />
      </TooltipTrigger>
      <TooltipContent side="top">
        {t(($) => $.message_list.copy_action)}
      </TooltipContent>
    </Tooltip>
  );
}

// Persisted "Replied in 38s" / "Failed after 12s" line under the assistant
// bubble. Reads `elapsed_ms` straight off the chat_message — server computes
// it once at task completion, so this caption is identical across reloads
// and devices. Skipped silently when null (legacy messages predating
// migration 063 + user messages).
function ElapsedCaption({
  variant,
  elapsedMs,
  className,
}: {
  variant: "replied" | "failed";
  elapsedMs: number;
  className?: string;
}) {
  const { t } = useT("chat");
  const text =
    variant === "replied"
      ? t(($) => $.message_list.replied_in, { elapsed: formatElapsedMs(elapsedMs) })
      : t(($) => $.message_list.failed_after, { elapsed: formatElapsedMs(elapsedMs) });
  return (
    <div className={cn("text-xs text-muted-foreground/80", className)}>
      {text}
    </div>
  );
}

function FailureBubble({
  reason,
  rawError,
  timeline,
  elapsedMs,
}: {
  reason: string;
  rawError: string;
  timeline: ChatTimelineItem[];
  elapsedMs?: number | null;
}) {
  const { t } = useT("chat");
  const [open, setOpen] = useState(false);
  // Map the back-end enum to copy via the shared label table; an unknown
  // reason (e.g. a future enum value the front-end doesn't ship yet)
  // falls back to a generic translated label.
  const label =
    failureReasonLabel[reason as TaskFailureReason] ??
    t(($) => $.message_list.task_failed_fallback);

  return (
    <div className="w-full space-y-1.5">
      {/* Failure read as an inline, low-key note — not a destructive
       *  alert. Intentionally borderless / no background tint: a chat
       *  failure is informational ("this didn't work"), not a system
       *  error. The icon + muted destructive text are signal enough,
       *  the rest stays in the normal reply rhythm. */}
      <div className="flex items-start gap-1.5 text-sm">
        <AlertTriangle className="size-3.5 shrink-0 text-destructive/80 mt-0.5" />
        <div className="flex-1 min-w-0">
          <div className="text-destructive/90">{label}</div>
          {rawError.trim() && (
            <Collapsible open={open} onOpenChange={setOpen}>
              <CollapsibleTrigger className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
                {open ? (
                  <ChevronDown className="size-3" />
                ) : (
                  <ChevronRight className="size-3" />
                )}
                <span>{t(($) => $.message_list.show_details)}</span>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <pre className="mt-1 max-h-40 overflow-auto rounded bg-muted/40 p-2 text-xs text-muted-foreground whitespace-pre-wrap break-all">
                  {rawError}
                </pre>
              </CollapsibleContent>
            </Collapsible>
          )}
        </div>
      </div>
      {timeline.length > 0 && <TimelineView items={timeline} />}
      {elapsedMs != null && (
        <ElapsedCaption variant="failed" elapsedMs={elapsedMs} />
      )}
    </div>
  );
}

// ─── Timeline: outer process fold + final text (Conductor-style) ─────────
//
// splitTimeline (lib/copy-text.ts) carves the items into:
//   preface — text before the first thinking/tool item
//   middle  — first → last non-text item (inclusive, may sandwich text)
//   final   — text after the last non-text item
//
// We render preface + final outside an outer Collapsible ("X steps") that
// wraps middle. The inner row Collapsibles (ThinkingRow / ToolCallRow /
// ToolResultRow) are unchanged — clicking them toggles independently of
// the outer fold. Copy mirrors what's visible when the outer fold is
// closed: preface + final, never middle. See extractCopyText for the
// authoritative copy logic.

function TimelineView({
  items,
  isStreaming,
}: {
  items: ChatTimelineItem[];
  isStreaming?: boolean;
}) {
  const { preface, middle, final } = splitTimeline(items);

  return (
    <>
      {preface.length > 0 && (
        <div className="text-sm leading-relaxed prose prose-sm dark:prose-invert max-w-none">
          <Markdown>{preface.map((t) => t.content ?? "").join("")}</Markdown>
        </div>
      )}
      {middle.length > 0 && (
        <OuterProcessFold items={middle} defaultOpen={!!isStreaming} />
      )}
      {final.length > 0 && (
        <div className="text-sm leading-relaxed prose prose-sm dark:prose-invert max-w-none">
          <Markdown>{final.map((t) => t.content ?? "").join("")}</Markdown>
        </div>
      )}
    </>
  );
}

function OuterProcessFold({
  items,
  defaultOpen,
}: {
  items: ChatTimelineItem[];
  defaultOpen?: boolean;
}) {
  const { t } = useT("chat");
  // useState seeds once at mount — subsequent renders never overwrite the
  // user's manual toggle. The streaming → completed transition unmounts
  // the live <TimelineView> and mounts the persisted AssistantMessage's
  // own <TimelineView>, so the persisted instance starts closed (default)
  // even if the live one was open. That's the desired collapsed-default.
  const [open, setOpen] = useState(defaultOpen ?? false);
  const stepCount = items.length;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
        {open ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}
        <span>{t(($) => $.message_list.process_steps, { count: stepCount })}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-1 rounded-lg border bg-muted/20 p-2 space-y-0.5">
          {items.map((item) =>
            item.type === "text" ? (
              <MiddleTextRow key={item.seq} item={item} />
            ) : (
              <ItemRow key={item.seq} item={item} />
            ),
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// Intermediate text segment rendered inside the outer fold. Visually
// down-shifted (xs / muted) so it reads as part of the agent's process,
// not the final answer — the final answer renders below the fold at full
// prose size.
function MiddleTextRow({ item }: { item: ChatTimelineItem }) {
  return (
    <div className="py-0.5 text-xs text-muted-foreground prose prose-sm dark:prose-invert max-w-none [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
      <Markdown>{item.content ?? ""}</Markdown>
    </div>
  );
}

// ─── Individual item rows ────────────────────────────────────────────────

function ItemRow({ item }: { item: ChatTimelineItem }) {
  switch (item.type) {
    case "tool_use":
      return <ToolCallRow item={item} />;
    case "tool_result":
      return <ToolResultRow item={item} />;
    case "thinking":
      return <ThinkingRow item={item} />;
    case "error":
      return <ErrorRow item={item} />;
    default:
      return null;
  }
}

function shortenPath(p: string): string {
  const parts = p.split("/");
  if (parts.length <= 3) return p;
  return ".../" + parts.slice(-2).join("/");
}

function getToolSummary(item: ChatTimelineItem): string {
  if (!item.input) return "";
  const inp = item.input as Record<string, string>;
  if (inp.query) return inp.query;
  if (inp.file_path) return shortenPath(inp.file_path);
  if (inp.path) return shortenPath(inp.path);
  if (inp.pattern) return inp.pattern;
  if (inp.description) return String(inp.description);
  if (inp.command) {
    const cmd = String(inp.command);
    return cmd.length > 100 ? cmd.slice(0, 100) + "..." : cmd;
  }
  if (inp.prompt) {
    const p = String(inp.prompt);
    return p.length > 100 ? p.slice(0, 100) + "..." : p;
  }
  if (inp.skill) return String(inp.skill);
  for (const v of Object.values(inp)) {
    if (typeof v === "string" && v.length > 0 && v.length < 120) return v;
  }
  return "";
}

function ToolCallRow({ item }: { item: ChatTimelineItem }) {
  const [open, setOpen] = useState(false);
  const summary = getToolSummary(item);
  const hasInput = item.input && Object.keys(item.input).length > 0;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <ChevronRight
          className={cn(
            "h-3 w-3 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
            !hasInput && "invisible",
          )}
        />
        <span className="font-medium text-foreground shrink-0">{item.tool}</span>
        {summary && <span className="truncate text-muted-foreground">{summary}</span>}
      </CollapsibleTrigger>
      {hasInput && (
        <CollapsibleContent>
          <pre className="ml-[18px] mt-0.5 max-h-32 overflow-auto rounded bg-muted/50 p-2 text-xs text-muted-foreground whitespace-pre-wrap break-all">
            {JSON.stringify(item.input, null, 2)}
          </pre>
        </CollapsibleContent>
      )}
    </Collapsible>
  );
}

function ToolResultRow({ item }: { item: ChatTimelineItem }) {
  const { t } = useT("chat");
  const [open, setOpen] = useState(false);
  const output = item.output ?? "";
  if (!output) return null;

  const preview = output.length > 120 ? output.slice(0, 120) + "..." : output;
  const labelPrefix = item.tool
    ? t(($) => $.message_list.tool_result_named, { tool: item.tool })
    : t(($) => $.message_list.tool_result_unnamed);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-start gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <ChevronRight
          className={cn("h-3 w-3 shrink-0 text-muted-foreground transition-transform mt-0.5", open && "rotate-90")}
        />
        <span className="text-muted-foreground/70 truncate">
          {labelPrefix}{preview}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="ml-[18px] mt-0.5 max-h-40 overflow-auto rounded bg-muted/50 p-2 text-xs text-muted-foreground whitespace-pre-wrap break-all">
          {output.length > 4000 ? output.slice(0, 4000) + "\n... (truncated)" : output}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

function ThinkingRow({ item }: { item: ChatTimelineItem }) {
  const [open, setOpen] = useState(false);
  const text = item.content ?? "";
  if (!text) return null;

  const preview = text.length > 150 ? text.slice(0, 150) + "..." : text;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-start gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <Brain className="h-3 w-3 shrink-0 text-muted-foreground/60 mt-0.5" />
        <span className="text-muted-foreground italic truncate">{preview}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="ml-[18px] mt-0.5 max-h-40 overflow-auto rounded bg-muted/30 p-2 text-xs text-muted-foreground whitespace-pre-wrap break-words">
          {text}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

function ErrorRow({ item }: { item: ChatTimelineItem }) {
  return (
    <div className="flex items-start gap-1.5 px-1 -mx-1 py-0.5 text-xs">
      <AlertCircle className="h-3 w-3 shrink-0 text-destructive mt-0.5" />
      <span className="text-destructive">{item.content}</span>
    </div>
  );
}

// ─── Shared ──────────────────────────────────────────────────────────────
