package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createHandlerTestRepositoryWithState(t *testing.T, name, sourceState string) RepositoryResponse {
	t.Helper()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/repositories?workspace_id="+testWorkspaceID, map[string]any{
		"name":         name,
		"source_state": sourceState,
	})
	testHandler.CreateRepository(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRepository(%s): expected 201, got %d: %s", sourceState, w.Code, w.Body.String())
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

func createHandlerTestRepositoryBinding(t *testing.T, repoID, daemonID, localPath string) RepositoryBindingResponse {
	t.Helper()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/repositories/"+repoID+"/bindings", map[string]any{
		"daemon_id":     daemonID,
		"machine_label": "Repository Operation Test",
		"binding_kind":  "local_dir",
		"local_path":    localPath,
		"state":         "ready",
		"metadata": map[string]any{
			"last_verified_path": localPath,
		},
	})
	req = withURLParam(req, "id", repoID)
	testHandler.CreateRepositoryBinding(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRepositoryBinding: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var binding RepositoryBindingResponse
	if err := json.NewDecoder(w.Body).Decode(&binding); err != nil {
		t.Fatalf("decode CreateRepositoryBinding: %v", err)
	}
	return binding
}

func createHandlerTestRepositoryOperation(t *testing.T, repoID, operationType string, body map[string]any) RepositoryOperationResponse {
	t.Helper()

	pathType := strings.ReplaceAll(operationType, "_", "-")
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/repositories/"+repoID+"/operations/"+pathType, body)
	req = withURLParam(req, "id", repoID)
	req = withURLParam(req, "operationType", pathType)
	testHandler.CreateRepositoryOperation(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRepositoryOperation(%s): expected 201, got %d: %s", operationType, w.Code, w.Body.String())
	}
	var op RepositoryOperationResponse
	if err := json.NewDecoder(w.Body).Decode(&op); err != nil {
		t.Fatalf("decode CreateRepositoryOperation: %v", err)
	}
	return op
}

func claimHandlerTestRepositoryOperation(t *testing.T, daemonID string) RepositoryOperationResponse {
	t.Helper()
	op := claimHandlerTestRepositoryOperationWithRuntime(t, daemonID, "")
	if op == nil {
		t.Fatal("expected claimed repository operation")
	}
	return *op
}

