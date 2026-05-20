import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import enCommon from "../../locales/en/common.json";
import enWorkflows from "../../locales/en/workflows.json";
import { buildSchemaLineDiff, WorkflowsPage } from "./workflows-page";

const workflow = {
  id: "workflow-1",
  workspace_id: "ws-1",
  name: "Trellis task",
  description: "Trellis-oriented task workflow",
  origin: "user",
  status: "active",
  current_revision_id: "revision-1",
  created_at: "2026-05-18T00:00:00Z",
  updated_at: "2026-05-18T00:00:00Z",
  current_revision: {
    id: "revision-1",
    workflow_definition_id: "workflow-1",
    schema: {
      name: "Trellis task",
      description: "Trellis-oriented task workflow",
      applicability: ["assignment"],
      source: {
        format: "markdown",
        body_template: "## Trellis Task Protocol",
      },
      steps: [
        {
          id: "context",
          title: "Read context",
          order: 1,
          description: "Read the issue and comments.",
          output: { description: "Context is understood." },
        },
        {
          id: "implement",
          title: "Implement",
          order: 2,
          depends_on: ["context"],
          description: "Make the requested change.",
        },
      ],
    },
    created_at: "2026-05-18T00:00:00Z",
  },
  draft_revision: {
    id: "revision-draft",
    workflow_definition_id: "workflow-1",
    schema: {
      name: "Trellis task",
      description: "Updated Trellis-oriented task workflow",
      applicability: ["assignment"],
      source: {
        format: "markdown",
        body_template: "## Trellis Task Protocol",
      },
      steps: [
        {
          id: "context",
          title: "Read context",
          order: 1,
          description: "Read the issue and comments.",
          output: { description: "Context is understood." },
        },
        {
          id: "implement",
          title: "Implement",
          order: 2,
          depends_on: ["context"],
          description: "Make the requested change.",
        },
      ],
    },
    created_at: "2026-05-18T00:00:00Z",
  },
} as const;

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock("@multica/core/api", () => ({
  api: {
    previewWorkflow: vi.fn().mockResolvedValue({ rendered_markdown: "Preview", warnings: [] }),
  },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/projects/queries", () => ({ projectListOptions: () => ({ queryKey: ["projects"] }) }));
vi.mock("@multica/core/projects/mutations", () => ({
  useUpdateProject: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@multica/core/workflows", () => ({
  workflowListOptions: (_workspaceId: string, filters?: { applicability?: string }) => ({
    queryKey: filters?.applicability ? ["workflows", filters.applicability] : ["workflows"],
  }),
  useCreateWorkflow: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteWorkflow: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteWorkflowDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useForkWorkflow: () => ({ mutateAsync: vi.fn(), isPending: false }),
  usePublishWorkflow: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateWorkflowDraft: () => ({
    mutateAsync: vi.fn().mockResolvedValue({ validation: { publishable: true, issues: [] } }),
    isPending: false,
  }),
}));
vi.mock("@tanstack/react-query", () => ({
  useQuery: ({ queryKey }: { queryKey: readonly unknown[] }) => {
    if (queryKey[0] === "projects") return { data: [], isLoading: false, error: null };
    return { data: [workflow], isLoading: false, error: null };
  },
}));

const resources = {
  en: {
    common: enCommon,
    workflows: enWorkflows,
  },
};

describe("WorkflowsPage", () => {
  it("counts schema line changes for side-by-side review diffs", () => {
    const diff = buildSchemaLineDiff("a\nb\nc", "a\nB\nc\nd");

    expect(diff.added).toBe(1);
    expect(diff.removed).toBe(0);
    expect(diff.modified).toBe(1);
    expect(diff.changed).toBe(2);
    expect(diff.rows.map((row) => row.kind)).toEqual([
      "unchanged",
      "modified",
      "unchanged",
      "added",
    ]);

    const shifted = buildSchemaLineDiff("a\nc", "a\nb\nc");
    expect(shifted.rows[2]).toMatchObject({
      kind: "unchanged",
      leftLine: 2,
      rightLine: 3,
    });
  });

  it("keeps editor source tabs horizontal when rendered inside vertical settings tabs", async () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <Tabs orientation="vertical" defaultValue="workflows">
          <TabsList>
            <TabsTrigger value="workflows">Workflows</TabsTrigger>
          </TabsList>
          <TabsContent value="workflows">
            <WorkflowsPage />
          </TabsContent>
        </Tabs>
      </I18nProvider>,
    );

    expect(await screen.findAllByText("Trellis task")).not.toHaveLength(0);
    expect(screen.getByTestId("workflow-editor-tabs")).toHaveClass("flex-col");
    expect(screen.getByTestId("workflow-editor-tabs-list").className).toContain("!flex-row");
    expect(screen.getByRole("tab", { name: /schema/i })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /source/i })).not.toBeInTheDocument();
  });

  it("keeps review schema diff vertically scrollable with wrapped lines", async () => {
    const user = userEvent.setup();

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowsPage />
      </I18nProvider>,
    );

    await screen.findAllByText("Trellis task");
    await user.click(
      screen.getByRole("button", { name: enWorkflows.review.open }),
    );
    await user.click(
      screen.getByRole("tab", { name: enWorkflows.review.schema_tab }),
    );

    expect(screen.getByTestId("schema-diff-scroll")).toHaveClass(
      "overflow-y-auto",
      "overflow-x-hidden",
    );
    for (const code of screen.getAllByTestId("schema-diff-code")) {
      expect(code).toHaveClass("whitespace-pre-wrap", "break-words");
      expect(code.className).toContain("[overflow-wrap:anywhere]");
    }
  });

  it("renders a collapsed step outline with a focused inspector", async () => {
    const user = userEvent.setup();

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowsPage />
      </I18nProvider>,
    );

    await screen.findAllByText("Trellis task");
    await user.click(screen.getByRole("tab", { name: /steps/i }));

    expect(await screen.findAllByText("Read context")).not.toHaveLength(0);
    expect(screen.getByText("Make the requested change.")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Basics" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Instructions" })).toBeInTheDocument();
    expect(screen.getByText("Context is understood.")).toBeInTheDocument();

    const stepsHeading = screen.getByText(enWorkflows.steps.title);
    const stepsPanel = stepsHeading.closest("[data-slot='tabs-content']");

    expect(stepsPanel?.className).toContain("h-full");
    expect(stepsPanel?.className).toContain("min-h-0");
    expect(stepsPanel?.className).toContain("overflow-hidden");
  });

  it("preserves spaces while editing step fields", async () => {
    const user = userEvent.setup();

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowsPage />
      </I18nProvider>,
    );

    await screen.findAllByText("Trellis task");
    await user.click(screen.getByRole("tab", { name: /steps/i }));

    const editTitle = screen.getAllByLabelText(enWorkflows.steps.edit_title)[0];
    expect(editTitle).toBeDefined();
    await user.click(editTitle!);
    const titleInput = screen.getByDisplayValue("Read context");
    await user.clear(titleInput);
    await user.type(titleInput, "Read source");
    expect(titleInput).toHaveValue("Read source");

    const doneWhen = screen.getByLabelText(enWorkflows.steps.output_label);
    await user.clear(doneWhen);
    await user.type(doneWhen, "Alpha beta");
    expect(doneWhen).toHaveValue("Alpha beta");
  });

  it("uses a selectable dependency list instead of a raw dependency input", async () => {
    const user = userEvent.setup();

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowsPage />
      </I18nProvider>,
    );

    await screen.findAllByText("Trellis task");
    await user.click(screen.getByRole("tab", { name: /steps/i }));
    await user.click(screen.getByText("Implement"));
    await user.click(screen.getByRole("tab", { name: "Inputs" }));

    expect(screen.queryByDisplayValue("context")).not.toBeInTheDocument();

    const dependencyTrigger = screen.getAllByRole("button", {
      name: /Read context/i,
    }).at(-1);
    expect(dependencyTrigger).toBeDefined();
    await user.click(dependencyTrigger!);
    expect(screen.getByRole("checkbox", { checked: true })).toBeInTheDocument();
  });
});
