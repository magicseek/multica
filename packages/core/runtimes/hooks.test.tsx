/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import type { AgentRuntime } from "../types";
import { useMyRuntimesNeedUpdate } from "./hooks";

vi.mock("../api", () => ({
  api: {
    listRuntimes: vi.fn(),
  },
}));

const userId = "user-1";

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function createRuntime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: "rt-1",
    workspace_id: "ws-1",
    daemon_id: "daemon-1",
    name: "Local Runtime",
    runtime_mode: "local",
    provider: "codex",
    launch_header: "",
    status: "online",
    device_info: "",
    metadata: { cli_version: "v0.1.0" },
    owner_id: userId,
    visibility: "private",
    timezone: "UTC",
    last_seen_at: "2026-05-20T00:00:00Z",
    created_at: "2026-05-20T00:00:00Z",
    updated_at: "2026-05-20T00:00:00Z",
    ...overrides,
  };
}

describe("runtime update hooks", () => {
  beforeEach(() => {
    const authStore = createAuthStore({
      api: {} as never,
      storage: {
        getItem: () => null,
        setItem: () => {},
        removeItem: () => {},
      },
    });
    authStore.setState({
      user: { id: userId, email: "user@example.test", name: "User" } as never,
      isLoading: false,
    });
    registerAuthStore(authStore);
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("does not fetch GitHub release metadata when no runtime can update", async () => {
    vi.mocked(api.listRuntimes).mockResolvedValue([
      createRuntime({ runtime_mode: "cloud" }),
      createRuntime({ metadata: { launched_by: "desktop", cli_version: "v0.1.0" } }),
    ]);
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderHook(() => useMyRuntimesNeedUpdate("ws-1"), {
      wrapper: createWrapper(qc),
    });

    await waitFor(() => expect(api.listRuntimes).toHaveBeenCalled());
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("fetches GitHub release metadata only when a caller-owned local runtime can update", async () => {
    vi.mocked(api.listRuntimes).mockResolvedValue([createRuntime()]);
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ tag_name: "v9.0.0" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useMyRuntimesNeedUpdate("ws-1"), {
      wrapper: createWrapper(qc),
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(result.current).toBe(true));
  });
});
