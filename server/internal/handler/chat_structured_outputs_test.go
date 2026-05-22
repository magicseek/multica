package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

var chatStructuredOutputAgentSeq atomic.Int64

func createChatStructuredOutputTestTask(t *testing.T, titleSource string) (agentID, sessionID, taskID string) {
	t.Helper()

	agentID = createHandlerTestAgent(t, fmt.Sprintf("ChatStructuredOutputAgent-%d", chatStructuredOutputAgentSeq.Add(1)), []byte("[]"))
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO chat_session (
			workspace_id, agent_id, creator_id, title, status, title_source
		)
		VALUES ($1, $2, $3, 'Initial chat title', 'active', $4)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID, titleSource).Scan(&sessionID); err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID)
	})

	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, chat_session_id, status, priority, started_at
		)
		VALUES ($1, $2, $3, 'running', 0, now())
		RETURNING id
	`, agentID, handlerTestRuntimeID(t), sessionID).Scan(&taskID); err != nil {
		t.Fatalf("create chat task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	return agentID, sessionID, taskID
}

func createChatStructuredOutputTestTaskForSession(t *testing.T, agentID, sessionID string) string {
	t.Helper()

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, chat_session_id, status, priority, started_at
		)
		VALUES ($1, $2, $3, 'running', 0, now())
		RETURNING id
	`, agentID, handlerTestRuntimeID(t), sessionID).Scan(&taskID); err != nil {
		t.Fatalf("create chat task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}

func completeChatStructuredOutputTask(t *testing.T, taskID string, structured map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output":             "Assistant reply from chat task",
		"structured_outputs": structured,
	}, testWorkspaceID, "chat-structured-output-daemon")
	req = withURLParam(req, "taskId", taskID)

	testHandler.CompleteTask(w, req)
	return w
}

