import type { ReactNode } from "react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";
import type { Repository, RuntimeDevice } from "@multica/core/types";

const createRepositoryMock = vi.hoisted(() => vi.fn());
const updateRepositoryMock = vi.hoisted(() => vi.fn());
const archiveRepositoryMock = vi.hoisted(() => vi.fn());
const repositoriesRef = vi.hoisted(() => ({
  current: [] as Repository[],
}));
const membersRef = vi.hoisted(() => ({
  current: [{ user_id: "user-1", role: "owner" as const }],
}));
const runtimesRef = vi.hoisted(() => ({
  current: [] as RuntimeDevice[],
}));

function makeRepository(overrides: Partial<Repository> = {}): Repository {
  return {
    id: "repo-1",
    workspace_id: "workspace-1",
    name: "multica",
    source_state: "remote_git",
    remote_url: "https://github.com/multica-ai/multica",
    remote_key: "github.com/multica-ai/multica",
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

function makeRuntime(overrides: Partial<RuntimeDevice> = {}): RuntimeDevice {
  return {
    id: "runtime-1",
    workspace_id: "workspace-1",
    daemon_id: "daemon-1",
    name: "Troy Mac",
    runtime_mode: "local",
    provider: "codex",
    launch_header: "",
    status: "online",
    device_info: "macOS",
    metadata: {},
    owner_id: "user-1",
    visibility: "private",
    timezone: "Asia/Shanghai",
    last_seen_at: "2026-05-17T00:00:00Z",
    created_at: "2026-05-17T00:00:00Z",
    updated_at: "2026-05-17T00:00:00Z",
    ...overrides,
  };
}

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey ?? [];
    if (key.includes("members")) return { data: membersRef.current };
    if (key.includes("runtimes")) return { data: runtimesRef.current };
    if (key.includes("repositories")) return { data: repositoriesRef.current, isLoading: false };
    return { data: undefined, isLoading: false };
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["workspaces", "workspace-1", "members"] }),
}));

vi.mock("@multica/core/runtimes", () => ({
  runtimeListOptions: () => ({ queryKey: ["runtimes", "workspace-1", "list"] }),
}));

vi.mock("@multica/core/repositories", () => ({
  repositoryListOptions: () => ({ queryKey: ["repositories", "workspace-1", "list"] }),
  useCreateRepository: () => ({ mutateAsync: createRepositoryMock, isPending: false }),
  useUpdateRepository: () => ({ mutateAsync: updateRepositoryMock, isPending: false }),
  useArchiveRepository: () => ({ mutateAsync: archiveRepositoryMock, isPending: false }),
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (sel?: (s: { user: { id: string } }) => unknown) =>
      sel ? sel({ user: { id: "user-1" } }) : { user: { id: "user-1" } },
    { getState: () => ({ user: { id: "user-1" } }) },
  );
  return { useAuthStore };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { RepositoriesTab } from "./repositories-tab";

const TEST_RESOURCES = {
  en: { common: enCommon, settings: enSettings },
};

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

describe("RepositoriesTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    repositoriesRef.current = [makeRepository()];
    membersRef.current = [{ user_id: "user-1", role: "owner" }];
    runtimesRef.current = [makeRuntime()];
    createRepositoryMock.mockResolvedValue(makeRepository({ id: "repo-new" }));
    updateRepositoryMock.mockResolvedValue(makeRepository());
    archiveRepositoryMock.mockResolvedValue(undefined);
  });

  it("renders first-class repositories in display mode", () => {
    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.getByText("multica")).toBeTruthy();
    expect(screen.getByText("https://github.com/multica-ai/multica")).toBeTruthy();
    expect(screen.getByText("Remote Git")).toBeTruthy();
  });

  it("creates a remote Git repository from the add row", async () => {
    const user = userEvent.setup();
    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /Add remote Git/ }));
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    await user.type(inputs[0]!, "API");
    await user.type(inputs[1]!, "git@github.com:multica-ai/api.git");
    await user.click(screen.getByRole("button", { name: "Save repository" }));

    await waitFor(() => {
      expect(createRepositoryMock).toHaveBeenCalledWith({
        name: "API",
        source_state: "remote_git",
        remote_url: "git@github.com:multica-ai/api.git",
      });
    });
  });

  it("creates a local directory repository with a runtime binding", async () => {
    const user = userEvent.setup();
    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /Add local dir/ }));
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    await user.type(inputs[0]!, "Local checkout");
    await user.type(inputs[1]!, "/Users/troy/workspace/local-checkout");
    await user.click(screen.getByRole("button", { name: "Save repository" }));

    await waitFor(() => {
      expect(createRepositoryMock).toHaveBeenCalledWith({
        name: "Local checkout",
        source_state: "local_dir",
        binding: {
          daemon_id: "daemon-1",
          runtime_id: "runtime-1",
          machine_label: "Troy Mac · macOS",
          binding_kind: "local_dir",
          local_path: "/Users/troy/workspace/local-checkout",
          state: "ready",
        },
      });
    });
  });

  it("updates an existing first-class repository", async () => {
    const user = userEvent.setup();
    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: "Edit repository" }));
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    await user.clear(inputs[0]!);
    await user.type(inputs[0]!, "multica app");
    await user.clear(inputs[1]!);
    await user.type(inputs[1]!, "https://github.com/multica-ai/app.git");
    await user.click(screen.getByRole("button", { name: "Save repository" }));

    await waitFor(() => {
      expect(updateRepositoryMock).toHaveBeenCalledWith({
        id: "repo-1",
        name: "multica app",
        remote_url: "https://github.com/multica-ai/app.git",
      });
    });
  });

  it("archives an existing first-class repository", async () => {
    const user = userEvent.setup();
    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: "Delete repository" }));

    await waitFor(() => {
      expect(archiveRepositoryMock).toHaveBeenCalledWith("repo-1");
    });
  });

  it("renders compatibility repositories as read-only", () => {
    repositoriesRef.current = [
      makeRepository({
        id: "compat-1",
        compatibility: true,
        compatibility_source: "workspace.repos",
      }),
    ];

    render(<RepositoriesTab />, { wrapper: I18nWrapper });

    expect(screen.getByText("Legacy")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Edit repository" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Delete repository" })).toBeNull();
  });
});
