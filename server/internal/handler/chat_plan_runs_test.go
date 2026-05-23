package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestListChatPlanEngines(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest(http.MethodGet, "/api/chat/plan-engines", nil)

	testHandler.ListChatPlanEngines(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListChatPlanEngines: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "protocol") {
		t.Fatalf("engine list response must not expose prompt protocol: %s", w.Body.String())
	}

	var resp ListPlanEnginesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DefaultEngine != defaultPlanEngineID {
		t.Fatalf("default_engine = %q, want %q", resp.DefaultEngine, defaultPlanEngineID)
	}
	foundDefault := false
	for _, engine := range resp.Engines {
		if engine.ID == defaultPlanEngineID {
			foundDefault = true
			if !engine.Default {
				t.Fatal("default engine entry is not marked default")
			}
			if len(engine.Version) != 16 {
				t.Fatalf("default engine version length = %d, want 16", len(engine.Version))
			}
		}
	}
	if !foundDefault {
		t.Fatalf("default engine %q not present in %+v", defaultPlanEngineID, resp.Engines)
	}
}

func TestSendChatPlanMessageCreatesAndContinuesPlanRun(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Plan Run Agent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	first := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":     "Plan the release work",
		"mode":        "plan",
		"plan_engine": "brainstorming",
	})
	if first.PlanRunID == "" {
		t.Fatal("expected plan_run_id on plan send response")
	}

	var planEngine, status, initialMessageID, latestMessageID, leadAgentID string
	if err := testPool.QueryRow(ctx, `
		SELECT plan_engine, status, initial_message_id::text, latest_message_id::text, lead_agent_id::text
		FROM chat_plan_run
		WHERE id = $1
	`, first.PlanRunID).Scan(&planEngine, &status, &initialMessageID, &latestMessageID, &leadAgentID); err != nil {
		t.Fatalf("load plan run: %v", err)
	}
	if planEngine != "brainstorming" {
		t.Fatalf("plan_engine = %q, want brainstorming", planEngine)
	}
	if status != "brainstorming" {
		t.Fatalf("status = %q, want brainstorming", status)
	}
	if initialMessageID != first.MessageID || latestMessageID != first.MessageID {
		t.Fatalf("initial/latest message = %s/%s, want %s", initialMessageID, latestMessageID, first.MessageID)
	}
	if leadAgentID != agentID {
		t.Fatalf("lead_agent_id = %s, want %s", leadAgentID, agentID)
	}

	var messagePlanRunID, authorType string
	if err := testPool.QueryRow(ctx, `
		SELECT plan_run_id::text, author_type
		FROM chat_message
		WHERE id = $1
	`, first.MessageID).Scan(&messagePlanRunID, &authorType); err != nil {
		t.Fatalf("load plan message: %v", err)
	}
	if messagePlanRunID != first.PlanRunID {
		t.Fatalf("chat_message.plan_run_id = %s, want %s", messagePlanRunID, first.PlanRunID)
	}
	if authorType != "member" {
		t.Fatalf("chat_message.author_type = %q, want member", authorType)
	}

	var taskPlanRunID, taskKind, triggerMessageID, taskAgentID string
	if err := testPool.QueryRow(ctx, `
		SELECT chat_plan_run_id::text, chat_task_kind, trigger_chat_message_id::text, agent_id::text
		FROM agent_task_queue
		WHERE id = $1
	`, first.TaskID).Scan(&taskPlanRunID, &taskKind, &triggerMessageID, &taskAgentID); err != nil {
		t.Fatalf("load plan task: %v", err)
	}
	if taskPlanRunID != first.PlanRunID {
		t.Fatalf("task.chat_plan_run_id = %s, want %s", taskPlanRunID, first.PlanRunID)
	}
	if taskKind != "plan_lead" {
		t.Fatalf("task.chat_task_kind = %q, want plan_lead", taskKind)
	}
	if triggerMessageID != first.MessageID {
		t.Fatalf("task.trigger_chat_message_id = %s, want %s", triggerMessageID, first.MessageID)
	}
	if taskAgentID != agentID {
		t.Fatalf("task.agent_id = %s, want %s", taskAgentID, agentID)
	}

	second := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":     "Also include rollout acceptance criteria",
		"plan_run_id": first.PlanRunID,
	})
	if second.PlanRunID != first.PlanRunID {
		t.Fatalf("continued plan_run_id = %s, want %s", second.PlanRunID, first.PlanRunID)
	}

	if err := testPool.QueryRow(ctx, `
		SELECT latest_message_id::text
		FROM chat_plan_run
		WHERE id = $1
	`, first.PlanRunID).Scan(&latestMessageID); err != nil {
		t.Fatalf("reload plan run: %v", err)
	}
	if latestMessageID != second.MessageID {
		t.Fatalf("latest_message_id after continue = %s, want %s", latestMessageID, second.MessageID)
	}

	var count int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM chat_plan_run
		WHERE chat_session_id = $1
	`, sessionID).Scan(&count); err != nil {
		t.Fatalf("count plan runs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one plan run after continuation, got %d", count)
	}
}

func TestPlanStructuredOutputsPersistSummaryAndProposal(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Plan Structured Agent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":     "Turn this idea into issues",
		"mode":        "plan",
		"plan_engine": "office_hours",
	})

	task, err := testHandler.Queries.GetAgentTask(ctx, util.MustParseUUID(sendResp.TaskID))
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	summary := "Implementation tasks for the agreed plan"
	req := newRequest(http.MethodPost, "/api/daemon/tasks/"+sendResp.TaskID+"/complete", nil)
	testHandler.processTaskStructuredOutputs(req, task, &TaskStructuredOutputsRequest{
		PlanSummary: &PlanSummaryManifestRequest{
			Version:               1,
			ConfirmedRequirements: []string{"  keep approval proposal-first  ", ""},
			RejectedOptions:       []string{"direct issue creation"},
			ConsensusNotes:        []string{"ship backend contract first"},
			OpenQuestions:         []string{"frontend copy"},
		},
		IssueProposals: &IssueProposalsManifestRequest{
			Version: 1,
			Proposals: []ChatIssueProposalManifestRequest{
				{
					Title:   "Plan implementation",
					Summary: &summary,
					Items: []ChatIssueProposalItemManifestRequest{
						{
							Title:       "Persist plan run metadata",
							Description: "Add storage and claim context for chat plan runs.",
						},
					},
				},
			},
		},
	})

	var rawSummary []byte
	var status string
	if err := testPool.QueryRow(ctx, `
		SELECT summary, status
		FROM chat_plan_run
		WHERE id = $1
	`, sendResp.PlanRunID).Scan(&rawSummary, &status); err != nil {
		t.Fatalf("load plan summary: %v", err)
	}
	if status != "ready_for_approval" {
		t.Fatalf("plan status = %q, want ready_for_approval", status)
	}
	var persisted map[string][]string
	if err := json.Unmarshal(rawSummary, &persisted); err != nil {
		t.Fatalf("decode plan summary: %v", err)
	}
	if got := persisted["confirmed_requirements"]; len(got) != 1 || got[0] != "keep approval proposal-first" {
		t.Fatalf("confirmed_requirements = %+v", got)
	}
	if got := persisted["rejected_options"]; len(got) != 1 || got[0] != "direct issue creation" {
		t.Fatalf("rejected_options = %+v", got)
	}

	var sourcePlanRunID sql.NullString
	if err := testPool.QueryRow(ctx, `
		SELECT source_plan_run_id::text
		FROM chat_issue_proposal
		WHERE source_task_id = $1
	`, sendResp.TaskID).Scan(&sourcePlanRunID); err != nil {
		t.Fatalf("load proposal source plan run: %v", err)
	}
	if !sourcePlanRunID.Valid || sourcePlanRunID.String != sendResp.PlanRunID {
		t.Fatalf("source_plan_run_id = %+v, want %s", sourcePlanRunID, sendResp.PlanRunID)
	}
}

func TestApproveChatIssueProposalCompletesSourcePlanRun(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Plan Approval Agent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content": "Plan an approval-completed flow",
		"mode":    "plan",
	})

	var proposalID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_issue_proposal (workspace_id, chat_session_id, source_plan_run_id, proposer_agent_id, title, summary)
		VALUES ($1, $2, $3, $4, 'Plan approval proposal', 'Approve this linked plan proposal')
		RETURNING id
	`, testWorkspaceID, sessionID, sendResp.PlanRunID, agentID).Scan(&proposalID); err != nil {
		t.Fatalf("create linked proposal: %v", err)
	}
	var itemID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_issue_proposal_item (proposal_id, position, title, description, priority)
		VALUES ($1, 0, 'Create issue from approved plan', 'Issue created by approval', 'high')
		RETURNING id
	`, proposalID).Scan(&itemID); err != nil {
		t.Fatalf("create proposal item: %v", err)
	}

	gotPlanRunEvent := make(chan events.Event, 1)
	testHandler.Bus.Subscribe(protocol.EventChatPlanRunsUpdated, func(e events.Event) {
		if e.ChatSessionID == sessionID {
			select {
			case gotPlanRunEvent <- e:
			default:
			}
		}
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/chat/issue-proposals/"+proposalID+"/approve", map[string]any{
		"item_ids": []string{itemID},
	})
	req = withURLParam(req, "proposalId", proposalID)
	req = withChatTestWorkspaceCtx(t, req)
	testHandler.ApproveChatIssueProposal(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApproveChatIssueProposal: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	var completedAt sql.NullTime
	if err := testPool.QueryRow(ctx, `
		SELECT status, completed_at
		FROM chat_plan_run
		WHERE id = $1
	`, sendResp.PlanRunID).Scan(&status, &completedAt); err != nil {
		t.Fatalf("load approved plan run: %v", err)
	}
	if status != "completed" || !completedAt.Valid {
		t.Fatalf("plan run status/completed_at = %s/%v, want completed with completed_at", status, completedAt)
	}

	select {
	case ev := <-gotPlanRunEvent:
		payload, ok := ev.Payload.(protocol.ChatPlanRunsUpdatedPayload)
		if !ok {
			t.Fatalf("plan run event payload = %T, want ChatPlanRunsUpdatedPayload", ev.Payload)
		}
		if payload.ChatSessionID != sessionID || payload.PlanRunID != sendResp.PlanRunID {
			t.Fatalf("plan run event payload = %+v, want session %s run %s", payload, sessionID, sendResp.PlanRunID)
		}
	default:
		t.Fatal("approval did not publish chat:plan_runs_updated")
	}
}

func TestCancelChatPlanRunMarksTerminalAndBlocksContinuation(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Plan Cancel Agent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content": "Plan something cancellable",
		"mode":    "plan",
	})

	cancelReq := newRequest(http.MethodPost, "/api/chat/plan-runs/"+sendResp.PlanRunID+"/cancel", nil)
	cancelReq = withURLParam(cancelReq, "planRunId", sendResp.PlanRunID)
	cancelReq = withChatTestWorkspaceCtx(t, cancelReq)
	cancelW := httptest.NewRecorder()
	testHandler.CancelChatPlanRun(cancelW, cancelReq)
	if cancelW.Code != http.StatusOK {
		t.Fatalf("CancelChatPlanRun: expected 200, got %d: %s", cancelW.Code, cancelW.Body.String())
	}
	var cancelled ChatPlanRunResponse
	if err := json.NewDecoder(cancelW.Body).Decode(&cancelled); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Status)
	}

	continueReq := newRequest(http.MethodPost, "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content":     "continue anyway",
		"plan_run_id": sendResp.PlanRunID,
	})
	continueReq = withURLParam(continueReq, "sessionId", sessionID)
	continueReq = withChatTestWorkspaceCtx(t, continueReq)
	continueW := httptest.NewRecorder()
	testHandler.SendChatMessage(continueW, continueReq)
	if continueW.Code != http.StatusBadRequest {
		t.Fatalf("continuing cancelled plan: expected 400, got %d: %s", continueW.Code, continueW.Body.String())
	}

	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM chat_plan_run WHERE id = $1`, sendResp.PlanRunID).Scan(&status); err != nil {
		t.Fatalf("load cancelled plan status: %v", err)
	}
	if status != "cancelled" {
		t.Fatalf("db status = %q, want cancelled", status)
	}
}