func TestCompleteTask_ChatStructuredOutputsPersistSummaryProposalAndOutputs(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID, sessionID, taskID := createChatStructuredOutputTestTask(t, "legacy")
	repo := createHandlerTestRepository(t, "Chat structured output repo", "https://github.com/multica-ai/chat-structured-output.git")

	outputManifest := map[string]any{
		"outputs": []map[string]any{
			{
				"repository_id": repo.ID,
				"relative_path": "docs/chat-handoff.md",
				"kind":          "doc",
				"size_bytes":    321,
				"mime_type":     "text/markdown",
				"metadata": map[string]any{
					"source": "chat",
				},
			},
		},
	}
	structuredPayload := map[string]any{
		"chat_summary": map[string]any{
			"version": 1,
			"title":   "  Agent summarized title  ",
		},
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title":   "Implementation follow-ups",
					"summary": "Create these issues after the user reviews them.",
					"items": []map[string]any{
						{
							"title":       "Build proposal review flow",
							"description": "Let users edit and accept issue proposals from chat.",
							"priority":    "medium",
							"labels":      []string{"chat", "backend"},
						},
					},
				},
			},
		},
		"outputs": outputManifest,
	}
	w := completeChatStructuredOutputTask(t, taskID, structuredPayload)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var title, titleSource string
	if err := testPool.QueryRow(ctx, `
		SELECT title, title_source FROM chat_session WHERE id = $1
	`, sessionID).Scan(&title, &titleSource); err != nil {
		t.Fatalf("query chat session title: %v", err)
	}
	if title != "Agent summarized title" || titleSource != "agent_summary" {
		t.Fatalf("chat title/source = %q/%q, want agent summary title", title, titleSource)
	}

	var assistantMessageID, assistantContent string
	if err := testPool.QueryRow(ctx, `
		SELECT id, content FROM chat_message
		WHERE chat_session_id = $1 AND task_id = $2 AND role = 'assistant'
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID, taskID).Scan(&assistantMessageID, &assistantContent); err != nil {
		t.Fatalf("query assistant chat message: %v", err)
	}
	if assistantContent != "Assistant reply from chat task" {
		t.Fatalf("assistant content = %q", assistantContent)
	}

	var (
		proposalID       string
		proposalTitle    string
		proposalSummary  sql.NullString
		sourceMessageID  string
		sourceTaskID     string
		proposerAgentID  string
		proposalStatus   string
		proposalRowCount int
	)
	if err := testPool.QueryRow(ctx, `
		SELECT id, title, summary, source_chat_message_id::text, source_task_id::text,
		       proposer_agent_id::text, status, count(*) OVER()
		FROM chat_issue_proposal
		WHERE chat_session_id = $1
	`, sessionID).Scan(
		&proposalID,
		&proposalTitle,
		&proposalSummary,
		&sourceMessageID,
		&sourceTaskID,
		&proposerAgentID,
		&proposalStatus,
		&proposalRowCount,
	); err != nil {
		t.Fatalf("query issue proposal: %v", err)
	}
	if proposalRowCount != 1 {
		t.Fatalf("proposal count = %d, want 1", proposalRowCount)
	}
	if proposalTitle != "Implementation follow-ups" || !proposalSummary.Valid || proposalSummary.String == "" {
		t.Fatalf("unexpected proposal title/summary: %q/%v", proposalTitle, proposalSummary)
	}
	if sourceMessageID != assistantMessageID || sourceTaskID != taskID || proposerAgentID != agentID {
		t.Fatalf("proposal provenance = message %s task %s agent %s", sourceMessageID, sourceTaskID, proposerAgentID)
	}
	if proposalStatus != "pending" {
		t.Fatalf("proposal status = %q, want pending", proposalStatus)
	}

	var (
		itemTitle       string
		itemDescription string
		itemPriority    sql.NullString
		itemLabels      []byte
		itemStatus      string
	)
	if err := testPool.QueryRow(ctx, `
		SELECT title, description, priority, labels, status
		FROM chat_issue_proposal_item
		WHERE proposal_id = $1
	`, proposalID).Scan(&itemTitle, &itemDescription, &itemPriority, &itemLabels, &itemStatus); err != nil {
		t.Fatalf("query issue proposal item: %v", err)
	}
	if itemTitle != "Build proposal review flow" || itemDescription == "" {
		t.Fatalf("unexpected item title/description: %q/%q", itemTitle, itemDescription)
	}
	if !itemPriority.Valid || itemPriority.String != "medium" {
		t.Fatalf("item priority = %v, want medium", itemPriority)
	}
	var labels []string
	if err := json.Unmarshal(itemLabels, &labels); err != nil {
		t.Fatalf("decode labels: %v", err)
	}
	if len(labels) != 2 || labels[0] != "chat" || labels[1] != "backend" {
		t.Fatalf("labels = %v", labels)
	}
	if itemStatus != "pending" {
		t.Fatalf("item status = %q, want pending", itemStatus)
	}

	var outputCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_output_metadata WHERE task_id = $1`, taskID).Scan(&outputCount); err != nil {
		t.Fatalf("count output metadata: %v", err)
	}
	if outputCount != 1 {
		t.Fatalf("output metadata count = %d, want 1", outputCount)
	}

	replayW := completeChatStructuredOutputTask(t, taskID, structuredPayload)
	if replayW.Code != http.StatusOK {
		t.Fatalf("CompleteTask replay: expected 200, got %d: %s", replayW.Code, replayW.Body.String())
	}
	var replayProposalCount, replayItemCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM chat_issue_proposal WHERE chat_session_id = $1`, sessionID).Scan(&replayProposalCount); err != nil {
		t.Fatalf("count replay proposals: %v", err)
	}
	if replayProposalCount != 1 {
		t.Fatalf("proposal count after replay = %d, want 1", replayProposalCount)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM chat_issue_proposal_item item
		JOIN chat_issue_proposal proposal ON proposal.id = item.proposal_id
		WHERE proposal.chat_session_id = $1
	`, sessionID).Scan(&replayItemCount); err != nil {
		t.Fatalf("count replay proposal items: %v", err)
	}
	if replayItemCount != 1 {
		t.Fatalf("proposal item count after replay = %d, want 1", replayItemCount)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_output_metadata WHERE task_id = $1`, taskID).Scan(&outputCount); err != nil {
		t.Fatalf("count output metadata after replay: %v", err)
	}
	if outputCount != 1 {
		t.Fatalf("output metadata count after replay = %d, want 1", outputCount)
	}

	uploadW := httptest.NewRecorder()
	uploadReq := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/outputs", outputManifest, testWorkspaceID, "chat-structured-output-daemon")
	uploadReq = withURLParam(uploadReq, "taskId", taskID)
	testHandler.UploadTaskOutputMetadata(uploadW, uploadReq)
	if uploadW.Code != http.StatusOK {
		t.Fatalf("UploadTaskOutputMetadata: expected 200, got %d: %s", uploadW.Code, uploadW.Body.String())
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_output_metadata WHERE task_id = $1`, taskID).Scan(&outputCount); err != nil {
		t.Fatalf("count output metadata after upload: %v", err)
	}
	if outputCount != 1 {
		t.Fatalf("output metadata count after compatibility upload = %d, want 1", outputCount)
	}
}

