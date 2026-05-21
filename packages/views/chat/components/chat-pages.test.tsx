import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { ChatIssueProposal, ChatIssueProposalItem } from "@multica/core/types";
import enChat from "../../locales/en/chat.json";
import { ProposedIssuesColumn } from "./chat-pages";

const resources = {
  en: {
    chat: enChat,
  },
};

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

function renderColumn() {
  return render(
    <I18nProvider locale="en" resources={resources}>
      <ProposedIssuesColumn
        items={[proposalItem]}
        selectedIds={new Set([proposalItem.item.id])}
        isApproving={false}
        onSelectItem={vi.fn()}
        onApproveSelected={vi.fn()}
      />
    </I18nProvider>,
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
});