func TestClaimTask_SquadPlanRunIncludesPlanContext(t *testing.T) {
	ctx := context.Background()
	leaderID, runtimeID, daemonID := createRuntimeGuardAgent(t, ctx)
	helperID := createHandlerTestAgent(t, "Plan Helper Agent", []byte("[]"))

	var squadID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, 'Plan Squad', '', $2, $3)
		RETURNING id
	`, testWorkspaceID, leaderID, testUserID).Scan(&squadID); err != nil {
		t.Fatalf("create squad: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM squad WHERE id = $1`, squadID) })

	if _, err := testPool.Exec(ctx, `
		INSERT INTO squad_member (squad_id, member_type, member_id, role)
		VALUES ($1, 'agent', $2, 'research')
	`, squadID, helperID); err != nil {
		t.Fatalf("add squad helper: %v", err)
	}

	var sessionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id, title, status)
		VALUES ($1, $2, $3, 'squad plan claim', 'active')
		RETURNING id
	`, testWorkspaceID, leaderID, testUserID).Scan(&sessionID); err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID) })

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan a squad-reviewed launch",
		"mode":            "plan",
		"plan_engine":     "office_hours",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/claim", nil, testWorkspaceID, daemonID)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.ClaimTaskByRuntime(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ClaimTaskByRuntime: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var claim struct {
		Task *AgentTaskResponse `json:"task"`
	}
	if err := json.NewDecoder(w.Body).Decode(&claim); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if claim.Task == nil {
		t.Fatal("expected claimed task")
	}
	if claim.Task.ID != sendResp.TaskID {
		t.Fatalf("claimed task ID = %s, want %s", claim.Task.ID, sendResp.TaskID)
	}
	if claim.Task.Plan == nil {
		t.Fatalf("expected plan context in claim: %+v", claim.Task)
	}
	plan := claim.Task.Plan
	if plan.RunID != sendResp.PlanRunID {
		t.Fatalf("plan.run_id = %s, want %s", plan.RunID, sendResp.PlanRunID)
	}
	if plan.ActorType != "squad" || plan.ActorID != squadID {
		t.Fatalf("plan actor = %s %s, want squad %s", plan.ActorType, plan.ActorID, squadID)
	}
	if plan.PlanEngine.ID != "office_hours" || strings.TrimSpace(plan.PlanEngine.Protocol) == "" {
		t.Fatalf("plan engine not populated: %+v", plan.PlanEngine)
	}
	if len(plan.Transcript) != 1 || plan.Transcript[0].Content != "Plan a squad-reviewed launch" {
		t.Fatalf("plan transcript = %+v", plan.Transcript)
	}
	if plan.Squad == nil {
		t.Fatal("expected squad context")
	}
	if !strings.Contains(plan.Squad.LeadMention, "mention://agent/"+leaderID) {
		t.Fatalf("lead mention = %q, want leader UUID %s", plan.Squad.LeadMention, leaderID)
	}
	if len(plan.Squad.Helpers) != 1 || plan.Squad.Helpers[0].AgentID != helperID || !strings.Contains(plan.Squad.Helpers[0].Mention, "mention://agent/"+helperID) {
		t.Fatalf("helpers = %+v, want helper %s", plan.Squad.Helpers, helperID)
	}
	if !strings.Contains(plan.PlanSummaryPath, sessionID) || !strings.Contains(plan.ProposalPath, sessionID) {
		t.Fatalf("structured paths missing session ID: summary=%q proposal=%q", plan.PlanSummaryPath, plan.ProposalPath)
	}
}

func TestClaimTask_PlanLeadContinuationDoesNotReuseLatestUserMessage(t *testing.T) {
	ctx := context.Background()
	leaderID, runtimeID, daemonID := createRuntimeGuardAgent(t, ctx)

	var sessionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id, title, status)
		VALUES ($1, $2, $3, 'plan continuation claim', 'active')
		RETURNING id
	`, testWorkspaceID, leaderID, testUserID).Scan(&sessionID); err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID) })

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content": "Original user planning request",
		"mode":    "plan",
	})
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed' WHERE id = $1`, sendResp.TaskID); err != nil {
		t.Fatalf("complete initial task fixture: %v", err)
	}
	session, err := testHandler.Queries.GetChatSession(ctx, util.MustParseUUID(sessionID))
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	run, err := testHandler.Queries.GetChatPlanRun(ctx, util.MustParseUUID(sendResp.PlanRunID))
	if err != nil {
		t.Fatalf("load plan run: %v", err)
	}
	continuation, err := testHandler.TaskService.EnqueuePlanLeadTask(ctx, session, run, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue continuation: %v", err)
	}

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/claim", nil, testWorkspaceID, daemonID)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.ClaimTaskByRuntime(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ClaimTaskByRuntime: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var claim struct {
		Task *AgentTaskResponse `json:"task"`
	}
	if err := json.NewDecoder(w.Body).Decode(&claim); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if claim.Task == nil {
		t.Fatal("expected claimed task")
	}
	if claim.Task.ID != util.UUIDToString(continuation.ID) {
		t.Fatalf("claimed task ID = %s, want %s", claim.Task.ID, util.UUIDToString(continuation.ID))
	}
	if claim.Task.ChatMessage != "" {
		t.Fatalf("lead continuation reused stale user message %q", claim.Task.ChatMessage)
	}
	if claim.Task.Plan == nil || len(claim.Task.Plan.Transcript) == 0 {
		t.Fatalf("expected plan transcript for continuation: %+v", claim.Task)
	}
	if claim.Task.Plan.Transcript[0].Content != "Original user planning request" {
		t.Fatalf("plan transcript = %+v", claim.Task.Plan.Transcript)
	}
}

