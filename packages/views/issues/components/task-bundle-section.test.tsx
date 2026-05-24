// @vitest-environment jsdom

import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent, Issue, Squad } from "@multica/core/types";
import enCommon from "../../locales/en/common.json";
import enIssues from "../../locales/en/issues.json";

const TEST_RESOURCES = { en: { common: enCommon, issues: enIssues } };

const mockApi = vi.hoisted(() => ({
  listTaskBundlesByIssue: vi.fn().mockResolvedValue([]),
  createTaskBundle: vi.fn().mockResolvedValue({
    id: "bundle-1",
    items: [],
  }),
  rerunTaskBundle: vi.fn().mockResolvedValue({
    id: "bundle-rerun",
    items: [],
  }),
}));

vi.mock("@multica/core/api", () => ({
  api: mockApi,
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { TaskBundleSection } from "./task-bundle-section";

function makeIssue(overrides: Partial<Issue>): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "First task",
    description: "",
    status: "todo",
    priority: "none",
    assignee_type: "agent",
    assignee_id: "agent-1",
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2026-05-24T00:00:00Z",
    updated_at: "2026-05-24T00:00:00Z",
    ...overrides,
  };
}

function makeAgent(overrides: Partial<Agent>): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    name: "Codex",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_env: {},
    custom_args: [],
    custom_env_redacted: false,
    visibility: "private",
    status: "idle",
    max_concurrent_tasks: 1,
    model: "",
    owner_id: "user-1",
    skills: [],
    created_at: "2026-05-24T00:00:00Z",
    updated_at: "2026-05-24T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function renderSection({
  issue = makeIssue({}),
  issues = [],
  agents = [],
  squads = [],
}: {
  issue?: Issue;
  issues?: Issue[];
  agents?: Agent[];
  squads?: Squad[];
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <TaskBundleSection
          workspaceId="ws-1"
          issue={issue}
          issues={issues}
          agents={agents}
          squads={squads}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

describe("TaskBundleSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.listTaskBundlesByIssue.mockResolvedValue([]);
    mockApi.createTaskBundle.mockResolvedValue({ id: "bundle-1", items: [] });
    mockApi.rerunTaskBundle.mockResolvedValue({ id: "bundle-rerun", items: [] });
  });

  it("stays hidden until at least one agent has request-efficient mode enabled", () => {
    renderSection({
      agents: [makeAgent({ request_efficient_enabled: false })],
    });

    expect(screen.queryByText("Task bundle")).not.toBeInTheDocument();
    expect(mockApi.listTaskBundlesByIssue).not.toHaveBeenCalled();
  });

  it("creates a bundle for selected eligible issues", async () => {
    const issueA = makeIssue({
      id: "issue-1",
      identifier: "MUL-1",
      title: "First task",
      assignee_id: "agent-1",
    });
    const issueB = makeIssue({
      id: "issue-2",
      identifier: "MUL-2",
      title: "Second task",
      assignee_id: "agent-1",
    });
    renderSection({
      issue: issueA,
      issues: [issueB],
      agents: [makeAgent({ request_efficient_enabled: true })],
    });

    fireEvent.click(await screen.findByText("Task bundle"));

    await waitFor(() => {
      expect(screen.getByText("First task")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Second task"));
    fireEvent.click(screen.getByRole("button", { name: "Start bundle" }));

    await waitFor(() => {
      expect(mockApi.createTaskBundle).toHaveBeenCalledWith({
        agent_id: "agent-1",
        issue_ids: ["issue-1", "issue-2"],
      });
    });
  });

  it("shows completed bundle history without exposing create controls or IDs", async () => {
    const bundleId = "beb14ee0-cfa1-44b1-bf6d-04dc5acf0000";
    const issueA = makeIssue({
      id: "issue-1",
      identifier: "MUL-1",
      title: "First task",
      status: "in_review",
      assignee_id: "agent-1",
    });
    const issueB = makeIssue({
      id: "issue-2",
      identifier: "MUL-2",
      title: "Second task",
      status: "in_review",
      assignee_id: "agent-1",
    });
    mockApi.listTaskBundlesByIssue.mockResolvedValue([
      {
        id: bundleId,
        workspace_id: "ws-1",
        agent_id: "agent-1",
        runtime_id: "runtime-1",
        status: "completed",
        changeset_mode: "per_issue",
        max_items: 5,
        runtime_budget_seconds: 18_000,
        rerun_scope: [],
        created_at: "2026-05-24T00:00:00Z",
        updated_at: "2026-05-24T00:00:00Z",
        completed_at: "2026-05-24T00:30:00Z",
        items: [
          {
            id: "item-1",
            bundle_id: bundleId,
            issue_id: "issue-1",
            position: 1,
            status: "completed",
            output_namespace: "bundle/internal/item-1",
            created_at: "2026-05-24T00:00:00Z",
            updated_at: "2026-05-24T00:15:00Z",
          },
          {
            id: "item-2",
            bundle_id: bundleId,
            issue_id: "issue-2",
            position: 2,
            status: "completed",
            output_namespace: "bundle/internal/item-2",
            created_at: "2026-05-24T00:00:00Z",
            updated_at: "2026-05-24T00:30:00Z",
          },
        ],
      },
    ]);

    renderSection({
      issue: issueA,
      issues: [issueB],
      agents: [makeAgent({ request_efficient_enabled: true })],
    });

    fireEvent.click(await screen.findByText("Task bundle"));

    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("2/2")).toBeInTheDocument();
    expect(screen.getByText("MUL-1 First task")).toBeInTheDocument();
    expect(screen.getByText("MUL-2 Second task")).toBeInTheDocument();
    expect(screen.queryByText(bundleId)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Start bundle" }),
    ).not.toBeInTheDocument();
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    expect(screen.queryByText(/selected/i)).not.toBeInTheDocument();
  });
});
