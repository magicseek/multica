package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabClientValidateAndCreateMergeRequest(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("PRIVATE-TOKEN"))
		if r.Header.Get("PRIVATE-TOKEN") != "good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/user":
			writeFixtureJSON(w, map[string]any{"id": 7, "username": "troy", "name": "Troy"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/group%2Frepo/repository/branches":
			if r.URL.Query().Get("branch") != "feat/test-branch" || r.URL.Query().Get("ref") != "main" {
				t.Fatalf("unexpected branch query: %s", r.URL.RawQuery)
			}
			writeFixtureJSON(w, GitLabBranch{Name: "feat/test-branch"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/group%2Frepo/merge_requests":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode MR body: %v", err)
			}
			if body["source_branch"] != "feat/test-branch" || body["target_branch"] != "main" {
				t.Fatalf("unexpected MR body: %#v", body)
			}
			writeFixtureJSON(w, GitLabMergeRequest{IID: 12, Title: "Test MR", State: "opened", SourceBranch: "feat/test-branch", TargetBranch: "main"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer srv.Close()

	client := NewGitLabClient(srv.URL+"/api/v4", srv.URL, srv.Client())
	validation, err := client.Validate(context.Background(), "good-token")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validation.Identity["username"] != "troy" || validation.Identity["id"] != "7" {
		t.Fatalf("identity mismatch: %#v", validation.Identity)
	}
	if _, err := client.CreateBranch(context.Background(), "good-token", GitLabCreateBranchRequest{
		ProjectID: "group/repo",
		Branch:    "feat/test-branch",
		Ref:       "main",
	}); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	mr, err := client.CreateMergeRequest(context.Background(), "good-token", GitLabCreateMergeRequestRequest{
		ProjectID:    "group/repo",
		SourceBranch: "feat/test-branch",
		TargetBranch: "main",
		Title:        "Test MR",
	})
	if err != nil {
		t.Fatalf("CreateMergeRequest: %v", err)
	}
	if mr.IID != 12 {
		t.Fatalf("MR IID = %d, want 12", mr.IID)
	}
	if len(seen) != 3 || !strings.Contains(seen[0], "good-token") {
		t.Fatalf("request trace mismatch: %#v", seen)
	}
}

func TestJiraClientSearchAndAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/2/myself":
			writeFixtureJSON(w, map[string]any{"accountId": "acct-1", "displayName": "Troy"})
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/search":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode search body: %v", err)
			}
			if body["jql"] != "project = MUL" {
				t.Fatalf("unexpected jql body: %#v", body)
			}
			writeFixtureJSON(w, JiraSearchIssuesResponse{
				Total:  1,
				Issues: []JiraIssue{{ID: "10001", Key: "MUL-1", Fields: map[string]any{"summary": "Fix it"}}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer srv.Close()

	client := NewJiraClient(srv.URL, srv.Client())
	validation, err := client.Validate(context.Background(), "good-token")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validation.Identity["accountId"] != "acct-1" {
		t.Fatalf("identity mismatch: %#v", validation.Identity)
	}
	search, err := client.SearchIssues(context.Background(), "good-token", JiraSearchIssuesRequest{JQL: "project = MUL"})
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if len(search.Issues) != 1 || search.Issues[0].Key != "MUL-1" {
		t.Fatalf("search mismatch: %#v", search)
	}
	if _, err := client.Validate(context.Background(), "bad-token"); !IsAuthError(err) {
		t.Fatalf("Validate bad token error = %v, want auth error", err)
	}
}

func TestWikiClientSearchAndRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/rest/api/user/current":
			writeFixtureJSON(w, map[string]any{"accountId": "acct-2", "displayName": "Docs User"})
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/rest/api/content/search":
			if r.URL.Query().Get("cql") != `space = "ENG"` {
				t.Fatalf("unexpected cql: %s", r.URL.Query().Get("cql"))
			}
			writeFixtureJSON(w, WikiSearchPagesResponse{
				Size:    1,
				Results: []WikiPage{{ID: "123", Type: "page", Title: "Runbook"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/rest/api/content/123":
			if !strings.Contains(r.URL.Query().Get("expand"), "body.storage") {
				t.Fatalf("missing body.storage expand: %s", r.URL.RawQuery)
			}
			writeFixtureJSON(w, WikiPage{ID: "123", Type: "page", Title: "Runbook", Body: map[string]any{"storage": map[string]any{"value": "<p>hello</p>"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL+"/wiki", srv.Client())
	validation, err := client.Validate(context.Background(), "good-token")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validation.Identity["displayName"] != "Docs User" {
		t.Fatalf("identity mismatch: %#v", validation.Identity)
	}
	search, err := client.SearchPages(context.Background(), "good-token", WikiSearchPagesRequest{CQL: `space = "ENG"`})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(search.Results) != 1 || search.Results[0].ID != "123" {
		t.Fatalf("search mismatch: %#v", search)
	}
	page, err := client.GetPage(context.Background(), "good-token", "123")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Title != "Runbook" {
		t.Fatalf("page title = %q", page.Title)
	}
}

func writeFixtureJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
