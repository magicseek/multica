// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enAgents from "../../locales/en/agents.json";
import enIssues from "../../locales/en/issues.json";
import type { AgentTask } from "@multica/core/types/agent";
import type { TaskOutputMetadata } from "@multica/core/types";

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents, issues: enIssues } };

const taskOutputsRef = vi.hoisted(() => ({
  current: [] as TaskOutputMetadata[],
}));
const clipboardWriteTextMock = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", async () => {
  const actual = await vi.importActual<typeof import("@tanstack/react-query")>(
    "@tanstack/react-query",
  );
  return {
    ...actual,
    useQuery: (opts: { queryKey?: readonly unknown[] }) => {
      if (opts.queryKey?.[0] === "task-outputs") {
        return { data: taskOutputsRef.current, isLoading: false };
      }
      return { data: undefined, isLoading: false };
    },
  };
});

vi.mock("@multica/core/api", () => ({
  api: {
    getAgent: vi.fn().mockResolvedValue(null),
    listRuntimes: vi.fn().mockResolvedValue([]),
  },
}));

vi.mock("../actor-avatar", () => ({
  ActorAvatar: () => <div data-testid="actor-avatar" />,
}));

import { AgentTranscriptDialog } from "./agent-transcript-dialog";

function makeTask(): AgentTask {
  return {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "completed",
    priority: 0,
    dispatched_at: null,
    started_at: "2026-05-17T00:00:00Z",
    completed_at: "2026-05-17T00:00:42Z",
    result: null,
    error: null,
    created_at: "2026-05-17T00:00:00Z",
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  taskOutputsRef.current = [
    {
      id: "output-1",
      workspace_id: "workspace-1",
      repository_id: "repo-1",
      task_id: "task-1",
      relative_path: "docs/design.md",
      filename: "design.md",
      kind: "doc",
      size_bytes: 42,
      mime_type: "text/markdown",
      metadata: {},
      created_at: "2026-05-17T00:01:00Z",
    },
  ];
  clipboardWriteTextMock.mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText: clipboardWriteTextMock },
  });
});

describe("AgentTranscriptDialog", () => {
  it("shows task output metadata and copies the relative path", async () => {
    render(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <AgentTranscriptDialog
          open
          onOpenChange={vi.fn()}
          task={makeTask()}
          items={[]}
          agentName="Builder"
        />
      </I18nProvider>,
    );

    expect(screen.getByText("1 output")).toBeTruthy();
    expect(screen.getByText("design.md")).toBeTruthy();
    expect(screen.getByText("Doc")).toBeTruthy();
    expect(screen.getByText("42 B")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Copy relative path" }));

    await waitFor(() => {
      expect(clipboardWriteTextMock).toHaveBeenCalledWith("docs/design.md");
    });
  });

  it("shows bundle item dividers in transcript order", () => {
    render(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <AgentTranscriptDialog
          open
          onOpenChange={vi.fn()}
          task={{
            ...makeTask(),
            task_bundle: {
              id: "bundle-1",
              workspace_id: "workspace-1",
              agent_id: "agent-1",
              runtime_id: "runtime-1",
              status: "running",
              changeset_mode: "per_issue",
              max_items: 5,
              runtime_budget_seconds: 3900,
              rerun_scope: [],
              created_at: "2026-05-17T00:00:00Z",
              updated_at: "2026-05-17T00:00:00Z",
              items: [
                {
                  id: "item-1",
                  bundle_id: "bundle-1",
                  issue_id: "issue-1",
                  position: 1,
                  status: "completed",
                  output_namespace: "bundle/bundle-1/item-01-issue-1",
                  checkpoint_seq: 1,
                  created_at: "2026-05-17T00:00:00Z",
                  updated_at: "2026-05-17T00:00:00Z",
                },
                {
                  id: "item-2",
                  bundle_id: "bundle-1",
                  issue_id: "issue-2",
                  position: 2,
                  status: "in_progress",
                  output_namespace: "bundle/bundle-1/item-02-issue-2",
                  created_at: "2026-05-17T00:00:00Z",
                  updated_at: "2026-05-17T00:00:00Z",
                },
              ],
            },
          }}
          items={[
            { seq: 1, type: "text", content: "finished issue 1" },
            {
              seq: 2,
              type: "tool_use",
              tool: "exec_command",
              input: {
                command: "multica task-bundle checkpoint item-1 --status completed",
              },
            },
            {
              seq: 3,
              type: "tool_result",
              tool: "exec_command",
              output: "checkpoint ok",
            },
            { seq: 4, type: "text", content: "starting issue 2" },
          ]}
          agentName="Builder"
        />
      </I18nProvider>,
    );

    expect(screen.getByText("Bundle item 1")).toBeInTheDocument();
    expect(screen.getByText("Bundle item 2")).toBeInTheDocument();
    expect(screen.getByText("2 bundle items")).toBeInTheDocument();
    expect(screen.getAllByText("Started")).toHaveLength(2);
    expect(screen.queryByText("Done")).not.toBeInTheDocument();
    expect(screen.queryByText("Current item")).not.toBeInTheDocument();

    const text = document.body.textContent ?? "";
    expect(text.indexOf("Bundle item 2")).toBeGreaterThan(
      text.indexOf("checkpoint ok"),
    );
    expect(text.indexOf("Bundle item 2")).toBeLessThan(
      text.indexOf("starting issue 2"),
    );
  });
});
