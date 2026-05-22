package connectors

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadConfigFromEnvDefaultsToNoProviders(t *testing.T) {
	t.Setenv(envConnectorProfiles, "")
	t.Setenv(envCredentialEncryptionKey, "")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv: %v", err)
	}
	if cfg.RingCentral.Enabled {
		t.Fatalf("RingCentral profile should be disabled by default")
	}
	if cfg.Vault != nil {
		t.Fatalf("vault should be nil when no credential-bearing profile is enabled")
	}
	if got := NewRegistry(cfg).Providers(); len(got) != 0 {
		t.Fatalf("default registry providers = %d, want 0", len(got))
	}
}

func TestLoadConfigFromEnvRequiresCredentialKeyForRingCentral(t *testing.T) {
	t.Setenv(envConnectorProfiles, "ringcentral")
	t.Setenv(envCredentialEncryptionKey, "")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv should fail without credential key")
	}
	if !strings.Contains(err.Error(), envCredentialEncryptionKey) {
		t.Fatalf("error %q should mention %s", err.Error(), envCredentialEncryptionKey)
	}
}

func TestLoadConfigFromEnvRequiresEndpointsForRingCentral(t *testing.T) {
	t.Setenv(envConnectorProfiles, "ringcentral")
	t.Setenv(envCredentialEncryptionKey, base64.StdEncoding.EncodeToString(make([]byte, credentialKeySize)))
	t.Setenv("RINGCENTRAL_GITLAB_API_BASE_URL", "")
	t.Setenv("RINGCENTRAL_GITLAB_WEB_BASE_URL", "https://git.example.test")
	t.Setenv("RINGCENTRAL_JIRA_BASE_URL", "https://jira.example.test")
	t.Setenv("RINGCENTRAL_WIKI_BASE_URL", "https://wiki.example.test")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv should fail without GitLab API endpoint")
	}
	if !strings.Contains(err.Error(), "RINGCENTRAL_GITLAB_API_BASE_URL") {
		t.Fatalf("error %q should mention RINGCENTRAL_GITLAB_API_BASE_URL", err.Error())
	}
}

func TestLoadConfigFromEnvRegistersRingCentralProviders(t *testing.T) {
	t.Setenv(envConnectorProfiles, "ringcentral")
	t.Setenv(envCredentialEncryptionKey, base64.StdEncoding.EncodeToString(make([]byte, credentialKeySize)))
	t.Setenv("RINGCENTRAL_GITLAB_API_BASE_URL", "https://git.example.test/api/v4")
	t.Setenv("RINGCENTRAL_GITLAB_WEB_BASE_URL", "https://git.example.test")
	t.Setenv("RINGCENTRAL_JIRA_BASE_URL", "https://jira.example.test")
	t.Setenv("RINGCENTRAL_WIKI_BASE_URL", "https://wiki.example.test")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv: %v", err)
	}
	if !cfg.RingCentral.Enabled {
		t.Fatalf("RingCentral profile should be enabled")
	}
	if cfg.Vault == nil {
		t.Fatalf("vault should be configured")
	}
	providers := NewRegistry(cfg).Providers()
	if len(providers) != 3 {
		t.Fatalf("provider count = %d, want 3", len(providers))
	}
	if providers[0].ID != ProviderRingCentralGitLab {
		t.Fatalf("first provider = %q, want %q", providers[0].ID, ProviderRingCentralGitLab)
	}
	if providers[0].Endpoints["api_base_url"] != "https://git.example.test/api/v4" {
		t.Fatalf("gitlab api endpoint = %q", providers[0].Endpoints["api_base_url"])
	}
}
