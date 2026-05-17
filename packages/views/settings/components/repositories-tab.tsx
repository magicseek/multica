"use client";

import { useMemo, useState } from "react";
import { Save, Plus, Trash2, Pencil, X, FolderGit2, Loader2 } from "lucide-react";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Badge } from "@multica/ui/components/ui/badge";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  repositoryListOptions,
  useArchiveRepository,
  useCreateRepository,
  useUpdateRepository,
} from "@multica/core/repositories";
import type { Repository, RepositorySourceState, RuntimeDevice } from "@multica/core/types";
import { useT } from "../../i18n";

type Draft = {
  name: string;
  remoteUrl: string;
  localPath: string;
  runtimeId: string;
};

type NewRepositoryDraft = Draft & {
  id: string;
  source_state: RepositorySourceState;
};

type RepositoryRow =
  | { kind: "repository"; repository: Repository }
  | { kind: "new"; draft: NewRepositoryDraft };

function rowID(row: RepositoryRow): string {
  return row.kind === "repository" ? row.repository.id : row.draft.id;
}

function rowSourceState(row: RepositoryRow): RepositorySourceState {
  return row.kind === "repository" ? row.repository.source_state : row.draft.source_state;
}

function rowCompatibility(row: RepositoryRow): boolean {
  return row.kind === "repository" && row.repository.compatibility === true;
}

function rowName(row: RepositoryRow): string {
  return row.kind === "repository" ? row.repository.name : row.draft.name;
}

function rowRemoteURL(row: RepositoryRow): string {
  return row.kind === "repository" ? row.repository.remote_url ?? "" : row.draft.remoteUrl;
}

function draftFromRow(row: RepositoryRow): Draft {
  return {
    name: rowName(row),
    remoteUrl: rowRemoteURL(row),
    localPath: row.kind === "new" ? row.draft.localPath : "",
    runtimeId: row.kind === "new" ? row.draft.runtimeId : "",
  };
}

function isRemoteEditable(row: RepositoryRow): boolean {
  return rowSourceState(row) === "remote_git";
}