func TestChatPlanSquadHelperLeadMentionResumesLead(t *testing.T) {
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "Plan Loop Leader", []byte("[]"))
	helperID := createHandlerTestAgent(t, "Plan Loop Helper", []byte("[]"))
	squadID := createPlanRunTestSquad(t, leaderID, helperID)
	sessionID := createHandlerTestChatSession(t, leaderID)

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan with the squad",
		"mode":            "plan",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})

	completeTaskForPlanTest(t, sendResp.TaskID, "Please review [@Helper](mention://agent/"+helperID+")")

	var helperTaskID string
	if err := testPool.QueryRow(ctx, `
		SELECT task_id::text
		FROM chat_plan_consultation
		WHERE plan_run_id = $1
		  AND target_agent_id = $2
	`, sendResp.PlanRunID, helperID).Scan(&helperTaskID); err != nil {
		t.Fatalf("load helper consultation task: %v", err)
	}
	if helperTaskID == "" {
		t.Fatal("expected helper consultation task")
	}

	completeTaskForPlanTest(t, helperTaskID, "I agree with the direction. [@Leader](mention://agent/"+leaderID+")")

	var status string
	var responseMessageID string
	if err := testPool.QueryRow(ctx, `
		SELECT status, response_message_id::text
		FROM chat_plan_consultation
		WHERE task_id = $1
	`, helperTaskID).Scan(&status, &responseMessageID); err != nil {
		t.Fatalf("load responded consultation: %v", err)
	}
	if status != "responded" || responseMessageID == "" {
		t.Fatalf("consultation status/response = %s/%s, want responded with response", status, responseMessageID)
	}

	var leadContinuationCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM agent_task_queue
		WHERE chat_plan_run_id = $1
		  AND chat_task_kind = 'plan_lead'
		  AND trigger_chat_message_id = $2
		  AND agent_id = $3
	`, sendResp.PlanRunID, responseMessageID, leaderID).Scan(&leadContinuationCount); err != nil {
		t.Fatalf("count lead continuation tasks: %v", err)
	}
	if leadContinuationCount != 1 {
		t.Fatalf("expected one lead continuation task, got %d", leadContinuationCount)
	}

	var waveCount int
	if err := testPool.QueryRow(ctx, `SELECT consultation_wave_count FROM chat_plan_run WHERE id = $1`, sendResp.PlanRunID).Scan(&waveCount); err != nil {
		t.Fatalf("load wave count: %v", err)
	}
	if waveCount != 1 {
		t.Fatalf("consultation_wave_count = %d, want 1", waveCount)
	}
}

func TestChatPlanSquadPlainHelperMentionEnqueuesConsultation(t *testing.T) {
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "Orion", []byte("[]"))
	helperID := createHandlerTestAgent(t, "Atlas", []byte("[]"))
	squadID := createPlanRunTestSquad(t, leaderID, helperID)
	sessionID := createHandlerTestChatSession(t, leaderID)

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan with named squad agents",
		"mode":            "plan",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})

	completeTaskForPlanTest(t, sendResp.TaskID, "@Atlas please challenge the rollout plan.")

	var helperTaskID string
	if err := testPool.QueryRow(ctx, `
		SELECT task_id::text
		FROM chat_plan_consultation
		WHERE plan_run_id = $1
		  AND target_agent_id = $2
	`, sendResp.PlanRunID, helperID).Scan(&helperTaskID); err != nil {
		t.Fatalf("load helper consultation task: %v", err)
	}
	if helperTaskID == "" {
		t.Fatal("expected plain @Atlas mention to enqueue helper consultation")
	}
}

func TestChatPlanSquadPlainLeadMentionResumesLead(t *testing.T) {
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "Orion", []byte("[]"))
	helperID := createHandlerTestAgent(t, "Atlas", []byte("[]"))
	squadID := createPlanRunTestSquad(t, leaderID, helperID)
	sessionID := createHandlerTestChatSession(t, leaderID)

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan with named lead reply",
		"mode":            "plan",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})
	completeTaskForPlanTest(t, sendResp.TaskID, "@Atlas please review this direction.")

	var helperTaskID string
	if err := testPool.QueryRow(ctx, `
		SELECT task_id::text
		FROM chat_plan_consultation
		WHERE plan_run_id = $1
		  AND target_agent_id = $2
	`, sendResp.PlanRunID, helperID).Scan(&helperTaskID); err != nil {
		t.Fatalf("load helper consultation task: %v", err)
	}
	completeTaskForPlanTest(t, helperTaskID, "I agree with the approach. @Orion")

	var status string
	var responseMessageID string
	if err := testPool.QueryRow(ctx, `
		SELECT status, response_message_id::text
		FROM chat_plan_consultation
		WHERE task_id = $1
	`, helperTaskID).Scan(&status, &responseMessageID); err != nil {
		t.Fatalf("load responded consultation: %v", err)
	}
	if status != "responded" || responseMessageID == "" {
		t.Fatalf("consultation status/response = %s/%s, want responded with response", status, responseMessageID)
	}

	var leadContinuationCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM agent_task_queue
		WHERE chat_plan_run_id = $1
		  AND chat_task_kind = 'plan_lead'
		  AND trigger_chat_message_id = $2
		  AND agent_id = $3
	`, sendResp.PlanRunID, responseMessageID, leaderID).Scan(&leadContinuationCount); err != nil {
		t.Fatalf("count lead continuation tasks: %v", err)
	}
	if leadContinuationCount != 1 {
		t.Fatalf("expected one lead continuation task, got %d", leadContinuationCount)
	}
}

