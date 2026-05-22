import type { ReactNode } from "react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";
import type { ConnectorCredential, ConnectorProvider, WorkspaceConnector } from "@multica/core/types";

const invalidateQueriesMock = vi.hoisted(() => vi.fn());
const updateWorkspaceConnectorMock = vi.hoisted(() => vi.fn());
const saveConnectorCredentialMock = vi.hoisted(() => vi.fn());
const deleteConnectorCredentialMock = vi.hoisted(() => vi.fn());
const connectorProvidersRef = vi.hoisted(() => ({
  current: [] as ConnectorProvider[],
}));
const connectorCredentialsRef = vi.hoisted(() => ({
  current: [] as ConnectorCredential[],
}));
const workspaceConnectorsRef = vi.hoisted(() => ({
  current: [] as WorkspaceConnector[],
}));
const membersRef = vi.hoisted(() => ({
  current: [{ user_id: "user-1", role: "owner" as const }],
}));

function makeProvider(overrides: Partial<ConnectorProvider> = {}): ConnectorProvider {
  return {
    id: "ringcentral_gitlab",
    display_name: "RingCentral GitLab",
    profile: "ringcentral",
    capabilities: [
      { id: "repo.read", display_name: "Read repository files", write: false },
      { id: "merge_request.create", display_name: "Create merge request", write: true },
    ],
    resource_types: ["ringcentral_gitlab_repo"],
    requires_user_credential: true,
    endpoints: {
      api_base_url: "https://gitlab.example.com/api/v4",
      web_base_url: "https://gitlab.example.com",
    },
    remote_write_policies: [
      { id: "disabled", display_name: "Disabled" },
      { id: "merge_request_preparation", display_name: "Merge request preparation" },
    ],
    ...overrides,
  };
}

vi.mock("@tanstack/react-query", () => ({
  queryOptions: (options: unknown) => options,
  useQueryClient: () => ({ invalidateQueries: invalidateQueriesMock }),
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey ?? [];
    if (key[0] === "workspaces" && key.includes("members")) return { data: membersRef.current };
    if (key[0] === "github") return { data: { configured: true, installations: [] } };
    if (key[0] === "connectors" && key.includes("providers")) {
      return { data: { providers: connectorProvidersRef.current } };
    }
    if (key[0] === "connectors" && key.includes("credentials")) {
      return { data: { credentials: connectorCredentialsRef.current } };
    }
    if (key[0] === "connectors" && key.includes("workspace")) {
      return { data: { connectors: workspaceConnectorsRef.current } };
    }
    return { data: undefined };
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (sel?: (s: { user: { id: string } }) => unknown) =>
      sel ? sel({ user: { id: "user-1" } }) : { user: { id: "user-1" } },
    { getState: () => ({ user: { id: "user-1" } }) },
  );
  return { useAuthStore };
});

