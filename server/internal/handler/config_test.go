package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/connectors"
)

func requireConnectorCredentialSchema(t *testing.T) {
	t.Helper()
	var ok bool
	if err := testPool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'connector_credential'
		)
	`).Scan(&ok); err != nil {
		t.Fatalf("inspect connector credential schema: %v", err)
	}
	if !ok {
		t.Skip("local test database has not applied connector foundation migration")
	}
}

func TestGetConfigIncludesRuntimeAuthConfig(t *testing.T) {
	origStorage := testHandler.Storage
	testHandler.Storage = &mockStorage{}
	defer func() { testHandler.Storage = origStorage }()

	t.Setenv("ALLOW_SIGNUP", "false")
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if cfg.CdnDomain != "cdn.example.com" {
		t.Fatalf("cdn_domain: want cdn.example.com, got %q", cfg.CdnDomain)
	}
	if cfg.AllowSignup {
		t.Fatalf("allow_signup: want false, got true")
	}
	if cfg.GoogleClientID != "google-client-id" {
		t.Fatalf("google_client_id: want google-client-id, got %q", cfg.GoogleClientID)
	}
	if cfg.PosthogKey != "phc_test" {
		t.Fatalf("posthog_key: want phc_test, got %q", cfg.PosthogKey)
	}
	if cfg.PosthogHost != "https://eu.i.posthog.com" {
		t.Fatalf("posthog_host: want https://eu.i.posthog.com, got %q", cfg.PosthogHost)
	}
	if cfg.AnalyticsEnvironment != "dev" {
		t.Fatalf("analytics_environment: want dev, got %q", cfg.AnalyticsEnvironment)
	}
}

func TestListConnectorProvidersDefaultEmpty(t *testing.T) {
	h := *testHandler
	h.cfg.ConnectorRegistry = nil

	req := newRequest(http.MethodGet, "/api/connectors/providers", nil)
	w := httptest.NewRecorder()

	h.ListConnectorProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListConnectorProviders: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body ConnectorProvidersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(body.Providers) != 0 {
		t.Fatalf("providers = %d, want 0", len(body.Providers))
	}
}

func TestListConnectorProvidersWithRingCentralProfile(t *testing.T) {
	h := *testHandler
	h.cfg.ConnectorRegistry = connectors.NewRegistry(connectors.Config{
		RingCentral: connectors.RingCentralConfig{
			Enabled:          true,
			GitLabAPIBaseURL: "https://git.example.test/api/v4",
			GitLabWebBaseURL: "https://git.example.test",
			JiraBaseURL:      "https://jira.example.test",
			WikiBaseURL:      "https://wiki.example.test",
		},
	})

	req := newRequest(http.MethodGet, "/api/connectors/providers", nil)
	w := httptest.NewRecorder()

	h.ListConnectorProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListConnectorProviders: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body ConnectorProvidersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(body.Providers) != 3 {
		t.Fatalf("providers = %d, want 3", len(body.Providers))
	}
	if body.Providers[0].ID != connectors.ProviderRingCentralGitLab {
		t.Fatalf("first provider = %q, want %q", body.Providers[0].ID, connectors.ProviderRingCentralGitLab)
	}
}

func TestConnectorCredentialLifecycle(t *testing.T) {
	requireConnectorCredentialSchema(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "glpat-secret-value" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": 100, "username": "connector-user"})
	}))
	defer upstream.Close()
	connectorConfig := connectors.Config{
		RingCentral: connectors.RingCentralConfig{
			Enabled:          true,
			GitLabAPIBaseURL: upstream.URL + "/api/v4",
			GitLabWebBaseURL: upstream.URL,
			JiraBaseURL:      upstream.URL,
			WikiBaseURL:      upstream.URL,
		},
	}
	h := *testHandler
	h.cfg.ConnectorRegistry = connectors.NewRegistry(connectorConfig)
	h.cfg.ConnectorClients = connectors.NewClientSet(connectorConfig, upstream.Client())
	vault, err := connectors.NewCredentialVault("test:v1", bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	h.cfg.ConnectorVault = vault
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM connector_credential WHERE workspace_id = $1`, testWorkspaceID)
	})

	req := newRequest(http.MethodPut, "/api/connectors/providers/ringcentral_gitlab/credential", map[string]any{
		"secret": "glpat-secret-value",
	})
	req = withURLParam(req, "providerID", connectors.ProviderRingCentralGitLab)
	w := httptest.NewRecorder()
	h.SaveConnectorCredential(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SaveConnectorCredential: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var saved ConnectorCredentialResponse
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved credential: %v", err)
	}
	if !saved.HasCredential || saved.Status != "valid" || saved.LastValidatedAt == nil {
		t.Fatalf("saved credential = %+v, want has_credential true, valid, and last_validated_at", saved)
	}

	var encrypted []byte
	if err := testPool.QueryRow(context.Background(), `
		SELECT encrypted_secret
		FROM connector_credential
		WHERE workspace_id = $1 AND provider_id = $2 AND owner_user_id = $3
	`, testWorkspaceID, connectors.ProviderRingCentralGitLab, testUserID).Scan(&encrypted); err != nil {
		t.Fatalf("load encrypted secret: %v", err)
	}
	if bytes.Contains(encrypted, []byte("glpat-secret-value")) {
		t.Fatal("encrypted_secret contains plaintext token")
	}

	req = newRequest(http.MethodGet, "/api/connectors/credentials", nil)
	w = httptest.NewRecorder()
	h.ListConnectorCredentials(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListConnectorCredentials: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var list ConnectorCredentialsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode credential list: %v", err)
	}
	if len(list.Credentials) != 1 || list.Credentials[0].ProviderID != connectors.ProviderRingCentralGitLab {
		t.Fatalf("credentials = %+v, want one gitlab credential", list.Credentials)
	}

	req = newRequest(http.MethodDelete, "/api/connectors/providers/ringcentral_gitlab/credential", nil)
	req = withURLParam(req, "providerID", connectors.ProviderRingCentralGitLab)
	w = httptest.NewRecorder()
	h.DeleteConnectorCredential(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteConnectorCredential: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}
