"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { BookOpen, GitBranch, KeyRound, Loader2, Save, ShieldCheck, Ticket, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
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
  const className = "h-5 w-5 text-muted-foreground";
  if (providerID.includes("gitlab")) return <GitBranch className={className} aria-hidden="true" />;
  if (providerID.includes("jira")) return <Ticket className={className} aria-hidden="true" />;
  return <BookOpen className={className} aria-hidden="true" />;
}

function compactEndpoint(value: string) {
  try {
    return new URL(value).host;
  } catch {
    return value.replace(/^https?:\/\//, "").replace(/\/$/, "");
  }
}

export function IntegrationsTab() {
  const { t } = useT("settings");
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  const user = useAuthStore((s) => s.user);
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const [connecting, setConnecting] = useState(false);

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
        <section className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-sm font-semibold">
              {t(($) => $.integrations.ringcentral_section_title)}
            </h2>
            <Badge variant="secondary" className="rounded-sm">
              {t(($) => $.integrations.ringcentral_profile_enabled)}
            </Badge>
          </div>

          <div className="grid gap-3 lg:grid-cols-3">
            {ringCentralProviders.map((provider) => (
              <RingCentralProviderCard
                key={provider.id}
                provider={provider}
                credential={connectorCredentialsByProvider.get(provider.id) ?? null}
                workspaceConnector={workspaceConnectorsByProvider.get(provider.id) ?? null}
                workspaceId={wsId}
                canManage={canManage}
                onCredentialChanged={() =>
                  queryClient.invalidateQueries({ queryKey: connectorKeys.credentials(wsId) })
                }
                onWorkspaceConnectorChanged={() =>
                  queryClient.invalidateQueries({ queryKey: connectorKeys.workspace(wsId) })
                }
              />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function RingCentralProviderCard({
  provider,
  credential,
  workspaceConnector,
  workspaceId,
  canManage,
  onCredentialChanged,
  onWorkspaceConnectorChanged,
}: {
  provider: ConnectorProvider;
  credential: ConnectorCredential | null;
  workspaceConnector: WorkspaceConnector | null;
  workspaceId: string;
  canManage: boolean;
  onCredentialChanged: () => void;
  onWorkspaceConnectorChanged: () => void;
}) {
  const { t } = useT("settings");
  const [secret, setSecret] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const endpointValues = Object.values(provider.endpoints ?? {}).filter(Boolean);
  const writeCount = provider.capabilities.filter((capability) => capability.write).length;
  const readCount = provider.capabilities.length - writeCount;
  const enabled = workspaceConnector?.enabled ?? true;
  const remoteWritePolicy = workspaceConnector?.settings?.remote_write_policy ?? "disabled";
  const status = credential?.has_credential
    ? t(($) => $.integrations.ringcentral_credential_saved)
    : t(($) => $.integrations.ringcentral_credential_missing);

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
    <Card>
      <CardContent className="space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 flex h-8 w-8 items-center justify-center rounded-md border bg-muted/40">
              {ringCentralIcon(provider.id)}
            </div>
            <div className="min-w-0 space-y-1">
              <p className="truncate text-sm font-medium">{provider.display_name}</p>
              <p className="text-xs text-muted-foreground">
                {provider.resource_types.join(", ")}
              </p>
            </div>
          </div>
          {provider.requires_user_credential && (
            <KeyRound className="mt-1 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          )}
        </div>

        <div className="flex flex-wrap gap-1.5">
          <Badge variant={credential?.has_credential ? "secondary" : "outline"} className="rounded-sm px-1.5 py-0 text-[10px]">
            {status}
          </Badge>
          <Badge variant={enabled ? "secondary" : "outline"} className="rounded-sm px-1.5 py-0 text-[10px]">
            {enabled
              ? t(($) => $.integrations.ringcentral_provider_enabled)
              : t(($) => $.integrations.ringcentral_provider_disabled)}
          </Badge>
          <Badge variant="outline" className="rounded-sm px-1.5 py-0 text-[10px]">
            {readCount} {t(($) => $.integrations.ringcentral_read_capabilities)}
          </Badge>
          {writeCount > 0 && (
            <Badge variant="secondary" className="rounded-sm px-1.5 py-0 text-[10px]">
              {writeCount} {t(($) => $.integrations.ringcentral_write_capabilities)}
            </Badge>
          )}
        </div>

        {endpointValues.length > 0 && (
          <div className="space-y-1">
            {endpointValues.map((endpoint) => (
              <div key={endpoint} className="truncate text-xs text-muted-foreground">
                {compactEndpoint(endpoint)}
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <ShieldCheck className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span>{t(($) => $.integrations.ringcentral_task_scoped)}</span>
        </div>

        {canManage && (
          <div className="space-y-2 rounded-md border p-2">
            <label className="flex items-center justify-between gap-3 text-xs">
              <span>{t(($) => $.integrations.ringcentral_provider_enable_label)}</span>
              <input
                type="checkbox"
                checked={enabled}
                onChange={async (event) => {
                  try {
                    await api.updateWorkspaceConnector(workspaceId, provider.id, {
                      enabled: event.target.checked,
                      settings: workspaceConnector?.settings ?? {},
                    });
                    onWorkspaceConnectorChanged();
                    toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
                  }
                }}
              />
            </label>
            {provider.remote_write_policies && provider.remote_write_policies.length > 0 && (
              <label className="space-y-1 text-xs">
                <span className="text-muted-foreground">
                  {t(($) => $.integrations.ringcentral_write_policy_label)}
                </span>
                <select
                  value={remoteWritePolicy}
                  onChange={async (event) => {
                    try {
                      await api.updateWorkspaceConnector(workspaceId, provider.id, {
                        enabled,
                        settings: { ...(workspaceConnector?.settings ?? {}), remote_write_policy: event.target.value },
                      });
                      onWorkspaceConnectorChanged();
                      toast.success(t(($) => $.integrations.ringcentral_workspace_saved_toast));
                    } catch (e) {
                      toast.error(e instanceof Error ? e.message : t(($) => $.integrations.ringcentral_workspace_save_failed));
                    }
                  }}
                  className="h-8 w-full rounded-md border bg-background px-2 text-xs"
                >
                  {provider.remote_write_policies.map((policy) => (
                    <option key={policy.id} value={policy.id}>
                      {policy.display_name}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
        )}

        {provider.requires_user_credential && (
          <div className="flex flex-col gap-2">
            <Input
              type="password"
              value={secret}
              onChange={(event) => setSecret(event.target.value)}
              placeholder={t(($) => $.integrations.ringcentral_token_placeholder)}
              autoComplete="off"
            />
            <div className="flex gap-2">
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
      </CardContent>
    </Card>
  );
}