func claimHandlerTestRepositoryOperationWithRuntime(t *testing.T, daemonID, runtimeID string) *RepositoryOperationResponse {
	t.Helper()

	path := "/api/daemon/repository-operations/claim"
	if strings.TrimSpace(runtimeID) != "" {
		path += "?runtime_id=" + runtimeID
	}
	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("GET", path, nil, testWorkspaceID, daemonID)
	testHandler.ClaimRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ClaimRepositoryOperation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Operation *RepositoryOperationResponse `json:"operation"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ClaimRepositoryOperation: %v", err)
	}
	return resp.Operation
}

func TestRepositoryOperationCreateListClaimStartCompleteCreateBinding(t *testing.T) {
	repo := createHandlerTestRepositoryWithState(t, "Agent managed operation", "agent_managed")
	privatePath := "/Users/tester/private/agent-managed-operation"

	op := createHandlerTestRepositoryOperation(t, repo.ID, "create_binding", map[string]any{
		"target_daemon_id": "repo-op-daemon",
		"request": map[string]any{
			"machine_label": "Repository Operation Mac",
		},
	})
	if op.Status != "queued" || op.OperationType != "create_binding" {
		t.Fatalf("created operation = %+v, want queued create_binding", op)
	}
	if op.Binding != nil {
		t.Fatalf("public create response included daemon-private binding path: %+v", op.Binding)
	}

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/repositories/"+repo.ID+"/operations", nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.ListRepositoryOperations(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositoryOperations: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), privatePath) {
		t.Fatalf("operation list leaked private path before completion: %s", w.Body.String())
	}
	var listResp struct {
		Operations []RepositoryOperationResponse `json:"operations"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListRepositoryOperations: %v", err)
	}
	if len(listResp.Operations) != 1 || listResp.Operations[0].ID != op.ID {
		t.Fatalf("unexpected operations list: %+v", listResp.Operations)
	}
	if listResp.Operations[0].Binding != nil {
		t.Fatalf("public list response included daemon-private binding path: %+v", listResp.Operations[0].Binding)
	}

	claimed := claimHandlerTestRepositoryOperation(t, "repo-op-daemon")
	if claimed.ID != op.ID || claimed.Status != "running" {
		t.Fatalf("claimed operation = %+v, want same operation running", claimed)
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+op.ID+"/start", nil, testWorkspaceID, "repo-op-daemon")
	req = withURLParam(req, "operationId", op.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("StartRepositoryOperation: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+op.ID+"/complete", map[string]any{
		"result": map[string]any{"created": true},
		"binding": map[string]any{
			"binding_kind":  "daemon_workdir",
			"machine_label": "Repository Operation Mac",
			"local_path":    privatePath,
			"state":         "ready",
			"metadata": map[string]any{
				"last_verified_path": privatePath,
			},
		},
	}, testWorkspaceID, "repo-op-daemon")
	req = withURLParam(req, "operationId", op.ID)
	testHandler.CompleteRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteRepositoryOperation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), privatePath) {
		t.Fatalf("complete response leaked private path: %s", w.Body.String())
	}
	var completed RepositoryOperationResponse
	if err := json.NewDecoder(w.Body).Decode(&completed); err != nil {
		t.Fatalf("decode CompleteRepositoryOperation: %v", err)
	}
	if completed.Status != "succeeded" || completed.CompletedAt == nil || completed.BindingID == nil {
		t.Fatalf("completed operation missing terminal fields: %+v", completed)
	}
	if completed.Binding != nil {
		t.Fatalf("complete response included daemon-private binding path: %+v", completed.Binding)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/repositories/"+repo.ID+"/operations", nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.ListRepositoryOperations(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositoryOperations after complete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), privatePath) {
		t.Fatalf("operation list leaked private path after completion: %s", w.Body.String())
	}
	listResp.Operations = nil
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListRepositoryOperations after complete: %v", err)
	}
	if len(listResp.Operations) != 1 || listResp.Operations[0].Binding != nil {
		t.Fatalf("public operation list after complete should not include daemon-private binding: %+v", listResp.Operations)
	}

	otherUserID := createRepositoryTestMember(t)
	w = httptest.NewRecorder()
	req = newRequestAs(otherUserID, "GET", "/api/repositories/"+repo.ID+"/bindings", nil)
	req = withURLParam(req, "id", repo.ID)
	testHandler.ListRepositoryBindings(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRepositoryBindings as other member: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), privatePath) {
		t.Fatalf("binding list leaked private path to non-owner: %s", w.Body.String())
	}
}

