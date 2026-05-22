"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  BookOpen,
  ChevronRight,
  GitBranch,
  Loader2,
  Pencil,
  RotateCcw,
  Save,
  ShieldCheck,
  Ticket,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Switch } from "@multica/ui/components/ui/switch";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions } from "@multica/core/workspace/queries";
import { githubInstallationsOptions } from "@multica/core/github/queries";
import {
  connectorCredentialsOptions,
  connectorKeys,
  connectorProvidersOptions,
  workspaceConnectorsOptions,
} from "@multica/core/connectors/queries";
import { api } from "@multica/core/api";
import type { ConnectorCredential, ConnectorProvider, WorkspaceConnector } from "@multica/core/types";
import { useT } from "../../i18n";

// lucide-react v1.x dropped brand marks (including Github). Render an inline
// SVG of the official GitHub octocat mark so the card is still recognizable.
function GitHubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" className={className} fill="currentColor">
      <path d="M12 .5C5.6.5.5 5.6.5 12c0 5.1 3.3 9.4 7.9 10.9.6.1.8-.2.8-.6v-2.2c-3.2.7-3.9-1.5-3.9-1.5-.5-1.3-1.3-1.7-1.3-1.7-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1 1.8 2.7 1.3 3.4 1 .1-.8.4-1.3.8-1.6-2.6-.3-5.3-1.3-5.3-5.7 0-1.3.5-2.3 1.2-3.1-.1-.3-.5-1.5.1-3.1 0 0 1-.3 3.3 1.2.9-.3 1.9-.4 2.9-.4s2 .1 2.9.4c2.3-1.5 3.3-1.2 3.3-1.2.6 1.6.2 2.8.1 3.1.7.8 1.2 1.8 1.2 3.1 0 4.4-2.7 5.4-5.3 5.7.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6 4.6-1.5 7.9-5.8 7.9-10.9C23.5 5.6 18.4.5 12 .5z" />
    </svg>
  );
}

function ringCentralIcon(providerID: string) {
  const className = "h-4 w-4 text-muted-foreground";
  if (providerID.includes("gitlab")) return <GitBranch className={className} aria-hidden="true" />;
  if (providerID.includes("jira")) return <Ticket className={className} aria-hidden="true" />;
  return <BookOpen className={className} aria-hidden="true" />;
}

type RingCentralProviderKind = "gitlab" | "jira" | "wiki" | "unknown";

function ringCentralProviderKind(providerID: string): RingCentralProviderKind {
  if (providerID.includes("gitlab")) return "gitlab";
  if (providerID.includes("jira")) return "jira";
  if (providerID.includes("wiki")) return "wiki";
  return "unknown";
}

