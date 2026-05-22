import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const connectorKeys = {
  all: (wsId: string) => ["connectors", wsId] as const,
  providers: (wsId: string) => [...connectorKeys.all(wsId), "providers"] as const,
  credentials: (wsId: string) => [...connectorKeys.all(wsId), "credentials"] as const,
};

export const connectorProvidersOptions = (wsId: string) =>
  queryOptions({
    queryKey: connectorKeys.providers(wsId),
    queryFn: () => api.listConnectorProviders(wsId),
    enabled: !!wsId,
  });

export const connectorCredentialsOptions = (wsId: string) =>
  queryOptions({
    queryKey: connectorKeys.credentials(wsId),
    queryFn: () => api.listConnectorCredentials(wsId),
    enabled: !!wsId,
  });
