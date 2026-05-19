import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WorkflowRun } from "@multica/core/types";
import enCommon from "../../locales/en/common.json";
import enWorkflows from "../../locales/en/workflows.json";
import { WorkflowRunViewer } from "./workflow-run-viewer";

const completedRun: WorkflowRun = {
  id: "run-1",
  workspace_id: "ws-1",
  agent_task_queue_id: "task-1",
  issue_id: "issue-1",
  trigger_type: "assignment",
  snapshot: { workflow_name: "Trellis task" },
  status: "completed",
  completed_at: "2026-05-19T08:00:00Z",
  created_at: "2026-05-19T07:00:00Z",
  updated_at: "2026-05-19T08:00:00Z",
};

const completedRunDetail: WorkflowRun = {
  ...completedRun,
  steps: [
    {
      id: "step-1",
      workflow_run_id: "run-1",
      step_definition_id: "context",
      title: "Context first",
      order_index: 1,
      required: true,
      status: "ready",
      execution_kind: "agent",
      attempt: 1,
      created_at: "2026-05-19T07:00:00Z",
      updated_at: "2026-05-19T07:01:00Z",
    },
    {
      id: "step-2",
      workflow_run_id: "run-1",
      step_definition_id: "implement",
      title: "Plan and implement",
      order_index: 2,
      required: true,
      status: "pending",
      execution_kind: "agent",
      attempt: 1,
      created_at: "2026-05-19T07:00:00Z",
      updated_at: "2026-05-19T07:01:00Z",
    },
  ],
  artifacts: [],
  reviews: [],
  quality_gate_results: [],
};

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock("@multica/core/api", () => ({
  api: {
    approveWorkflowReview: vi.fn(),
    cancelWorkflowRun: vi.fn(),
    completeManualWorkflowStepRun: vi.fn(),
    rejectWorkflowReview: vi.fn(),
    rerunWorkflowRun: vi.fn(),
    retryWorkflowStepRun: vi.fn(),
  },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/workflows", () => ({
  workflowKeys: { all: (workspaceId: string) => ["workflows", workspaceId] },
  workflowRunListOptions: (_workspaceId: string, filter: unknown) => ({
    queryKey: ["workflows", "ws-1", "runs", filter],
  }),
  workflowRunDetailOptions: (_workspaceId: string, id: string | null | undefined) => ({
    queryKey: ["workflows", "ws-1", "run", id],
    enabled: !!id,
  }),
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
  useQuery: ({ queryKey }: { queryKey: readonly unknown[] }) => {
    if (queryKey[2] === "runs") return { data: [completedRun] };
    if (queryKey[2] === "run") return { data: completedRunDetail };
    return { data: undefined };
  },
}));

const resources = {
  en: {
    common: enCommon,
    workflows: enWorkflows,
  },
};

describe("WorkflowRunViewer", () => {
  it("does not display stale pending steps once the run is completed", () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowRunViewer issueId="issue-1" />
      </I18nProvider>,
    );

    expect(screen.getByText("2/2 steps")).toBeTruthy();
    expect(screen.getByText("100%")).toBeTruthy();
    expect(screen.queryByText("pending")).toBeNull();
    expect(screen.queryByText("ready")).toBeNull();
    expect(screen.queryByText("Artifacts")).toBeNull();
    expect(screen.queryByText("Reviews")).toBeNull();
    expect(screen.queryByText("Quality")).toBeNull();
  });
});
