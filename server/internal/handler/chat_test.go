package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// withChatTestWorkspaceCtx injects the workspace+member context that the
// real chi middleware chain would normally set. SendChatMessage (and most
// other chat handlers) read workspace ID from ctxWorkspaceID; without this
// the test harness, which calls handlers directly, gets "invalid workspace
// id" on the parseUUIDOrBadRequest call inside SendChatMessage.
func withChatTestWorkspaceCtx(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	return withChatTestWorkspaceCtxAs(t, req, testUserID)
}

func withChatTestWorkspaceCtxAs(t *testing.T, req *http.Request, userID string) *http.Request {
	t.Helper()
	memberRow, err := testHandler.Queries.GetMemberByUserAndWorkspace(context.Background(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      util.MustParseUUID(userID),
		WorkspaceID: util.MustParseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load test member row: %v", err)
	}
	return req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, memberRow))
}

// TestSendChatMessage_LinksAttachments verifies that attachments uploaded
// against a chat_session (chat_message_id NULL) are back-filled with the
// message_id when SendChatMessage receives the matching attachment_ids.
func TestSendChatMessage_LinksAttachments(t *testing.T) {
	origStorage := testHandler.Storage
	testHandler.Storage = &mockStorage{}
	defer func() { testHandler.Storage = origStorage }()

	agentID := createHandlerTestAgent(t, "ChatSendAttachAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	// 1. Upload a file against the chat session.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "send-link.png")
	part.Write([]byte("\x89PNG\r\n\x1a\nbytes"))
	writer.WriteField("chat_session_id", sessionID)
	writer.Close()

	uploadReq := httptest.NewRequest("POST", "/api/upload-file", &body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("X-User-ID", testUserID)
	uploadReq.Header.Set("X-Workspace-ID", testWorkspaceID)

	uploadW := httptest.NewRecorder()
	testHandler.UploadFile(uploadW, uploadReq)
	if uploadW.Code != http.StatusOK {
		t.Fatalf("upload precondition: %d %s", uploadW.Code, uploadW.Body.String())
	}
	var uploadResp AttachmentResponse
	if err := json.Unmarshal(uploadW.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	attachmentID := uploadResp.ID
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM attachment WHERE id = $1`, attachmentID)
	})

	// 2. Send a chat message that references the attachment.
	sendReq := newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content":        "look at this ![](" + uploadResp.URL + ")",
		"attachment_ids": []string{attachmentID},
	})
	sendReq = withURLParam(sendReq, "sessionId", sessionID)
	sendReq = withChatTestWorkspaceCtx(t, sendReq)
	sendW := httptest.NewRecorder()
	testHandler.SendChatMessage(sendW, sendReq)
	if sendW.Code != http.StatusCreated {
		t.Fatalf("SendChatMessage: expected 201, got %d: %s", sendW.Code, sendW.Body.String())
	}

	var sendResp SendChatMessageResponse
	if err := json.Unmarshal(sendW.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("decode send: %v", err)
	}
	if sendResp.MessageID == "" {
		t.Fatal("expected non-empty message_id in send response")
	}

	// 3. Verify the attachment row now points at the new message.
	var dbMessageID *string
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT chat_message_id::text FROM attachment WHERE id = $1`,
		attachmentID,
	).Scan(&dbMessageID); err != nil {
		t.Fatalf("query attachment: %v", err)
	}
	if dbMessageID == nil {
		t.Fatal("chat_message_id is still NULL after send")
	}
	if *dbMessageID != sendResp.MessageID {
		t.Fatalf("chat_message_id mismatch: want %s, got %s", sendResp.MessageID, *dbMessageID)
	}
}

// TestUpdateChatSession_RenamesTitle confirms PATCH writes the new title,
// returns the updated row, and the server-side row reflects it.
func TestUpdateChatSession_RenamesTitle(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatRenameAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	req := newRequest("PATCH", "/api/chat/sessions/"+sessionID, map[string]any{
		"title": "  Renamed Session  ",
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.UpdateChatSession(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateChatSession: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ChatSessionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if resp.Title != "Renamed Session" {
		t.Fatalf("response title: want %q, got %q", "Renamed Session", resp.Title)
	}
	if resp.TitleSource != "user" {
		t.Fatalf("response title_source: want user, got %q", resp.TitleSource)
	}

	var dbTitle string
	var titleSource string
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT title, title_source FROM chat_session WHERE id = $1`,
		sessionID,
	).Scan(&dbTitle, &titleSource); err != nil {
		t.Fatalf("query chat_session: %v", err)
	}
	if dbTitle != "Renamed Session" {
		t.Fatalf("db title: want %q, got %q", "Renamed Session", dbTitle)
	}
	if titleSource != "user" {
		t.Fatalf("db title_source: want user, got %q", titleSource)
	}
}

// TestUpdateChatSession_RejectsBlank refuses an empty/whitespace title with 400.
// (Untitled is a render-side fallback, not a stored value.)
func TestUpdateChatSession_RejectsBlank(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatRenameBlankAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	req := newRequest("PATCH", "/api/chat/sessions/"+sessionID, map[string]any{
		"title": "   ",
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.UpdateChatSession(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("UpdateChatSession blank: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSendChatMessage_InvalidAttachmentIDs rejects malformed UUIDs in
// attachment_ids with 400 before any side effects (no message row created).
func TestSendChatMessage_InvalidAttachmentIDs(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatBadAttachAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	req := newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content":        "hi",
		"attachment_ids": []string{"not-a-uuid"},
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.SendChatMessage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("SendChatMessage with bad attachment id: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Confirm no message row was created.
	var count int
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM chat_message WHERE chat_session_id = $1`,
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("count chat_message: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 chat_message rows after rejected send, got %d", count)
	}
}

func TestSendChatMessage_SetsFirstMessageTitleForUntitledLegacySession(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatFirstTitleAgent", []byte("[]"))
	sessionID := createLooseChatSessionRow(t, agentID, "", "now")

	req := newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content": "   Build a route-owned chat session composer   ",
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.SendChatMessage(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("SendChatMessage: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var title string
	var titleSource string
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT title, title_source FROM chat_session WHERE id = $1`,
		sessionID,
	).Scan(&title, &titleSource); err != nil {
		t.Fatalf("query chat_session: %v", err)
	}
	if title != "Build a route-owned chat session composer" {
		t.Fatalf("title: want first message summary, got %q", title)
	}
	if titleSource != "first_message" {
		t.Fatalf("title_source: want first_message, got %q", titleSource)
	}
}

func TestSendChatMessage_DoesNotOverwriteUserTitle(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatUserTitleAgent", []byte("[]"))
	sessionID := createLooseChatSessionRow(t, agentID, "", "now")
	if _, err := testPool.Exec(
		context.Background(),
		`UPDATE chat_session SET title = 'User picked title', title_source = 'user' WHERE id = $1`,
		sessionID,
	); err != nil {
		t.Fatalf("seed user title: %v", err)
	}

	req := newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content": "This should not replace the title",
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.SendChatMessage(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("SendChatMessage: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var title string
	var titleSource string
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT title, title_source FROM chat_session WHERE id = $1`,
		sessionID,
	).Scan(&title, &titleSource); err != nil {
		t.Fatalf("query chat_session: %v", err)
	}
	if title != "User picked title" {
		t.Fatalf("title should not be overwritten, got %q", title)
	}
	if titleSource != "user" {
		t.Fatalf("title_source should remain user, got %q", titleSource)
	}
}

func TestCreateChatSession_ProjectAssociationCapturesSnapshotAndFilters(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatProjectAgent", []byte("[]"))
	projectID := createHandlerTestProject(t, "Project Chat Context", "planned")

	req := newRequest("POST", "/api/chat/sessions", map[string]any{
		"agent_id":   agentID,
		"project_id": projectID,
		"title":      "  Project Kickoff  ",
	})
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.CreateChatSession(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateChatSession: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created ChatSessionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, created.ID)
	})
	if created.Title != "Project Kickoff" {
		t.Fatalf("title should be trimmed, got %q", created.Title)
	}
	if created.ProjectID == nil || *created.ProjectID != projectID {
		t.Fatalf("project_id: want %s, got %v", projectID, created.ProjectID)
	}
	if created.ProjectContextKind != "project" {
		t.Fatalf("project_context_kind: want project, got %q", created.ProjectContextKind)
	}
	if created.TitleSource != "first_message" {
		t.Fatalf("title_source: want first_message, got %q", created.TitleSource)
	}

	var snapshot ProjectContextSnapshot
	if err := json.Unmarshal(created.ProjectSnapshot, &snapshot); err != nil {
		t.Fatalf("decode project snapshot: %v", err)
	}
	if snapshot.ID != projectID || snapshot.Title != "Project Chat Context" {
		t.Fatalf("snapshot mismatch: %+v", snapshot)
	}

	projectReq := newRequest("GET", "/api/chat/sessions?scope=project&project_id="+projectID+"&status=all", nil)
	projectReq = withChatTestWorkspaceCtx(t, projectReq)
	projectW := httptest.NewRecorder()
	testHandler.ListChatSessions(projectW, projectReq)
	if projectW.Code != http.StatusOK {
		t.Fatalf("ListChatSessions project: expected 200, got %d: %s", projectW.Code, projectW.Body.String())
	}
	var projectSessions []ChatSessionResponse
	if err := json.Unmarshal(projectW.Body.Bytes(), &projectSessions); err != nil {
		t.Fatalf("decode project sessions: %v", err)
	}
	if !chatSessionListContains(projectSessions, created.ID) {
		t.Fatalf("project-scoped list did not include created session")
	}

	looseReq := newRequest("GET", "/api/chat/sessions?scope=loose&status=all", nil)
	looseReq = withChatTestWorkspaceCtx(t, looseReq)
	looseW := httptest.NewRecorder()
	testHandler.ListChatSessions(looseW, looseReq)
	if looseW.Code != http.StatusOK {
		t.Fatalf("ListChatSessions loose: expected 200, got %d: %s", looseW.Code, looseW.Body.String())
	}
	var looseSessions []ChatSessionResponse
	if err := json.Unmarshal(looseW.Body.Bytes(), &looseSessions); err != nil {
		t.Fatalf("decode loose sessions: %v", err)
	}
	if chatSessionListContains(looseSessions, created.ID) {
		t.Fatalf("loose-scoped list included project session")
	}
}

func TestProjectDelete_PreservesChatProjectSnapshot(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatProjectDeleteAgent", []byte("[]"))
	projectID := createHandlerTestProject(t, "Disposable Project", "planned")
	sessionID := createProjectChatSessionRow(t, agentID, projectID, "Project-linked history", "now")

	if _, err := testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	var projectIDText *string
	var contextKind string
	var snapshot []byte
	if err := testPool.QueryRow(context.Background(), `
		SELECT project_id::text, project_context_kind, project_snapshot
		FROM chat_session
		WHERE id = $1
	`, sessionID).Scan(&projectIDText, &contextKind, &snapshot); err != nil {
		t.Fatalf("query chat session: %v", err)
	}
	if projectIDText != nil {
		t.Fatalf("project_id should be null after project delete, got %s", *projectIDText)
	}
	if contextKind != "project" {
		t.Fatalf("project_context_kind should remain project, got %q", contextKind)
	}
	if !strings.Contains(string(snapshot), "Disposable Project") {
		t.Fatalf("project_snapshot should preserve deleted project display data, got %s", string(snapshot))
	}
}

func TestDeleteChatSession_ArchivesInsteadOfHardDeleting(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatArchiveAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)

	req := newRequest("DELETE", "/api/chat/sessions/"+sessionID, nil)
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.DeleteChatSession(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteChatSession: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM chat_session WHERE id = $1`, sessionID).Scan(&status); err != nil {
		t.Fatalf("query archived chat session: %v", err)
	}
	if status != "archived" {
		t.Fatalf("status: want archived, got %q", status)
	}
}

func TestListChatSidebar_FiltersRecentActiveProjectAndLooseSessions(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatSidebarAgent", []byte("[]"))
	activeProjectID := createHandlerTestProject(t, "Sidebar Active Project", "planned")
	completedProjectID := createHandlerTestProject(t, "Sidebar Completed Project", "completed")
	projectSessionID := createProjectChatSessionRow(t, agentID, activeProjectID, "Recent project chat", "now")
	looseSessionID := createLooseChatSessionRow(t, agentID, "Recent loose chat", "now")
	staleSessionID := createLooseChatSessionRow(t, agentID, "Stale loose chat", "stale")
	completedProjectSessionID := createProjectChatSessionRow(t, agentID, completedProjectID, "Completed project chat", "now")

	req := newRequest("GET", "/api/chat/sidebar?recent_days=5", nil)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.ListChatSidebar(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListChatSidebar: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ChatSidebarResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar response: %v", err)
	}
	if len(resp.Projects) != 1 {
		t.Fatalf("expected one active project group, got %d: %+v", len(resp.Projects), resp.Projects)
	}
	if resp.Projects[0].Project.ID != activeProjectID {
		t.Fatalf("project group id: want %s, got %s", activeProjectID, resp.Projects[0].Project.ID)
	}
	if !chatSessionListContains(resp.Projects[0].Sessions, projectSessionID) {
		t.Fatalf("active project session missing from sidebar")
	}
	if chatSessionListContains(resp.Projects[0].Sessions, completedProjectSessionID) {
		t.Fatalf("completed project session should not be in active project sidebar")
	}
	if !chatSessionListContains(resp.Loose, looseSessionID) {
		t.Fatalf("recent loose session missing from sidebar")
	}
	if chatSessionListContains(resp.Loose, staleSessionID) {
		t.Fatalf("stale loose session should not be in five-day sidebar")
	}
}

func TestUpdateChatSession_DoesNotAllowProjectOrAgentMove(t *testing.T) {
	agentID := createHandlerTestAgent(t, "ChatNoMoveAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	projectID := createHandlerTestProject(t, "Forbidden Move Project", "planned")

	req := newRequest("PATCH", "/api/chat/sessions/"+sessionID, map[string]any{
		"project_id": projectID,
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w := httptest.NewRecorder()
	testHandler.UpdateChatSession(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("project-only patch: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	req = newRequest("PATCH", "/api/chat/sessions/"+sessionID, map[string]any{
		"agent_id": agentID,
	})
	req = withURLParam(req, "sessionId", sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	w = httptest.NewRecorder()
	testHandler.UpdateChatSession(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("agent-only patch: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func createHandlerTestProject(t *testing.T, title, status string) string {
	t.Helper()
	var projectID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO project (workspace_id, title, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`, testWorkspaceID, title, status).Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})
	return projectID
}