vi.mock("@multica/core/api", () => ({
  api: {
    getGitHubConnectURL: vi.fn(),
    updateWorkspaceConnector: updateWorkspaceConnectorMock,
    saveConnectorCredential: saveConnectorCredentialMock,
    deleteConnectorCredential: deleteConnectorCredentialMock,
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { IntegrationsTab } from "./integrations-tab";

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

describe("IntegrationsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    updateWorkspaceConnectorMock.mockResolvedValue({
      provider_id: "ringcentral_gitlab",
      enabled: false,
      settings: {},
    });
    saveConnectorCredentialMock.mockResolvedValue({
      provider_id: "ringcentral_jira",
      has_credential: true,
      status: "valid",
      last_validated_at: null,
      invalidated_at: null,
      updated_at: "2026-05-22T00:00:00Z",
    });
    deleteConnectorCredentialMock.mockResolvedValue(undefined);
    connectorProvidersRef.current = [
      makeProvider(),
      makeProvider({
        id: "ringcentral_jira",
        display_name: "RingCentral Jira",
        capabilities: [{ id: "issue.search", display_name: "Search issues", write: false }],
        resource_types: ["ringcentral_jira_issue", "ringcentral_jira_project"],
        endpoints: { base_url: "https://jira.example.com" },
        remote_write_policies: undefined,
      }),
      makeProvider({
        id: "ringcentral_wiki",
        display_name: "RingCentral Wiki",
        capabilities: [{ id: "page.read", display_name: "Read page", write: false }],
        resource_types: ["ringcentral_wiki_page", "ringcentral_wiki_space"],
        endpoints: { base_url: "https://wiki.example.com" },
        remote_write_policies: undefined,
      }),
    ];
    connectorCredentialsRef.current = [
      {
        provider_id: "ringcentral_gitlab",
        has_credential: true,
        status: "valid",
        last_validated_at: null,
        invalidated_at: null,
        updated_at: "2026-05-22T00:00:00Z",
      },
    ];
    workspaceConnectorsRef.current = [
      { provider_id: "ringcentral_gitlab", enabled: true, settings: {} },
      { provider_id: "ringcentral_jira", enabled: true, settings: {} },
      { provider_id: "ringcentral_wiki", enabled: false, settings: {} },
    ];
    membersRef.current = [{ user_id: "user-1", role: "owner" }];
  });

  it("renders RingCentral integrations as roomy rows without internal resource IDs", () => {
    render(<IntegrationsTab />, { wrapper: I18nWrapper });

    expect(screen.getByText("GitLab")).toBeTruthy();
    expect(screen.getByText("Jira")).toBeTruthy();
    expect(screen.getByText("Wiki")).toBeTruthy();
    expect(screen.getByText("Repositories, branches, and merge requests for task-scoped code work.")).toBeTruthy();
    expect(screen.queryByText(/ringcentral_gitlab_repo/)).toBeNull();
    expect(screen.queryByText(/issue.search/)).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.getAllByRole("switch")).toHaveLength(3);
    expect(screen.getByPlaceholderText("GitLab personal access token")).toBeTruthy();
    expect(screen.getByPlaceholderText("Jira personal access token")).toBeTruthy();
    expect(screen.getByPlaceholderText("Wiki personal access token")).toBeTruthy();
  });

  it("opens a focused detail view from a row", async () => {
    const user = userEvent.setup();
    render(<IntegrationsTab />, { wrapper: I18nWrapper });

    await user.click(screen.getAllByRole("button", { name: /Details/ })[0]!);

    expect(screen.getByRole("heading", { name: "GitLab" })).toBeTruthy();
    expect(screen.getByText("Information")).toBeTruthy();
    expect(screen.getByText("Agent use")).toBeTruthy();
    expect(screen.getAllByText("gitlab.example.com")).toHaveLength(2);
    expect(screen.queryByText(/ringcentral_gitlab_repo/)).toBeNull();
    expect(screen.queryByPlaceholderText("GitLab personal access token")).toBeNull();
  });

  it("uses the workspace connector switch to enable and disable a row", async () => {
    const user = userEvent.setup();
    render(<IntegrationsTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("switch", { name: "Toggle Jira integration" }));

    await waitFor(() => {
      expect(updateWorkspaceConnectorMock).toHaveBeenCalledWith("workspace-1", "ringcentral_jira", {
        enabled: false,
        settings: {},
      });
    });
    expect(invalidateQueriesMock).toHaveBeenCalledWith({
      queryKey: ["connectors", "workspace-1", "workspace"],
    });
  });

  it("saves access tokens directly from the integration list", async () => {
    const user = userEvent.setup();
    render(<IntegrationsTab />, { wrapper: I18nWrapper });

    await user.type(screen.getByPlaceholderText("Jira personal access token"), "jira-token");
    await user.click(screen.getAllByRole("button", { name: "Save token" })[1]!);

    await waitFor(() => {
      expect(saveConnectorCredentialMock).toHaveBeenCalledWith("workspace-1", "ringcentral_jira", {
        secret: "jira-token",
      });
    });
    expect(invalidateQueriesMock).toHaveBeenCalledWith({
      queryKey: ["connectors", "workspace-1", "credentials"],
    });
  });
});
