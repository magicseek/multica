// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent, MemberWithUser, RuntimeDevice } from "@multica/core/types";
import type { ReactNode } from "react";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";

const TEST_RESOURCES = { en: { agents: enAgents, common: enCommon } };

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  api: {
    listSkills: vi.fn().mockResolvedValue([]),
    uploadFile: vi.fn(),
  },
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}));

import { AgentDetailInspector } from "./agent-detail-inspector";

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    name: "Codex",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_env: {},
    custom_args: [],
    custom_env_redacted: false,
    visibility: "private",
    status: "idle",
    max_concurrent_tasks: 1,
    model: "",
    owner_id: "user-1",
    skills: [],
    created_at: "2026-05-25T00:00:00Z",
    updated_at: "2026-05-25T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function makeRuntime(overrides: Partial<RuntimeDevice> = {}): RuntimeDevice {
  return {
    id: "runtime-1",
    workspace_id: "ws-1",
    daemon_id: null,
    name: "Codex Local",
    runtime_mode: "local",
    provider: "github-copilot",
    launch_header: "",
    status: "offline",
    device_info: "host.local",
    metadata: {},
    owner_id: "user-1",
    visibility: "private",
    timezone: "UTC",
    last_seen_at: null,
    created_at: "2026-05-25T00:00:00Z",
    updated_at: "2026-05-25T00:00:00Z",
    ...overrides,
  };
}

const owner: MemberWithUser = {
  id: "member-1",
  user_id: "user-1",
  workspace_id: "ws-1",
  role: "member",
  name: "Owner",
  email: "owner@example.test",
  avatar_url: null,
  created_at: "2026-05-25T00:00:00Z",
};

function renderInspector(runtime = makeRuntime()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <AgentDetailInspector
          agent={makeAgent()}
          runtime={runtime}
          owner={owner}
          presence={null}
          runtimes={[runtime]}
          members={[owner]}
          currentUserId="user-1"
          canEdit={false}
          onUpdate={vi.fn()}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

describe("AgentDetailInspector", () => {
  it("explains request-efficient mode from the inspector", () => {
    renderInspector();

    expect(
      screen.getByRole("button", {
        name: "Recommended for request-priced providers; bundle several issues into one provider run.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Recommended for request-priced providers; bundle several issues into one provider run.",
      ),
    ).toBeInTheDocument();
  });
});
