import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import enCommon from "../../locales/en/common.json";
import enWorkflows from "../../locales/en/workflows.json";
import { WorkflowsPage } from "./workflows-page";

const workflow = {
  id: "workflow-1",
  workspace_id: "ws-1",
  name: "Trellis task",
  description: "Trellis-oriented task workflow",
  origin: "system_seeded",
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
      steps: [],
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

  it("keeps the steps tab as a height-constrained scroll region", async () => {
    const user = userEvent.setup();

    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkflowsPage />
      </I18nProvider>,
    );

    await screen.findAllByText("Trellis task");
    await user.click(screen.getByRole("tab", { name: /steps/i }));

    const stepsHeading = await screen.findByText(enWorkflows.steps.title);
    const stepsPanel = stepsHeading.closest("[data-slot='tabs-content']");

    expect(stepsPanel?.className).toContain("h-full");
    expect(stepsPanel?.className).toContain("min-h-0");
    expect(stepsPanel?.className).toContain("overflow-y-auto");
  });
});
