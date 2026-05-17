import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

const longRepoUrl =
  "https://github.com/multica-ai/a-very-long-repository-name-that-needs-a-tooltip";

const mocks = vi.hoisted(() => ({
  createProject: vi.fn(),
  createRepository: vi.fn(),
  setProjectRepositories: vi.fn(),
  clearDraft: vi.fn(),
  setDraft: vi.fn(),
  routerPush: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    if (options.queryKey?.includes("repositories")) {
      return {
        data: [
          {
            id: "repo-1",
            workspace_id: "workspace-1",
            name: "",
            source_state: "remote_git",
            remote_url: longRepoUrl,
            remote_key: "github.com/multica-ai/a-very-long-repository-name-that-needs-a-tooltip",
            default_branch: null,
            lead_agent_id: null,
            created_by: "user-1",
            created_by_agent_id: null,
            status: "ready",
            metadata: {},
            created_at: "2026-05-17T00:00:00Z",
            updated_at: "2026-05-17T00:00:00Z",
          },
        ],
      };
    }
    if (options.queryKey?.includes("agents")) {
      return {
        data: [
          {
            id: "agent-1",
            name: "Builder",
            archived_at: null,
            runtime_id: "runtime-1",
          },
        ],
      };
    }
    return { data: [] };
  },
}));

vi.mock("@multica/core/projects/mutations", () => ({
  useCreateProject: () => ({ mutateAsync: mocks.createProject }),
}));

vi.mock("@multica/core/projects", () => ({
  useProjectDraftStore: (selector: (state: unknown) => unknown) =>
    selector({
      draft: {
        title: "",
        description: "",
        status: "planned",
        priority: "medium",
        leadType: undefined,
        leadId: undefined,
        icon: undefined,
      },
      setDraft: mocks.setDraft,
      clearDraft: mocks.clearDraft,
    }),
}));

vi.mock("@multica/core/repositories", () => ({
  repositoryListOptions: () => ({ queryKey: ["repositories", "workspace-1", "list"] }),
  useCreateRepository: () => ({ mutateAsync: mocks.createRepository }),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    setProjectRepositories: mocks.setProjectRepositories,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({
    id: "workspace-1",
    name: "Test Workspace",
    slug: "test-workspace",
  }),
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/test-workspace/projects/${id}`,
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: vi.fn() }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: vi.fn() }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (_type: string, id: string) => id }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: mocks.routerPush }),
}));

vi.mock("../editor", () => {
  const ContentEditor = React.forwardRef<{ getMarkdown: () => string }, { placeholder?: string }>(
    ({ placeholder }, ref) => {
      React.useImperativeHandle(ref, () => ({ getMarkdown: () => "" }), []);
      return <textarea placeholder={placeholder} />;
    },
  );
  ContentEditor.displayName = "ContentEditor";

  return {
    ContentEditor,
    TitleEditor: ({
      placeholder,
      onChange,
    }: {
      placeholder?: string;
      onChange?: (value: string) => void;
    }) => <input placeholder={placeholder} onChange={(e) => onChange?.(e.target.value)} />,
  };
});

vi.mock("../issues/components/priority-icon", () => ({
  PriorityIcon: () => <span data-testid="priority-icon" />,
}));

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
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
    className,
    disabled,
    onClick,
    type = "button",
  }: {
    children: React.ReactNode;
    className?: string;
    disabled?: boolean;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
  }) => (
    <button type={type} className={className} disabled={disabled} onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("@multica/ui/components/common/emoji-picker", () => ({
  EmojiPicker: () => null,
}));

vi.mock("@multica/ui/lib/utils", () => ({
  cn: (...values: Array<string | false | null | undefined>) =>
    values.filter(Boolean).join(" "),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { CreateProjectModal } from "./create-project";

describe("CreateProjectModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.createProject.mockResolvedValue({ id: "project-1" });
    mocks.createRepository.mockResolvedValue({ id: "repo-agent-managed" });
    mocks.setProjectRepositories.mockResolvedValue({ repositories: [], total: 0 });
  });

  it("exposes full repository URLs in the repository picker", () => {
    render(<CreateProjectModal onClose={vi.fn()} />);

    const tooltipText = `${longRepoUrl} · ${longRepoUrl}`;
    expect(screen.getByTitle(tooltipText)).toHaveTextContent(longRepoUrl);
    expect(screen.getByRole("tooltip", { name: tooltipText })).toBeInTheDocument();
  });

  it("creates and attaches an agent-managed repository when an agent-led project has no selected repository", async () => {
    const onClose = vi.fn();
    const { container } = render(<CreateProjectModal onClose={onClose} />);

    fireEvent.change(screen.getAllByRole("textbox")[0]!, {
      target: { value: "Build from scratch" },
    });
    fireEvent.click(screen.getByText("Builder"));
    const submit = container.querySelector("button.shrink-0");
    if (!(submit instanceof HTMLButtonElement)) {
      throw new Error("submit button not found");
    }
    await waitFor(() => expect(submit).not.toBeDisabled());
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mocks.createProject).toHaveBeenCalledWith({
        title: "Build from scratch",
        description: undefined,
        icon: undefined,
        status: "planned",
        priority: "medium",
        lead_type: "agent",
        lead_id: "agent-1",
      });
      expect(mocks.createRepository).toHaveBeenCalledWith({
        name: "Build from scratch",
        source_state: "agent_managed",
        lead_agent_id: "agent-1",
      });
    });
    expect(mocks.setProjectRepositories).toHaveBeenCalledWith("project-1", {
      repositories: [
        {
          repository_id: "repo-agent-managed",
          role: "primary",
          position: 0,
        },
      ],
    });
    expect(onClose).toHaveBeenCalled();
    expect(mocks.routerPush).toHaveBeenCalledWith("/test-workspace/projects/project-1");
  });
});
