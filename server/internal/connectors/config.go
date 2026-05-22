package connectors

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"strings"
)

const (
	ProfileRingCentral = "ringcentral"

	ProviderRingCentralGitLab = "ringcentral_gitlab"
	ProviderRingCentralJira   = "ringcentral_jira"
	ProviderRingCentralWiki   = "ringcentral_wiki"

	envConnectorProfiles       = "MULTICA_CONNECTOR_PROFILES"
	envCredentialEncryptionKey = "CONNECTOR_CREDENTIAL_ENCRYPTION_KEY"
)

type Config struct {
	Profiles    []string
	RingCentral RingCentralConfig
	Vault       *CredentialVault
}

type RingCentralConfig struct {
	Enabled          bool
	GitLabAPIBaseURL string
	GitLabWebBaseURL string
	JiraBaseURL      string
	WikiBaseURL      string
}

func LoadConfigFromEnv() (Config, error) {
	profiles := parseProfiles(os.Getenv(envConnectorProfiles))
	cfg := Config{
		Profiles: profiles,
		RingCentral: RingCentralConfig{
			Enabled:          slices.Contains(profiles, ProfileRingCentral),
			GitLabAPIBaseURL: strings.TrimSpace(os.Getenv("RINGCENTRAL_GITLAB_API_BASE_URL")),
			GitLabWebBaseURL: strings.TrimSpace(os.Getenv("RINGCENTRAL_GITLAB_WEB_BASE_URL")),
			JiraBaseURL:      strings.TrimSpace(os.Getenv("RINGCENTRAL_JIRA_BASE_URL")),
			WikiBaseURL:      strings.TrimSpace(os.Getenv("RINGCENTRAL_WIKI_BASE_URL")),
		},
	}
	if !cfg.RingCentral.Enabled {
		return cfg, nil
	}

	vault, err := NewCredentialVaultFromKeyString(os.Getenv(envCredentialEncryptionKey))
	if err != nil {
		return Config{}, err
	}
	cfg.Vault = vault
	return cfg, nil
}

func parseProfiles(raw string) []string {
	parts := strings.Split(raw, ",")
	profiles := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		profile := strings.ToLower(strings.TrimSpace(part))
		if profile == "" {
			continue
		}
		if _, ok := seen[profile]; ok {
			continue
		}
		seen[profile] = struct{}{}
		profiles = append(profiles, profile)
	}
	return profiles
}

func decodeCredentialKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("ringcentral connector profile requires CONNECTOR_CREDENTIAL_ENCRYPTION_KEY")
	}

	if key, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(key) == credentialKeySize {
		return key, nil
	}
	if key, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(key) == credentialKeySize {
		return key, nil
	}
	if key, err := hex.DecodeString(trimmed); err == nil && len(key) == credentialKeySize {
		return key, nil
	}
	if len([]byte(trimmed)) == credentialKeySize {
		return []byte(trimmed), nil
	}
	return nil, errors.New("CONNECTOR_CREDENTIAL_ENCRYPTION_KEY must decode to 32 bytes")
}
