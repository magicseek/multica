"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  BookOpen,
  ChevronRight,
  GitBranch,
  KeyRound,
  Loader2,
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

function primaryEndpoint(provider: ConnectorProvider) {
  const endpoints = provider.endpoints ?? {};
  const endpoint =
    endpoints.web_base_url ?? endpoints.base_url ?? endpoints.api_base_url ?? Object.values(endpoints).find(Boolean);
  return endpoint ? compactEndpoint(endpoint) : null;
}

function endpointLabel(key: string, serviceName: string) {
  if (key === "api_base_url") return `${serviceName} API`;
  if (key === "web_base_url") return serviceName;
  if (key === "base_url") return serviceName;
  return serviceName;
}

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
            onCredentialChanged={() =>
              queryClient.invalidateQueries({ queryKey: connectorKeys.credentials(wsId) })
            }
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
                  {t(($) => $.integrations.ringcentral_section_description)}
                </p>
              </div>
              <Badge variant="secondary" className="rounded-sm">
                {t(($) => $.integrations.ringcentral_profile_enabled)}
              </Badge>
            </div>

            <Card>
              <CardContent className="p-0">
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
  onWorkspaceConnectorChanged,
}: {
  provider: ConnectorProvider;
  credential: ConnectorCredential | null;
  workspaceConnector: WorkspaceConnector | null;
  workspaceId: string;
  canManage: boolean;
  onSelect: () => void;
  onWorkspaceConnectorChanged: () => void;
}) {
  const { t } = useT("settings");
  const [savingWorkspace, setSavingWorkspace] = useState(false);
  const copy = useRingCentralProviderCopy(provider);
  const enabled = workspaceConnector?.enabled ?? true;
  const endpoint = primaryEndpoint(provider);
  const credentialLabel = credential?.has_credential
    ? credential.status === "invalid"
      ? t(($) => $.integrations.ringcentral_status_invalid_token)
      : t(($) => $.integrations.ringcentral_status_connected)
    : t(($) => $.integrations.ringcentral_status_needs_token);

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

  return (
    <div className="group flex items-center gap-2 px-3 py-2.5 transition-colors hover:bg-muted/30">
      <button
        type="button"
        className="flex min-w-0 flex-1 items-center gap-3 rounded-sm text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={onSelect}
      >
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border bg-muted/30">
          {ringCentralIcon(provider.id)}
        </div>
        <div className="min-w-0 flex-1 space-y-0.5">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <span className="truncate text-sm font-medium">{copy.title}</span>
            <Badge variant={enabled ? "secondary" : "outline"} className="rounded-sm px-1.5 py-0 text-[10px]">
              {enabled
                ? t(($) => $.integrations.ringcentral_provider_enabled)
                : t(($) => $.integrations.ringcentral_provider_disabled)}
            </Badge>
            <Badge
              variant={credential?.has_credential ? "secondary" : "outline"}
              className="rounded-sm px-1.5 py-0 text-[10px]"
            >
              {credentialLabel}
            </Badge>
          </div>
          <p className="truncate text-xs text-muted-foreground">{copy.description}</p>
        </div>
        {endpoint && (
          <div className="hidden max-w-[180px] truncate text-xs text-muted-foreground md:block" title={endpoint}>
            {endpoint}
          </div>
        )}
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
      </button>
      {canManage && (
        <Switch
          size="sm"
          checked={enabled}
          onCheckedChange={handleEnabledChange}
          disabled={savingWorkspace}
          aria-label={t(($) => $.integrations.ringcentral_enable_aria, { name: copy.title })}
        />
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
  onCredentialChanged,
  onWorkspaceConnectorChanged,
}: {
  provider: ConnectorProvider;
  credential: ConnectorCredential | null;
  workspaceConnector: WorkspaceConnector | null;
  workspaceId: string;
  canManage: boolean;
  onBack: () => void;
  onCredentialChanged: () => void;
  onWorkspaceConnectorChanged: () => void;
}) {
  const { t } = useT("settings");
  const [secret, setSecret] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [savingWorkspace, setSavingWorkspace] = useState(false);
  const copy = useRingCentralProviderCopy(provider);
  const endpoints = Object.entries(provider.endpoints ?? {}).filter(([, value]) => Boolean(value));
  const enabled = workspaceConnector?.enabled ?? true;
  const remoteWritePolicy = workspaceConnector?.settings?.remote_write_policy ?? "disabled";
  const credentialLabel = credential?.has_credential
    ? credential.status === "invalid"
      ? t(($) => $.integrations.ringcentral_status_invalid_token)
      : t(($) => $.integrations.ringcentral_status_connected)
    : t(($) => $.integrations.ringcentral_status_needs_token);

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

  async function handleSave() {
    const nextSecret = secret.trim();
    if (!nextSecret) return;
    setSaving(true);
    try {
      await api.saveConnectorCredential(workspaceId, provider.id, { secret: nextSecret });
      setSecret("");
      onCredentialChanged();
      toast.success(t(($) => $.integrations.ringcentral_credential_saved_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_credential_save_failed));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.deleteConnectorCredential(workspaceId, provider.id);
      onCredentialChanged();
      toast.success(t(($) => $.integrations.ringcentral_credential_removed_toast));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_credential_remove_failed));
    } finally {
      setDeleting(false);
    }
  }

  return (
    <section className="space-y-4">
      <div className="space-y-3">
        <Button variant="ghost" size="sm" className="-ml-2 text-muted-foreground" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" />
          {t(($) => $.integrations.ringcentral_detail_back)}
        </Button>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 items-start gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border bg-muted/30">
              {ringCentralIcon(provider.id)}
            </div>
            <div className="min-w-0 space-y-1">
              <div className="flex min-w-0 flex-wrap items-center gap-1.5">
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
              aria-label={t(($) => $.integrations.ringcentral_enable_aria, { name: copy.title })}
            />
          )}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Card>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <h3 className="text-sm font-semibold">
                {t(($) => $.integrations.ringcentral_settings_title)}
              </h3>
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <span>{t(($) => $.integrations.ringcentral_task_scoped)}</span>
              </div>
            </div>

            {canManage && provider.remote_write_policies && provider.remote_write_policies.length > 0 && (
              <label className="space-y-1.5 text-xs">
                <span className="font-medium">{t(($) => $.integrations.ringcentral_write_policy_label)}</span>
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
                  className="h-8 w-full rounded-md border bg-background px-2 text-xs"
                >
                  {provider.remote_write_policies.map((policy) => (
                    <option key={policy.id} value={policy.id}>
                      {policy.id === "disabled"
                        ? t(($) => $.integrations.ringcentral_write_policy_disabled)
                        : policy.id === "merge_request_preparation"
                          ? t(($) => $.integrations.ringcentral_write_policy_mr_preparation)
                          : policy.display_name}
                    </option>
                  ))}
                </select>
              </label>
            )}

            {provider.requires_user_credential && canManage && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium">
                  <KeyRound className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
                  {t(($) => $.integrations.ringcentral_credentials_title)}
                </div>
                <Input
                  type="password"
                  value={secret}
                  onChange={(event) => setSecret(event.target.value)}
                  placeholder={t(($) => $.integrations.ringcentral_token_placeholder)}
                  autoComplete="off"
                />
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" onClick={handleSave} disabled={saving || !secret.trim()}>
                    {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                    {t(($) => $.integrations.ringcentral_save_token)}
                  </Button>
                  {credential?.has_credential && (
                    <Button size="sm" variant="outline" onClick={handleDelete} disabled={deleting}>
                      {deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                      {t(($) => $.integrations.ringcentral_remove_token)}
                    </Button>
                  )}
                </div>
              </div>
            )}

            {!canManage && (
              <p className="text-xs text-muted-foreground">{t(($) => $.integrations.manage_hint)}</p>
            )}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <section className="space-y-2">
            <h3 className="text-sm font-semibold">
              {t(($) => $.integrations.ringcentral_information_title)}
            </h3>
            <div className="overflow-hidden rounded-md border">
              <InfoRow
                label={t(($) => $.integrations.ringcentral_status_label)}
                value={
                  enabled
                    ? t(($) => $.integrations.ringcentral_provider_enabled)
                    : t(($) => $.integrations.ringcentral_provider_disabled)
                }
              />
              <InfoRow
                label={t(($) => $.integrations.ringcentral_credentials_title)}
                value={credentialLabel}
              />
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
            <h3 className="text-sm font-semibold">
              {t(($) => $.integrations.ringcentral_agent_use_title)}
            </h3>
            <div className="rounded-md border p-3">
              <ul className="space-y-1.5 text-xs text-muted-foreground">
                {copy.agentUse.map((item) => (
                  <li key={item} className="flex gap-2">
                    <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-muted-foreground/60" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          </section>
        </div>
      </div>
    </section>
  );
}

function InfoRow({ label, value, title }: { label: string; value: string; title?: string }) {
  return (
    <div className="grid grid-cols-[120px_minmax(0,1fr)] border-b px-3 py-2 text-xs last:border-b-0">
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 truncate text-foreground" title={title ?? value}>
        {value}
      </div>
    </div>
  );
}

function useRingCentralProviderCopy(provider: ConnectorProvider) {
  const { t } = useT("settings");
  const kind = ringCentralProviderKind(provider.id);
  if (kind === "gitlab") {
    return {
      title: t(($) => $.integrations.ringcentral_gitlab_title),
      description: t(($) => $.integrations.ringcentral_gitlab_description),
      agentUse: [
        t(($) => $.integrations.ringcentral_gitlab_use_repo),
        t(($) => $.integrations.ringcentral_gitlab_use_mr),
        t(($) => $.integrations.ringcentral_gitlab_use_write),
      ],
    };
  }
  if (kind === "jira") {
    return {
      title: t(($) => $.integrations.ringcentral_jira_title),
      description: t(($) => $.integrations.ringcentral_jira_description),
      agentUse: [
        t(($) => $.integrations.ringcentral_jira_use_project),
        t(($) => $.integrations.ringcentral_jira_use_issue),
      ],
    };
  }
  if (kind === "wiki") {
    return {
      title: t(($) => $.integrations.ringcentral_wiki_title),
      description: t(($) => $.integrations.ringcentral_wiki_description),
      agentUse: [
        t(($) => $.integrations.ringcentral_wiki_use_space),
        t(($) => $.integrations.ringcentral_wiki_use_page),
      ],
    };
  }
  return {
    title: provider.display_name,
    description: t(($) => $.integrations.ringcentral_unknown_description),
    agentUse: [t(($) => $.integrations.ringcentral_unknown_use)],
  };
}
