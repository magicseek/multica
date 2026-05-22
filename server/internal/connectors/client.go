package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrUnsupportedProvider = errors.New("connector provider is not supported")
	ErrUnauthorized        = errors.New("connector credential was rejected by upstream")
	ErrNotFound            = errors.New("connector upstream resource was not found")
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type ValidationResult struct {
	Identity map[string]string
}

type ClientSet struct {
	clients    map[string]ProviderClient
	httpClient HTTPDoer
}

type ProviderClient interface {
	Validate(ctx context.Context, token string) (ValidationResult, error)
}

type GitLabProvider interface {
	ProviderClient
	ListBranches(ctx context.Context, token string, req GitLabListBranchesRequest) ([]GitLabBranch, error)
	GetBranch(ctx context.Context, token string, req GitLabGetBranchRequest) (GitLabBranch, error)
	ListMergeRequests(ctx context.Context, token string, req GitLabListMergeRequestsRequest) ([]GitLabMergeRequest, error)
	CreateBranch(ctx context.Context, token string, req GitLabCreateBranchRequest) (GitLabBranch, error)
	CreateCommit(ctx context.Context, token string, req GitLabCreateCommitRequest) (GitLabCommit, error)
	CreateMergeRequest(ctx context.Context, token string, req GitLabCreateMergeRequestRequest) (GitLabMergeRequest, error)
}

type JiraProvider interface {
	ProviderClient
	ListProjects(ctx context.Context, token string) ([]JiraProject, error)
	SearchIssues(ctx context.Context, token string, req JiraSearchIssuesRequest) (JiraSearchIssuesResponse, error)
	GetIssue(ctx context.Context, token string, key string) (JiraIssue, error)
}

type WikiProvider interface {
	ProviderClient
	SearchSpaces(ctx context.Context, token string, req WikiSearchSpacesRequest) (WikiSearchSpacesResponse, error)
	SearchPages(ctx context.Context, token string, req WikiSearchPagesRequest) (WikiSearchPagesResponse, error)
	GetPage(ctx context.Context, token string, id string) (WikiPage, error)
}

func NewClientSet(cfg Config, httpClient HTTPDoer) *ClientSet {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	set := &ClientSet{clients: make(map[string]ProviderClient), httpClient: httpClient}
	if cfg.RingCentral.Enabled {
		set.clients[ProviderRingCentralGitLab] = NewGitLabClient(cfg.RingCentral.GitLabAPIBaseURL, cfg.RingCentral.GitLabWebBaseURL, httpClient)
		set.clients[ProviderRingCentralJira] = NewJiraClient(cfg.RingCentral.JiraBaseURL, httpClient)
		set.clients[ProviderRingCentralWiki] = NewWikiClient(cfg.RingCentral.WikiBaseURL, httpClient)
	}
	return set
}

func NewProviderClient(providerID string, endpoints map[string]string, httpClient HTTPDoer) (ProviderClient, bool) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	switch providerID {
	case ProviderRingCentralGitLab:
		return NewGitLabClient(endpoints["api_base_url"], endpoints["web_base_url"], httpClient), true
	case ProviderRingCentralJira:
		return NewJiraClient(endpoints["base_url"], httpClient), true
	case ProviderRingCentralWiki:
		return NewWikiClient(endpoints["base_url"], httpClient), true
	default:
		return nil, false
	}
}

func (s *ClientSet) Get(providerID string) (ProviderClient, bool) {
	if s == nil {
		return nil, false
	}
	client, ok := s.clients[providerID]
	return client, ok
}

func (s *ClientSet) GetWithEndpoints(providerID string, endpoints map[string]string) (ProviderClient, bool) {
	if s == nil {
		return nil, false
	}
	return NewProviderClient(providerID, endpoints, s.httpClient)
}

func (s *ClientSet) Validate(ctx context.Context, providerID, token string) (ValidationResult, error) {
	client, ok := s.Get(providerID)
	if !ok {
		return ValidationResult{}, ErrUnsupportedProvider
	}
	return client.Validate(ctx, token)
}

func (s *ClientSet) GitLab() (GitLabProvider, bool) {
	client, ok := s.Get(ProviderRingCentralGitLab)
	if !ok {
		return nil, false
	}
	gitlab, ok := client.(GitLabProvider)
	return gitlab, ok
}

func (s *ClientSet) Jira() (JiraProvider, bool) {
	client, ok := s.Get(ProviderRingCentralJira)
	if !ok {
		return nil, false
	}
	jira, ok := client.(JiraProvider)
	return jira, ok
}

func (s *ClientSet) Wiki() (WikiProvider, bool) {
	client, ok := s.Get(ProviderRingCentralWiki)
	if !ok {
		return nil, false
	}
	wiki, ok := client.(WikiProvider)
	return wiki, ok
}

type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

func IsAuthError(err error) bool {
	if errors.Is(err, ErrUnauthorized) {
		return true
	}
	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		return upstream.StatusCode == http.StatusUnauthorized || upstream.StatusCode == http.StatusForbidden
	}
	return false
}

func joinBaseURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func pathEscape(raw string) string {
	return url.PathEscape(strings.TrimSpace(raw))
}

func doJSON(ctx context.Context, client HTTPDoer, method, rawURL, token string, auth authMode, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch auth {
	case authPrivateToken:
		req.Header.Set("PRIVATE-TOKEN", token)
	case authBearer:
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &UpstreamError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type authMode int

const (
	authPrivateToken authMode = iota + 1
	authBearer
)

func stringMap(values map[string]any, keys ...string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := values[key]; ok {
			switch typed := value.(type) {
			case string:
				if typed != "" {
					out[key] = typed
				}
			case float64:
				out[key] = fmt.Sprintf("%.0f", typed)
			}
		}
	}
	return out
}