function compactEndpoint(value: string) {
  try {
    return new URL(value).host;
  } catch {
    return value.replace(/^https?:\/\//, "").replace(/\/$/, "");
  }
}

function endpointOverrides(workspaceConnector: WorkspaceConnector | null) {
  const raw = workspaceConnector?.settings.endpoint_overrides;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
  return Object.fromEntries(Object.entries(raw).filter(([, value]) => typeof value === "string")) as Record<
    string,
    string
  >;
}

function effectiveEndpoints(provider: ConnectorProvider, workspaceConnector: WorkspaceConnector | null) {
  return {
    ...(provider.endpoints ?? {}),
    ...endpointOverrides(workspaceConnector),
  };
}

function hasEndpointOverrides(workspaceConnector: WorkspaceConnector | null) {
  return Object.keys(endpointOverrides(workspaceConnector)).length > 0;
}

function endpointEntries(provider: ConnectorProvider, workspaceConnector: WorkspaceConnector | null) {
  return Object.entries(effectiveEndpoints(provider, workspaceConnector)).filter(([, value]) => Boolean(value));
}

function primaryEndpoint(provider: ConnectorProvider, workspaceConnector: WorkspaceConnector | null) {
  const endpoints = effectiveEndpoints(provider, workspaceConnector);
  const endpoint =
    endpoints.web_base_url ?? endpoints.base_url ?? endpoints.api_base_url ?? Object.values(endpoints).find(Boolean);
  return endpoint ? compactEndpoint(endpoint) : null;
}

function endpointLabel(key: string, serviceName: string) {
  if (key === "api_base_url") return `${serviceName} API`;
  if (key === "web_base_url") return `${serviceName} web`;
  if (key === "base_url") return serviceName;
  return serviceName;
}

type RingCentralProviderCopy = {
  title: string;
  description: string;
  tokenPlaceholder: string;
  agentUse: string[];
};

function ringCentralProviderCopy(provider: ConnectorProvider): RingCentralProviderCopy {
  const kind = ringCentralProviderKind(provider.id);
  if (kind === "gitlab") {
    return {
      title: "GitLab",
      description: "Repositories, branches, and merge requests for task-scoped code work.",
      tokenPlaceholder: "GitLab personal access token",
      agentUse: [
        "Inspect repositories attached to a project or issue.",
        "Read merge request context when a task needs code review details.",
        "Prepare branches, commits, and merge requests when remote write access allows it.",
      ],
    };
  }
  if (kind === "jira") {
    return {
      title: "Jira",
      description: "Projects and issues that agents can inspect while working on tasks.",
      tokenPlaceholder: "Jira personal access token",
      agentUse: [
        "Search project issues and bring the relevant task context into an agent run.",
        "Read linked issue details, status, and discussion context.",
      ],
    };
  }
  if (kind === "wiki") {
    return {
      title: "Wiki",
      description: "Spaces and pages that agents can search and read as task context.",
      tokenPlaceholder: "Wiki personal access token",
      agentUse: [
        "Search knowledge spaces associated with a task.",
        "Read linked pages and use them as grounded reference material.",
      ],
    };
  }
  return {
    title: provider.display_name,
    description: "Task-scoped connector managed by the server profile.",
    tokenPlaceholder: "Personal access token",
    agentUse: ["Use connector resources only when they are attached to the current task."],
  };
}

function credentialStatusLabel(credential: ConnectorCredential | null) {
  if (!credential?.has_credential) return "Needs token";
  if (credential.status === "invalid") return "Invalid token";
  return "Connected";
}

function remoteWritePolicyLabel(policyID: string, fallback: string) {
  if (policyID === "disabled") return "Disabled";
  if (policyID === "merge_request_preparation") return "Merge request preparation";
  return fallback;
}

const RINGCENTRAL_SECTION_DESCRIPTION = "Private workspace connectors for task-scoped access to company tools.";
const DETAILS_LABEL = "Details";
const RINGCENTRAL_LABEL = "RingCentral";
const SETTINGS_LABEL = "Settings";
const REMOTE_WRITE_ACCESS_LABEL = "Remote write access";
const SERVICE_ADDRESSES_LABEL = "Service addresses";
const EDIT_LABEL = "Edit";
const CANCEL_LABEL = "Cancel";
const SAVE_ADDRESSES_LABEL = "Save addresses";
const USE_DEFAULTS_LABEL = "Use defaults";
const CUSTOM_SERVICE_LABEL = "Custom service";
const DEFAULT_LABEL = "Default";
const CUSTOM_LABEL = "Custom";
const INFORMATION_LABEL = "Information";
const AGENT_USE_LABEL = "Agent use";
const STATUS_LABEL = "Status";
const CREDENTIAL_LABEL = "Credential";

export function IntegrationsTab() {
  const { t } = useT("settings");
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  const user = useAuthStore((s) => s.user);
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const [connecting, setConnecting] = useState(false);
  const [selectedRingCentralProviderID, setSelectedRingCentralProviderID] = useState<string | null>(null);

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManage = currentMember?.role === "owner" || currentMember?.role === "admin";

  // Only used to gate the Connect button + show a "not configured" hint;
  // we no longer render the installation list here — admins manage existing
  // installations on GitHub directly via the Connect flow.
  const { data } = useQuery({
    ...githubInstallationsOptions(wsId),
    enabled: !!wsId && canManage,
  });
  const configured = data?.configured ?? false;
  const { data: connectorProviders } = useQuery({
    ...connectorProvidersOptions(wsId),
    enabled: !!wsId,
  });
  const { data: connectorCredentials } = useQuery({
    ...connectorCredentialsOptions(wsId),
    enabled: !!wsId,
  });
  const { data: workspaceConnectors } = useQuery({
    ...workspaceConnectorsOptions(wsId),
    enabled: !!wsId,
  });
  const ringCentralProviders =
    connectorProviders?.providers.filter((provider) => provider.profile === "ringcentral") ?? [];
  const connectorCredentialsByProvider = new Map(
    (connectorCredentials?.credentials ?? []).map((credential) => [credential.provider_id, credential]),
  );
  const workspaceConnectorsByProvider = new Map(
    (workspaceConnectors?.connectors ?? []).map((connector) => [connector.provider_id, connector]),
  );
  const selectedRingCentralProvider =
    ringCentralProviders.find((provider) => provider.id === selectedRingCentralProviderID) ?? null;

  async function handleConnect() {
    setConnecting(true);
    try {
      const resp = await api.getGitHubConnectURL(wsId);
      if (!resp.configured || !resp.url) {
        toast.error(t(($) => $.integrations.toast_not_configured));
        return;
      }
      window.open(resp.url, "_blank", "noopener");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.toast_open_failed));
    } finally {
      setConnecting(false);
    }
  }

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">{t(($) => $.integrations.section_title)}</h2>

        <Card>
          <CardContent className="space-y-4">
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-start gap-3">
                <GitHubMark className="h-6 w-6 mt-0.5 shrink-0" />
                <div className="space-y-1">
                  <p className="text-sm font-medium">{t(($) => $.integrations.github_title)}</p>
                  <p className="text-xs text-muted-foreground">
                    {t(($) => $.integrations.github_description_prefix)}{" "}
                    <code className="rounded bg-muted px-1 py-0.5 text-[10px]">
                      {t(($) => $.integrations.github_identifier_example)}
                    </code>{" "}
                    {t(($) => $.integrations.github_description_suffix)}{" "}
                    <strong>{t(($) => $.integrations.github_description_done)}</strong>.
                  </p>
                </div>
              </div>
              {canManage && (
                <Button
                  size="sm"
                  onClick={handleConnect}
                  disabled={connecting || !configured}
                  title={!configured ? t(($) => $.integrations.connect_disabled_tooltip) : undefined}
                >
                  {connecting
                    ? t(($) => $.integrations.connect_opening)
                    : t(($) => $.integrations.connect_github)}
                </Button>
              )}
            </div>

            {canManage && !configured && (
              <p className="text-xs text-muted-foreground">
                {t(($) => $.integrations.not_configured)}{" "}
                <code className="rounded bg-muted px-1 py-0.5 text-[10px]">GITHUB_APP_SLUG</code>{" "}
                {t(($) => $.integrations.not_configured_and)}{" "}
                <code className="rounded bg-muted px-1 py-0.5 text-[10px]">GITHUB_WEBHOOK_SECRET</code>.
              </p>
            )}

            {!canManage && (
              <p className="text-xs text-muted-foreground">
                {t(($) => $.integrations.manage_hint)}
              </p>
            )}
          </CardContent>
        </Card>
      </section>

      {ringCentralProviders.length > 0 && (
        selectedRingCentralProvider ? (
          <RingCentralProviderDetail
            provider={selectedRingCentralProvider}
            credential={connectorCredentialsByProvider.get(selectedRingCentralProvider.id) ?? null}
            workspaceConnector={workspaceConnectorsByProvider.get(selectedRingCentralProvider.id) ?? null}
            workspaceId={wsId}
            canManage={canManage}
            onBack={() => setSelectedRingCentralProviderID(null)}
            onWorkspaceConnectorChanged={() =>
              queryClient.invalidateQueries({ queryKey: connectorKeys.workspace(wsId) })
            }
          />
        ) : (
          <section className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="space-y-1">
                <h2 className="text-sm font-semibold">
                  {t(($) => $.integrations.ringcentral_section_title)}
                </h2>
                <p className="text-xs text-muted-foreground">
                  {RINGCENTRAL_SECTION_DESCRIPTION}
                </p>
              </div>
              <Badge variant="secondary" className="rounded-sm">
                {t(($) => $.integrations.ringcentral_profile_enabled)}
              </Badge>
            </div>

            <Card>
              <CardContent>
                <div className="divide-y">
                  {ringCentralProviders.map((provider) => (
                    <RingCentralProviderRow
                      key={provider.id}
                      provider={provider}
                      credential={connectorCredentialsByProvider.get(provider.id) ?? null}
                      workspaceConnector={workspaceConnectorsByProvider.get(provider.id) ?? null}
                      workspaceId={wsId}
                      canManage={canManage}
                      onSelect={() => setSelectedRingCentralProviderID(provider.id)}
                      onCredentialChanged={() =>
                        queryClient.invalidateQueries({ queryKey: connectorKeys.credentials(wsId) })
                      }
                      onWorkspaceConnectorChanged={() =>
                        queryClient.invalidateQueries({ queryKey: connectorKeys.workspace(wsId) })
                      }
                    />
                  ))}
                </div>
                {!canManage && (
                  <p className="border-t px-3 py-2 text-xs text-muted-foreground">
                    {t(($) => $.integrations.manage_hint)}
                  </p>
                )}
              </CardContent>
            </Card>
          </section>
        )
      )}
    </div>
  );
}