func TestCompleteTask_ChatIssueProposalsSupersedeOnlyWithinSameChat(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID, sessionID, firstTaskID := createChatStructuredOutputTestTask(t, "legacy")
	_, otherSessionID, otherTaskID := createChatStructuredOutputTestTask(t, "legacy")

	firstPayload := map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title": "First chat proposal",
					"items": []map[string]any{
						{"title": "Old pending item", "description": "This should be replaced by the next same-chat task."},
					},
				},
			},
		},
	}
	otherPayload := map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title": "Other chat proposal",
					"items": []map[string]any{
						{"title": "Other pending item", "description": "This belongs to a different chat."},
					},
				},
			},
		},
	}
	for taskID, payload := range map[string]map[string]any{
		firstTaskID: firstPayload,
		otherTaskID: otherPayload,
	} {
		w := completeChatStructuredOutputTask(t, taskID, payload)
		if w.Code != http.StatusOK {
			t.Fatalf("CompleteTask(%s): expected 200, got %d: %s", taskID, w.Code, w.Body.String())
		}
	}

	secondTaskID := createChatStructuredOutputTestTaskForSession(t, agentID, sessionID)
	secondPayload := map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title": "Second chat proposal",
					"items": []map[string]any{
						{"title": "New pending item", "description": "This is the current same-chat proposal."},
					},
				},
			},
		},
	}
	w := completeChatStructuredOutputTask(t, secondTaskID, secondPayload)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask second: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	rows, err := testPool.Query(ctx, `
		SELECT p.title, p.status, i.status
		FROM chat_issue_proposal p
		JOIN chat_issue_proposal_item i ON i.proposal_id = p.id
		WHERE p.chat_session_id = $1
		ORDER BY p.title ASC
	`, sessionID)
	if err != nil {
		t.Fatalf("query same-chat proposals: %v", err)
	}
	defer rows.Close()
	got := map[string][2]string{}
	for rows.Next() {
		var title, proposalStatus, itemStatus string
		if err := rows.Scan(&title, &proposalStatus, &itemStatus); err != nil {
			t.Fatalf("scan same-chat proposal: %v", err)
		}
		got[title] = [2]string{proposalStatus, itemStatus}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate same-chat proposals: %v", err)
	}
	if got["First chat proposal"] != [2]string{"superseded", "skipped"} {
		t.Fatalf("first proposal state = %v, want superseded/skipped", got["First chat proposal"])
	}
	if got["Second chat proposal"] != [2]string{"pending", "pending"} {
		t.Fatalf("second proposal state = %v, want pending/pending", got["Second chat proposal"])
	}

	var otherProposalStatus, otherItemStatus string
	if err := testPool.QueryRow(ctx, `
		SELECT p.status, i.status
		FROM chat_issue_proposal p
		JOIN chat_issue_proposal_item i ON i.proposal_id = p.id
		WHERE p.chat_session_id = $1
	`, otherSessionID).Scan(&otherProposalStatus, &otherItemStatus); err != nil {
		t.Fatalf("query other-chat proposal: %v", err)
	}
	if otherProposalStatus != "pending" || otherItemStatus != "pending" {
		t.Fatalf("other chat proposal = %s/%s, want pending/pending", otherProposalStatus, otherItemStatus)
	}
}

