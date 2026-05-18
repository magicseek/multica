package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createChatIssueProposalApprovalFixture(t *testing.T) (sessionID, proposalID string, itemIDs []string) {
	t.Helper()

	_, sessionID, taskID := createChatStructuredOutputTestTask(t, "legacy")
	w := completeChatStructuredOutputTask(t, taskID, map[string]any{
		"issue_proposals": map[string]any{
			"version": 1,
			"proposals": []map[string]any{
				{
					"title":   "Approval fixture",
					"summary": "Create selected follow-ups",
					"items": []map[string]any{
						{
							"title":       "First approved issue",
							"description": "First issue description",
							"priority":    "high",
							"labels":      []string{"chat"},
						},
						{
							"title":       "Second approved issue",
							"description": "Second issue description",
							"priority":    "low",
						},
					},
				},
			},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask fixture: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := testPool.QueryRow(context.Background(), `
		SELECT id FROM chat_issue_proposal WHERE chat_session_id = $1
	`, sessionID).Scan(&proposalID); err != nil {
		t.Fatalf("query proposal: %v", err)
	}
	rows, err := testPool.Query(context.Background(), `
		SELECT id FROM chat_issue_proposal_item
		WHERE proposal_id = $1
		ORDER BY position ASC
	`, proposalID)
	if err != nil {
		t.Fatalf("query proposal items: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan item id: %v", err)
		}
		itemIDs = append(itemIDs, id)
	}
	if len(itemIDs) != 2 {
		t.Fatalf("fixture item count = %d, want 2", len(itemIDs))
	}
	return sessionID, proposalID, itemIDs
}

func TestApproveChatIssueProposal_CreatesBacklogIssuesAndRestoresSkippedItems(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	sessionID, proposalID, itemIDs := createChatIssueProposalApprovalFixture(t)

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
	var approveResp ApproveChatIssueProposalResponse
	if err := json.NewDecoder(approveW.Body).Decode(&approveResp); err != nil {
		t.Fatalf("decode approve response: %v", err)
	}
	if len(approveResp.Issues) != 1 {
		t.Fatalf("approved issue count = %d, want 1", len(approveResp.Issues))
	}
	created := approveResp.Issues[0]
	if created.Status != "backlog" || created.CreatorType != "member" || created.CreatorID != testUserID {
		t.Fatalf("created issue status/creator = %s/%s/%s", created.Status, created.CreatorType, created.CreatorID)
	}
	if created.Labels == nil || len(*created.Labels) != 1 || (*created.Labels)[0].Name != "chat" {
		t.Fatalf("created issue labels = %+v, want chat label", created.Labels)
	}

	var originType sql.NullString
	var originID string
	if err := testPool.QueryRow(ctx, `
		SELECT origin_type, origin_id::text FROM issue WHERE id = $1
	`, created.ID).Scan(&originType, &originID); err != nil {
		t.Fatalf("query created issue origin: %v", err)
	}
	if !originType.Valid || originType.String != "chat_session" || originID != sessionID {
		t.Fatalf("created issue origin = %v/%s, want chat_session/%s", originType, originID, sessionID)
	}
	var attachedLabelName string
	if err := testPool.QueryRow(ctx, `
		SELECT l.name
		FROM issue_label l
		JOIN issue_to_label itl ON itl.label_id = l.id
		WHERE itl.issue_id = $1
	`, created.ID).Scan(&attachedLabelName); err != nil {
		t.Fatalf("query created issue label: %v", err)
	}
	if attachedLabelName != "chat" {
		t.Fatalf("attached label = %s, want chat", attachedLabelName)
	}

	var firstStatus, secondStatus, proposalStatus string
	var firstIssueID sql.NullString
	if err := testPool.QueryRow(ctx, `
		SELECT i1.status, i1.issue_id::text, i2.status, p.status
		FROM chat_issue_proposal p
		JOIN chat_issue_proposal_item i1 ON i1.proposal_id = p.id AND i1.id = $2
		JOIN chat_issue_proposal_item i2 ON i2.proposal_id = p.id AND i2.id = $3
		WHERE p.id = $1
	`, proposalID, itemIDs[0], itemIDs[1]).Scan(&firstStatus, &firstIssueID, &secondStatus, &proposalStatus); err != nil {
		t.Fatalf("query proposal state after approval: %v", err)
	}
	if firstStatus != "created" || !firstIssueID.Valid || firstIssueID.String != created.ID {
		t.Fatalf("first item status/issue = %s/%v", firstStatus, firstIssueID)
	}
	if secondStatus != "skipped" || proposalStatus != "partially_accepted" {
		t.Fatalf("second/proposal status = %s/%s, want skipped/partially_accepted", secondStatus, proposalStatus)
	}

	restoreW := httptest.NewRecorder()
	restoreReq := newRequest("POST", "/api/chat/issue-proposals/"+proposalID+"/items/"+itemIDs[1]+"/restore", nil)
	restoreReq = withURLParam(restoreReq, "proposalId", proposalID)
	restoreReq = withURLParam(restoreReq, "itemId", itemIDs[1])
	restoreReq = withChatTestWorkspaceCtx(t, restoreReq)
	testHandler.RestoreChatIssueProposalItem(restoreW, restoreReq)
	if restoreW.Code != http.StatusOK {
		t.Fatalf("RestoreChatIssueProposalItem: expected 200, got %d: %s", restoreW.Code, restoreW.Body.String())
	}
	if err := testPool.QueryRow(ctx, `
		SELECT status FROM chat_issue_proposal_item WHERE id = $1
	`, itemIDs[1]).Scan(&secondStatus); err != nil {
		t.Fatalf("query restored item: %v", err)
	}
	if secondStatus != "pending" {
		t.Fatalf("restored item status = %s, want pending", secondStatus)
	}

	dismissW := httptest.NewRecorder()
	dismissReq := newRequest("POST", "/api/chat/issue-proposals/"+proposalID+"/dismiss", nil)
	dismissReq = withURLParam(dismissReq, "proposalId", proposalID)
	dismissReq = withChatTestWorkspaceCtx(t, dismissReq)
	testHandler.DismissChatIssueProposal(dismissW, dismissReq)
	if dismissW.Code != http.StatusOK {
		t.Fatalf("DismissChatIssueProposal after partial approval: expected 200, got %d: %s", dismissW.Code, dismissW.Body.String())
	}
	if err := testPool.QueryRow(ctx, `
		SELECT i2.status, p.status
		FROM chat_issue_proposal p
		JOIN chat_issue_proposal_item i2 ON i2.proposal_id = p.id AND i2.id = $2
		WHERE p.id = $1
	`, proposalID, itemIDs[1]).Scan(&secondStatus, &proposalStatus); err != nil {
		t.Fatalf("query proposal state after partial dismiss: %v", err)
	}
	if secondStatus != "skipped" || proposalStatus != "partially_accepted" {
		t.Fatalf("partial dismiss state = %s/%s, want skipped/partially_accepted", secondStatus, proposalStatus)
	}

	listW := httptest.NewRecorder()
	listReq := newRequest("GET", "/api/chat/sessions/"+sessionID+"/issues", nil)
	listReq = withURLParam(listReq, "sessionId", sessionID)
	listReq = withChatTestWorkspaceCtx(t, listReq)
	testHandler.ListChatIssues(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListChatIssues: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp ChatSessionIssuesResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode issues response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Issues) != 1 || listResp.Issues[0].ID != created.ID {
		t.Fatalf("list chat issues = %+v, want created issue", listResp)
	}
}

func TestApproveChatIssueProposal_InvalidAssigneeCreatesNoPartialBatch(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	_, sessionID, _ := createChatStructuredOutputTestTask(t, "legacy")

	var proposalID, itemID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_issue_proposal (workspace_id, chat_session_id, title)
		VALUES ($1, $2, 'Invalid assignee proposal')
		RETURNING id
	`, testWorkspaceID, sessionID).Scan(&proposalID); err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_issue_proposal_item (
			proposal_id, position, title, description, priority, labels, assignee_type, assignee_id
		)
		VALUES (
			$1, 0, 'Invalid assignee item', 'Should not create an issue', 'medium', '[]'::jsonb,
			'member', '11111111-1111-1111-1111-111111111111'
		)
		RETURNING id
	`, proposalID).Scan(&itemID); err != nil {
		t.Fatalf("create proposal item: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/chat/issue-proposals/"+proposalID+"/approve", map[string]any{
		"item_ids": []string{itemID},
	})
	req = withURLParam(req, "proposalId", proposalID)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.ApproveChatIssueProposal(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("ApproveChatIssueProposal: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var issueCount int
	var itemStatus string
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM issue WHERE origin_type = 'chat_session' AND origin_id = $1
	`, sessionID).Scan(&issueCount); err != nil {
		t.Fatalf("count chat issues: %v", err)
	}
	if issueCount != 0 {
		t.Fatalf("created issue count = %d, want 0", issueCount)
	}
	if err := testPool.QueryRow(ctx, `SELECT status FROM chat_issue_proposal_item WHERE id = $1`, itemID).Scan(&itemStatus); err != nil {
		t.Fatalf("query item status: %v", err)
	}
	if itemStatus != "pending" {
		t.Fatalf("item status = %s, want pending", itemStatus)
	}
}

func TestUpdateAndDismissChatIssueProposal(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	_, proposalID, itemIDs := createChatIssueProposalApprovalFixture(t)

	updateW := httptest.NewRecorder()
	updateReq := newRequest("PATCH", "/api/chat/issue-proposals/"+proposalID+"/items/"+itemIDs[0], map[string]any{
		"title":       "Edited issue title",
		"description": "Edited description",
		"priority":    "medium",
		"labels":      []string{"edited"},
	})
	updateReq = withURLParam(updateReq, "proposalId", proposalID)
	updateReq = withURLParam(updateReq, "itemId", itemIDs[0])
	updateReq = withChatTestWorkspaceCtx(t, updateReq)
	testHandler.UpdateChatIssueProposalItem(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("UpdateChatIssueProposalItem: expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var title string
	var priority sql.NullString
	if err := testPool.QueryRow(ctx, `
		SELECT title, priority FROM chat_issue_proposal_item WHERE id = $1
	`, itemIDs[0]).Scan(&title, &priority); err != nil {
		t.Fatalf("query updated item: %v", err)
	}
	if title != "Edited issue title" || !priority.Valid || priority.String != "medium" {
		t.Fatalf("updated item = %q/%v", title, priority)
	}

	dismissW := httptest.NewRecorder()
	dismissReq := newRequest("POST", "/api/chat/issue-proposals/"+proposalID+"/dismiss", nil)
	dismissReq = withURLParam(dismissReq, "proposalId", proposalID)
	dismissReq = withChatTestWorkspaceCtx(t, dismissReq)
	testHandler.DismissChatIssueProposal(dismissW, dismissReq)
	if dismissW.Code != http.StatusOK {
		t.Fatalf("DismissChatIssueProposal: expected 200, got %d: %s", dismissW.Code, dismissW.Body.String())
	}
	var proposalStatus string
	var pendingItems int
	if err := testPool.QueryRow(ctx, `SELECT status FROM chat_issue_proposal WHERE id = $1`, proposalID).Scan(&proposalStatus); err != nil {
		t.Fatalf("query proposal status: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM chat_issue_proposal_item WHERE proposal_id = $1 AND status = 'pending'
	`, proposalID).Scan(&pendingItems); err != nil {
		t.Fatalf("count pending items: %v", err)
	}
	if proposalStatus != "dismissed" || pendingItems != 0 {
		t.Fatalf("dismiss state = %s with %d pending items", proposalStatus, pendingItems)
	}
}

func TestChatIssueProposalMutation_PrivateAgentGate(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID, _, memberID := privateAgentTestFixture(t)

	var sessionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id, title, status)
		VALUES ($1, $2, $3, 'proposal mutation private gate', 'active')
		RETURNING id
	`, testWorkspaceID, agentID, memberID).Scan(&sessionID); err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID)
	})

	var proposalID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_issue_proposal (workspace_id, chat_session_id, title)
		VALUES ($1, $2, 'Private proposal')
		RETURNING id
	`, testWorkspaceID, sessionID).Scan(&proposalID); err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO chat_issue_proposal_item (proposal_id, position, title, description, labels)
		VALUES ($1, 0, 'Private item', '', '[]'::jsonb)
	`, proposalID); err != nil {
		t.Fatalf("create proposal item: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequestAs(memberID, "POST", "/api/chat/issue-proposals/"+proposalID+"/dismiss", nil)
	req = withURLParam(req, "proposalId", proposalID)
	req = withChatTestWorkspaceCtxAs(t, req, memberID)
	testHandler.DismissChatIssueProposal(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("DismissChatIssueProposal with inaccessible private agent: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM chat_issue_proposal WHERE id = $1`, proposalID).Scan(&status); err != nil {
		t.Fatalf("query proposal status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("proposal status = %s, want pending", status)
	}
}
