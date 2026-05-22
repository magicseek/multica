package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/connectors"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ConnectorProvidersResponse struct {
	Providers []connectors.ProviderDefinition `json:"providers"`
}

type ConnectorCredentialResponse struct {
	ProviderID      string  `json:"provider_id"`
	Status          string  `json:"status"`
	HasCredential   bool    `json:"has_credential"`
	LastValidatedAt *string `json:"last_validated_at"`
	InvalidatedAt   *string `json:"invalidated_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type ConnectorCredentialsResponse struct {
	Credentials []ConnectorCredentialResponse `json:"credentials"`
}

type WorkspaceConnectorResponse struct {
	ProviderID string         `json:"provider_id"`
	Enabled    bool           `json:"enabled"`
	Settings   map[string]any `json:"settings"`
	UpdatedAt  *string        `json:"updated_at,omitempty"`
}

type WorkspaceConnectorsResponse struct {
	Connectors []WorkspaceConnectorResponse `json:"connectors"`
}

type SaveConnectorCredentialRequest struct {
	Secret string `json:"secret"`
}

type UpdateWorkspaceConnectorRequest struct {
	Enabled  *bool          `json:"enabled"`
	Settings map[string]any `json:"settings"`
}

func (h *Handler) ListConnectorProviders(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ConnectorRegistry == nil {
		writeJSON(w, http.StatusOK, ConnectorProvidersResponse{Providers: []connectors.ProviderDefinition{}})
		return
	}
	writeJSON(w, http.StatusOK, ConnectorProvidersResponse{Providers: h.cfg.ConnectorRegistry.Providers()})
}

func (h *Handler) ListConnectorCredentials(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListConnectorCredentialsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list connector credentials")
		return
	}

	credentials := make([]ConnectorCredentialResponse, 0, len(rows))
	for _, row := range rows {
		if uuidToString(row.OwnerUserID) != userID {
			continue
		}
		if h.cfg.ConnectorRegistry != nil && !h.cfg.ConnectorRegistry.Enabled(row.ProviderID) {
			continue
		}
		credentials = append(credentials, connectorCredentialToResponse(row))
	}
	writeJSON(w, http.StatusOK, ConnectorCredentialsResponse{Credentials: credentials})
}

func (h *Handler) ListWorkspaceConnectors(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	if h.cfg.ConnectorRegistry == nil {
		writeJSON(w, http.StatusOK, WorkspaceConnectorsResponse{Connectors: []WorkspaceConnectorResponse{}})
		return
	}
	rows, err := h.Queries.ListWorkspaceConnectors(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspace connectors")
		return
	}
	rowsByProvider := make(map[string]db.WorkspaceConnector, len(rows))
	for _, row := range rows {
		rowsByProvider[row.ProviderID] = row
	}
	resp := make([]WorkspaceConnectorResponse, 0, len(h.cfg.ConnectorRegistry.Providers()))
	for _, provider := range h.cfg.ConnectorRegistry.Providers() {
		settings := defaultWorkspaceConnectorSettings(provider)
		item := WorkspaceConnectorResponse{
			ProviderID: provider.ID,
			Enabled:    true,
			Settings:   settings,
		}
		if row, ok := rowsByProvider[provider.ID]; ok {
			item.Enabled = row.Enabled
			if len(row.Settings) > 0 {
				_ = json.Unmarshal(row.Settings, &settings)
				item.Settings = settings
			}
			updated := timestampToString(row.UpdatedAt)
			item.UpdatedAt = &updated
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, WorkspaceConnectorsResponse{Connectors: resp})
}

func (h *Handler) UpdateWorkspaceConnector(w http.ResponseWriter, r *http.Request) {
	workspaceIDRaw := h.resolveWorkspaceID(r)
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDRaw, "workspace id")
	if !ok {
		return
	}
	member, ok := h.workspaceMember(w, r, workspaceIDRaw)
	if !ok {
		return
	}
	if !roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}
	providerID := chi.URLParam(r, "providerID")
	provider, ok := h.connectorProvider(providerID)
	if !ok {
		writeError(w, http.StatusNotFound, "connector provider not found")
		return
	}
	var req UpdateWorkspaceConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	settings, err := normalizeWorkspaceConnectorSettings(provider, req.Settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode connector settings")
		return
	}
	userID, _ := requireUserID(w, r)
	row, err := h.Queries.UpsertWorkspaceConnector(r.Context(), db.UpsertWorkspaceConnectorParams{
		WorkspaceID: workspaceID,
		ProviderID:  provider.ID,
		Enabled:     enabled,
		Settings:    settingsJSON,
		CreatedBy:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workspace connector")
		return
	}
	writeJSON(w, http.StatusOK, workspaceConnectorToResponse(row, provider))
}

func (h *Handler) SaveConnectorCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	providerID := chi.URLParam(r, "providerID")
	if !h.connectorProviderEnabled(providerID) {
		writeError(w, http.StatusNotFound, "connector provider not found")
		return
	}
	if h.cfg.ConnectorVault == nil {
		writeError(w, http.StatusServiceUnavailable, "connector credential vault is not configured")
		return
	}

	var req SaveConnectorCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Secret == "" {
		writeError(w, http.StatusBadRequest, "secret is required")
		return
	}
	if h.cfg.ConnectorClients == nil {
		writeError(w, http.StatusServiceUnavailable, "connector provider client is not configured")
		return
	}
	validation, err := h.cfg.ConnectorClients.Validate(r.Context(), providerID, req.Secret)
	if err != nil {
		switch {
		case connectors.IsAuthError(err):
			writeError(w, http.StatusUnauthorized, "connector credential was rejected by upstream")
		case errors.Is(err, connectors.ErrUnsupportedProvider):
			writeError(w, http.StatusNotFound, "connector provider not found")
		default:
			writeError(w, http.StatusBadGateway, "failed to validate connector credential")
		}
		return
	}
	identity, err := json.Marshal(validation.Identity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode upstream identity")
		return
	}

	ownerUserID := parseUUID(userID)
	encrypted, err := h.cfg.ConnectorVault.Encrypt(req.Secret, connectorCredentialAssociatedData(workspaceID, providerID, ownerUserID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt connector credential")
		return
	}
	credential, err := h.Queries.UpsertConnectorCredential(r.Context(), db.UpsertConnectorCredentialParams{
		WorkspaceID:      workspaceID,
		ProviderID:       providerID,
		OwnerUserID:      ownerUserID,
		EncryptedSecret:  encrypted.Ciphertext,
		SecretNonce:      encrypted.Nonce,
		KeyID:            encrypted.KeyID,
		Status:           "valid",
		UpstreamIdentity: identity,
		LastValidatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save connector credential")
		return
	}
	writeJSON(w, http.StatusOK, connectorCredentialToResponse(credential))
}

func (h *Handler) DeleteConnectorCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	providerID := chi.URLParam(r, "providerID")
	if !h.connectorProviderEnabled(providerID) {
		writeError(w, http.StatusNotFound, "connector provider not found")
		return
	}
	if err := h.Queries.DeleteConnectorCredential(r.Context(), db.DeleteConnectorCredentialParams{
		WorkspaceID: workspaceID,
		ProviderID:  providerID,
		OwnerUserID: parseUUID(userID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete connector credential")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) connectorProviderEnabled(providerID string) bool {
	if providerID == "" || h.cfg.ConnectorRegistry == nil {
		return false
	}
	return h.cfg.ConnectorRegistry.Enabled(providerID)
}

func (h *Handler) connectorProvider(providerID string) (connectors.ProviderDefinition, bool) {
	if providerID == "" || h.cfg.ConnectorRegistry == nil {
		return connectors.ProviderDefinition{}, false
	}
	return h.cfg.ConnectorRegistry.Get(providerID)
}

func workspaceConnectorToResponse(row db.WorkspaceConnector, provider connectors.ProviderDefinition) WorkspaceConnectorResponse {
	settings := defaultWorkspaceConnectorSettings(provider)
	if len(row.Settings) > 0 {
		_ = json.Unmarshal(row.Settings, &settings)
	}
	updated := timestampToString(row.UpdatedAt)
	return WorkspaceConnectorResponse{
		ProviderID: row.ProviderID,
		Enabled:    row.Enabled,
		Settings:   settings,
		UpdatedAt:  &updated,
	}
}

func defaultWorkspaceConnectorSettings(provider connectors.ProviderDefinition) map[string]any {
	settings := map[string]any{}
	if len(provider.RemoteWritePolicies) > 0 {
		settings["remote_write_policy"] = "disabled"
	}
	return settings
}

func normalizeWorkspaceConnectorSettings(provider connectors.ProviderDefinition, raw map[string]any) (map[string]any, error) {
	settings := defaultWorkspaceConnectorSettings(provider)
	if len(raw) == 0 {
		return settings, nil
	}
	if len(provider.RemoteWritePolicies) > 0 {
		policy := strings.TrimSpace(asString(raw["remote_write_policy"]))
		if policy == "" {
			policy = asString(settings["remote_write_policy"])
		}
		if !slices.ContainsFunc(provider.RemoteWritePolicies, func(candidate connectors.RemoteWritePolicy) bool {
			return candidate.ID == policy
		}) {
			return nil, errors.New("invalid remote_write_policy")
		}
		settings["remote_write_policy"] = policy
	}
	return settings, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func connectorCredentialToResponse(row db.ConnectorCredential) ConnectorCredentialResponse {
	return ConnectorCredentialResponse{
		ProviderID:      row.ProviderID,
		Status:          row.Status,
		HasCredential:   len(row.EncryptedSecret) > 0,
		LastValidatedAt: timestampToPtr(row.LastValidatedAt),
		InvalidatedAt:   timestampToPtr(row.InvalidatedAt),
		UpdatedAt:       timestampToString(row.UpdatedAt),
	}
}

func connectorCredentialAssociatedData(workspaceID pgtype.UUID, providerID string, ownerUserID pgtype.UUID) []byte {
	return []byte(uuidToString(workspaceID) + ":" + providerID + ":" + uuidToString(ownerUserID))
}
