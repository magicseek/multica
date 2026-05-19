package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAgentAssignedIssueCreatesWorkflowRunAtQueueTime(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Runtime Agent", []byte("[]"))

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Queue-time workflow run",
		"status":        "todo",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	t.Cleanup(func() {
		req := newRequest("DELETE", "/api/issues/"+created.ID, nil)
		req = withURLParam(req, "id", created.ID)
		testHandler.DeleteIssue(httptest.NewRecorder(), req)
	})

	var taskID string
	if err := testPool.QueryRow(ctx, `
		SELECT id
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, created.ID).Scan(&taskID); err != nil {
		t.Fatalf("load queued task: %v", err)
	}

	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatalf("GetAgentTask: %v", err)
	}
	if len(task.WorkflowSnapshot) == 0 {
		t.Fatalf("queued task missing workflow snapshot")
	}

	run, err := testHandler.Queries.GetWorkflowRunByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWorkflowRunByTask: %v", err)
	}
	if run.Status != "queued" {
		t.Fatalf("workflow run status = %q, want queued", run.Status)
	}
	if run.TriggerType != "assignment" {
		t.Fatalf("workflow run trigger_type = %q, want assignment", run.TriggerType)
	}
	if !run.WorkflowDefinitionID.Valid || !run.WorkflowRevisionID.Valid {
		t.Fatalf("workflow run should preserve definition and revision ids")
	}
	assertJSONEqual(t, run.Snapshot, string(task.WorkflowSnapshot))

	steps, err := testHandler.Queries.ListWorkflowStepRunsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStepRunsByRun: %v", err)
	}
	if len(steps) == 0 {
		t.Fatalf("workflow run should create initial step runs")
	}
	if steps[0].Status != "ready" {
		t.Fatalf("first workflow step status = %q, want ready", steps[0].Status)
	}
	for _, step := range steps {
		if step.StepDefinitionID == "" {
			t.Fatalf("step run missing step_definition_id: %#v", step)
		}
		if len(step.Snapshot) == 0 {
			t.Fatalf("step %q missing immutable snapshot", step.StepDefinitionID)
		}
	}
}
