package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createHandlerTestRepository(t *testing.T, name, remoteURL string) RepositoryResponse {
	t.Helper()

	body := map[string]any{
		"name":         name,
		"source_state": "remote_git",
		"remote_url":   remoteURL,
	}
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/repositories?workspace_id="+testWorkspaceID, body)
	testHandler.CreateRepository(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRepository: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var repo RepositoryResponse
	if err := json.NewDecoder(w.Body).Decode(&repo); err != nil {
		t.Fatalf("decode CreateRepository: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM repository WHERE id = $1`, repo.ID)
	})
	return repo
}

func createRepositoryTestProject(t *testing.T, title string) string {
	t.Helper()

	var projectID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO project (workspace_id, title)
		VALUES ($1, $2)
		RETURNING id
	`, testWorkspaceID, title).Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})
	return projectID
}

func createRepositoryTestMember(t *testing.T) string {
	t.Helper()

	email := strings.ToLower(strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())) + "@repository-test.multica"
	var userID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO "user" (name, email)
		VALUES ('Repository Test Member', $1)
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID)
	})
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, testWorkspaceID, userID); err != nil {
		t.Fatalf("create member: %v", err)
	}
	return userID
}

func TestRepositoryLifecycle(t *testing.T) {
	remoteURL := "https://github.com/multica-ai/repository-lifecycle.git"
	repo := createHandlerTestRepository(t, "Repository lifecycle", remoteURL)
	if repo.SourceState != "remote_git" {
		t.Fatalf("source_state = %q, want remote_git", repo.SourceState)
	}
	if repo.RemoteURL == nil || *repo.RemoteURL != remoteURL {
		t.Fatalf("remote_url = %v, want %q", repo.RemoteURL, remoteURL)
	}
	if repo.RemoteKey == nil || *repo.RemoteKey == "" {
		t.Fatal("expected normalized remote_key")
	}

	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/repositories/"+repo.ID, map[string]any{
		"name":           "Renamed repository",
		"default_branch": "main",
	})
	req = withURLParam(req, "id", repo.ID)
	testHandler.UpdateRepository(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateRepository: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated RepositoryResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode UpdateRepository: %v", err)
	}
	if updated.Name != "Renamed repository" {
		t.Fatalf("updated name = %q", updated.Name)
	}
	if updated.DefaultBranch == nil || *updated.DefaultBranch != "main" {
		t.Fatalf("updated default_branch = %v", updated.DefaultBranch)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/repositories?workspace_id="+testWorkspaceID, nil)
	testHandler.ListRepositories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositories: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Repositories []RepositoryResponse `json:"repositories"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListRepositories: %v", err)
	}
	found := false
	for _, item := range listResp.Repositories {
		if item.ID == repo.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created repository %s not found in list", repo.ID)
	}

	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/repositories/"+repo.ID, nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.DeleteRepository(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteRepository: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/repositories?workspace_id="+testWorkspaceID, nil)
	testHandler.ListRepositories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositories after delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	listResp.Repositories = nil
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode post-delete ListRepositories: %v", err)
	}
	for _, item := range listResp.Repositories {
		if item.ID == repo.ID {
			t.Fatalf("archived repository %s should not be listed", repo.ID)
		}
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/repositories/"+repo.ID, nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.GetRepository(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetRepository after delete: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRepositoryCompatibilityReadsLegacyWorkspaceAndProjectRepos(t *testing.T) {
	workspaceURL := "https://github.com/multica-ai/legacy-workspace-read-model.git"
	projectURL := "git@github.com:multica-ai/legacy-project-read-model.git"
	if _, err := testPool.Exec(context.Background(), `
		UPDATE workspace SET repos = $1 WHERE id = $2
	`, []byte(fmt.Sprintf(`[{"url":%q}]`, workspaceURL)), testWorkspaceID); err != nil {
		t.Fatalf("update workspace repos: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET repos = '[]'::jsonb WHERE id = $1`, testWorkspaceID)
	})

	projectID := createRepositoryTestProject(t, "Legacy project repository read model")
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, position)
		VALUES ($1, $2, 'github_repo', $3::jsonb, 0)
	`, projectID, testWorkspaceID, []byte(fmt.Sprintf(`{"url":%q}`, projectURL))); err != nil {
		t.Fatalf("create project_resource: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/repositories?workspace_id="+testWorkspaceID, nil)
	testHandler.ListRepositories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositories: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Repositories []RepositoryResponse `json:"repositories"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListRepositories: %v", err)
	}
	foundWorkspace := false
	foundProject := false
	for _, repo := range listResp.Repositories {
		if repo.RemoteURL != nil && *repo.RemoteURL == workspaceURL && repo.Compatibility && repo.CompatibilitySource == "workspace.repos" {
			foundWorkspace = true
		}
		if repo.RemoteURL != nil && *repo.RemoteURL == projectURL && repo.Compatibility && repo.CompatibilitySource == "project_resource.github_repo" {
			foundProject = true
		}
	}
	if !foundWorkspace {
		t.Fatalf("legacy workspace repo %q missing from compatibility list", workspaceURL)
	}
	if !foundProject {
		t.Fatalf("legacy project repo %q missing from compatibility list", projectURL)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/projects/"+projectID+"/repositories", nil)
	req = withURLParam(req, "id", projectID)
	testHandler.ListProjectRepositories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListProjectRepositories: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var projectResp struct {
		Repositories []ProjectRepositoryResponse `json:"repositories"`
	}
	if err := json.NewDecoder(w.Body).Decode(&projectResp); err != nil {
		t.Fatalf("decode ListProjectRepositories: %v", err)
	}
	if len(projectResp.Repositories) != 1 {
		t.Fatalf("expected 1 compatibility project repository, got %d", len(projectResp.Repositories))
	}
	if projectResp.Repositories[0].Repository.RemoteURL == nil || *projectResp.Repositories[0].Repository.RemoteURL != projectURL {
		t.Fatalf("project compatibility repo URL = %v, want %q", projectResp.Repositories[0].Repository.RemoteURL, projectURL)
	}
	if !projectResp.Repositories[0].Repository.Compatibility {
		t.Fatal("expected project repository read to mark legacy resource as compatibility")
	}
}

