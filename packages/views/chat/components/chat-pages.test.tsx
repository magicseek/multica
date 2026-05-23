import { useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { chatKeys } from "@multica/core/chat/queries";
import type { ChatIssueProposal, ChatIssueProposalItem, ChatPlanRun } from "@multica/core/types";
import enChat from "../../locales/en/chat.json";
import { ChatComposerPlanControls, ProposedIssuesColumn, buildChatPlanSendVariables } from "./chat-pages";
import { ChatMessageList } from "./chat-message-list";

const resources = {
  en: {
    chat: enChat,
  },
};

Object.defineProperty(HTMLElement.prototype, "scrollTo", {
  configurable: true,
  value: vi.fn(),
});

const proposalItem: {
  proposal: ChatIssueProposal;
  item: ChatIssueProposalItem;
} = {
  proposal: {
    id: "proposal-1",
    workspace_id: "ws-1",
    chat_session_id: "session-1",
    source_chat_message_id: "message-1",
    source_task_id: null,
    proposer_agent_id: "agent-1",
    source_plan_run_id: null,
    title: "BTC ETH SOL trend proposal",
    summary: "Create the market-analysis work items proposed by the agent.",
    status: "pending",
    created_at: "2026-05-20T00:00:00Z",
    updated_at: "2026-05-20T00:00:00Z",
    items: [],
  },
  item: {
    id: "item-1",
    proposal_id: "proposal-1",
    position: 1,
    title: "Define trend framework",
    description: "Document the analysis scope, time horizons, and success criteria.",
    priority: "high",
    labels: ["crypto", "analysis"],
    assignee_type: null,
    assignee_id: null,
    status: "pending",
    issue_id: null,
    approved_snapshot: null,
    created_at: "2026-05-20T00:00:00Z",
    updated_at: "2026-05-20T00:00:00Z",
  },
};

const planRun: ChatPlanRun = {
  id: "plan-1",
  workspace_id: "ws-1",
  chat_session_id: "session-1",
  creator_user_id: "user-1",
  actor_type: "squad",
  actor_id: "squad-1",
  lead_agent_id: "agent-1",
  plan_engine: "grill_with_docs",
  engine_version: "v1",
  status: "ready_for_approval",
  initial_message_id: "message-0",
  latest_message_id: "message-1",
  summary: {
    confirmed_requirements: ["Use React Query as the source of truth"],
    rejected_options: ["Do not store plan state in Zustand"],
    consensus_notes: ["Split UI work from backend routing"],
    open_questions: ["Whether summaries become editable later"],
  },
  created_at: "2026-05-23T00:00:00Z",
  updated_at: "2026-05-23T00:00:00Z",
};

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderWithProviders(node: ReactNode, qc = createTestQueryClient()) {
  return render(
    <QueryClientProvider client={qc}>
      <I18nProvider locale="en" resources={resources}>
        {node}
      </I18nProvider>
    </QueryClientProvider>,
  );
}

function renderColumn(overrides: {
  item?: typeof proposalItem;
  planRuns?: ChatPlanRun[];
} = {}) {
  const item = overrides.item ?? proposalItem;
  return renderWithProviders(
    <ProposedIssuesColumn
      items={[item]}
      planRuns={overrides.planRuns}
      selectedIds={new Set([item.item.id])}
      isApproving={false}
      onSelectItem={vi.fn()}
      onApproveSelected={vi.fn()}
    />,
  );
}

function PlanControlsHarness() {
  const [planMode, setPlanMode] = useState(false);
  const [engine, setEngine] = useState("grill_with_docs");
  return (
    <ChatComposerPlanControls
      planMode={planMode}
      onPlanModeChange={setPlanMode}
      engines={[
        { id: "grill_with_docs", label: "Grill with docs", description: "Ask focused questions.", version: "v1" },
        { id: "brainstorming", label: "Brainstorming", description: "Explore options first.", version: "v1" },
      ]}
      selectedEngineId={engine}
      onEngineChange={setEngine}
      actorPicker={<span>Agent picker</span>}
    />
  );
}

describe("ProposedIssuesColumn", () => {
  it("matches the board column heading scale", () => {
    renderColumn();

    const heading = screen.getByText("Proposed").closest("span");
    expect(heading).toHaveClass("text-xs");
    expect(heading).toHaveClass("font-semibold");
  });

  it("previews a proposed issue without using the selection checkbox", async () => {
    const user = userEvent.setup();
    renderColumn();

    await user.click(
      screen.getByRole("checkbox", {
        name: /select proposed issue: define trend framework/i,
      }),
    );
    expect(screen.queryByText("Proposal context")).not.toBeInTheDocument();

    await user.click(screen.getByText("Define trend framework"));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getAllByText("Document the analysis scope, time horizons, and success criteria.")).toHaveLength(2);
    expect(screen.getByText("Proposal context")).toBeInTheDocument();
  });

  it("shows a plan summary near linked proposed issues", () => {
    renderColumn({
      item: {
        proposal: { ...proposalItem.proposal, source_plan_run_id: "plan-1" },
        item: proposalItem.item,
      },
      planRuns: [planRun],
    });

    expect(screen.getByText("Plan summary")).toBeInTheDocument();
    expect(screen.getByText(/Use React Query as the source of truth/)).toBeInTheDocument();
  });
});

