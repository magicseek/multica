package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowRunToResponseCoalescesCompletedRunSteps(t *testing.T) {
	completedAt := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	run := db.WorkflowRun{
		ID:               parseUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:      parseUUID("22222222-2222-2222-2222-222222222222"),
		AgentTaskQueueID: parseUUID("33333333-3333-3333-3333-333333333333"),
		TriggerType:      "assignment",
		Status:           "completed",
		CompletedAt:      pgtype.Timestamptz{Time: completedAt, Valid: true},
		CreatedAt:        pgtype.Timestamptz{Time: completedAt.Add(-time.Minute), Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: completedAt, Valid: true},
	}
	steps := []db.WorkflowStepRun{
		{
			ID:               parseUUID("44444444-4444-4444-4444-444444444444"),
			WorkflowRunID:    run.ID,
			StepDefinitionID: "context",
			Title:            "Context first",
			Status:           "ready",
			ExecutionKind:    "agent",
			CreatedAt:        run.CreatedAt,
			UpdatedAt:        run.UpdatedAt,
		},
		{
			ID:               parseUUID("55555555-5555-5555-5555-555555555555"),
			WorkflowRunID:    run.ID,
			StepDefinitionID: "implement",
			Title:            "Implement",
			Status:           "pending",
			ExecutionKind:    "agent",
			CreatedAt:        run.CreatedAt,
			UpdatedAt:        run.UpdatedAt,
		},
		{
			ID:               parseUUID("66666666-6666-6666-6666-666666666666"),
			WorkflowRunID:    run.ID,
			StepDefinitionID: "blocked",
			Title:            "Blocked path",
			Status:           "skipped",
			ExecutionKind:    "agent",
			CreatedAt:        run.CreatedAt,
			UpdatedAt:        run.UpdatedAt,
		},
	}

	resp := workflowRunToResponse(run, steps, nil, nil, nil)

	if got := resp.Steps[0].Status; got != "completed" {
		t.Fatalf("ready step status = %q, want completed", got)
	}
	if got := resp.Steps[1].Status; got != "completed" {
		t.Fatalf("pending step status = %q, want completed", got)
	}
	if resp.Steps[1].CompletedAt == nil || *resp.Steps[1].CompletedAt != completedAt.Format(time.RFC3339) {
		t.Fatalf("coalesced step completed_at = %v, want run completed_at", resp.Steps[1].CompletedAt)
	}
	if got := resp.Steps[2].Status; got != "skipped" {
		t.Fatalf("successful terminal step status = %q, want skipped", got)
	}
}

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

func TestCompleteManualWorkflowStepRunHonorsRequiredReview(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Manual Review Agent", []byte("[]"))
	workflowID := createManualReviewWorkflowDefinition(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":                           "Manual review workflow run",
		"status":                          "todo",
		"priority":                        "medium",
		"assignee_type":                   "agent",
		"assignee_id":                     agentID,
		"workflow_override_definition_id": workflowID,
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

	var runID string
	if err := testPool.QueryRow(ctx, `
		SELECT wr.id
		FROM workflow_run wr
		JOIN agent_task_queue atq ON atq.id = wr.agent_task_queue_id
		WHERE atq.issue_id = $1
	`, created.ID).Scan(&runID); err != nil {
		t.Fatalf("load workflow run: %v", err)
	}

	steps, err := testHandler.Queries.ListWorkflowStepRunsByRun(ctx, parseUUID(runID))
	if err != nil {
		t.Fatalf("ListWorkflowStepRunsByRun: %v", err)
	}
	var manualStepID string
	for _, step := range steps {
		if step.StepDefinitionID == "triage" {
			manualStepID = uuidToString(step.ID)
			if step.Status != "waiting_manual" {
				t.Fatalf("manual step initial status = %q, want waiting_manual", step.Status)
			}
		}
	}
	if manualStepID == "" {
		t.Fatal("manual triage step not found")
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/workflow-step-runs/"+manualStepID+"/manual-complete", nil)
	req = withURLParam(req, "id", manualStepID)
	testHandler.CompleteManualWorkflowStepRun(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteManualWorkflowStepRun: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var completed WorkflowStepRunResponse
	if err := json.NewDecoder(w.Body).Decode(&completed); err != nil {
		t.Fatalf("decode completed step: %v", err)
	}
	if completed.Status != "waiting_review" {
		t.Fatalf("manual required-review step status = %q, want waiting_review", completed.Status)
	}

	detail, ok := testHandler.workflowRunDetailResponse(httptest.NewRecorder(), newRequest("GET", "/api/workflow-runs/"+runID, nil), mustParseWorkflowRun(t, runID))
	if !ok {
		t.Fatal("workflowRunDetailResponse failed")
	}
	if len(detail.Reviews) != 1 {
		t.Fatalf("reviews count = %d, want 1", len(detail.Reviews))
	}
	for _, step := range detail.Steps {
		if step.StepDefinitionID == "implementation" && step.Status != "pending" {
			t.Fatalf("dependent implementation status = %q, want pending before review approval", step.Status)
		}
	}
}

func createManualReviewWorkflowDefinition(t *testing.T) string {
	t.Helper()

	schema := `{
		"schema_version": 2,
		"name": "Manual review workflow",
		"description": "Manual review regression workflow",
		"applicability": ["assignment"],
		"source": {
			"format": "markdown",
			"body_template": "Manual review regression"
		},
		"steps": [
			{
				"id": "triage",
				"title": "Triage",
				"order": 1,
				"execution": { "kind": "manual" },
				"review": { "required": true }
			},
			{
				"id": "implementation",
				"title": "Implementation",
				"order": 2,
				"depends_on": ["triage"],
				"execution": { "kind": "agent" }
			}
		]
	}`

	var workflowID, revisionID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO workflow_definition (workspace_id, name, description, origin, created_by)
		VALUES ($1, 'Manual review workflow', 'Manual review regression workflow', 'user', $2)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&workflowID); err != nil {
		t.Fatalf("insert workflow definition: %v", err)
	}
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO workflow_revision (workflow_definition_id, revision_number, status, schema, created_by, published_at)
		VALUES ($1, 1, 'published', $2::jsonb, $3, now())
		RETURNING id
	`, workflowID, schema, testUserID).Scan(&revisionID); err != nil {
		t.Fatalf("insert workflow revision: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE workflow_definition
		SET current_published_revision_id = $1
		WHERE id = $2
	`, revisionID, workflowID); err != nil {
		t.Fatalf("set current revision: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workflow_definition WHERE id = $1`, workflowID)
	})

	return workflowID
}

func mustParseWorkflowRun(t *testing.T, id string) db.WorkflowRun {
	t.Helper()
	run, err := testHandler.Queries.GetWorkflowRun(context.Background(), parseUUID(id))
	if err != nil {
		t.Fatalf("GetWorkflowRun: %v", err)
	}
	return run
}
