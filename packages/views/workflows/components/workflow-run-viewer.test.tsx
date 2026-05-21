import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
      snapshot: { input_requests: { allowed: true, max_rounds: 3 } },
      created_at: "2026-05-19T07:00:00Z",
      updated_at: "2026-05-19T07:01:00Z",
    },
  ],
  artifacts: [],
  reviews: [],
  quality_gate_results: [],
};

let currentRunDetail: WorkflowRun = completedRunDetail;

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
    if (queryKey[2] === "run") return { data: currentRunDetail };
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
  beforeEach(() => {
    currentRunDetail = completedRunDetail;
  });

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
    expect(screen.getByText("Can ask")).toBeTruthy();
    expect(screen.queryByText("Artifacts")).toBeNull();
    expect(screen.queryByText("Reviews")).toBeNull();
    expect(screen.queryByText("Quality")).toBeNull();
  });

  it("renders saved workflow artifact content for review", () => {
    currentRunDetail = {
      ...completedRunDetail,
      artifacts: [
        {
          id: "artifact-1",
          workflow_run_id: "run-1",
          workflow_step_run_id: "step-2",
          logical_name: "implementation-plan",
          version: 1,
          content_kind: "markdown",
          content_text: "# Plan\n\nReview the first plan before implementation.",
          producer_type: "agent",
          producer_id: "agent-1",
          created_at: "2026-05-19T07:30:00Z",
        },
        {
          id: "artifact-2",
          workflow_run_id: "run-1",
          workflow_step_run_id: "step-2",
          logical_name: "implementation-plan",
          version: 2,
          content_kind: "markdown",
          content_text: "# Plan\n\nReview this revised plan before implementation.",
          producer_type: "agent",
          producer_id: "agent-1",
          created_at: "2026-05-19T07:30:00Z",
        },
      ],
    };

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowRunViewer issueId="issue-1" />
      </I18nProvider>,
    );

    expect(screen.getByText("implementation-plan")).toBeTruthy();
    expect(screen.queryByText("Review this revised plan before implementation.")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Preview implementation-plan" }));

    expect(screen.getByText("Review this revised plan before implementation.")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Open implementation-plan" }));

    expect(screen.getByText("v1 to v2")).toBeTruthy();
    expect(screen.getByText("Review the first plan before implementation.")).toBeTruthy();
  });

  it("renders an activity card with an embedded comment surface", () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowRunViewer
          issueId="issue-1"
          variant="activity"
          commentComposer={<div>Workflow comment composer</div>}
        />
      </I18nProvider>,
    );

    expect(screen.getByTestId("workflow-run-activity")).toBeTruthy();
    expect(screen.getByText("Workflow run")).toBeTruthy();
    expect(screen.getByText("Workflow comment composer")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /Workflow run/i }));

    expect(screen.queryByText("Workflow comment composer")).toBeNull();
  });
});
