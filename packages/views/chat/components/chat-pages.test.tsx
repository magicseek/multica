import { useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
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

function renderWithProviders(node: ReactNode) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
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
});
