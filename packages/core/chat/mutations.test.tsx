/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import type { SendChatMessageResponse } from "../types";
import { chatKeys } from "./queries";
import { useSendChatMessage } from "./mutations";

vi.mock("../api", () => ({
  api: {
    sendChatMessage: vi.fn(),
  },
}));

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe("useSendChatMessage", () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it("seeds the message and pending task before the send request resolves", async () => {
    const qc = createQueryClient();
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    qc.setQueryData(chatKeys.pendingTasks("ws-1"), { tasks: [] });
    const pending = deferred<SendChatMessageResponse>();
    vi.mocked(api.sendChatMessage).mockReturnValue(pending.promise);
    const onSessionResolved = vi.fn();

    const { result } = renderHook(
      () => useSendChatMessage({
        resolveSessionId: async () => "session-1",
        onSessionResolved,
      }),
      { wrapper: createWrapper(qc) },
    );

    let mutation!: Promise<unknown>;
    act(() => {
      mutation = result.current.mutateAsync({ content: "hello", attachmentIds: ["att-1"] });
    });

    await waitFor(() => expect(api.sendChatMessage).toHaveBeenCalledWith("session-1", "hello", ["att-1"]));
    const optimisticMessages = qc.getQueryData(chatKeys.messages("session-1"));
    expect(optimisticMessages).toMatchObject([
      {
        chat_session_id: "session-1",
        role: "user",
        content: "hello",
        task_id: null,
      },
    ]);
    expect(qc.getQueryData(chatKeys.pendingTask("session-1"))).toMatchObject({
      status: "queued",
      task_id: expect.stringMatching(/^optimistic-/),
    });
    expect(qc.getQueryData(chatKeys.pendingTasks("ws-1"))).toMatchObject({
      tasks: [
        {
          chat_session_id: "session-1",
          status: "queued",
          task_id: expect.stringMatching(/^optimistic-/),
        },
      ],
    });
    expect(onSessionResolved).toHaveBeenCalledWith("session-1");

    pending.resolve({
      message_id: "message-1",
      task_id: "task-1",
      created_at: "2026-05-21T20:00:00Z",
    });
    await mutation;

    expect(qc.getQueryData(chatKeys.messages("session-1"))).toMatchObject([
      {
        id: "message-1",
        content: "hello",
        task_id: "task-1",
      },
    ]);
    expect(qc.getQueryData(chatKeys.pendingTask("session-1"))).toEqual({
      task_id: "task-1",
      status: "queued",
      created_at: "2026-05-21T20:00:00Z",
    });
    expect(qc.getQueryData(chatKeys.pendingTasks("ws-1"))).toEqual({
      tasks: [
        {
          task_id: "task-1",
          status: "queued",
          chat_session_id: "session-1",
        },
      ],
    });
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("rolls back optimistic chat cache writes when send fails", async () => {
    const qc = createQueryClient();
    qc.setQueryData(chatKeys.pendingTasks("ws-1"), { tasks: [] });
    vi.mocked(api.sendChatMessage).mockRejectedValue(new Error("network down"));
    const onError = vi.fn();

    const { result } = renderHook(
      () => useSendChatMessage({
        resolveSessionId: async () => "session-1",
        onError,
      }),
      { wrapper: createWrapper(qc) },
    );

    await expect(result.current.mutateAsync({ content: "fail" })).rejects.toThrow("network down");

    expect(qc.getQueryData(chatKeys.messages("session-1"))).toEqual([]);
    expect(qc.getQueryData(chatKeys.pendingTask("session-1"))).toEqual({});
    expect(qc.getQueryData(chatKeys.pendingTasks("ws-1"))).toEqual({ tasks: [] });
    await waitFor(() => expect(onError).toHaveBeenCalled());
  });
});
