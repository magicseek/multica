import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ApiClient", () => {
  it("preserves HTTP status on failed requests", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: "workspace slug already exists" }), {
          status: 409,
          statusText: "Conflict",
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const client = new ApiClient("https://api.example.test");

    try {
      await client.createWorkspace({ name: "Test", slug: "test" });
      throw new Error("expected createWorkspace to fail");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({
        message: "workspace slug already exists",
        status: 409,
        statusText: "Conflict",
      });
    }
  });

  it("parses wrapped chat issue proposal list responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            proposals: [
              {
                id: "proposal-1",
                workspace_id: "workspace-1",
                chat_session_id: "session-1",
                title: "Follow-ups",
                status: "pending",
                items: [],
                created_at: "2026-05-18T00:00:00Z",
                updated_at: "2026-05-18T00:00:00Z",
              },
            ],
            total: 1,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    const client = new ApiClient("https://api.example.test");
    const proposals = await client.listChatIssueProposals("session-1");

    expect(proposals).toHaveLength(1);
    expect(proposals[0]?.id).toBe("proposal-1");
  });

  it("falls back to an empty proposal list when the wrapped response is malformed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ proposals: null, total: 1 }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const client = new ApiClient("https://api.example.test");
    const proposals = await client.listChatIssueProposals("session-1");

    expect(proposals).toEqual([]);
  });

  it("uses the expected HTTP contract for autopilot endpoints", async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(
      new Response(JSON.stringify({ autopilots: [], runs: [], total: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    ));
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient("https://api.example.test");

    await client.listAutopilots({ status: "active" });
    await client.getAutopilot("ap-1");
    await client.createAutopilot({
      title: "Daily triage",
      assignee_id: "agent-1",
      execution_mode: "create_issue",
    });
    await client.updateAutopilot("ap-1", { status: "paused" });
    await client.deleteAutopilot("ap-1");
    await client.triggerAutopilot("ap-1");
    await client.listAutopilotRuns("ap-1", { limit: 10, offset: 20 });
    await client.createAutopilotTrigger("ap-1", {
      kind: "schedule",
      cron_expression: "0 9 * * *",
      timezone: "UTC",
    });
    await client.updateAutopilotTrigger("ap-1", "tr-1", { enabled: false });
    await client.deleteAutopilotTrigger("ap-1", "tr-1");

    const calls = fetchMock.mock.calls.map(([url, init]) => ({
      url,
      method: init?.method ?? "GET",
      body: init?.body,
    }));

    expect(calls).toMatchObject([
      { url: "https://api.example.test/api/autopilots?status=active", method: "GET" },
      { url: "https://api.example.test/api/autopilots/ap-1", method: "GET" },
      {
        url: "https://api.example.test/api/autopilots",
        method: "POST",
        body: JSON.stringify({
          title: "Daily triage",
          assignee_id: "agent-1",
          execution_mode: "create_issue",
        }),
      },
      {
        url: "https://api.example.test/api/autopilots/ap-1",
        method: "PATCH",
        body: JSON.stringify({ status: "paused" }),
      },
      { url: "https://api.example.test/api/autopilots/ap-1", method: "DELETE" },
      { url: "https://api.example.test/api/autopilots/ap-1/trigger", method: "POST" },
      { url: "https://api.example.test/api/autopilots/ap-1/runs?limit=10&offset=20", method: "GET" },
      {
        url: "https://api.example.test/api/autopilots/ap-1/triggers",
        method: "POST",
        body: JSON.stringify({
          kind: "schedule",
          cron_expression: "0 9 * * *",
          timezone: "UTC",
        }),
      },
      {
        url: "https://api.example.test/api/autopilots/ap-1/triggers/tr-1",
        method: "PATCH",
        body: JSON.stringify({ enabled: false }),
      },
      { url: "https://api.example.test/api/autopilots/ap-1/triggers/tr-1", method: "DELETE" },
    ]);
  });

  it("uses the expected HTTP contract for repository endpoints", async () => {
    const repo = {
      id: "repo-1",
      workspace_id: "ws-1",
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
    };
    const binding = {
      id: "binding-1",
      repository_id: "repo-1",
      workspace_id: "ws-1",
      owner_user_id: "user-1",
      daemon_id: "daemon-1",
      runtime_id: null,
      machine_label: "Mac",
      binding_kind: "local_dir",
      local_path: "/repo",
      state: "ready",
      last_seen_at: null,
      metadata: {},
      created_at: "2026-05-17T00:00:00Z",
      updated_at: "2026-05-17T00:00:00Z",
      local_path_visible: true,
    };
    const operation = {
      id: "op-1",
      repository_id: "repo-1",
      workspace_id: "ws-1",
      operation_type: "create_binding",
      status: "queued",
      requested_by_type: "member",
      requested_by_id: "user-1",
      target_daemon_id: "daemon-1",
      target_runtime_id: null,
      binding_id: null,
      request: {},
      result: {},
      error: null,
      created_at: "2026-05-17T00:00:00Z",
      updated_at: "2026-05-17T00:00:00Z",
      completed_at: null,
    };
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      if (method === "DELETE") {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      let body: unknown = repo;
      if (url.endsWith("/api/repositories")) body = { repositories: [repo], total: 1 };
      if (url.endsWith("/bindings")) body = method === "GET" ? { bindings: [binding], total: 1 } : binding;
      if (url.endsWith("/operations") || url.endsWith("/operations/create_binding")) {
        body = method === "GET" ? { operations: [operation], total: 1 } : operation;
      }
      if (url.endsWith("/projects/project-1/repositories")) {
        body = {
          repositories: [
            {
              project_id: "project-1",
              repository_id: "repo-1",
              role: "primary",
              position: 0,
              created_at: "2026-05-17T00:00:00Z",
              repository: repo,
            },
          ],
          total: 1,
        };
      }
      if (url.endsWith("/api/tasks/task-1/outputs")) {
        body = {
          outputs: [
            {
              id: "output-1",
              workspace_id: "ws-1",
              repository_id: "repo-1",
              task_id: "task-1",
              relative_path: "docs/design.md",
              filename: "design.md",
              kind: "doc",
              size_bytes: 42,
              mime_type: "text/markdown",
              metadata: {},
              created_at: "2026-05-17T00:00:00Z",
            },
          ],
          total: 1,
        };
      }
      return Promise.resolve(
        new Response(JSON.stringify(body), {
          status: method === "POST" ? 201 : 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient("https://api.example.test");

    await client.listRepositories();
    await client.getRepository("repo-1");
    await client.createRepository({
      source_state: "remote_git",
      remote_url: "https://github.com/multica-ai/multica",
    });
    await client.updateRepository("repo-1", { name: "Multica" });
    await client.deleteRepository("repo-1");
    await client.listRepositoryBindings("repo-1");
    await client.createRepositoryBinding("repo-1", {
      daemon_id: "daemon-1",
      local_path: "/repo",
    });
    await client.deleteRepositoryBinding("repo-1", "binding-1");
    await client.listRepositoryOperations("repo-1");
    await client.createRepositoryOperation("repo-1", {
      operation_type: "create_binding",
      target_daemon_id: "daemon-1",
    });
    await client.listProjectRepositories("project-1");
    await client.setProjectRepositories("project-1", {
      repositories: [{ repository_id: "repo-1", role: "primary" }],
    });
    await client.listTaskOutputMetadata("task-1");

    const calls = fetchMock.mock.calls.map(([url, init]) => ({
      url,
      method: init?.method ?? "GET",
      body: init?.body,
    }));

    expect(calls).toMatchObject([
      { url: "https://api.example.test/api/repositories", method: "GET" },
      { url: "https://api.example.test/api/repositories/repo-1", method: "GET" },
      {
        url: "https://api.example.test/api/repositories",
        method: "POST",
        body: JSON.stringify({
          source_state: "remote_git",
          remote_url: "https://github.com/multica-ai/multica",
        }),
      },
      {
        url: "https://api.example.test/api/repositories/repo-1",
        method: "PATCH",
        body: JSON.stringify({ name: "Multica" }),
      },
      { url: "https://api.example.test/api/repositories/repo-1", method: "DELETE" },
      { url: "https://api.example.test/api/repositories/repo-1/bindings", method: "GET" },
      {
        url: "https://api.example.test/api/repositories/repo-1/bindings",
        method: "POST",
        body: JSON.stringify({ daemon_id: "daemon-1", local_path: "/repo" }),
      },
      { url: "https://api.example.test/api/repositories/repo-1/bindings/binding-1", method: "DELETE" },
      { url: "https://api.example.test/api/repositories/repo-1/operations", method: "GET" },
      {
        url: "https://api.example.test/api/repositories/repo-1/operations/create_binding",
        method: "POST",
        body: JSON.stringify({
          operation_type: "create_binding",
          target_daemon_id: "daemon-1",
        }),
      },
      { url: "https://api.example.test/api/projects/project-1/repositories", method: "GET" },
      {
        url: "https://api.example.test/api/projects/project-1/repositories",
        method: "PUT",
        body: JSON.stringify({
          repositories: [{ repository_id: "repo-1", role: "primary" }],
        }),
      },
      { url: "https://api.example.test/api/tasks/task-1/outputs", method: "GET" },
    ]);
  });

  it("falls back to an empty repository list for malformed responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ repositories: [{ id: "repo-1" }], total: 1 }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const client = new ApiClient("https://api.example.test");
    const repositories = await client.listRepositories();

    expect(repositories).toEqual({ repositories: [], total: 0 });
  });

  it("emits X-Client-* headers when identity is configured", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient("https://api.example.test", {
      identity: { platform: "desktop", version: "1.2.3", os: "macos" },
    });
    await client.listWorkspaces();

    const headers = fetchMock.mock.calls[0]![1]!.headers as Record<string, string>;
    expect(headers["X-Client-Platform"]).toBe("desktop");
    expect(headers["X-Client-Version"]).toBe("1.2.3");
    expect(headers["X-Client-OS"]).toBe("macos");
  });

  it("omits X-Client-* headers when identity is not configured", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient("https://api.example.test");
    await client.listWorkspaces();

    const headers = fetchMock.mock.calls[0]![1]!.headers as Record<string, string>;
    expect(headers["X-Client-Platform"]).toBeUndefined();
    expect(headers["X-Client-Version"]).toBeUndefined();
    expect(headers["X-Client-OS"]).toBeUndefined();
  });

  describe("getAttachment", () => {
    it("returns the parsed attachment for a well-formed response", async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response(
            JSON.stringify({
              id: "att-1",
              workspace_id: "ws-1",
              issue_id: null,
              comment_id: null,
              uploader_type: "member",
              uploader_id: "u-1",
              filename: "report.md",
              url: "https://static.example.test/ws/att-1.md",
              download_url:
                "https://static.example.test/ws/att-1.md?Policy=p&Signature=s&Key-Pair-Id=k",
              content_type: "text/markdown",
              size_bytes: 123,
              created_at: "2026-05-11T00:00:00Z",
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        ),
      );

      const client = new ApiClient("https://api.example.test");
      const att = await client.getAttachment("att-1");

      expect(att.id).toBe("att-1");
      expect(att.download_url).toContain("Policy=");
    });

    it("falls back to an empty attachment when the response is missing download_url", async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response(JSON.stringify({ id: "att-1" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        ),
      );

      const client = new ApiClient("https://api.example.test");
      const att = await client.getAttachment("att-1");

      // parseWithFallback returns the EMPTY_ATTACHMENT record so callers can
      // safely read `download_url` without crashing — they'll see "" and
      // surface a user-facing error instead of opening `undefined`.
      expect(att.id).toBe("");
      expect(att.download_url).toBe("");
    });
  });

  describe("getAttachmentTextContent", () => {
    it("returns body text and the original content type from the X-* header", async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response("# heading\n\nbody\n", {
            status: 200,
            headers: {
              "Content-Type": "text/plain; charset=utf-8",
              "X-Original-Content-Type": "text/markdown",
            },
          }),
        ),
      );

      const client = new ApiClient("https://api.example.test");
      const { text, originalContentType } =
        await client.getAttachmentTextContent("att-1");

      expect(text).toBe("# heading\n\nbody\n");
      expect(originalContentType).toBe("text/markdown");
    });

    it("throws PreviewTooLargeError on 413", async () => {
      const { PreviewTooLargeError } = await import("./client");
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response("", { status: 413, statusText: "Payload Too Large" }),
        ),
      );

      const client = new ApiClient("https://api.example.test");
      await expect(client.getAttachmentTextContent("att-1")).rejects.toBeInstanceOf(
        PreviewTooLargeError,
      );
    });

    it("throws PreviewUnsupportedError on 415", async () => {
      const { PreviewUnsupportedError } = await import("./client");
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response("", { status: 415, statusText: "Unsupported Media Type" }),
        ),
      );

      const client = new ApiClient("https://api.example.test");
      await expect(client.getAttachmentTextContent("att-1")).rejects.toBeInstanceOf(
        PreviewUnsupportedError,
      );
    });
  });

  describe("chat attachment wiring", () => {
    it("uploadFile includes chat_session_id in the FormData body", async () => {
      const fetchMock = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ id: "att-1", url: "https://cdn/x" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      vi.stubGlobal("fetch", fetchMock);

      const client = new ApiClient("https://api.example.test");
      const file = new File(["hi"], "hi.png", { type: "image/png" });
      await client.uploadFile(file, { chatSessionId: "session-123" });

      expect(fetchMock).toHaveBeenCalledTimes(1);
      const [url, init] = fetchMock.mock.calls[0]!;
      expect(url).toBe("https://api.example.test/api/upload-file");
      expect(init?.method).toBe("POST");
      const body = init?.body as FormData;
      expect(body).toBeInstanceOf(FormData);
      expect(body.get("chat_session_id")).toBe("session-123");
      expect(body.get("issue_id")).toBeNull();
      expect(body.get("comment_id")).toBeNull();
    });

    it("sendChatMessage serialises attachment_ids onto the JSON body when present", async () => {
      const fetchMock = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ message_id: "m1", task_id: "t1", created_at: "" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      );
      vi.stubGlobal("fetch", fetchMock);

      const client = new ApiClient("https://api.example.test");
      await client.sendChatMessage("session-1", "hello", ["att-1", "att-2"]);

      const [, init] = fetchMock.mock.calls[0]!;
      expect(JSON.parse(init?.body as string)).toEqual({
        content: "hello",
        attachment_ids: ["att-1", "att-2"],
      });
    });

    it("sendChatMessage omits attachment_ids when the list is empty or undefined", async () => {
      const fetchMock = vi.fn().mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({ message_id: "m1", task_id: "t1", created_at: "" }), {
            status: 201,
            headers: { "Content-Type": "application/json" },
          }),
        ),
      );
      vi.stubGlobal("fetch", fetchMock);

      const client = new ApiClient("https://api.example.test");
      await client.sendChatMessage("session-1", "hello");
      await client.sendChatMessage("session-1", "again", []);

      expect(JSON.parse(fetchMock.mock.calls[0]![1]?.body as string)).toEqual({ content: "hello" });
      expect(JSON.parse(fetchMock.mock.calls[1]![1]?.body as string)).toEqual({ content: "again" });
    });
  });
});