func TestChatPlanSquadHelperMissingLeadMentionDoesNotResumeLead(t *testing.T) {
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "Plan Missing Lead Leader", []byte("[]"))
	helperID := createHandlerTestAgent(t, "Plan Missing Lead Helper", []byte("[]"))
	squadID := createPlanRunTestSquad(t, leaderID, helperID)
	sessionID := createHandlerTestChatSession(t, leaderID)

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan with a missing mention helper",
		"mode":            "plan",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})
	completeTaskForPlanTest(t, sendResp.TaskID, "Please review [@Helper](mention://agent/"+helperID+")")

	var helperTaskID string
	if err := testPool.QueryRow(ctx, `
		SELECT task_id::text
		FROM chat_plan_consultation
		WHERE plan_run_id = $1
		  AND target_agent_id = $2
	`, sendResp.PlanRunID, helperID).Scan(&helperTaskID); err != nil {
		t.Fatalf("load helper consultation task: %v", err)
	}
	completeTaskForPlanTest(t, helperTaskID, "I agree, but I forgot the explicit lead mention.")

	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM chat_plan_consultation WHERE task_id = $1`, helperTaskID).Scan(&status); err != nil {
		t.Fatalf("load consultation status: %v", err)
	}
	if status != "failed" {
		t.Fatalf("consultation status = %q, want failed", status)
	}

	var leadContinuationCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM agent_task_queue
		WHERE chat_plan_run_id = $1
		  AND chat_task_kind = 'plan_lead'
		  AND trigger_chat_message_id IS NOT NULL
		  AND trigger_chat_message_id <> $2
	`, sendResp.PlanRunID, sendResp.MessageID).Scan(&leadContinuationCount); err != nil {
		t.Fatalf("count lead continuations: %v", err)
	}
	if leadContinuationCount != 0 {
		t.Fatalf("expected no lead continuation when helper omits lead mention, got %d", leadContinuationCount)
	}
}

