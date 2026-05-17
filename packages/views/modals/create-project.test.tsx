import React from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const longRepoUrl =
  "https://github.com/multica-ai/a-very-long-repository-name-that-needs-a-tooltip";

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
    return { data: [] };
  },
}));

vi.mock("@multica/core/projects/mutations", () => ({
  useCreateProject: () => ({ mutateAsync: vi.fn() }),
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
      setDraft: vi.fn(),
      clearDraft: vi.fn(),
    }),
}));

vi.mock("@multica/core/repositories", () => ({
  repositoryListOptions: () => ({ queryKey: ["repositories", "workspace-1", "list"] }),
  useCreateRepository: () => ({ mutateAsync: vi.fn() }),
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
  useActorName: () => ({ getActorName: vi.fn() }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("../editor", () => {
  const ContentEditor = React.forwardRef<HTMLTextAreaElement, { placeholder?: string }>(
    ({ placeholder }, ref) => <textarea ref={ref} placeholder={placeholder} />,
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
    disabled,
    onClick,
    type = "button",
  }: {
    children: React.ReactNode;
    disabled?: boolean;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
  }) => (
    <button type={type} disabled={disabled} onClick={onClick}>
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
  it("exposes full repository URLs in the repository picker", () => {
    render(<CreateProjectModal onClose={vi.fn()} />);

    const tooltipText = `${longRepoUrl} · ${longRepoUrl}`;
    expect(screen.getByTitle(tooltipText)).toHaveTextContent(longRepoUrl);
    expect(screen.getByRole("tooltip", { name: tooltipText })).toBeInTheDocument();
  });
});
