import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  localDirectoryPickerHealthPort,
  pickLocalDirectory,
} from "./local-directory-picker";

describe("local directory picker", () => {
  beforeEach(() => {
    delete (window as Window & { desktopAPI?: unknown }).desktopAPI;
  });

  it("uses the daemon bridge path before browser directory handles", async () => {
    const fetchFn = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          success: true,
          canceled: false,
          path: "/Users/troy/workspace/codex-mobile",
        }),
        { status: 200 },
      ),
    );

    const result = await pickLocalDirectory({
      daemonId: "daemon-1",
      healthPort: 19999,
      fetchFn,
      directoryPicker: vi.fn(),
    });

    expect(result).toEqual({
      source: "daemon",
      path: "/Users/troy/workspace/codex-mobile",
      name: "codex-mobile",
    });
    expect(fetchFn).toHaveBeenCalledWith(
      "http://127.0.0.1:19999/folder/select?daemon_id=daemon-1",
      { method: "GET" },
    );
  });

  it("falls back from a browser handle to manual absolute path confirmation", async () => {
    const result = await pickLocalDirectory({
      fetchFn: vi.fn().mockRejectedValue(new Error("daemon offline")),
      directoryPicker: async () => ({ name: "codex-mobile" }),
      promptFn: () => "/Users/troy/workspace/codex-mobile",
    });

    expect(result).toEqual({
      source: "manual",
      path: "/Users/troy/workspace/codex-mobile",
      name: "codex-mobile",
    });
  });

  it("does not invent a path when browser handle confirmation is canceled", async () => {
    const result = await pickLocalDirectory({
      fetchFn: vi.fn().mockRejectedValue(new Error("daemon offline")),
      directoryPicker: async () => ({ name: "codex-mobile" }),
      promptFn: () => null,
    });

    expect(result).toBeNull();
  });

  it("reads the daemon bridge port from runtime metadata", () => {
    expect(localDirectoryPickerHealthPort({ health_port: 19542 })).toBe(19542);
    expect(localDirectoryPickerHealthPort({ health_port: "19543" })).toBe(19543);
    expect(localDirectoryPickerHealthPort({ health_port: "not-a-port" })).toBeNull();
  });
});
