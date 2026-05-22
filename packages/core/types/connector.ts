export interface ConnectorCapability {
  id: string;
  display_name: string;
  write: boolean;
}

export interface ConnectorRemoteWritePolicy {
  id: string;
  display_name: string;
}

export interface ConnectorProvider {
  id: string;
  display_name: string;
  profile: string;
  capabilities: ConnectorCapability[];
  resource_types: string[];
  requires_user_credential: boolean;
  endpoints?: Record<string, string>;
  metadata?: Record<string, string>;
  remote_write_policies?: ConnectorRemoteWritePolicy[];
}

export interface ConnectorProvidersResponse {
  providers: ConnectorProvider[];
}

export interface ConnectorCredential {
  provider_id: string;
  status: "never_validated" | "valid" | "invalid" | string;
  has_credential: boolean;
  last_validated_at: string | null;
  invalidated_at: string | null;
  updated_at: string;
}

export interface ConnectorCredentialsResponse {
  credentials: ConnectorCredential[];
}

export interface SaveConnectorCredentialRequest {
  secret: string;
}