func TestRepositoryBindingFiltersLocalPathForNonOwner(t *testing.T) {
	repo := createHandlerTestRepository(t, "Binding filter", "https://github.com/multica-ai/binding-filter.git")
	localPath := "/Users/tester/private/source"

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/repositories/"+repo.ID+"/bindings", map[string]any{
		"daemon_id":     "daemon-binding-filter",
		"machine_label": "Tester Mac",
		"binding_kind":  "local_dir",
		"local_path":    localPath,
		"state":         "ready",
		"metadata": map[string]any{
			"last_verified_path": localPath,
		},
	})
	req = withURLParam(req, "id", repo.ID)
	testHandler.CreateRepositoryBinding(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRepositoryBinding: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var binding RepositoryBindingResponse
	if err := json.NewDecoder(w.Body).Decode(&binding); err != nil {
		t.Fatalf("decode CreateRepositoryBinding: %v", err)
	}
	if binding.LocalPath == nil || *binding.LocalPath != localPath || !binding.LocalPathVisible {
		t.Fatalf("owner response should include local path, got %+v", binding)
	}
	if !strings.Contains(string(binding.Metadata), "last_verified_path") {
		t.Fatalf("owner response should include private binding metadata, got %s", string(binding.Metadata))
	}

	otherUserID := createRepositoryTestMember(t)
	w = httptest.NewRecorder()
	req = newRequestAs(otherUserID, "GET", "/api/repositories/"+repo.ID+"/bindings", nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.ListRepositoryBindings(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositoryBindings as other member: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Bindings []RepositoryBindingResponse `json:"bindings"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListRepositoryBindings: %v", err)
	}
	if len(listResp.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(listResp.Bindings))
	}
	if listResp.Bindings[0].LocalPath != nil || listResp.Bindings[0].LocalPathVisible {
		t.Fatalf("non-owner response must hide local path, got %+v", listResp.Bindings[0])
	}
	if string(listResp.Bindings[0].Metadata) != "{}" {
		t.Fatalf("non-owner response must hide private binding metadata, got %s", string(listResp.Bindings[0].Metadata))
	}
}

func TestSetProjectRepositoriesLinksFirstClassRows(t *testing.T) {
	repo := createHandlerTestRepository(t, "Project repository", "https://github.com/multica-ai/project-repository.git")
	projectID := createRepositoryTestProject(t, "First-class project repository")

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/projects/"+projectID+"/repositories", map[string]any{
		"repositories": []map[string]any{
			{
				"repository_id": repo.ID,
				"role":          "primary",
				"position":      0,
			},
		},
	})
	req = withURLParam(req, "id", projectID)
	testHandler.SetProjectRepositories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SetProjectRepositories: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Repositories []ProjectRepositoryResponse `json:"repositories"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode SetProjectRepositories: %v", err)
	}
	if len(resp.Repositories) != 1 {
		t.Fatalf("expected 1 project repository, got %d", len(resp.Repositories))
	}
	if resp.Repositories[0].RepositoryID != repo.ID || resp.Repositories[0].Role != "primary" {
		t.Fatalf("unexpected project repository response: %+v", resp.Repositories[0])
	}
	if resp.Repositories[0].Repository.Compatibility {
		t.Fatal("first-class project repository should not be marked compatibility")
	}
}

func TestChatSessionDefaultRepositoryReference(t *testing.T) {
	repo := createHandlerTestRepository(t, "Chat default repository", "https://github.com/multica-ai/chat-default-repository.git")
	agentID := createHandlerTestAgent(t, "ChatDefaultRepoAgent", []byte("[]"))

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/chat/sessions", map[string]any{
		"agent_id":              agentID,
		"title":                 "Chat with repository",
		"default_repository_id": repo.ID,
	})
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.CreateChatSession(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateChatSession: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var session ChatSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatalf("decode CreateChatSession: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, session.ID)
	})
	if session.DefaultRepositoryID == nil || *session.DefaultRepositoryID != repo.ID {
		t.Fatalf("default_repository_id = %v, want %s", session.DefaultRepositoryID, repo.ID)
	}

	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/chat/sessions/"+session.ID, map[string]any{
		"default_repository_id": nil,
	})
	req = withURLParam(req, "sessionId", session.ID)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.UpdateChatSession(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateChatSession clear repository: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatalf("decode clear UpdateChatSession: %v", err)
	}
	if session.DefaultRepositoryID != nil {
		t.Fatalf("default_repository_id should clear to nil, got %v", session.DefaultRepositoryID)
	}
}