describe("ChatComposerPlanControls", () => {
  it("exposes Plan mode and the default engine selector", async () => {
    const user = userEvent.setup();
    renderWithProviders(<PlanControlsHarness />);

    await user.click(screen.getByRole("button", { name: /plan/i }));

    expect(screen.getByRole("button", { name: /grill with docs/i })).toBeInTheDocument();
    expect(screen.getByText("Agent picker")).toBeInTheDocument();
  });

  it("shows active server plan state and active sends continue the plan run", () => {
    renderWithProviders(
      <ChatComposerPlanControls
        planMode={false}
        onPlanModeChange={vi.fn()}
        engines={[{ id: "grill_with_docs", label: "Grill with docs", description: "", version: "v1" }]}
        selectedEngineId="grill_with_docs"
        onEngineChange={vi.fn()}
        activePlanRun={planRun}
        onCancelActivePlan={vi.fn()}
      />,
    );

    expect(screen.getByText("Planning")).toBeInTheDocument();
    expect(screen.getByText("Ready for approval")).toBeInTheDocument();
    expect(buildChatPlanSendVariables({
      content: "Yes, continue",
      activePlanRun: planRun,
      planMode: false,
      selectedEngineId: "brainstorming",
      selectedActor: null,
    })).toEqual({
      content: "Yes, continue",
      planRunId: "plan-1",
    });
  });
});