function RingCentralProviderRow({
  provider,
  credential,
  workspaceConnector,
  workspaceId,
  canManage,
  onSelect,
  onCredentialChanged,
  onWorkspaceConnectorChanged,
}: {
  provider: ConnectorProvider;
  credential: ConnectorCredential | null;
  workspaceConnector: WorkspaceConnector | null;
  workspaceId: string;
  canManage: boolean;
  onSelect: () => void;
  onCredentialChanged: () => void;
  onWorkspaceConnectorChanged: () => void;
}) {
  const { t } = useT("settings");
  const [secret, setSecret] = useState("");
  const [savingCredential, setSavingCredential] = useState(false);
  const [deletingCredential, setDeletingCredential] = useState(false);
  const [savingWorkspace, setSavingWorkspace] = useState(false);
  const copy = ringCentralProviderCopy(provider);
  const enabled = workspaceConnector?.enabled ?? true;
  const endpoint = primaryEndpoint(provider, workspaceConnector);
  const credentialLabel = credentialStatusLabel(credential);
  const customEndpoint = hasEndpointOverrides(workspaceConnector);

  async function handleEnabledChange(checked: boolean) {
    setSavingWorkspace(true);
    try {
      await api.updateWorkspaceConnector(workspaceId, provider.id, {
        enabled: checked,
        settings: workspaceConnector?.settings ?? {},
      });
      onWorkspaceConnectorChanged();
      toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
    } finally {
      setSavingWorkspace(false);
    }
  }

  async function handleSaveCredential() {
    const nextSecret = secret.trim();
    if (!nextSecret) return;
    setSavingCredential(true);
    try {
      await api.saveConnectorCredential(workspaceId, provider.id, { secret: nextSecret });
      setSecret("");
      onCredentialChanged();
      toast.success(t(($) => $.integrations.ringcentral_credential_saved_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_credential_save_failed));
    } finally {
      setSavingCredential(false);
    }
  }

  async function handleDeleteCredential() {
    setDeletingCredential(true);
    try {
      await api.deleteConnectorCredential(workspaceId, provider.id);
      onCredentialChanged();
      toast.success(t(($) => $.integrations.ringcentral_credential_removed_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_credential_remove_failed));
    } finally {
      setDeletingCredential(false);
    }
  }

  return (
    <div className="py-4 first:pt-0 last:pb-0">
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border bg-muted/30">
            {ringCentralIcon(provider.id)}
          </div>
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="truncate text-sm font-medium">{copy.title}</span>
              <Badge variant={enabled ? "secondary" : "outline"} className="rounded-sm">
                {enabled
                  ? t(($) => $.integrations.ringcentral_provider_enabled)
                  : t(($) => $.integrations.ringcentral_provider_disabled)}
              </Badge>
              <Badge variant={credential?.has_credential ? "secondary" : "outline"} className="rounded-sm">
                {credentialLabel}
              </Badge>
              {endpoint && (
                <span className="truncate text-xs text-muted-foreground" title={endpoint}>
                  {endpoint}
                </span>
              )}
              {customEndpoint && (
                <Badge variant="outline" className="rounded-sm">
                  {CUSTOM_SERVICE_LABEL}
                </Badge>
              )}
            </div>
            <p className="max-w-3xl text-sm text-muted-foreground">{copy.description}</p>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onSelect}>
            {DETAILS_LABEL}
            <ChevronRight className="h-4 w-4" />
          </Button>
          {canManage && (
            <Switch
              checked={enabled}
              onCheckedChange={handleEnabledChange}
              disabled={savingWorkspace}
              aria-label={`Toggle ${copy.title} integration`}
            />
          )}
        </div>
      </div>

      {canManage && provider.requires_user_credential && (
        <div className="mt-3 grid gap-2 pl-0 sm:grid-cols-[minmax(220px,420px)_max-content_max-content] sm:pl-[52px]">
          <Input
            type="password"
            value={secret}
            onChange={(event) => setSecret(event.target.value)}
            placeholder={copy.tokenPlaceholder}
            autoComplete="off"
            className="h-9"
          />
          <Button size="sm" onClick={handleSaveCredential} disabled={savingCredential || !secret.trim()}>
            {savingCredential ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            {t(($) => $.integrations.ringcentral_save_token)}
          </Button>
          {credential?.has_credential && (
            <Button size="sm" variant="outline" onClick={handleDeleteCredential} disabled={deletingCredential}>
              {deletingCredential ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
              {t(($) => $.integrations.ringcentral_remove_token)}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

function RingCentralProviderDetail({
  provider,
  credential,
  workspaceConnector,
  workspaceId,
  canManage,
  onBack,
  onWorkspaceConnectorChanged,
}: {
  provider: ConnectorProvider;
  credential: ConnectorCredential | null;
  workspaceConnector: WorkspaceConnector | null;
  workspaceId: string;
  canManage: boolean;
  onBack: () => void;
  onWorkspaceConnectorChanged: () => void;
}) {
  const { t } = useT("settings");
  const [savingWorkspace, setSavingWorkspace] = useState(false);
  const [editingEndpoints, setEditingEndpoints] = useState(false);
  const [savingEndpoints, setSavingEndpoints] = useState(false);
  const [endpointDraft, setEndpointDraft] = useState<Record<string, string>>({});
  const copy = ringCentralProviderCopy(provider);
  const endpoints = endpointEntries(provider, workspaceConnector);
  const enabled = workspaceConnector?.enabled ?? true;
  const remoteWritePolicy = workspaceConnector?.settings?.remote_write_policy ?? "disabled";
  const credentialLabel = credentialStatusLabel(credential);
  const providerEndpoints = provider.endpoints ?? {};
  const customEndpoints = endpointOverrides(workspaceConnector);
  const hasCustomEndpoints = Object.keys(customEndpoints).length > 0;

  async function handleEnabledChange(checked: boolean) {
    setSavingWorkspace(true);
    try {
      await api.updateWorkspaceConnector(workspaceId, provider.id, {
        enabled: checked,
        settings: workspaceConnector?.settings ?? {},
      });
      onWorkspaceConnectorChanged();
      toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
    } finally {
      setSavingWorkspace(false);
    }
  }

  function beginEndpointEdit() {
    setEndpointDraft(effectiveEndpoints(provider, workspaceConnector));
    setEditingEndpoints(true);
  }

  async function saveEndpointOverrides(useDefaults = false) {
    const nextSettings = { ...(workspaceConnector?.settings ?? {}) };
    const nextOverrides: Record<string, string> = {};
    if (!useDefaults) {
      for (const [key, defaultValue] of Object.entries(providerEndpoints)) {
        const value = endpointDraft[key]?.trim() ?? "";
        if (value && value !== defaultValue) {
          nextOverrides[key] = value;
        }
      }
    }
    if (Object.keys(nextOverrides).length > 0) {
      nextSettings.endpoint_overrides = nextOverrides;
    } else {
      delete nextSettings.endpoint_overrides;
    }
    setSavingEndpoints(true);
    try {
      await api.updateWorkspaceConnector(workspaceId, provider.id, {
        enabled,
        settings: nextSettings,
      });
      setEditingEndpoints(false);
      onWorkspaceConnectorChanged();
      toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
    } finally {
      setSavingEndpoints(false);
    }
  }

  return (
    <section className="space-y-4">
      <Button variant="ghost" size="sm" className="-ml-2 text-muted-foreground" onClick={onBack}>
        <ArrowLeft className="h-4 w-4" />
        {RINGCENTRAL_LABEL}
      </Button>

      <Card>
        <CardContent className="space-y-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border bg-muted/30">
                {ringCentralIcon(provider.id)}
              </div>
              <div className="min-w-0 space-y-1">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <h2 className="truncate text-base font-semibold">{copy.title}</h2>
                  <Badge variant={enabled ? "secondary" : "outline"} className="rounded-sm">
                    {enabled
                      ? t(($) => $.integrations.ringcentral_provider_enabled)
                      : t(($) => $.integrations.ringcentral_provider_disabled)}
                  </Badge>
                  <Badge variant={credential?.has_credential ? "secondary" : "outline"} className="rounded-sm">
                    {credentialLabel}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground">{copy.description}</p>
              </div>
            </div>
            {canManage && (
              <Switch
                checked={enabled}
                onCheckedChange={handleEnabledChange}
                disabled={savingWorkspace}
                aria-label={`Toggle ${copy.title} integration`}
              />
            )}
          </div>

          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
            <section className="space-y-3">
              <div>
                <h3 className="text-sm font-semibold">{SETTINGS_LABEL}</h3>
                <div className="mt-1 flex items-center gap-2 text-sm text-muted-foreground">
                  <ShieldCheck className="h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{t(($) => $.integrations.ringcentral_task_scoped)}</span>
                </div>
              </div>

              {canManage && provider.remote_write_policies && provider.remote_write_policies.length > 0 && (
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">{REMOTE_WRITE_ACCESS_LABEL}</span>
                  <select
                    value={remoteWritePolicy}
                    onChange={async (event) => {
                      setSavingWorkspace(true);
                      try {
                        await api.updateWorkspaceConnector(workspaceId, provider.id, {
                          enabled,
                          settings: { ...(workspaceConnector?.settings ?? {}), remote_write_policy: event.target.value },
                        });
                        onWorkspaceConnectorChanged();
                        toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
                      } catch (e) {
                        toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
                      } finally {
                        setSavingWorkspace(false);
                      }
                    }}
                    disabled={savingWorkspace}
                    className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                  >
                    {provider.remote_write_policies.map((policy) => (
                      <option key={policy.id} value={policy.id}>
                        {remoteWritePolicyLabel(policy.id, policy.display_name)}
                      </option>
                    ))}
                  </select>
                </label>
              )}

              <section className="space-y-3 rounded-md border p-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h4 className="text-sm font-medium">{SERVICE_ADDRESSES_LABEL}</h4>
                    {hasCustomEndpoints && !editingEndpoints && (
                      <p className="text-xs text-muted-foreground">{CUSTOM_SERVICE_LABEL}</p>
                    )}
                  </div>
                  {canManage && !editingEndpoints && (
                    <Button variant="outline" size="sm" onClick={beginEndpointEdit}>
                      <Pencil className="h-4 w-4" />
                      {EDIT_LABEL}
                    </Button>
                  )}
                </div>

                {editingEndpoints ? (
                  <div className="space-y-3">
                    <div className="space-y-2">
                      {Object.entries(providerEndpoints).map(([key, defaultValue]) => (
                        <label key={key} className="space-y-1.5 text-sm">
                          <span className="text-muted-foreground">{endpointLabel(key, copy.title)}</span>
                          <Input
                            value={endpointDraft[key] ?? defaultValue}
                            onChange={(event) =>
                              setEndpointDraft((current) => ({ ...current, [key]: event.target.value }))
                            }
                            placeholder={defaultValue}
                            autoComplete="off"
                            className="h-9"
                          />
                        </label>
                      ))}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" onClick={() => saveEndpointOverrides()} disabled={savingEndpoints}>
                        {savingEndpoints ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                        {SAVE_ADDRESSES_LABEL}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => saveEndpointOverrides(true)}
                        disabled={savingEndpoints}
                      >
                        <RotateCcw className="h-4 w-4" />
                        {USE_DEFAULTS_LABEL}
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => setEditingEndpoints(false)}
                        disabled={savingEndpoints}
                      >
                        {CANCEL_LABEL}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {Object.entries(providerEndpoints).map(([key, defaultValue]) => {
                      const value = customEndpoints[key] ?? defaultValue;
                      const custom = customEndpoints[key] != null;
                      return (
                        <div
                          key={key}
                          className="grid gap-1 text-sm sm:grid-cols-[112px_minmax(0,1fr)_max-content]"
                        >
                          <span className="text-muted-foreground">{endpointLabel(key, copy.title)}</span>
                          <span className="min-w-0 break-words">{value}</span>
                          <Badge variant={custom ? "secondary" : "outline"} className="w-fit rounded-sm">
                            {custom ? CUSTOM_LABEL : DEFAULT_LABEL}
                          </Badge>
                        </div>
                      );
                    })}
                  </div>
                )}
              </section>

              {!canManage && (
                <p className="text-sm text-muted-foreground">{t(($) => $.integrations.manage_hint)}</p>
              )}
            </section>

            <div className="space-y-5">
              <section className="space-y-2">
                <h3 className="text-sm font-semibold">{INFORMATION_LABEL}</h3>
                <div className="overflow-hidden rounded-md border">
                  <InfoRow
                    label={STATUS_LABEL}
                    value={
                      enabled
                        ? t(($) => $.integrations.ringcentral_provider_enabled)
                        : t(($) => $.integrations.ringcentral_provider_disabled)
                    }
                  />
                  <InfoRow label={CREDENTIAL_LABEL} value={credentialLabel} />
                  {endpoints.map(([key, value]) => (
                    <InfoRow
                      key={`${key}-${value}`}
                      label={endpointLabel(key, copy.title)}
                      value={compactEndpoint(value)}
                      title={value}
                    />
                  ))}
                </div>
              </section>

              <section className="space-y-2">
                <h3 className="text-sm font-semibold">{AGENT_USE_LABEL}</h3>
                <div className="rounded-md border p-3">
                  <ul className="space-y-2 text-sm text-muted-foreground">
                    {copy.agentUse.map((item) => (
                      <li key={item} className="flex gap-2">
                        <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-muted-foreground/60" />
                        <span>{item}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </section>
            </div>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

function InfoRow({ label, value, title }: { label: string; value: string; title?: string }) {
  return (
    <div className="grid gap-1 border-b px-3 py-2 text-sm last:border-b-0 sm:grid-cols-[112px_minmax(0,1fr)]">
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 break-words text-foreground" title={title ?? value}>
        {value}
      </div>
    </div>
  );
}