export function RepositoriesTab() {
  const { t } = useT("settings");
  const user = useAuthStore((s) => s.user);
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const { data: repositories = [], isLoading } = useQuery(repositoryListOptions(wsId));
  const createRepository = useCreateRepository(wsId);
  const updateRepository = useUpdateRepository(wsId);
  const archiveRepository = useArchiveRepository(wsId);

  const [newDrafts, setNewDrafts] = useState<NewRepositoryDraft[]>([]);
  const [editingIDs, setEditingIDs] = useState<Set<string>>(new Set());
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [savingID, setSavingID] = useState<string | null>(null);

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const localRuntimes = useMemo(
    () =>
      runtimes.filter(
        (runtime) =>
          runtime.runtime_mode === "local" &&
          runtime.status === "online" &&
          runtime.owner_id === user?.id &&
          Boolean(runtime.daemon_id),
      ),
    [runtimes, user?.id],
  );

  const rows = useMemo<RepositoryRow[]>(() => {
    const activeRepositories = repositories.filter((repo) => repo.status !== "archived");
    return [
      ...activeRepositories.map((repository) => ({ kind: "repository" as const, repository })),
      ...newDrafts.map((draft) => ({ kind: "new" as const, draft })),
    ];
  }, [repositories, newDrafts]);

  const sourceLabel = (sourceState: RepositorySourceState) => {
    switch (sourceState) {
      case "local_dir":
        return t(($) => $.repositories.source_local_dir);
      case "agent_managed":
        return t(($) => $.repositories.source_agent_managed);
      case "local_git":
        return t(($) => $.repositories.source_local_git);
      case "remote_git":
      default:
        return t(($) => $.repositories.source_remote_git);
    }
  };

  const runtimeLabel = (runtime: RuntimeDevice) =>
    runtime.device_info ? `${runtime.name} · ${runtime.device_info}` : runtime.name;

  const handleAddRemote = () => {
    const id = `new-${Date.now()}`;
    const draft: NewRepositoryDraft = {
      id,
      source_state: "remote_git",
      name: "",
      remoteUrl: "",
      localPath: "",
      runtimeId: "",
    };
    setNewDrafts((prev) => [...prev, draft]);
    setDrafts((prev) => ({ ...prev, [id]: draft }));
    setEditingIDs((prev) => new Set(prev).add(id));
  };

  const handleAddLocalDir = () => {
    const id = `new-${Date.now()}`;
    const draft: NewRepositoryDraft = {
      id,
      source_state: "local_dir",
      name: "",
      remoteUrl: "",
      localPath: "",
      runtimeId: localRuntimes[0]?.id ?? "",
    };
    setNewDrafts((prev) => [...prev, draft]);
    setDrafts((prev) => ({ ...prev, [id]: draft }));
    setEditingIDs((prev) => new Set(prev).add(id));
  };

  const handleEdit = (row: RepositoryRow) => {
    const id = rowID(row);
    setDrafts((prev) => ({ ...prev, [id]: draftFromRow(row) }));
    setEditingIDs((prev) => new Set(prev).add(id));
  };

  const handleDraftChange = (id: string, patch: Partial<Draft>) => {
    setDrafts((prev) => ({
      ...prev,
      [id]: { ...(prev[id] ?? { name: "", remoteUrl: "", localPath: "", runtimeId: "" }), ...patch },
    }));
  };

  const clearEditing = (id: string) => {
    setEditingIDs((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    setDrafts((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
  };

  const handleCancel = (row: RepositoryRow) => {
    const id = rowID(row);
    if (row.kind === "new") {
      setNewDrafts((prev) => prev.filter((draft) => draft.id !== id));
    }
    clearEditing(id);
  };

  const handleSave = async (row: RepositoryRow) => {
    const id = rowID(row);
    const draft = drafts[id] ?? draftFromRow(row);
    const name = draft.name.trim();
    const remoteUrl = draft.remoteUrl.trim();
    const localPath = draft.localPath.trim();
    const selectedRuntime = localRuntimes.find((runtime) => runtime.id === draft.runtimeId) ?? null;
    if (isRemoteEditable(row) && !remoteUrl) {
      toast.error(t(($) => $.repositories.toast_url_required));
      return;
    }
    if (rowSourceState(row) === "local_dir" && row.kind === "new") {
      if (!localPath) {
        toast.error(t(($) => $.repositories.toast_local_path_required));
        return;
      }
      if (!selectedRuntime?.daemon_id) {
        toast.error(t(($) => $.repositories.toast_runtime_required));
        return;
      }
    }
    setSavingID(id);
    try {
      if (row.kind === "new") {
        if (row.draft.source_state === "local_dir") {
          await createRepository.mutateAsync({
            name: name || localPath.split("/").filter(Boolean).at(-1) || undefined,
            source_state: "local_dir",
            binding: {
              daemon_id: selectedRuntime?.daemon_id ?? "",
              runtime_id: selectedRuntime?.id ?? null,
              machine_label: selectedRuntime ? runtimeLabel(selectedRuntime) : undefined,
              binding_kind: "local_dir",
              local_path: localPath,
              state: "ready",
            },
          });
        } else {
          await createRepository.mutateAsync({
            name: name || undefined,
            source_state: "remote_git",
            remote_url: remoteUrl,
          });
        }
        setNewDrafts((prev) => prev.filter((draft) => draft.id !== id));
      } else {
        await updateRepository.mutateAsync({
          id: row.repository.id,
          name: name || row.repository.name,
          ...(isRemoteEditable(row) ? { remote_url: remoteUrl } : {}),
        });
      }
      clearEditing(id);
      toast.success(t(($) => $.repositories.toast_saved));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.repositories.toast_save_failed));
    } finally {
      setSavingID(null);
    }
  };

  const handleRemove = async (row: RepositoryRow) => {
    const id = rowID(row);
    if (row.kind === "new") {
      setNewDrafts((prev) => prev.filter((draft) => draft.id !== id));
      clearEditing(id);
      return;
    }
    if (row.repository.compatibility) return;
    setSavingID(id);
    try {
      await archiveRepository.mutateAsync(row.repository.id);
      toast.success(t(($) => $.repositories.toast_archived));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.repositories.toast_archive_failed));
    } finally {
      setSavingID(null);
    }
  };

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">{t(($) => $.repositories.section_title)}</h2>

        <Card>
          <CardContent className="space-y-3">
            <p className="text-xs text-muted-foreground">
              {t(($) => $.repositories.description)}
            </p>

            {isLoading && (
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Loader2 className="size-3.5 animate-spin" />
                {t(($) => $.repositories.loading)}
              </div>
            )}

            {!isLoading && rows.length === 0 && (
              <p className="text-xs text-muted-foreground italic">
                {t(($) => $.repositories.empty)}
              </p>
            )}

            {rows.map((row) => {
              const id = rowID(row);
              const isEditing = editingIDs.has(id);
              const draft = drafts[id] ?? draftFromRow(row);
              const sourceState = rowSourceState(row);
              const compatibility = rowCompatibility(row);
              const canEditRow = canManageWorkspace && !compatibility;
              const saving = savingID === id;
              return (
                <div
                  key={id}
                  className="group flex items-start gap-2 rounded-md border bg-background px-3 py-2"
                >
                  <FolderGit2 className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1 space-y-2">
                    {isEditing ? (
                      <div className="grid gap-2 sm:grid-cols-[minmax(0,0.6fr)_minmax(0,1fr)]">
                        <Input
                          type="text"
                          value={draft.name}
                          onChange={(e) => handleDraftChange(id, { name: e.target.value })}
                          disabled={!canEditRow || saving}
                          placeholder={t(($) => $.repositories.name_placeholder)}
                          className="min-w-0 text-sm"
                        />
                        {isRemoteEditable(row) && (
                          <Input
                            type="text"
                            value={draft.remoteUrl}
                            onChange={(e) =>
                              handleDraftChange(id, { remoteUrl: e.target.value })
                            }
                            disabled={!canEditRow || saving}
                            placeholder={t(($) => $.repositories.url_placeholder)}
                            className="min-w-0 font-mono text-xs"
                          />
                        )}
                        {row.kind === "new" && row.draft.source_state === "local_dir" && (
                          <>
                            <Input
                              type="text"
                              value={draft.localPath}
                              onChange={(e) =>
                                handleDraftChange(id, { localPath: e.target.value })
                              }
                              disabled={!canEditRow || saving}
                              placeholder={t(($) => $.repositories.local_path_placeholder)}
                              className="min-w-0 font-mono text-xs"
                            />
                            <select
                              aria-label={t(($) => $.repositories.runtime_select_aria)}
                              value={draft.runtimeId}
                              onChange={(e) => handleDraftChange(id, { runtimeId: e.target.value })}
                              disabled={!canEditRow || saving || localRuntimes.length === 0}
                              className="min-w-0 rounded-md border bg-background px-3 py-2 text-sm text-foreground"
                            >
                              {localRuntimes.length === 0 ? (
                                <option value="">
                                  {t(($) => $.repositories.runtime_select_empty)}
                                </option>
                              ) : (
                                localRuntimes.map((runtime) => (
                                  <option key={runtime.id} value={runtime.id}>
                                    {runtimeLabel(runtime)}
                                  </option>
                                ))
                              )}
                            </select>
                          </>
                        )}
                      </div>
                    ) : (
                      <div className="min-w-0 space-y-1">
                        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                          <span className="min-w-0 truncate text-sm font-medium">
                            {rowName(row) || t(($) => $.repositories.name_empty)}
                          </span>
                          <Badge variant="secondary" className="rounded-sm px-1.5 py-0 text-[10px]">
                            {sourceLabel(sourceState)}
                          </Badge>
                          {compatibility && (
                            <Badge variant="outline" className="rounded-sm px-1.5 py-0 text-[10px]">
                              {t(($) => $.repositories.compatibility_badge)}
                            </Badge>
                          )}
                        </div>
                        <div
                          className="truncate font-mono text-xs text-muted-foreground"
                          title={rowRemoteURL(row)}
                        >
                          {rowRemoteURL(row) || t(($) => $.repositories.no_remote_url)}
                        </div>
                      </div>
                    )}
                  </div>

                  {canEditRow && (
                    <div
                      className={
                        isEditing
                          ? "flex shrink-0 items-center gap-0.5"
                          : "flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 [@media(hover:none)]:opacity-100"
                      }
                    >
                      {isEditing ? (
                        <>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label={t(($) => $.repositories.save_aria)}
                            className="text-muted-foreground hover:text-foreground"
                            onClick={() => handleSave(row)}
                            disabled={saving}
                          >
                            {saving ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Save className="h-3.5 w-3.5" />
                            )}
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label={t(($) => $.repositories.cancel_aria)}
                            className="text-muted-foreground hover:text-foreground"
                            onClick={() => handleCancel(row)}
                            disabled={saving}
                          >
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </>
                      ) : (
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={t(($) => $.repositories.edit_aria)}
                          className="text-muted-foreground hover:text-foreground"
                          onClick={() => handleEdit(row)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={t(($) => $.repositories.delete_aria)}
                        className="text-muted-foreground hover:text-destructive"
                        onClick={() => handleRemove(row)}
                        disabled={saving}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  )}
                </div>
              );
            })}

            {canManageWorkspace ? (
              <div className="flex flex-wrap items-center gap-2 pt-1">
                <Button variant="outline" size="sm" onClick={handleAddRemote}>
                  <Plus className="h-3 w-3" />
                  {t(($) => $.repositories.add_remote_git)}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleAddLocalDir}
                  disabled={localRuntimes.length === 0}
                >
                  <Plus className="h-3 w-3" />
                  {t(($) => $.repositories.add_local_dir)}
                </Button>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">
                {t(($) => $.repositories.manage_hint)}
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
