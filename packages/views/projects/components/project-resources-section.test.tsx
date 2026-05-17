// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enProjects from "../../locales/en/projects.json";
import type { ProjectRepository, Repository } from "@multica/core/types";

const TEST_RESOURCES = { en: { common: enCommon, projects: enProjects } };

const setProjectRepositoriesMock = vi.hoisted(() => vi.fn());
const projectRepositoriesRef = vi.hoisted(() => ({
  current: [] as ProjectRepository[],
}));
const workspaceRepositoriesRef = vi.hoisted(() => ({
  current: [] as Repository[],
}));

function makeRepository(overrides: Partial<Repository>): Repository {
  return {
    id: "repo-1",
    workspace_id: "workspace-1",
    name: "Repo",
    source_state: "remote_git",
    remote_url: "https://github.com/multica-ai/repo.git",
    remote_key: "github.com/multica-ai/repo",
    default_branch: null,
    lead_agent_id: null,
    created_by: "user-1",
    created_by_agent_id: null,
    status: "ready",
    metadata: {},
    created_at: "2026-05-17T00:00:00Z",
    updated_at: "2026-05-17T00:00:00Z",
    ...overrides,
  };
}

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey ?? [];
    if (key.includes("project-repositories")) {
      return { data: projectRepositoriesRef.current };
    }
    if (key.includes("repository-list")) {
      return { data: workspaceRepositoriesRef.current };
    }
    return { data: [] };
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/projects", () => ({
  projectResourcesOptions: () => ({ queryKey: ["resources"] }),
  useDeleteProjectResource: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@multica/core/repositories", () => ({
  projectRepositoriesOptions: () => ({ queryKey: ["project-repositories"] }),
  repositoryListOptions: () => ({ queryKey: ["repository-list"] }),
  useCreateRepository: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetProjectRepositories: () => ({
    mutateAsync: setProjectRepositoriesMock,
    isPending: false,
  }),
}));

vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div role="tooltip">{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({
    children,
    onClick,
    type = "button",
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
  }) => (
    <button type={type} onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { ProjectResourcesSection } from "./project-resources-section";

describe("ProjectResourcesSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const primary = makeRepository({ id: "repo-1", name: "Primary repo" });
    const secondary = makeRepository({
      id: "repo-2",
      name: "Second repo",
      remote_url: "https://github.com/multica-ai/second.git",
      remote_key: "github.com/multica-ai/second",
    });
    workspaceRepositoriesRef.current = [primary, secondary];
    projectRepositoriesRef.current = [
      {
        project_id: "project-1",
        repository_id: "repo-1",
        role: "primary",
        position: 0,
        created_at: "2026-05-17T00:00:00Z",
        repository: primary,
      },
    ];
    setProjectRepositoriesMock.mockResolvedValue({ repositories: [], total: 0 });
  });

  it("appends a selected workspace repository to project repositories", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <ProjectResourcesSection projectId="project-1" />
      </I18nProvider>,
    );

    expect(screen.getAllByText("Primary repo").length).toBeGreaterThan(0);

    await user.click(screen.getByText("Second repo"));

    expect(setProjectRepositoriesMock).toHaveBeenCalledWith({
      repositories: [
        { repository_id: "repo-1", role: "primary", position: 0 },
        { repository_id: "repo-2", role: "secondary", position: 1 },
      ],
    });
  });
});