describe("ChatMessageList plan consultations", () => {
  it("renders sender identity, directed recipients, and routing warnings", () => {
    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[
            {
              id: "message-directed",
              chat_session_id: "session-1",
              role: "assistant",
              content: "I need review from the squad.",
              task_id: null,
              author_type: "agent",
              author_agent_id: "agent-lead",
              sender: { type: "agent", id: "agent-lead" },
              recipients: [
                {
                  id: "edge-1",
                  message_id: "message-directed",
                  recipient_type: "squad",
                  recipient_id: "squad-1",
                  resolved_agent_id: "agent-helper",
                  source: "explicit_mention",
                  status: "routed",
                  task_id: "task-helper",
                  warning_code: "",
                  warning_message: "",
                  created_at: "2026-05-23T00:00:00Z",
                  updated_at: "2026-05-23T00:00:00Z",
                },
                {
                  id: "edge-2",
                  message_id: "message-directed",
                  recipient_type: "agent",
                  recipient_id: "agent-outside",
                  resolved_agent_id: "agent-outside",
                  source: "explicit_mention",
                  status: "blocked",
                  task_id: null,
                  warning_code: "out_of_scope",
                  warning_message: "Lead can only consult agents in the selected squad.",
                  created_at: "2026-05-23T00:00:00Z",
                  updated_at: "2026-05-23T00:00:00Z",
                },
              ],
              routing_warnings: [
                {
                  recipient_id: "edge-2",
                  code: "out_of_scope",
                  message: "Lead can only consult agents in the selected squad.",
                },
              ],
              created_at: "2026-05-23T00:00:00Z",
            },
          ]}
          pendingTask={null}
          availability={undefined}
          agents={[
            { id: "agent-lead", name: "Lead Agent" },
            { id: "agent-helper", name: "Helper Agent" },
          ]}
          squads={[{ id: "squad-1", name: "Review Squad", avatar_url: null }]}
        />
      </div>,
    );

    expect(screen.getByText("Lead Agent")).toBeInTheDocument();
    expect(screen.getByText("Review Squad")).toBeInTheDocument();
    expect(screen.getByText("Lead can only consult agents in the selected squad.")).toBeInTheDocument();
    expect(screen.getByText(/review from the squad/i)).toBeInTheDocument();
  });

  it("renders human messages with visible sender identity", () => {
    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[
            {
              id: "message-human",
              chat_session_id: "session-1",
              role: "user",
              content: "Please turn this into issues.",
              task_id: null,
              author_type: "member",
              author_member_id: "user-1",
              sender: { type: "member", id: "user-1" },
              created_at: "2026-05-23T00:00:00Z",
            },
          ]}
          pendingTask={null}
          availability={undefined}
        />
      </div>,
    );

    expect(screen.getByText("You")).toBeInTheDocument();
    expect(screen.getByText(/turn this into issues/i)).toBeInTheDocument();
  });

  it("renders helper-authored consultation messages with helper identity", () => {
    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[
            {
              id: "message-helper",
              chat_session_id: "session-1",
              role: "assistant",
              content: "The API contract should stay tolerant of server drift.",
              task_id: null,
              author_type: "agent",
              author_agent_id: "agent-helper",
              plan_run_id: "plan-1",
              consultation_id: "consult-1",
              created_at: "2026-05-23T00:00:00Z",
            },
          ]}
          pendingTask={null}
          availability={undefined}
          agents={[{ id: "agent-helper", name: "API Reviewer" }]}
        />
      </div>,
    );

    expect(screen.getByText("API Reviewer consulted")).toBeInTheDocument();
    expect(screen.getByText(/server drift/)).toBeInTheDocument();
  });

  it("renders final plan and proposal addons inside the assistant row", () => {
    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[
            {
              id: "message-final",
              chat_session_id: "session-1",
              role: "assistant",
              content: "Here is the final proposal.",
              task_id: null,
              author_type: "agent",
              author_agent_id: "agent-atlas",
              created_at: "2026-05-23T00:00:00Z",
            },
          ]}
          pendingTask={null}
          availability={undefined}
          agents={[{ id: "agent-atlas", name: "Atlas" }]}
          renderAfterMessage={(message) =>
            message.id === "message-final" ? (
              <div>Final plan and proposed issues</div>
            ) : null
          }
        />
      </div>,
    );

    const messageRow = screen.getByTestId("chat-message-row");
    expect(within(messageRow).getByText("Atlas")).toBeInTheDocument();
    expect(within(messageRow).getByText("Final plan and proposed issues")).toBeInTheDocument();
  });

  it("renders live pending task output with the running agent identity", () => {
    const qc = createTestQueryClient();
    qc.setQueryData(chatKeys.taskMessages("task-live"), [
      {
        task_id: "task-live",
        issue_id: "",
        seq: 1,
        type: "text",
        content: "I am drafting the plan now.",
      },
    ]);

    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[]}
          pendingTask={{
            task_id: "task-live",
            status: "running",
            agent_id: "agent-atlas",
            created_at: "2026-05-23T00:00:00Z",
          }}
          availability={undefined}
          agents={[{ id: "agent-atlas", name: "Atlas" }]}
        />
      </div>,
      qc,
    );

    expect(screen.getByText("Atlas")).toBeInTheDocument();
    expect(screen.getByText(/drafting the plan/)).toBeInTheDocument();
  });

  it("renders pending task status inside the live agent row before output streams", () => {
    const qc = createTestQueryClient();
    qc.setQueryData(chatKeys.taskMessages("task-live"), []);

    renderWithProviders(
      <div className="flex h-80">
        <ChatMessageList
          messages={[]}
          pendingTask={{
            task_id: "task-live",
            status: "running",
            agent_id: "agent-atlas",
            created_at: new Date().toISOString(),
          }}
          availability={undefined}
          agents={[{ id: "agent-atlas", name: "Atlas" }]}
        />
      </div>,
      qc,
    );

    const liveRow = screen.getByTestId("chat-live-task-row");
    const liveBody = within(liveRow).getByTestId("chat-live-task-body");
    expect(within(liveRow).getByText("Atlas")).toBeInTheDocument();
    expect(liveBody).toHaveClass("space-y-1.5");
    expect(within(liveBody).getByText(/Thinking/)).toBeInTheDocument();
  });
});
