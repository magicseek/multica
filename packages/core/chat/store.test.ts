import { describe, expect, it } from "vitest";
import { createChatStore } from "./store";
import type { StorageAdapter } from "../types";

function memoryStorage(initial: Record<string, string> = {}): StorageAdapter {
  const values = new Map(Object.entries(initial));
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => {
      values.set(key, value);
    },
    removeItem: (key) => {
      values.delete(key);
    },
  };
}

describe("createChatStore", () => {
  it("defaults the chat panel to closed when the user has no stored preference", () => {
    const store = createChatStore({ storage: memoryStorage() });

    expect(store.getState().isOpen).toBe(false);
  });

  it("honors an explicit stored open preference", () => {
    const store = createChatStore({
      storage: memoryStorage({ "multica:chat:isOpen": "true" }),
    });

    expect(store.getState().isOpen).toBe(true);
  });
});
