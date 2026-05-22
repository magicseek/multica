package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/connectors"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestConnectorActionRequiresTaskScopeAndWritePolicy(t *testing.T) {
	requireConnectorCredentialSchema(t)
	ctx := context.Background()
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/user":
			writeJSON(w, http.StatusOK, map[string]any{"id": 1, "username": "delegated"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/group%2Frepo/repository/branches":
			upstreamCalls++
			writeJSON(w, http.StatusOK, connectors.GitLabBranch{Name: r.URL.Query().Get("branch")})
		default:
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer upstream.Close()

	cfg := connectors.Config{RingCentral: connectors.RingCentralConfig{
		Enabled:          true,
		GitLabAPIBaseURL: upstream.URL + "/api/v4",
		GitLabWebBaseURL: upstream.URL,
		JiraBaseURL:      upstream.URL,
		WikiBaseURL:      upstream.URL,
	}}
	h := *testHandler
	h.cfg.ConnectorRegistry = connectors.NewRegistry(cfg)
	h.cfg.ConnectorClients = connectors.NewClientSet(cfg, upstream.Client())
	vault, err := connectors.NewCredentialVault("test:v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewCredentialVault: %v", err)
	}
	h.cfg.ConnectorVault = vault
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM connector_capability_audit_event WHERE workspace_id = $1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM connector_credential WHERE workspace_id = $1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM workspace_connector WHERE workspace_id = $1`, testWorkspaceID)
	})

	projectID := createConnectorActionProject(t)
	defer func() {
		req := newRequest("DELETE", "/api/projects/"+projectID, nil)
		req = withURLParam(req, "id", projectID)
		testHandler.DeleteProject(httptest.NewRecorder(), req)
	}()
	resourceID := createConnectorActionResource(t, &h, projectID)
	agentID := handlerTestAgentID(t)
	taskID := createConnectorActionTask(t, agentID, projectID)

	saveReq := newRequest(http.MethodPut, "/api/connectors/providers/ringcentral_gitlab/credential", map[string]any{
		"secret": "good-token",
	})
	saveReq = withURLParam(saveReq, "providerID", connectors.ProviderRingCentralGitLab)
	saveW := httptest.NewRecorder()
	h.SaveConnectorCredential(saveW, saveReq)
	if saveW.Code != http.StatusOK {
		t.Fatalf("SaveConnectorCredential: %d %s", saveW.Code, saveW.Body.String())
	}

	actionBody := map[string]any{
		"provider_id":         connectors.ProviderRingCentralGitLab,
		"capability":          "branch.create",
		"project_resource_id": resourceID,
		"input": map[string]any{
			"branch": "feat/task-branch",
			"ref":    "main",
		},
	}
	missingIdentity := newRequest(http.MethodPost, "/api/connectors/actions", actionBody)
	w := httptest.NewRecorder()
	h.RunConnectorAction(w, missingIdentity)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing identity: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	req := newRequest(http.MethodPost, "/api/connectors/actions", actionBody)
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Task-ID", taskID)
	w = httptest.NewRecorder()
	h.RunConnectorAction(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("write policy disabled: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	_, err = h.Queries.UpsertWorkspaceConnector(ctx, db.UpsertWorkspaceConnectorParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		ProviderID:  connectors.ProviderRingCentralGitLab,
		Enabled:     true,
		Settings:    []byte(`{"remote_write_policy":"merge_request_preparation"}`),
		CreatedBy:   parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("UpsertWorkspaceConnector: %v", err)
	}

	badBranch := newRequest(http.MethodPost, "/api/connectors/actions", map[string]any{
		"provider_id":         connectors.ProviderRingCentralGitLab,
		"capability":          "branch.create",
		"project_resource_id": resourceID,
		"input": map[string]any{
			"branch": "feature/task-branch",
			"ref":    "main",
		},
	})
	badBranch.Header.Set("X-Agent-ID", agentID)
	badBranch.Header.Set("X-Task-ID", taskID)
	w = httptest.NewRecorder()
	h.RunConnectorAction(w, badBranch)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bad branch: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	req = newRequest(http.MethodPost, "/api/connectors/actions", actionBody)
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Task-ID", taskID)
	w = httptest.NewRecorder()
	h.RunConnectorAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunConnectorAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstreamCalls = %d, want 1", upstreamCalls)
	}
	var auditCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM connector_capability_audit_event
		WHERE workspace_id = $1 AND provider_id = $2 AND status = 'succeeded'
	`, testWorkspaceID, connectors.ProviderRingCentralGitLab).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("succeeded audit count = %d, want 1", auditCount)
	}
}

func TestConnectorActionScopesExternalQueriesToAttachedResource(t *testing.T) {
	jql := scopedJiraJQL("ABC", `summary ~ "project migration" ORDER BY updated DESC`)
	if jql != `project = ABC AND (summary ~ "project migration" ORDER BY updated DESC)` {
		t.Fatalf("scopedJiraJQL preserved unscoped JQL: %q", jql)
	}
	if got := scopedJiraJQL("ABC", ""); got != "project = ABC" {
		t.Fatalf("scopedJiraJQL empty = %q", got)
	}

	cql := scopedWikiCQL("ENG", `title ~ "space planning"`)
	if cql != `space = "ENG" AND (title ~ "space planning")` {
		t.Fatalf("scopedWikiCQL preserved unscoped CQL: %q", cql)
	}
	if got := scopedWikiCQL("ENG", ""); got != `space = "ENG"` {
		t.Fatalf("scopedWikiCQL empty = %q", got)
	}
}

func TestValidateGitLabWriteBranchRequiresSemanticTaskPrefix(t *testing.T) {
	valid := []string{"feat/task-123", "fix/RC-123_bug", "docs/runbook.update"}
	for _, branch := range valid {
		if err := validateGitLabWriteBranch(branch, "main"); err != nil {
			t.Fatalf("validateGitLabWriteBranch(%q): %v", branch, err)
		}
	}

	invalid := []string{"feature/task-123", "main", "feat/../main", "/feat/task", "feat/task/"}
	for _, branch := range invalid {
		if err := validateGitLabWriteBranch(branch, "main"); err == nil {
			t.Fatalf("validateGitLabWriteBranch(%q) expected error", branch)
		}
	}
}

func createConnectorActionProject(t *testing.T) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Connector action project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: %d %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return project.ID
}

func createConnectorActionResource(t *testing.T, h *Handler, projectID string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects/"+projectID+"/resources", map[string]any{
		"resource_type": connectors.ResourceRingCentralGitLabRepo,
		"resource_ref": map[string]any{
			"project_id":     "group/repo",
			"default_branch": "main",
		},
	})
	req = withURLParam(req, "id", projectID)
	h.CreateProjectResource(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProjectResource: %d %s", w.Code, w.Body.String())
	}
	var resource ProjectResourceResponse
	if err := json.NewDecoder(w.Body).Decode(&resource); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	return resource.ID
}

func handlerTestAgentID(t *testing.T) string {
	t.Helper()
	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM agent WHERE workspace_id = $1 ORDER BY created_at ASC LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load test agent: %v", err)
	}
	return agentID
}

func createConnectorActionTask(t *testing.T, agentID, projectID string) string {
	t.Helper()
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_id, assignee_type, creator_type, creator_id, project_id
		)
		VALUES ($1, 'Connector action issue', '', 'todo', 'medium', $2, 'agent', 'member', $3, $4)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID, projectID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority,
			started_at, connector_delegated_user_id
		)
		VALUES ($1, $2, $3, 'running', 5, now(), $4)
		RETURNING id
	`, agentID, testRuntimeID, issueID, testUserID).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return taskID
}
