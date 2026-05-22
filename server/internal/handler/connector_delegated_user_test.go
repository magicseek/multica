package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func latestTaskConnectorDelegatedUser(t *testing.T, where string, args ...any) string {
	t.Helper()
	query := `SELECT COALESCE(connector_delegated_user_id::text, '')
FROM agent_task_queue
WHERE ` + where + `
ORDER BY created_at DESC
LIMIT 1`
	var delegatedUserID string
	if err := testPool.QueryRow(context.Background(), query, args...).Scan(&delegatedUserID); err != nil {
		t.Fatalf("load connector delegated user: %v", err)
	}
	return delegatedUserID
}

func requireCurrentWorkflowRunSchema(t *testing.T) {
	t.Helper()
	var ok bool
	if err := testPool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'workflow_run'
			  AND column_name = 'agent_task_queue_id'
		)
	`).Scan(&ok); err != nil {
		t.Fatalf("inspect workflow_run schema: %v", err)
	}
	if !ok {
		t.Skip("local test database has a pre-095 workflow_run schema")
	}
}

func TestIssueTaskCapturesConnectorDelegatedUserFromCreator(t *testing.T) {
	requireCurrentWorkflowRunSchema(t)

	agentID := createHandlerTestAgent(t, "ConnectorDelegatedIssueAgent", []byte("[]"))

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Connector delegated user issue",
		"status":        "todo",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, issue.ID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issue.ID)
	})

	got := latestTaskConnectorDelegatedUser(t, "issue_id = $1 AND agent_id = $2", issue.ID, agentID)
	if got != testUserID {
		t.Fatalf("connector_delegated_user_id = %q, want %q", got, testUserID)
	}
}

func TestAgentCommentTaskInheritsConnectorDelegatedUser(t *testing.T) {
	requireCurrentWorkflowRunSchema(t)

	ctx := context.Background()
	authorAgentID := createHandlerTestAgent(t, "ConnectorDelegatedAuthorAgent", []byte("[]"))
	targetAgentID := createHandlerTestAgent(t, "ConnectorDelegatedTargetAgent", []byte("[]"))

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "Connector delegated user inherit",
		"status": "todo",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issueResp IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issueResp); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueResp.ID)
		testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id = $1`, issueResp.ID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueResp.ID)
	})

	if _, err := testPool.Exec(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, connector_delegated_user_id
		)
		VALUES ($1, $2, $3, 'completed', 0, $4)
	`, authorAgentID, handlerTestRuntimeID(t), issueResp.ID, testUserID); err != nil {
		t.Fatalf("seed parent task: %v", err)
	}

	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueResp.ID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	comment, err := testHandler.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     parseUUID(issueResp.ID),
		WorkspaceID: parseUUID(testWorkspaceID),
		AuthorType:  "agent",
		AuthorID:    parseUUID(authorAgentID),
		Content:     "please take this over",
		Type:        "comment",
	})
	if err != nil {
		t.Fatalf("create agent comment: %v", err)
	}

	task, err := testHandler.TaskService.EnqueueTaskForMention(ctx, issue, parseUUID(targetAgentID), comment.ID)
	if err != nil {
		t.Fatalf("EnqueueTaskForMention: %v", err)
	}
	if got := uuidToString(task.ConnectorDelegatedUserID); got != testUserID {
		t.Fatalf("connector_delegated_user_id = %q, want %q", got, testUserID)
	}
}

func TestChatTaskCapturesConnectorDelegatedUserFromSessionCreator(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ConnectorDelegatedChatAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content": "run this from chat",
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.SendChatMessage(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("SendChatMessage: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE chat_session_id = $1`, sessionID)
	})

	got := latestTaskConnectorDelegatedUser(t, "chat_session_id = $1", sessionID)
	if got != testUserID {
		t.Fatalf("connector_delegated_user_id = %q, want %q", got, testUserID)
	}
}

func TestAutopilotRunOnlyTaskCapturesExplicitConnectorDelegatedUser(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ConnectorDelegatedAutopilotAgent", []byte("[]"))

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/autopilots?workspace_id="+testWorkspaceID, map[string]any{
		"title":                       "Connector delegated autopilot",
		"assignee_id":                 agentID,
		"execution_mode":              "run_only",
		"connector_delegated_user_id": testUserID,
	})
	testHandler.CreateAutopilot(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAutopilot: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var autopilot AutopilotResponse
	if err := json.NewDecoder(w.Body).Decode(&autopilot); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE autopilot_run_id IN (SELECT id FROM autopilot_run WHERE autopilot_id = $1)`, autopilot.ID)
		testPool.Exec(context.Background(), `DELETE FROM autopilot WHERE id = $1`, autopilot.ID)
	})
	if autopilot.ConnectorDelegatedUserID == nil || *autopilot.ConnectorDelegatedUserID != testUserID {
		t.Fatalf("response connector_delegated_user_id = %v, want %q", autopilot.ConnectorDelegatedUserID, testUserID)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/autopilots/"+autopilot.ID+"/trigger?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "id", autopilot.ID)
	testHandler.TriggerAutopilot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("TriggerAutopilot: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got := latestTaskConnectorDelegatedUser(t, "autopilot_run_id IN (SELECT id FROM autopilot_run WHERE autopilot_id = $1)", autopilot.ID)
	if got != testUserID {
		t.Fatalf("connector_delegated_user_id = %q, want %q", got, testUserID)
	}
}