func createProjectChatSessionRow(t *testing.T, agentID, projectID, title, recency string) string {
	t.Helper()
	updatedExpr := "now()"
	if recency == "stale" {
		updatedExpr = "now() - interval '6 days'"
	}
	var projectTitle string
	if err := testPool.QueryRow(context.Background(), `SELECT title FROM project WHERE id = $1`, projectID).Scan(&projectTitle); err != nil {
		t.Fatalf("load project title: %v", err)
	}
	var sessionID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO chat_session (
			workspace_id, agent_id, creator_id, title, status,
			project_id, project_context_kind, project_snapshot, title_source, updated_at
		)
		VALUES (
			$1, $2, $3, $4, 'active',
			$5::uuid, 'project', jsonb_build_object('id', ($5::uuid)::text, 'title', $6::text, 'status', 'planned'), 'legacy', `+updatedExpr+`
		)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID, title, projectID, projectTitle).Scan(&sessionID); err != nil {
		t.Fatalf("create project chat session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID)
	})
	return sessionID
}

func createLooseChatSessionRow(t *testing.T, agentID, title, recency string) string {
	t.Helper()
	updatedExpr := "now()"
	if recency == "stale" {
		updatedExpr = "now() - interval '6 days'"
	}
	var sessionID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO chat_session (
			workspace_id, agent_id, creator_id, title, status,
			project_context_kind, title_source, updated_at
		)
		VALUES ($1, $2, $3, $4, 'active', 'loose', 'legacy', `+updatedExpr+`)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID, title).Scan(&sessionID); err != nil {
		t.Fatalf("create loose chat session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, sessionID)
	})
	return sessionID
}

func chatSessionListContains(sessions []ChatSessionResponse, sessionID string) bool {
	for _, session := range sessions {
		if session.ID == sessionID {
			return true
		}
	}
	return false
}