func TestRepositoryOperationInitGitPublishRemoteAndFailTransitions(t *testing.T) {
	repo := createHandlerTestRepositoryWithState(t, "Local operation transitions", "local_dir")
	localPath := "/Users/tester/private/local-operation"
	binding := createHandlerTestRepositoryBinding(t, repo.ID, "repo-op-transition-daemon", localPath)

	initOp := createHandlerTestRepositoryOperation(t, repo.ID, "init_git", map[string]any{
		"target_daemon_id": "repo-op-transition-daemon",
		"binding_id":       binding.ID,
	})
	if initOp.Binding != nil {
		t.Fatalf("public create response included daemon-private binding path: %+v", initOp.Binding)
	}
	claimed := claimHandlerTestRepositoryOperation(t, "repo-op-transition-daemon")
	if claimed.ID != initOp.ID {
		t.Fatalf("claimed init operation %s, want %s", claimed.ID, initOp.ID)
	}
	if claimed.Binding == nil || claimed.Binding.LocalPath != localPath {
		t.Fatalf("claim response missing target daemon binding path: %+v", claimed.Binding)
	}

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+initOp.ID+"/start", nil, testWorkspaceID, "repo-op-transition-daemon")
	req = withURLParam(req, "operationId", initOp.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("StartRepositoryOperation init_git: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var startedInit RepositoryOperationResponse
	if err := json.NewDecoder(w.Body).Decode(&startedInit); err != nil {
		t.Fatalf("decode StartRepositoryOperation init_git: %v", err)
	}
	if startedInit.Binding == nil || startedInit.Binding.LocalPath != localPath {
		t.Fatalf("start response missing target daemon binding path: %+v", startedInit.Binding)
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+initOp.ID+"/complete", map[string]any{
		"result": map[string]any{"head_commit": "abc123"},
		"repository": map[string]any{
			"default_branch": "main",
			"metadata": map[string]any{
				"head_commit": "abc123",
			},
		},
	}, testWorkspaceID, "repo-op-transition-daemon")
	req = withURLParam(req, "operationId", initOp.ID)
	testHandler.CompleteRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteRepositoryOperation init_git: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var completedInit RepositoryOperationResponse
	if err := json.NewDecoder(w.Body).Decode(&completedInit); err != nil {
		t.Fatalf("decode CompleteRepositoryOperation init_git: %v", err)
	}
	if completedInit.Binding != nil || strings.Contains(w.Body.String(), localPath) {
		t.Fatalf("complete init_git response leaked binding path: %+v body=%s", completedInit.Binding, w.Body.String())
	}

	var sourceState, defaultBranch string
	if err := testPool.QueryRow(context.Background(), `
		SELECT source_state, COALESCE(default_branch, '') FROM repository WHERE id = $1
	`, repo.ID).Scan(&sourceState, &defaultBranch); err != nil {
		t.Fatalf("read repository after init_git: %v", err)
	}
	if sourceState != "local_git" || defaultBranch != "main" {
		t.Fatalf("repository after init_git = (%s, %s), want (local_git, main)", sourceState, defaultBranch)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/repositories/"+repo.ID+"/operations/publish-remote", map[string]any{
		"target_daemon_id": "repo-op-transition-daemon",
		"binding_id":       binding.ID,
	})
	req = withURLParam(req, "id", repo.ID)
	req = withURLParam(req, "operationType", "publish-remote")
	testHandler.CreateRepositoryOperation(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "request.remote_url is required") {
		t.Fatalf("publish_remote without remote_url = %d %s, want 400 remote_url required", w.Code, w.Body.String())
	}

	remoteURL := "https://github.com/multica-ai/repository-operation-publish.git"
	publishOp := createHandlerTestRepositoryOperation(t, repo.ID, "publish_remote", map[string]any{
		"target_daemon_id": "repo-op-transition-daemon",
		"binding_id":       binding.ID,
		"request": map[string]any{
			"remote_url": remoteURL,
		},
	})
	claimed = claimHandlerTestRepositoryOperation(t, "repo-op-transition-daemon")
	if claimed.ID != publishOp.ID {
		t.Fatalf("claimed publish operation %s, want %s", claimed.ID, publishOp.ID)
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+publishOp.ID+"/complete", map[string]any{
		"repository": map[string]any{
			"remote_url":     remoteURL,
			"default_branch": "main",
		},
	}, testWorkspaceID, "repo-op-transition-daemon")
	req = withURLParam(req, "operationId", publishOp.ID)
	testHandler.CompleteRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteRepositoryOperation publish_remote: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var storedRemote string
	if err := testPool.QueryRow(context.Background(), `
		SELECT source_state, COALESCE(remote_url, '') FROM repository WHERE id = $1
	`, repo.ID).Scan(&sourceState, &storedRemote); err != nil {
		t.Fatalf("read repository after publish_remote: %v", err)
	}
	if sourceState != "remote_git" || storedRemote != remoteURL {
		t.Fatalf("repository after publish_remote = (%s, %s), want (remote_git, %s)", sourceState, storedRemote, remoteURL)
	}

	failRepo := createHandlerTestRepositoryWithState(t, "Fail operation transitions", "local_dir")
	failBinding := createHandlerTestRepositoryBinding(t, failRepo.ID, "repo-op-fail-daemon", "/Users/tester/private/fail-operation")
	failOp := createHandlerTestRepositoryOperation(t, failRepo.ID, "init_git", map[string]any{
		"target_daemon_id": "repo-op-fail-daemon",
		"binding_id":       failBinding.ID,
	})

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+failOp.ID+"/start", nil, "00000000-0000-0000-0000-000000000000", "repo-op-fail-daemon")
	req = withURLParam(req, "operationId", failOp.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace StartRepositoryOperation: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+failOp.ID+"/start", nil, testWorkspaceID, "repo-op-fail-daemon")
	req = withURLParam(req, "operationId", failOp.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("StartRepositoryOperation queued op: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+failOp.ID+"/fail", map[string]any{
		"error": "git init failed in /Users/tester/private/fail-operation",
		"result": map[string]any{
			"reason": "permission_denied",
		},
	}, testWorkspaceID, "repo-op-fail-daemon")
	req = withURLParam(req, "operationId", failOp.ID)
	testHandler.FailRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("FailRepositoryOperation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "/Users/tester/private/fail-operation") {
		t.Fatalf("fail response leaked private path: %s", w.Body.String())
	}
	var failed RepositoryOperationResponse
	if err := json.NewDecoder(w.Body).Decode(&failed); err != nil {
		t.Fatalf("decode FailRepositoryOperation: %v", err)
	}
	if failed.Status != "failed" || failed.Error == nil || !strings.Contains(*failed.Error, "[REDACTED LOCAL PATH]") {
		t.Fatalf("failed operation did not sanitize error: %+v", failed)
	}
	if failed.Binding != nil {
		t.Fatalf("fail response included daemon-private binding path: %+v", failed.Binding)
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+failOp.ID+"/complete", nil, testWorkspaceID, "repo-op-fail-daemon")
	req = withURLParam(req, "operationId", failOp.ID)
	testHandler.CompleteRepositoryOperation(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("CompleteRepositoryOperation terminal op: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRepositoryOperationRuntimeScopedMutationsRejectSiblingRuntime(t *testing.T) {
	repo := createHandlerTestRepositoryWithState(t, "Runtime scoped operation", "local_dir")

	runtimeID := createRuntimeLocalSkillTestRuntime(t, testUserID)
	var daemonID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT daemon_id FROM agent_runtime WHERE id = $1
	`, runtimeID).Scan(&daemonID); err != nil {
		t.Fatalf("read runtime daemon_id: %v", err)
	}
	var siblingRuntimeID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status,
			device_info, metadata, owner_id, last_seen_at
		)
		VALUES ($1, $2, 'Repository Operation Sibling Runtime', 'local', 'codex',
			'online', 'Repository operation sibling runtime', '{}'::jsonb, $3, now())
		RETURNING id
	`, testWorkspaceID, daemonID, testUserID).Scan(&siblingRuntimeID); err != nil {
		t.Fatalf("create sibling runtime: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, siblingRuntimeID)
	})

	binding := createHandlerTestRepositoryBinding(t, repo.ID, daemonID, "/Users/tester/private/runtime-scoped-operation")
	op := createHandlerTestRepositoryOperation(t, repo.ID, "init_git", map[string]any{
		"target_runtime_id": runtimeID,
		"binding_id":        binding.ID,
	})

	if op.Binding != nil {
		t.Fatalf("public create response included daemon-private binding path: %+v", op.Binding)
	}
	if siblingClaim := claimHandlerTestRepositoryOperationWithRuntime(t, daemonID, siblingRuntimeID); siblingClaim != nil {
		t.Fatalf("sibling runtime claimed target runtime operation: %+v", siblingClaim)
	}
	targetClaim := claimHandlerTestRepositoryOperationWithRuntime(t, daemonID, runtimeID)
	if targetClaim == nil {
		t.Fatal("target runtime did not claim repository operation")
	}
	if targetClaim.Binding == nil || targetClaim.Binding.LocalPath != "/Users/tester/private/runtime-scoped-operation" {
		t.Fatalf("target runtime claim missing binding path: %+v", targetClaim.Binding)
	}

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+op.ID+"/start?runtime_id="+siblingRuntimeID, nil, testWorkspaceID, daemonID)
	req = withURLParam(req, "operationId", op.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("sibling runtime StartRepositoryOperation: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest("POST", "/api/daemon/repository-operations/"+op.ID+"/start?runtime_id="+runtimeID, nil, testWorkspaceID, daemonID)
	req = withURLParam(req, "operationId", op.ID)
	testHandler.StartRepositoryOperation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("target runtime StartRepositoryOperation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
