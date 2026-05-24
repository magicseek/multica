package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createRequestEfficientHandlerAgent(t *testing.T, name string, enabled bool) string {
	t.Helper()
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id,
			instructions, custom_env, custom_args, request_efficient_enabled
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'private', 1, $4, '', '{}'::jsonb, '[]'::jsonb, $5)
		RETURNING id
	`, testWorkspaceID, name, handlerTestRuntimeID(t), testUserID, enabled).Scan(&agentID); err != nil {
		t.Fatalf("failed to create request-efficient handler test agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})
	return agentID
}

func createBundledHandlerIssue(t *testing.T, title, agentID string) string {
	t.Helper()
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, status, priority, creator_type, creator_id,
			assignee_type, assignee_id
		)
		VALUES ($1, $2, 'todo', 'none', 'member', $3, 'agent', $4)
		RETURNING id
	`, testWorkspaceID, title, testUserID, agentID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create bundled handler issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	return issueID
}

func issueStatusForTest(t *testing.T, issueID string) string {
	t.Helper()
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("failed to load issue status: %v", err)
	}
	return status
}

func createTaskBundleViaHandler(t *testing.T, agentID string, issueIDs []string) TaskBundleResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/task-bundles", map[string]any{
		"agent_id":  agentID,
		"issue_ids": issueIDs,
	})
	testHandler.CreateTaskBundle(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateTaskBundle: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp TaskBundleResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode CreateTaskBundle response: %v", err)
	}
	return resp
}

func checkpointBundleItemViaHandler(t *testing.T, taskID, itemID, status string) TaskBundleResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/bundle/checkpoint", map[string]any{
		"item_id": itemID,
		"status":  status,
		"result":  map[string]any{"summary": status},
	})
	req = withURLParam(req, "taskId", taskID)
	testHandler.CheckpointTaskBundleItem(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CheckpointTaskBundleItem(%s): expected 200, got %d: %s", status, w.Code, w.Body.String())
	}
	var resp TaskBundleResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode CheckpointTaskBundleItem response: %v", err)
	}
	return resp
}

func TestTaskBundleHandlerSequentialCheckpointAndRerun(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID := createRequestEfficientHandlerAgent(t, "bundle-handler-agent", true)
	issueA := createBundledHandlerIssue(t, "bundle issue A", agentID)
	issueB := createBundledHandlerIssue(t, "bundle issue B", agentID)

	created := createTaskBundleViaHandler(t, agentID, []string{issueA, issueB})
	if created.Status != "queued" {
		t.Fatalf("bundle status = %q, want queued", created.Status)
	}
	if created.Task == nil {
		t.Fatal("expected one provider execution task on the bundle response")
	}
	if got := len(created.Items); got != 2 {
		t.Fatalf("items length = %d, want 2", got)
	}

	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'dispatched' WHERE id = $1`, created.Task.ID); err != nil {
		t.Fatalf("mark task dispatched: %v", err)
	}
	if _, err := testHandler.TaskService.StartTask(ctx, parseUUID(created.Task.ID)); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if got := issueStatusForTest(t, issueA); got != "in_progress" {
		t.Fatalf("first issue status after StartTask = %q, want in_progress", got)
	}
	if got := issueStatusForTest(t, issueB); got != "todo" {
		t.Fatalf("second issue status before checkpoint = %q, want todo", got)
	}

	afterFirst := checkpointBundleItemViaHandler(t, created.Task.ID, created.Items[0].ID, "completed")
	if afterFirst.Status != "running" {
		t.Fatalf("bundle status after first checkpoint = %q, want running", afterFirst.Status)
	}
	if afterFirst.Items[0].Status != "completed" || afterFirst.Items[1].Status != "in_progress" {
		t.Fatalf("items after first checkpoint = [%s, %s], want [completed, in_progress]",
			afterFirst.Items[0].Status, afterFirst.Items[1].Status)
	}
	if got := issueStatusForTest(t, issueA); got != "in_review" {
		t.Fatalf("first issue status after completion checkpoint = %q, want in_review", got)
	}
	if got := issueStatusForTest(t, issueB); got != "in_progress" {
		t.Fatalf("second issue status after first checkpoint = %q, want in_progress", got)
	}

	afterSecond := checkpointBundleItemViaHandler(t, created.Task.ID, created.Items[1].ID, "blocked")
	if afterSecond.Status != "blocked" {
		t.Fatalf("bundle status after blocked checkpoint = %q, want blocked", afterSecond.Status)
	}
	if got := issueStatusForTest(t, issueB); got != "blocked" {
		t.Fatalf("second issue status after blocked checkpoint = %q, want blocked", got)
	}

	if _, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(created.Task.ID), []byte(`{"summary":"bundle done"}`), "", ""); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	rerunW := httptest.NewRecorder()
	rerunReq := newRequest(http.MethodPost, "/api/task-bundles/"+created.ID+"/rerun", map[string]any{})
	rerunReq = withURLParam(rerunReq, "id", created.ID)
	testHandler.RerunTaskBundle(rerunW, rerunReq)
	if rerunW.Code != http.StatusCreated {
		t.Fatalf("RerunTaskBundle: expected 201, got %d: %s", rerunW.Code, rerunW.Body.String())
	}
	var rerun TaskBundleResponse
	if err := json.NewDecoder(rerunW.Body).Decode(&rerun); err != nil {
		t.Fatalf("decode rerun response: %v", err)
	}
	if rerun.RerunOfBundleID == nil || *rerun.RerunOfBundleID != created.ID {
		t.Fatalf("rerun_of_bundle_id = %v, want %s", rerun.RerunOfBundleID, created.ID)
	}
	if got := len(rerun.Items); got != 1 {
		t.Fatalf("rerun item count = %d, want only the blocked item", got)
	}
	if rerun.Items[0].IssueID != issueB {
		t.Fatalf("rerun issue = %s, want %s", rerun.Items[0].IssueID, issueB)
	}
}

func TestTaskBundleHandlerRejectsAgentWithoutRequestEfficientMode(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	agentID := createRequestEfficientHandlerAgent(t, "bundle-disabled-agent", false)
	issueID := createBundledHandlerIssue(t, "bundle disabled issue", agentID)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/task-bundles", map[string]any{
		"agent_id":  agentID,
		"issue_ids": []string{issueID},
	})
	testHandler.CreateTaskBundle(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateTaskBundle disabled agent: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got == "" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("expected JSON error body, got %q", got)
	}
}
