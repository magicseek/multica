package connectors

import "slices"

type ProviderDefinition struct {
	ID                     string              `json:"id"`
	DisplayName            string              `json:"display_name"`
	Profile                string              `json:"profile"`
	Capabilities           []Capability        `json:"capabilities"`
	ResourceTypes          []string            `json:"resource_types"`
	RequiresUserCredential bool                `json:"requires_user_credential"`
	Endpoints              map[string]string   `json:"endpoints,omitempty"`
	Metadata               map[string]string   `json:"metadata,omitempty"`
	RemoteWritePolicies    []RemoteWritePolicy `json:"remote_write_policies,omitempty"`
}

type Capability struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Write       bool   `json:"write"`
}

type RemoteWritePolicy struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Registry struct {
	providers []ProviderDefinition
	byID      map[string]ProviderDefinition
}

func NewRegistry(cfg Config) *Registry {
	r := &Registry{byID: make(map[string]ProviderDefinition)}
	if cfg.RingCentral.Enabled {
		r.register(ringCentralProviders(cfg.RingCentral)...)
	}
	return r
}

func (r *Registry) Providers() []ProviderDefinition {
	out := make([]ProviderDefinition, len(r.providers))
	copy(out, r.providers)
	return out
}

func (r *Registry) Get(id string) (ProviderDefinition, bool) {
	if r == nil {
		return ProviderDefinition{}, false
	}
	provider, ok := r.byID[id]
	return provider, ok
}

func (r *Registry) Enabled(id string) bool {
	_, ok := r.Get(id)
	return ok
}

func (r *Registry) register(providers ...ProviderDefinition) {
	for _, provider := range providers {
		if provider.ID == "" {
			continue
		}
		if _, exists := r.byID[provider.ID]; exists {
			continue
		}
		provider.Capabilities = sortedCapabilities(provider.Capabilities)
		provider.ResourceTypes = sortedStrings(provider.ResourceTypes)
		r.providers = append(r.providers, provider)
		r.byID[provider.ID] = provider
	}
	slices.SortFunc(r.providers, func(a, b ProviderDefinition) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func sortedCapabilities(in []Capability) []Capability {
	out := make([]Capability, len(in))
	copy(out, in)
	slices.SortFunc(out, func(a, b Capability) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}

func sortedStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	slices.Sort(out)
	return out
}

func ringCentralProviders(cfg RingCentralConfig) []ProviderDefinition {
	return []ProviderDefinition{
		{
			ID:                     ProviderRingCentralGitLab,
			DisplayName:            "RingCentral GitLab",
			Profile:                ProfileRingCentral,
			RequiresUserCredential: true,
			Endpoints: map[string]string{
				"api_base_url": cfg.GitLabAPIBaseURL,
				"web_base_url": cfg.GitLabWebBaseURL,
			},
			ResourceTypes: []string{ResourceRingCentralGitLabRepo},
			Capabilities: []Capability{
				{ID: "validate", DisplayName: "Validate token", Write: false},
				{ID: "repo.read", DisplayName: "Read repository files", Write: false},
				{ID: "branch.list", DisplayName: "List branches", Write: false},
				{ID: "merge_request.list", DisplayName: "List merge requests", Write: false},
				{ID: "branch.create", DisplayName: "Create semantic task branch", Write: true},
				{ID: "commit.create", DisplayName: "Commit file changes", Write: true},
				{ID: "merge_request.create", DisplayName: "Create merge request", Write: true},
			},
			RemoteWritePolicies: []RemoteWritePolicy{
				{ID: "disabled", DisplayName: "Disabled"},
				{ID: "merge_request_preparation", DisplayName: "Merge request preparation"},
			},
		},
		{
			ID:                     ProviderRingCentralJira,
			DisplayName:            "RingCentral Jira",
			Profile:                ProfileRingCentral,
			RequiresUserCredential: true,
			Endpoints: map[string]string{
				"base_url": cfg.JiraBaseURL,
			},
			ResourceTypes: []string{ResourceRingCentralJiraIssue, ResourceRingCentralJiraProject},
			Capabilities: []Capability{
				{ID: "validate", DisplayName: "Validate token", Write: false},
				{ID: "project.list", DisplayName: "List projects", Write: false},
				{ID: "issue.search", DisplayName: "Search issues", Write: false},
				{ID: "issue.read", DisplayName: "Read issue", Write: false},
			},
		},
		{
			ID:                     ProviderRingCentralWiki,
			DisplayName:            "RingCentral Wiki",
			Profile:                ProfileRingCentral,
			RequiresUserCredential: true,
			Endpoints: map[string]string{
				"base_url": cfg.WikiBaseURL,
			},
			ResourceTypes: []string{ResourceRingCentralWikiPage, ResourceRingCentralWikiSpace},
			Capabilities: []Capability{
				{ID: "validate", DisplayName: "Validate token", Write: false},
				{ID: "space.search", DisplayName: "Search spaces", Write: false},
				{ID: "page.search", DisplayName: "Search pages", Write: false},
				{ID: "page.read", DisplayName: "Read page", Write: false},
			},
		},
	}
}
