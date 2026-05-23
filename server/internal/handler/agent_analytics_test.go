package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentAnalyticsProjectScopeIncludesIssueAndChatRuns(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Analytics Project Agent", []byte("[]"))
	projectID := createHandlerTestProject(t, "Analytics Project", "planned")
	otherProjectID := createHandlerTestProject(t, "Other Analytics Project", "planned")
	issueID := createAnalyticsIssue(t, projectID, "Analytics Issue")
	otherIssueID := createAnalyticsIssue(t, otherProjectID, "Other Analytics Issue")
	chatID := createProjectChatSessionRow(t, agentID, projectID, "Analytics Chat", "recent")

	now := time.Now().UTC()
	createAnalyticsTaskWithUsage(t, analyticsTaskFixture{
		AgentID:      agentID,
		IssueID:      issueID,
		Status:       "completed",
		CreatedAt:    now.Add(-20 * time.Minute),
		DispatchedAt: now.Add(-19 * time.Minute),
		StartedAt:    now.Add(-18 * time.Minute),
		CompletedAt:  now.Add(-10 * time.Minute),
		InputTokens:  100,
		OutputTokens: 50,
		CacheRead:    20,
		CacheWrite:   5,
		PromptBytes:  1000,
		FirstTextMs:  1200,
		ToolUses:     2,
		ToolBytes:    700,
	})
	createAnalyticsTaskWithUsage(t, analyticsTaskFixture{
		AgentID:      agentID,
		ChatID:       chatID,
		Status:       "failed",
		CreatedAt:    now.Add(-16 * time.Minute),
		DispatchedAt: now.Add(-15 * time.Minute),
		StartedAt:    now.Add(-14 * time.Minute),
		CompletedAt:  now.Add(-12 * time.Minute),
		InputTokens:  200,
		OutputTokens: 30,
		PromptBytes:  2000,
		FirstTextMs:  900,
	})
	createAnalyticsTaskWithUsage(t, analyticsTaskFixture{
		AgentID:      agentID,
		IssueID:      otherIssueID,
		Status:       "completed",
		CreatedAt:    now.Add(-8 * time.Minute),
		DispatchedAt: now.Add(-7 * time.Minute),
		StartedAt:    now.Add(-6 * time.Minute),
		CompletedAt:  now.Add(-5 * time.Minute),
		InputTokens:  999,
		OutputTokens: 999,
	})

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/analytics/agent-runs?scope=project&scope_id="+projectID+"&days=30", nil)
	testHandler.GetAgentAnalyticsRuns(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetAgentAnalyticsRuns project: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp AgentAnalyticsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode analytics response: %v", err)
	}
	if resp.Summary.TaskCount != 2 {
		t.Fatalf("task_count: want 2, got %d", resp.Summary.TaskCount)
	}
	if resp.Summary.CompletedCount != 1 || resp.Summary.FailedCount != 1 {
		t.Fatalf("status counts: got completed=%d failed=%d", resp.Summary.CompletedCount, resp.Summary.FailedCount)
	}
	if resp.Summary.InputTokens != 300 || resp.Summary.TotalTokens != 405 {
		t.Fatalf("token totals: got input=%d total=%d", resp.Summary.InputTokens, resp.Summary.TotalTokens)
	}
	if len(resp.Daily) == 0 {
		t.Fatal("project analytics should include daily trend buckets")
	}
	var dailyTokens int64
	for _, row := range resp.Daily {
		if row.Date == "" {
			t.Fatalf("daily trend bucket should include a date: %+v", row)
		}
		dailyTokens += row.TotalTokens
	}
	if dailyTokens != resp.Summary.TotalTokens {
		t.Fatalf("daily trend tokens should match summary total: daily=%d summary=%d", dailyTokens, resp.Summary.TotalTokens)
	}
	if resp.Summary.PromptBytes != 3000 || resp.Summary.ToolUseCount != 2 || resp.Summary.ToolResultBytes != 700 {
		t.Fatalf("tracing summary mismatch: %+v", resp.Summary)
	}
	if resp.PreviousSummary == nil {
		t.Fatal("bounded project window should include previous_summary")
	}
	if resp.Pagination.Total != 2 || len(resp.Runs) != 2 {
		t.Fatalf("run pagination: total=%d len=%d", resp.Pagination.Total, len(resp.Runs))
	}

	var sawIssue, sawChat bool
	for _, source := range resp.Sources {
		switch source.SourceType {
		case "issue":
			sawIssue = true
			if source.IssueIdentifier == nil || *source.IssueIdentifier == "" {
				t.Fatalf("issue source should include identifier: %+v", source)
			}
		case "chat":
			sawChat = true
		}
	}
	if !sawIssue || !sawChat {
		t.Fatalf("expected issue and chat source rows, got %+v", resp.Sources)
	}

	var projectTaskCount int
	for _, row := range resp.Runs {
		if row.SourceType == "issue" && row.IssueID != nil && *row.IssueID == issueID {
			projectTaskCount++
		}
		if row.SourceType == "chat" && row.ChatSessionID != nil && *row.ChatSessionID == chatID {
			projectTaskCount++
		}
		if row.IssueID != nil && *row.IssueID == otherIssueID {
			t.Fatalf("other project run leaked into project analytics: %+v", row)
		}
	}
	if projectTaskCount != 2 {
		t.Fatalf("expected project issue+chat runs, matched %d", projectTaskCount)
	}

	var persisted int
	if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_task_queue WHERE issue_id = $1`, otherIssueID).Scan(&persisted); err != nil {
		t.Fatalf("verify other task persisted: %v", err)
	}
	if persisted != 1 {
		t.Fatalf("other project fixture missing: %d", persisted)
	}
}

func TestAgentAnalyticsChatScopeDefaultsToAllWindow(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	agentID := createHandlerTestAgent(t, "Analytics Chat Agent", []byte("[]"))
	chatID := createHandlerTestChatSession(t, agentID)
	otherChatID := createHandlerTestChatSession(t, agentID)
	now := time.Now().UTC()

	createAnalyticsTaskWithUsage(t, analyticsTaskFixture{
		AgentID:     agentID,
		ChatID:      chatID,
		Status:      "completed",
		CreatedAt:   now.Add(-2 * time.Hour),
		StartedAt:   now.Add(-90 * time.Minute),
		CompletedAt: now.Add(-80 * time.Minute),
		InputTokens: 25,
	})
	createAnalyticsTaskWithUsage(t, analyticsTaskFixture{
		AgentID:     agentID,
		ChatID:      otherChatID,
		Status:      "completed",
		CreatedAt:   now.Add(-1 * time.Hour),
		StartedAt:   now.Add(-55 * time.Minute),
		CompletedAt: now.Add(-50 * time.Minute),
		InputTokens: 75,
	})

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/analytics/agent-runs?scope=chat&scope_id="+chatID, nil)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.GetAgentAnalyticsRuns(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetAgentAnalyticsRuns chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp AgentAnalyticsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode analytics response: %v", err)
	}
	if resp.Window.Days != nil || resp.PreviousSummary != nil {
		t.Fatalf("chat default should be all-time without previous window: window=%+v previous=%+v", resp.Window, resp.PreviousSummary)
	}
	if resp.Summary.TaskCount != 1 || resp.Summary.InputTokens != 25 {
		t.Fatalf("chat summary should include only target chat: %+v", resp.Summary)
	}
	if len(resp.Daily) != 1 || resp.Daily[0].TotalTokens != 25 {
		t.Fatalf("chat analytics should include the target chat daily trend bucket: %+v", resp.Daily)
	}
	if resp.Pagination.Total != 1 || len(resp.Runs) != 1 {
		t.Fatalf("chat runs: total=%d len=%d", resp.Pagination.Total, len(resp.Runs))
	}
	if resp.Runs[0].ChatSessionID == nil || *resp.Runs[0].ChatSessionID != chatID {
		t.Fatalf("run should belong to target chat: %+v", resp.Runs[0])
	}
}

type analyticsTaskFixture struct {
	AgentID      string
	IssueID      string
	ChatID       string
	Status       string
	CreatedAt    time.Time
	DispatchedAt time.Time
	StartedAt    time.Time
	CompletedAt  time.Time
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	PromptBytes  int64
	FirstTextMs  int64
	ToolUses     int64
	ToolBytes    int64
}

func createAnalyticsIssue(t *testing.T, projectID string, title string) string {
	t.Helper()
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, creator_id, creator_type, project_id, number)
		VALUES (
			$1, $2, $3, 'member', $4,
			(SELECT COALESCE(MAX(number), 0) + 1 FROM issue WHERE workspace_id = $1)
		)
		RETURNING id
	`, testWorkspaceID, title, testUserID, projectID).Scan(&issueID); err != nil {
		t.Fatalf("create analytics issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	return issueID
}

func createAnalyticsTaskWithUsage(t *testing.T, fixture analyticsTaskFixture) string {
	t.Helper()
	var issueID any
	if fixture.IssueID != "" {
		issueID = fixture.IssueID
	}
	var chatID any
	if fixture.ChatID != "" {
		chatID = fixture.ChatID
	}
	var dispatchedAt any
	if !fixture.DispatchedAt.IsZero() {
		dispatchedAt = fixture.DispatchedAt
	}
	var startedAt any
	if !fixture.StartedAt.IsZero() {
		startedAt = fixture.StartedAt
	}
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, issue_id, chat_session_id, runtime_id, status,
			created_at, dispatched_at, started_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, fixture.AgentID, issueID, chatID, handlerTestRuntimeID(t), fixture.Status, fixture.CreatedAt, dispatchedAt, startedAt, fixture.CompletedAt).Scan(&taskID); err != nil {
		t.Fatalf("create analytics task: %v", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"prompt_bytes":                fixture.PromptBytes,
		"first_text_ms":               fixture.FirstTextMs,
		"task_message_tool_use_count": fixture.ToolUses,
		"tool_result_bytes":           fixture.ToolBytes,
	})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO task_usage (
			task_id, provider, model, input_tokens, output_tokens,
			cache_read_tokens, cache_write_tokens, metadata, created_at
		)
		VALUES ($1, 'openai', 'gpt-5.4', $2, $3, $4, $5, $6::jsonb, $7)
	`, taskID, fixture.InputTokens, fixture.OutputTokens, fixture.CacheRead, fixture.CacheWrite, metadata, fixture.CompletedAt); err != nil {
		t.Fatalf("create analytics task usage: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}