func TestChatPlanSquadConsultationLimitSkipsSixthWave(t *testing.T) {
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "Plan Limit Leader", []byte("[]"))
	helperID := createHandlerTestAgent(t, "Plan Limit Helper", []byte("[]"))
	squadID := createPlanRunTestSquad(t, leaderID, helperID)
	sessionID := createHandlerTestChatSession(t, leaderID)

	sendResp := sendPlanMessageForTest(t, sessionID, map[string]any{
		"content":         "Plan with consultation limit",
		"mode":            "plan",
		"plan_actor_type": "squad",
		"plan_actor_id":   squadID,
	})
	if _, err := testPool.Exec(ctx, `UPDATE chat_plan_run SET consultation_wave_count = 5 WHERE id = $1`, sendResp.PlanRunID); err != nil {
		t.Fatalf("seed wave count: %v", err)
	}

	completeTaskForPlanTest(t, sendResp.TaskID, "One more review [@Helper](mention://agent/"+helperID+")")

	var consultationCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM chat_plan_consultation WHERE plan_run_id = $1`, sendResp.PlanRunID).Scan(&consultationCount); err != nil {
		t.Fatalf("count consultations: %v", err)
	}
	if consultationCount != 0 {
		t.Fatalf("expected no sixth-wave consultation, got %d", consultationCount)
	}

	var warningCode string
	if err := testPool.QueryRow(ctx, `
		SELECT warning_code
		FROM chat_message_recipient
		WHERE chat_session_id = $1
		  AND recipient_id = $2
		  AND status = 'skipped'
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID, helperID).Scan(&warningCode); err != nil {
		t.Fatalf("load skipped edge: %v", err)
	}
	if warningCode != "consultation_limit_reached" {
		t.Fatalf("warning_code = %q, want consultation_limit_reached", warningCode)
	}
}

