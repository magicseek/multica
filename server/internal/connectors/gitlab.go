package connectors

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type GitLabClient struct {
	apiBaseURL string
	webBaseURL string
	httpClient HTTPDoer
}

type GitLabBranch struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Protected bool   `json:"protected"`
	WebURL    string `json:"web_url,omitempty"`
}

type GitLabMergeRequest struct {
	ID           int64  `json:"id"`
	IID          int64  `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	WebURL       string `json:"web_url"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

type GitLabCommit struct {
	ID        string `json:"id"`
	ShortID   string `json:"short_id"`
	Title     string `json:"title"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
}

type GitLabListBranchesRequest struct {
	ProjectID string
	Search    string
}

type GitLabGetBranchRequest struct {
	ProjectID string
	Branch    string
}

type GitLabListMergeRequestsRequest struct {
	ProjectID string
	State     string
}

type GitLabCreateBranchRequest struct {
	ProjectID string
	Branch    string
	Ref       string
}

type GitLabCommitAction struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content,omitempty"`
}

type GitLabCreateCommitRequest struct {
	ProjectID     string
	Branch        string
	CommitMessage string
	Actions       []GitLabCommitAction
}

type GitLabCreateMergeRequestRequest struct {
	ProjectID    string
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	RemoveSource bool
}

func NewGitLabClient(apiBaseURL, webBaseURL string, httpClient HTTPDoer) *GitLabClient {
	return &GitLabClient{
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		webBaseURL: strings.TrimRight(webBaseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *GitLabClient) Validate(ctx context.Context, token string) (ValidationResult, error) {
	var body map[string]any
	if err := doJSON(ctx, c.httpClient, http.MethodGet, joinBaseURL(c.apiBaseURL, "/user"), token, authPrivateToken, nil, &body); err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Identity: stringMap(body, "id", "username", "name", "email")}, nil
}

func (c *GitLabClient) ListBranches(ctx context.Context, token string, req GitLabListBranchesRequest) ([]GitLabBranch, error) {
	q := url.Values{}
	q.Set("per_page", "100")
	if strings.TrimSpace(req.Search) != "" {
		q.Set("search", strings.TrimSpace(req.Search))
	}
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/repository/branches")
	if len(q) > 0 {
		rawURL += "?" + q.Encode()
	}
	var branches []GitLabBranch
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authPrivateToken, nil, &branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func (c *GitLabClient) GetBranch(ctx context.Context, token string, req GitLabGetBranchRequest) (GitLabBranch, error) {
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/repository/branches/"+pathEscape(req.Branch))
	var branch GitLabBranch
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authPrivateToken, nil, &branch); err != nil {
		return GitLabBranch{}, err
	}
	return branch, nil
}

func (c *GitLabClient) ListMergeRequests(ctx context.Context, token string, req GitLabListMergeRequestsRequest) ([]GitLabMergeRequest, error) {
	q := url.Values{}
	q.Set("per_page", "100")
	if strings.TrimSpace(req.State) != "" {
		q.Set("state", strings.TrimSpace(req.State))
	}
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/merge_requests")
	if len(q) > 0 {
		rawURL += "?" + q.Encode()
	}
	var mrs []GitLabMergeRequest
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authPrivateToken, nil, &mrs); err != nil {
		return nil, err
	}
	return mrs, nil
}

func (c *GitLabClient) CreateBranch(ctx context.Context, token string, req GitLabCreateBranchRequest) (GitLabBranch, error) {
	q := url.Values{}
	q.Set("branch", req.Branch)
	q.Set("ref", req.Ref)
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/repository/branches") + "?" + q.Encode()
	var branch GitLabBranch
	if err := doJSON(ctx, c.httpClient, http.MethodPost, rawURL, token, authPrivateToken, nil, &branch); err != nil {
		return GitLabBranch{}, err
	}
	return branch, nil
}

func (c *GitLabClient) CreateCommit(ctx context.Context, token string, req GitLabCreateCommitRequest) (GitLabCommit, error) {
	body := map[string]any{
		"branch":         req.Branch,
		"commit_message": req.CommitMessage,
		"actions":        req.Actions,
	}
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/repository/commits")
	var commit GitLabCommit
	if err := doJSON(ctx, c.httpClient, http.MethodPost, rawURL, token, authPrivateToken, body, &commit); err != nil {
		return GitLabCommit{}, err
	}
	return commit, nil
}

func (c *GitLabClient) CreateMergeRequest(ctx context.Context, token string, req GitLabCreateMergeRequestRequest) (GitLabMergeRequest, error) {
	body := map[string]any{
		"source_branch":        req.SourceBranch,
		"target_branch":        req.TargetBranch,
		"title":                req.Title,
		"remove_source_branch": req.RemoveSource,
	}
	if strings.TrimSpace(req.Description) != "" {
		body["description"] = req.Description
	}
	rawURL := joinBaseURL(c.apiBaseURL, "/projects/"+pathEscape(req.ProjectID)+"/merge_requests")
	var mr GitLabMergeRequest
	if err := doJSON(ctx, c.httpClient, http.MethodPost, rawURL, token, authPrivateToken, body, &mr); err != nil {
		return GitLabMergeRequest{}, err
	}
	return mr, nil
}
