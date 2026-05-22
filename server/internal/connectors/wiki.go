package connectors

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type WikiClient struct {
	baseURL    string
	httpClient HTTPDoer
}

type WikiSpace struct {
	ID   int64  `json:"id,omitempty"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type WikiPage struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Space   *WikiSpace     `json:"space,omitempty"`
	Links   map[string]any `json:"_links,omitempty"`
	Body    map[string]any `json:"body,omitempty"`
	Version map[string]any `json:"version,omitempty"`
}

type WikiSearchSpacesRequest struct {
	Query string
	Limit int
}

type WikiSearchSpacesResponse struct {
	Results []WikiSpace `json:"results"`
	Size    int         `json:"size"`
}

type WikiSearchPagesRequest struct {
	CQL   string
	Limit int
}

type WikiSearchPagesResponse struct {
	Results []WikiPage `json:"results"`
	Size    int        `json:"size"`
}

func NewWikiClient(baseURL string, httpClient HTTPDoer) *WikiClient {
	return &WikiClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *WikiClient) Validate(ctx context.Context, token string) (ValidationResult, error) {
	var body map[string]any
	if err := doJSON(ctx, c.httpClient, http.MethodGet, joinBaseURL(c.baseURL, "/rest/api/user/current"), token, authBearer, nil, &body); err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Identity: stringMap(body, "accountId", "username", "displayName", "email")}, nil
}

func (c *WikiClient) SearchSpaces(ctx context.Context, token string, req WikiSearchSpacesRequest) (WikiSearchSpacesResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(req.Query) != "" {
		q.Set("spaceKey", strings.TrimSpace(req.Query))
	}
	rawURL := joinBaseURL(c.baseURL, "/rest/api/space") + "?" + q.Encode()
	var result WikiSearchSpacesResponse
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authBearer, nil, &result); err != nil {
		return WikiSearchSpacesResponse{}, err
	}
	return result, nil
}

func (c *WikiClient) SearchPages(ctx context.Context, token string, req WikiSearchPagesRequest) (WikiSearchPagesResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	q := url.Values{}
	q.Set("cql", strings.TrimSpace(req.CQL))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("expand", "space,version")
	rawURL := joinBaseURL(c.baseURL, "/rest/api/content/search") + "?" + q.Encode()
	var result WikiSearchPagesResponse
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authBearer, nil, &result); err != nil {
		return WikiSearchPagesResponse{}, err
	}
	return result, nil
}

func (c *WikiClient) GetPage(ctx context.Context, token string, id string) (WikiPage, error) {
	q := url.Values{}
	q.Set("expand", "body.storage,space,version")
	rawURL := joinBaseURL(c.baseURL, "/rest/api/content/"+url.PathEscape(strings.TrimSpace(id))) + "?" + q.Encode()
	var page WikiPage
	if err := doJSON(ctx, c.httpClient, http.MethodGet, rawURL, token, authBearer, nil, &page); err != nil {
		return WikiPage{}, err
	}
	return page, nil
}
