package handler

import (
	"encoding/json"
	"net/http"

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

type SaveConnectorCredentialRequest struct {
	Secret string `json:"secret"`
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
		Status:           "never_validated",
		UpstreamIdentity: []byte("{}"),
		LastValidatedAt:  pgtype.Timestamptz{},
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
