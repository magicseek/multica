"use client";

type DesktopDirectoryPickerResult =
  | { canceled: true }
  | { canceled: false; path: string };

type DesktopDirectoryPickerAPI = {
  selectDirectory?: () => Promise<DesktopDirectoryPickerResult>;
};

type BrowserDirectoryHandle = {
  name: string;
};

type BrowserDirectoryPickerWindow = Window & {
  desktopAPI?: DesktopDirectoryPickerAPI;
  showDirectoryPicker?: () => Promise<BrowserDirectoryHandle>;
};

type PromptFn = (message?: string, defaultValue?: string) => string | null;
type FetchFn = typeof fetch;

const DEFAULT_DAEMON_HEALTH_PORT = 19514;

export type LocalDirectoryPickerResult =
  | {
      source: "desktop";
      path: string;
      name: string;
    }
  | {
      source: "daemon";
      path: string;
      name: string;
    }
  | {
      source: "manual";
      path: string;
      name: string;
    }
  | {
      source: "browser";
      path: null;
      name: string;
      reason: "absolute_path_unavailable";
    };

type PickLocalDirectoryOptions = {
  daemonId?: string | null;
  healthPort?: number | null;
  fetchFn?: FetchFn;
  promptFn?: PromptFn;
  directoryPicker?: () => Promise<BrowserDirectoryHandle>;
};

function pickerWindow(): BrowserDirectoryPickerWindow | null {
  if (typeof window === "undefined") return null;
  return window as BrowserDirectoryPickerWindow;
}

export function directoryNameFromPath(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/\/+$/, "");
  return normalized.split("/").filter(Boolean).at(-1) || normalized || path;
}

export function hasLocalDirectoryPicker(): boolean {
  const win = pickerWindow();
  return Boolean(win?.desktopAPI?.selectDirectory || (win && typeof fetch === "function") || win?.showDirectoryPicker);
}

export function localDirectoryPickerHealthPort(metadata?: Record<string, unknown> | null): number | null {
  const raw = metadata?.health_port;
  const port = typeof raw === "number" ? raw : typeof raw === "string" ? Number.parseInt(raw, 10) : NaN;
  return Number.isFinite(port) && port > 0 && port < 65536 ? port : null;
}

function getPromptFn(override?: PromptFn): PromptFn | null {
  if (override) return override;
  if (typeof prompt === "function") return prompt.bind(globalThis);
  return null;
}

function normalizeManualPath(path: string | null | undefined): string | null {
  const trimmed = path?.trim();
  return trimmed || null;
}

function daemonHealthPorts(port?: number | null): number[] {
  const ports = [port, DEFAULT_DAEMON_HEALTH_PORT].filter(
    (value): value is number => typeof value === "number" && value > 0 && value < 65536,
  );
  return [...new Set(ports)];
}

async function pickFromLocalDaemon(
  options: PickLocalDirectoryOptions,
): Promise<LocalDirectoryPickerResult | null | undefined> {
  const fetchImpl = options.fetchFn ?? (typeof fetch === "function" ? fetch.bind(globalThis) : null);
  if (!fetchImpl) return undefined;

  for (const port of daemonHealthPorts(options.healthPort)) {
    const params = new URLSearchParams();
    if (options.daemonId) params.set("daemon_id", options.daemonId);
    try {
      const response = await fetchImpl(`http://127.0.0.1:${port}/folder/select?${params.toString()}`, {
        method: "GET",
      });
      if (!response.ok) continue;
      const raw = (await response.json()) as {
        success?: boolean;
        canceled?: boolean;
        path?: unknown;
      };
      if (raw.canceled) return null;
      const path = typeof raw.path === "string" ? raw.path.trim() : "";
      if (raw.success && path) {
        return {
          source: "daemon",
          path,
          name: directoryNameFromPath(path),
        };
      }
    } catch {
      // Keep trying fallbacks: the daemon may be offline, on another profile
      // port, or blocked by browser private-network policy.
    }
  }

  return undefined;
}

function promptForAbsolutePath(
  promptFn: PromptFn | null,
  selectedName?: string,
): LocalDirectoryPickerResult | null {
  if (!promptFn) return null;
  const path = normalizeManualPath(
    promptFn(
      selectedName
        ? `Browser selected folder: "${selectedName}"\n\nEnter the full absolute path visible to the selected runtime:`
        : "Enter the full absolute path visible to the selected runtime:",
      selectedName || undefined,
    ),
  );
  if (!path) return null;
  return {
    source: "manual",
    path,
    name: directoryNameFromPath(path),
  };
}

export async function pickLocalDirectory(
  options: PickLocalDirectoryOptions = {},
): Promise<LocalDirectoryPickerResult | null> {
  const win = pickerWindow();
  const promptFn = getPromptFn(options.promptFn);

  const desktopPicker = win?.desktopAPI?.selectDirectory;
  if (desktopPicker) {
    const result = await desktopPicker();
    if (result.canceled) return null;
    return {
      source: "desktop",
      path: result.path,
      name: directoryNameFromPath(result.path),
    };
  }

  const daemonResult = await pickFromLocalDaemon(options);
  if (daemonResult !== undefined) return daemonResult;

  const directoryPicker = options.directoryPicker ?? win?.showDirectoryPicker;
  if (directoryPicker) {
    try {
      const handle = await directoryPicker();
      const manual = promptForAbsolutePath(promptFn, handle.name);
      if (manual || promptFn) return manual;
      return {
        source: "browser",
        path: null,
        name: handle.name,
        reason: "absolute_path_unavailable",
      };
    } catch (error) {
      const name =
        error && typeof error === "object" && "name" in error
          ? String(error.name)
          : "";
      if (name === "AbortError") {
        return null;
      }
      throw error;
    }
  }

  return promptForAbsolutePath(promptFn);
}