func TestCompleteTask_ChatIssueProposalReplayPreservesUserActions(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	_, sessionID, taskID := createChatStructuredOutputTestTask(t, "legacy")

	structuredPayload := map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title": "Replay-safe proposal",
					"items": []map[string]any{
						{
							"title":       "Preserve created issue",
							"description": "This one is approved before replay.",
							"priority":    "medium",
						},
						{
							"title":       "Preserve skipped item",
							"description": "This one is skipped by selecting the first item.",
							"priority":    "low",
						},
					},
				},
			},
		},
	}
	w := completeChatStructuredOutputTask(t, taskID, structuredPayload)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var proposalID string
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM chat_issue_proposal WHERE chat_session_id = $1
	`, sessionID).Scan(&proposalID); err != nil {
		t.Fatalf("query proposal: %v", err)
	}
	rows, err := testPool.Query(ctx, `
		SELECT id FROM chat_issue_proposal_item
		WHERE proposal_id = $1
		ORDER BY position ASC
	`, proposalID)
	if err != nil {
		t.Fatalf("query proposal items: %v", err)
	}
	var itemIDs []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			rows.Close()
			t.Fatalf("scan item id: %v", err)
		}
		itemIDs = append(itemIDs, itemID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatalf("iterate proposal items: %v", err)
	}
	rows.Close()
	if len(itemIDs) != 2 {
		t.Fatalf("item count = %d, want 2", len(itemIDs))
	}

	approveW := httptest.NewRecorder()
	approveReq := newRequest("POST", "/api/chat/issue-proposals/"+proposalID+"/approve", map[string]any{
		"item_ids": []string{itemIDs[0]},
	})
	approveReq = withURLParam(approveReq, "proposalId", proposalID)
	approveReq = withChatTestWorkspaceCtx(t, approveReq)
	testHandler.ApproveChatIssueProposal(approveW, approveReq)
	if approveW.Code != http.StatusOK {
		t.Fatalf("ApproveChatIssueProposal: expected 200, got %d: %s", approveW.Code, approveW.Body.String())
	}

	replayW := completeChatStructuredOutputTask(t, taskID, structuredPayload)
	if replayW.Code != http.StatusOK {
		t.Fatalf("CompleteTask replay: expected 200, got %d: %s", replayW.Code, replayW.Body.String())
	}

	var proposalCount, issueCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM chat_issue_proposal WHERE chat_session_id = $1
	`, sessionID).Scan(&proposalCount); err != nil {
		t.Fatalf("count proposals after replay: %v", err)
	}
	if proposalCount != 1 {
		t.Fatalf("proposal count after replay = %d, want 1", proposalCount)
	}

	var firstStatus, secondStatus string
	var firstIssueID sql.NullString
	var approvedSnapshot []byte
	if err := testPool.QueryRow(ctx, `
		SELECT i1.status, i1.issue_id::text, i1.approved_snapshot, i2.status
		FROM chat_issue_proposal p
		JOIN chat_issue_proposal_item i1 ON i1.proposal_id = p.id AND i1.id = $2
		JOIN chat_issue_proposal_item i2 ON i2.proposal_id = p.id AND i2.id = $3
		WHERE p.id = $1
	`, proposalID, itemIDs[0], itemIDs[1]).Scan(&firstStatus, &firstIssueID, &approvedSnapshot, &secondStatus); err != nil {
		t.Fatalf("query proposal state after replay: %v", err)
	}
	if firstStatus != "created" || !firstIssueID.Valid || len(approvedSnapshot) == 0 {
		t.Fatalf("first item after replay = status %s issue %v snapshot bytes %d", firstStatus, firstIssueID, len(approvedSnapshot))
	}
	if secondStatus != "skipped" {
		t.Fatalf("second item after replay = %s, want skipped", secondStatus)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM issue WHERE origin_type = 'chat_session' AND origin_id = $1
	`, sessionID).Scan(&issueCount); err != nil {
		t.Fatalf("count chat-origin issues after replay: %v", err)
	}
	if issueCount != 1 {
		t.Fatalf("chat-origin issue count after replay = %d, want 1", issueCount)
	}
}

func TestCompleteTask_ChatStructuredSummaryDoesNotOverwriteUserTitle(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	_, sessionID, taskID := createChatStructuredOutputTestTask(t, "user")
	w := completeChatStructuredOutputTask(t, taskID, map[string]any{
		"chat_summary": map[string]any{
			"version": 1,
			"title":   "Agent should not win",
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var title, titleSource string
	if err := testPool.QueryRow(context.Background(), `
		SELECT title, title_source FROM chat_session WHERE id = $1
	`, sessionID).Scan(&title, &titleSource); err != nil {
		t.Fatalf("query chat session title: %v", err)
	}
	if title != "Initial chat title" || titleSource != "user" {
		t.Fatalf("user title was overwritten: %q/%q", title, titleSource)
	}
}

func TestCompleteTask_InvalidIssueProposalDoesNotLeaveTaskRunning(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	_, sessionID, taskID := createChatStructuredOutputTestTask(t, "legacy")
	w := completeChatStructuredOutputTask(t, taskID, map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title": "Invalid proposal",
					"items": []map[string]any{
						{
							"title": "",
						},
					},
				},
			},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatalf("query task status: %v", err)
	}
	if status != "completed" {
		t.Fatalf("task status = %q, want completed", status)
	}

	var messageCount, proposalCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM chat_message
		WHERE chat_session_id = $1 AND task_id = $2 AND role = 'assistant'
	`, sessionID, taskID).Scan(&messageCount); err != nil {
		t.Fatalf("count assistant messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("assistant message count = %d, want 1", messageCount)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM chat_issue_proposal WHERE chat_session_id = $1`, sessionID).Scan(&proposalCount); err != nil {
		t.Fatalf("count proposals: %v", err)
	}
	if proposalCount != 0 {
		t.Fatalf("proposal count = %d, want 0", proposalCount)
	}
}