func sendPlanMessageForTest(t *testing.T, sessionID string, body map[string]any) SendChatMessageResponse {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/chat-sessions/"+sessionID+"/messages", body)
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.SendChatMessage(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("SendChatMessage: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp SendChatMessageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode send response: %v", err)
	}
	if resp.MessageID == "" || resp.TaskID == "" {
		t.Fatalf("expected message_id and task_id: %+v", resp)
	}
	return resp
}

func createPlanRunTestSquad(t *testing.T, leaderID string, helperIDs ...string) string {
	t.Helper()
	ctx := context.Background()
	var squadID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, $2, '', $3, $4)
		RETURNING id
	`, testWorkspaceID, "Plan Squad "+t.Name(), leaderID, testUserID).Scan(&squadID); err != nil {
		t.Fatalf("create squad: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM squad WHERE id = $1`, squadID) })
	for _, helperID := range helperIDs {
		if _, err := testPool.Exec(ctx, `
			INSERT INTO squad_member (squad_id, member_type, member_id, role)
			VALUES ($1, 'agent', $2, 'helper')
		`, squadID, helperID); err != nil {
			t.Fatalf("add squad helper: %v", err)
		}
	}
	return squadID
}

func completeTaskForPlanTest(t *testing.T, taskID, output string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'running', started_at = COALESCE(started_at, now())
		WHERE id = $1
	`, taskID); err != nil {
		t.Fatalf("mark task running: %v", err)
	}
	result, err := json.Marshal(protocol.TaskCompletedPayload{TaskID: taskID, Output: output})
	if err != nil {
		t.Fatalf("marshal task completion: %v", err)
	}
	if _, err := testHandler.TaskService.CompleteTask(context.Background(), util.MustParseUUID(taskID), result, "", ""); err != nil {
		t.Fatalf("CompleteTask(%s): %v", taskID, err)
	}
}
