package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func uploadTaskOutputMetadataForTest(t *testing.T, taskID, relativePath, kind string) {
	t.Helper()

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/outputs", map[string]any{
		"outputs": []map[string]any{
			{
				"relative_path": relativePath,
				"kind":          kind,
				"size_bytes":    42,
				"mime_type":     "text/markdown",
			},
		},
	}, testWorkspaceID, "chat-output-aggregation-daemon")
	req = withURLParam(req, "taskId", taskID)
	testHandler.UploadTaskOutputMetadata(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UploadTaskOutputMetadata(%s): expected 200, got %d: %s", taskID, w.Code, w.Body.String())
	}
}

func createChatOriginIssueForOutputsTest(t *testing.T, sessionID string) string {
	t.Helper()

	ctx := context.Background()
	workspaceUUID := parseUUID(testWorkspaceID)
	issueNumber, err := testHandler.Queries.IncrementIssueCounter(ctx, workspaceUUID)
	if err != nil {
		t.Fatalf("increment issue counter: %v", err)
	}
	issue, err := testHandler.Queries.CreateIssueWithOrigin(ctx, db.CreateIssueWithOriginParams{
		WorkspaceID:   workspaceUUID,
		Title:         "Chat-origin output issue",
		Description:   pgtype.Text{String: "Issue created from chat proposal", Valid: true},
		Status:        "backlog",
		Priority:      "none",
		AssigneeType:  pgtype.Text{},
		AssigneeID:    pgtype.UUID{},
		CreatorType:   "member",
		CreatorID:     parseUUID(testUserID),
		ParentIssueID: pgtype.UUID{},
		Position:      0,
		DueDate:       pgtype.Timestamptz{},
		Number:        issueNumber,
		ProjectID:     pgtype.UUID{},
		OriginType:    pgtype.Text{String: "chat_session", Valid: true},
		OriginID:      parseUUID(sessionID),
	})
	if err != nil {
		t.Fatalf("create chat-origin issue: %v", err)
	}
	issueID := uuidToString(issue.ID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	return issueID
}

func createIssueOutputTaskForTest(t *testing.T, agentID, issueID string) string {
	t.Helper()

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, started_at)
		VALUES ($1, $2, $3, 'running', 0, now())
		RETURNING id
	`, agentID, handlerTestRuntimeID(t), issueID).Scan(&taskID); err != nil {
		t.Fatalf("create issue output task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}

func TestListChatOutputsAggregatesDirectAndChatOriginIssueTasks(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID, sessionID, chatTaskID := createChatStructuredOutputTestTask(t, "legacy")

	projectID := createRepositoryTestProject(t, "Deleted project for chat outputs")
	if _, err := testPool.Exec(ctx, `
		UPDATE chat_session
		SET project_id = $1::uuid,
		    project_context_kind = 'project',
		    project_snapshot = jsonb_build_object('id', ($1::uuid)::text, 'title', 'Deleted project for chat outputs')
		WHERE id = $2
	`, projectID, sessionID); err != nil {
		t.Fatalf("attach project snapshot to chat session: %v", err)
	}
	if _, err := testPool.Exec(ctx, `DELETE FROM project WHERE id = $1`, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	uploadTaskOutputMetadataForTest(t, chatTaskID, "docs/direct-chat-output.md", "doc")

	chatIssueID := createChatOriginIssueForOutputsTest(t, sessionID)
	chatIssueTaskID := createIssueOutputTaskForTest(t, agentID, chatIssueID)
	uploadTaskOutputMetadataForTest(t, chatIssueTaskID, "docs/chat-origin-issue-output.md", "report")

	unrelatedIssueID := createTestIssue(t, "Unrelated project issue output", "todo", "medium")
	unrelatedTaskID := createIssueOutputTaskForTest(t, agentID, unrelatedIssueID)
	uploadTaskOutputMetadataForTest(t, unrelatedTaskID, "docs/unrelated-output.md", "doc")

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/chat/sessions/"+sessionID+"/outputs", nil)
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.ListChatOutputs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListChatOutputs: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Outputs []TaskOutputMetadataResponse `json:"outputs"`
		Total   int                          `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode chat outputs: %v", err)
	}
	if resp.Total != 2 || len(resp.Outputs) != 2 {
		t.Fatalf("chat output count = total %d len %d, want 2", resp.Total, len(resp.Outputs))
	}

	byPath := make(map[string]TaskOutputMetadataResponse, len(resp.Outputs))
	for _, output := range resp.Outputs {
		byPath[output.RelativePath] = output
	}
	direct, ok := byPath["docs/direct-chat-output.md"]
	if !ok {
		t.Fatalf("direct chat output missing: %+v", resp.Outputs)
	}
	if direct.SourceType == nil || *direct.SourceType != "chat_task" {
		t.Fatalf("direct output source_type = %v, want chat_task", direct.SourceType)
	}
	if direct.SourceIssueID != nil || direct.SourceIssueIdentifier != nil || direct.SourceIssueTitle != nil {
		t.Fatalf("direct output should not have issue source: %+v", direct)
	}

	issueOutput, ok := byPath["docs/chat-origin-issue-output.md"]
	if !ok {
		t.Fatalf("chat-origin issue output missing: %+v", resp.Outputs)
	}
	if issueOutput.SourceType == nil || *issueOutput.SourceType != "issue_task" {
		t.Fatalf("issue output source_type = %v, want issue_task", issueOutput.SourceType)
	}
	if issueOutput.SourceIssueID == nil || *issueOutput.SourceIssueID != chatIssueID {
		t.Fatalf("issue output source_issue_id = %v, want %s", issueOutput.SourceIssueID, chatIssueID)
	}
	if issueOutput.SourceIssueIdentifier == nil || *issueOutput.SourceIssueIdentifier == "" {
		t.Fatalf("issue output source_issue_identifier missing: %+v", issueOutput)
	}
	if issueOutput.SourceIssueTitle == nil || *issueOutput.SourceIssueTitle != "Chat-origin output issue" {
		t.Fatalf("issue output source_issue_title = %v", issueOutput.SourceIssueTitle)
	}
	if _, ok := byPath["docs/unrelated-output.md"]; ok {
		t.Fatalf("unrelated issue output was included: %+v", resp.Outputs)
	}
}
