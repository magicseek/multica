package connectors

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type JiraClient struct {
	baseURL    string
	httpClient HTTPDoer
}

type JiraProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Self string `json:"self"`
}

type JiraIssue struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Self   string         `json:"self"`
	Fields map[string]any `json:"fields"`
}

type JiraSearchIssuesRequest struct {
	JQL        string
	MaxResults int
}

type JiraSearchIssuesResponse struct {
	StartAt    int         `json:"startAt"`
	MaxResults int         `json:"maxResults"`
	Total      int         `json:"total"`
	Issues     []JiraIssue `json:"issues"`
}

func NewJiraClient(baseURL string, httpClient HTTPDoer) *JiraClient {
	return &JiraClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *JiraClient) Validate(ctx context.Context, token string) (ValidationResult, error) {
	var body map[string]any
	if err := doJSON(ctx, c.httpClient, http.MethodGet, joinBaseURL(c.baseURL, "/rest/api/2/myself"), token, authBearer, nil, &body); err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Identity: stringMap(body, "accountId", "name", "displayName", "emailAddress")}, nil
}

func (c *JiraClient) ListProjects(ctx context.Context, token string) ([]JiraProject, error) {
	var projects []JiraProject
	if err := doJSON(ctx, c.httpClient, http.MethodGet, joinBaseURL(c.baseURL, "/rest/api/2/project"), token, authBearer, nil, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (c *JiraClient) SearchIssues(ctx context.Context, token string, req JiraSearchIssuesRequest) (JiraSearchIssuesResponse, error) {
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > 100 {
		maxResults = 25
	}
	body := map[string]any{
		"jql":        strings.TrimSpace(req.JQL),
		"maxResults": maxResults,
	}
	var result JiraSearchIssuesResponse
	if err := doJSON(ctx, c.httpClient, http.MethodPost, joinBaseURL(c.baseURL, "/rest/api/2/search"), token, authBearer, body, &result); err != nil {
		return JiraSearchIssuesResponse{}, err
	}
	return result, nil
}

func (c *JiraClient) GetIssue(ctx context.Context, token string, key string) (JiraIssue, error) {
	rawURL := joinBaseURL(c.baseURL, "/rest/api/2/issue/"+url.PathEscape(strings.TrimSpace(key)))
	var issue JiraIssue
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authBearer, nil, &issue); err != nil {
		return JiraIssue{}, err
	}
	return issue, nil
}
