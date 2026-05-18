package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createTaskOutputMetadataTestTask(t *testing.T) (taskID string) {
	t.Helper()

	issueID := createTestIssue(t, "Task output metadata test", "todo", "medium")
	t.Cleanup(func() { deleteTestIssue(t, issueID) })
	agentID := createHandlerTestAgent(t, "TaskOutputMetadataAgent", []byte("[]"))

	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
		VALUES ($1, $2, $3, 'running', 0)
		RETURNING id
	`, agentID, handlerTestRuntimeID(t), issueID).Scan(&taskID); err != nil {
		t.Fatalf("create task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}

func TestTaskOutputMetadataUploadAndList(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	taskID := createTaskOutputMetadataTestTask(t)
	repo := createHandlerTestRepository(t, "Task output metadata repo", "https://github.com/multica-ai/task-output-metadata.git")

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/outputs", map[string]any{
		"outputs": []map[string]any{
			{
				"repository_id": repo.ID,
				"relative_path": "docs/design.md",
				"kind":          "doc",
				"size_bytes":    1234,
				"mime_type":     "text/markdown",
				"metadata": map[string]any{
					"label": "design",
				},
			},
		},
	}, testWorkspaceID, "task-output-daemon")
	req = withURLParam(req, "taskId", taskID)
	testHandler.UploadTaskOutputMetadata(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UploadTaskOutputMetadata: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var uploadResp struct {
		Outputs []TaskOutputMetadataResponse `json:"outputs"`
		Total   int                          `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploadResp.Total != 1 || len(uploadResp.Outputs) != 1 {
		t.Fatalf("expected 1 output, got total=%d len=%d", uploadResp.Total, len(uploadResp.Outputs))
	}
	output := uploadResp.Outputs[0]
	if output.RepositoryID == nil || *output.RepositoryID != repo.ID {
		t.Fatalf("repository_id = %v, want %s", output.RepositoryID, repo.ID)
	}
	if output.RelativePath != "docs/design.md" || output.Filename != "design.md" || output.Kind != "doc" {
		t.Fatalf("unexpected output metadata: %+v", output)
	}
	if output.SizeBytes == nil || *output.SizeBytes != 1234 {
		t.Fatalf("size_bytes = %v, want 1234", output.SizeBytes)
	}
	if output.MimeType == nil || *output.MimeType != "text/markdown" {
		t.Fatalf("mime_type = %v, want text/markdown", output.MimeType)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/tasks/"+taskID+"/outputs", nil)
	req = withURLParam(req, "taskId", taskID)
	testHandler.ListTaskOutputMetadata(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListTaskOutputMetadata: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Outputs []TaskOutputMetadataResponse `json:"outputs"`
		Total   int                          `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Outputs) != 1 {
		t.Fatalf("expected 1 listed output, got total=%d len=%d", listResp.Total, len(listResp.Outputs))
	}
	if listResp.Outputs[0].RelativePath != "docs/design.md" {
		t.Fatalf("listed relative_path = %q", listResp.Outputs[0].RelativePath)
	}
}

func TestListTaskOutputMetadata_ChatTaskRequiresChatSessionAccess(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID, ownerID, memberID := privateAgentTestFixture(t)
	repo := createHandlerTestRepository(t, "Private chat task output repo", "https://github.com/multica-ai/private-chat-task-output.git")

	var sessionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id, title, status)
		VALUES ($1, $2, $3, 'private output session', 'active')
		RETURNING id
	`, testWorkspaceID, agentID, ownerID).Scan(&sessionID); err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID)
	})

	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, chat_session_id, status, priority)
		VALUES ($1, $2, $3, 'running', 0)
		RETURNING id
	`, agentID, handlerTestRuntimeID(t), sessionID).Scan(&taskID); err != nil {
		t.Fatalf("create chat task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	uploadW := httptest.NewRecorder()
	uploadReq := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/outputs", map[string]any{
		"outputs": []map[string]any{
			{
				"repository_id": repo.ID,
				"relative_path": "docs/private-chat-output.md",
				"kind":          "doc",
			},
		},
	}, testWorkspaceID, "private-chat-output-daemon")
	uploadReq = withURLParam(uploadReq, "taskId", taskID)
	testHandler.UploadTaskOutputMetadata(uploadW, uploadReq)
	if uploadW.Code != http.StatusOK {
		t.Fatalf("UploadTaskOutputMetadata: expected 200, got %d: %s", uploadW.Code, uploadW.Body.String())
	}

	ownerW := httptest.NewRecorder()
	ownerReq := newRequestAs(ownerID, "GET", "/api/tasks/"+taskID+"/outputs", nil)
	ownerReq = withURLParam(ownerReq, "taskId", taskID)
	ownerReq = withChatTestWorkspaceCtxAs(t, ownerReq, ownerID)
	testHandler.ListTaskOutputMetadata(ownerW, ownerReq)
	if ownerW.Code != http.StatusOK {
		t.Fatalf("ListTaskOutputMetadata as chat owner: expected 200, got %d: %s", ownerW.Code, ownerW.Body.String())
	}

	memberW := httptest.NewRecorder()
	memberReq := newRequestAs(memberID, "GET", "/api/tasks/"+taskID+"/outputs", nil)
	memberReq = withURLParam(memberReq, "taskId", taskID)
	memberReq = withChatTestWorkspaceCtxAs(t, memberReq, memberID)
	testHandler.ListTaskOutputMetadata(memberW, memberReq)
	if memberW.Code != http.StatusForbidden {
		t.Fatalf("ListTaskOutputMetadata as unrelated member: expected 403, got %d: %s", memberW.Code, memberW.Body.String())
	}
}

func TestTaskOutputMetadataRejectsUnsafeManifest(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	taskID := createTaskOutputMetadataTestTask(t)
	tests := []struct {
		name string
		item map[string]any
	}{
		{
			name: "absolute path",
			item: map[string]any{"relative_path": "/Users/tester/private.md", "kind": "doc"},
		},
		{
			name: "parent traversal",
			item: map[string]any{"relative_path": "../private.md", "kind": "doc"},
		},
		{
			name: "backslash",
			item: map[string]any{"relative_path": `docs\private.md`, "kind": "doc"},
		},
		{
			name: "home path",
			item: map[string]any{"relative_path": "~/private.md", "kind": "doc"},
		},
		{
			name: "windows drive",
			item: map[string]any{"relative_path": "C:/Users/tester/private.md", "kind": "doc"},
		},
		{
			name: "private metadata",
			item: map[string]any{
				"relative_path": "docs/safe.md",
				"kind":          "doc",
				"metadata": map[string]any{
					"local_path": "/Users/tester/private.md",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/outputs", map[string]any{
				"outputs": []map[string]any{tt.item},
			}, testWorkspaceID, "task-output-daemon")
			req = withURLParam(req, "taskId", taskID)
			testHandler.UploadTaskOutputMetadata(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
