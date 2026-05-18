package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// chatSessionTitleMaxLen caps the rename input. Long enough to fit a
// meaningful summary, short enough to keep the dropdown row scannable.
const (
	chatSessionTitleMaxLen             = 200
	chatSessionFirstMessageTitleMaxLen = 80
)

// ---------------------------------------------------------------------------
// Chat Sessions
// ---------------------------------------------------------------------------

type CreateChatSessionRequest struct {
	AgentID             string  `json:"agent_id"`
	Title               string  `json:"title"`
	DefaultRepositoryID *string `json:"default_repository_id"`
	ProjectID           *string `json:"project_id"`
}

func (h *Handler) CreateChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	var req CreateChatSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	// Verify agent exists in workspace.
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
		ID:          agentID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if agent.ArchivedAt.Valid {
		writeError(w, http.StatusBadRequest, "agent is archived")
		return
	}
	// Private-agent gate: members must be in allowed_principals to start
	// a chat with a private agent. Agent-to-agent chat sessions bypass
	// the gate so A2A collaboration still works.
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	if !h.canAccessPrivateAgent(r.Context(), agent, actorType, actorID, workspaceID) {
		writeError(w, http.StatusForbidden, "you do not have access to this agent")
		return
	}

	var defaultRepositoryID pgtype.UUID
	if req.DefaultRepositoryID != nil && strings.TrimSpace(*req.DefaultRepositoryID) != "" {
		repoUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.DefaultRepositoryID), "default_repository_id")
		if !ok {
			return
		}
		repo, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
			ID:          repoUUID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil || repo.Status == "archived" {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		defaultRepositoryID = repoUUID
	}

	projectContextKind := "loose"
	var projectID pgtype.UUID
	var projectSnapshot []byte
	if req.ProjectID != nil && strings.TrimSpace(*req.ProjectID) != "" {
		projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.ProjectID), "project_id")
		if !ok {
			return
		}
		project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID:          projectUUID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		projectContextKind = "project"
		projectID = projectUUID
		snapshot, err := json.Marshal(ProjectContextSnapshot{
			ID:          uuidToString(project.ID),
			Title:       project.Title,
			Icon:        textToPtr(project.Icon),
			Status:      project.Status,
			CapturedAt:  time.Now().UTC().Format(time.RFC3339Nano),
			WorkspaceID: uuidToString(project.WorkspaceID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to capture project context")
			return
		}
		projectSnapshot = snapshot
	}

	title := strings.TrimSpace(req.Title)
	titleSource := "legacy"
	if title != "" {
		titleSource = "first_message"
	}

	session, err := h.Queries.CreateChatSession(r.Context(), db.CreateChatSessionParams{
		WorkspaceID:         workspaceUUID,
		AgentID:             agentID,
		CreatorID:           parseUUID(userID),
		Title:               title,
		DefaultRepositoryID: defaultRepositoryID,
		ProjectID:           projectID,
		ProjectContextKind:  projectContextKind,
		ProjectSnapshot:     projectSnapshot,
		TitleSource:         titleSource,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat session")
		return
	}

	writeJSON(w, http.StatusCreated, chatSessionToResponse(session))
}

func (h *Handler) ListChatSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	// Compute the accessible-agents set once and use it to drop sessions
	// whose target agent the caller no longer has access to — without this,
	// a member whose role was downgraded would still see the session list
	// (and transcripts via ListChatMessages) for any private agent they
	// previously had access to. Falls back to the user's role from the
	// workspace member context.
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	allowed, ok := h.accessibleAgentIDs(r.Context(), workspaceID, actorType, actorID, member.Role)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to resolve agent access")
		return
	}

	filters, ok := parseChatSessionListFilters(w, r)
	if !ok {
		return
	}

	var resp []ChatSessionResponse
	if filters.status == "all" {
		rows, err := h.Queries.ListAllChatSessionsByCreatorFiltered(r.Context(), db.ListAllChatSessionsByCreatorFilteredParams{
			WorkspaceID: filters.workspaceID,
			CreatorID:   parseUUID(userID),
			Scope:       filters.scope,
			ProjectID:   filters.projectID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list chat sessions")
			return
		}
		resp = make([]ChatSessionResponse, 0, len(rows))
		for _, s := range rows {
			if _, ok := allowed[uuidToString(s.AgentID)]; !ok {
				continue
			}
			resp = append(resp, chatSessionListAllFilteredRowToResponse(s))
		}
	} else {
		rows, err := h.Queries.ListChatSessionsByCreatorFiltered(r.Context(), db.ListChatSessionsByCreatorFilteredParams{
			WorkspaceID: filters.workspaceID,
			CreatorID:   parseUUID(userID),
			Scope:       filters.scope,
			ProjectID:   filters.projectID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list chat sessions")
			return
		}
		resp = make([]ChatSessionResponse, 0, len(rows))
		for _, s := range rows {
			if _, ok := allowed[uuidToString(s.AgentID)]; !ok {
				continue
			}
			resp = append(resp, chatSessionListFilteredRowToResponse(s))
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type chatSessionListFilters struct {
	workspaceID pgtype.UUID
	scope       pgtype.Text
	projectID   pgtype.UUID
	status      string
}

func parseChatSessionListFilters(w http.ResponseWriter, r *http.Request) (chatSessionListFilters, bool) {
	workspaceID := ctxWorkspaceID(r.Context())
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return chatSessionListFilters{}, false
	}
	filters := chatSessionListFilters{
		workspaceID: workspaceUUID,
		status:      r.URL.Query().Get("status"),
	}
	if filters.status == "" {
		filters.status = "active"
	}
	if filters.status != "active" && filters.status != "all" {
		writeError(w, http.StatusBadRequest, "invalid status")
		return chatSessionListFilters{}, false
	}

	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		return filters, true
	}
	if scope != "loose" && scope != "project" {
		writeError(w, http.StatusBadRequest, "invalid scope")
		return chatSessionListFilters{}, false
	}
	filters.scope = pgtype.Text{String: scope, Valid: true}
	if scope == "project" {
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID == "" {
			writeError(w, http.StatusBadRequest, "project_id is required")
			return chatSessionListFilters{}, false
		}
		projectUUID, ok := parseUUIDOrBadRequest(w, projectID, "project_id")
		if !ok {
			return chatSessionListFilters{}, false
		}
		filters.projectID = projectUUID
	}
	return filters, true
}

func (h *Handler) ListChatSidebar(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	allowed, ok := h.accessibleAgentIDs(r.Context(), workspaceID, actorType, actorID, member.Role)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to resolve agent access")
		return
	}

	recentDays := int32(5)
	if raw := strings.TrimSpace(r.URL.Query().Get("recent_days")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 30 {
			writeError(w, http.StatusBadRequest, "recent_days must be between 1 and 30")
			return
		}
		recentDays = int32(n)
	}

	params := db.ListRecentProjectChatSessionsByCreatorParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatorID:   parseUUID(userID),
		RecentDays:  recentDays,
	}
	projectRows, err := h.Queries.ListRecentProjectChatSessionsByCreator(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat sidebar")
		return
	}
	looseRows, err := h.Queries.ListRecentLooseChatSessionsByCreator(r.Context(), db.ListRecentLooseChatSessionsByCreatorParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatorID:   parseUUID(userID),
		RecentDays:  recentDays,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat sidebar")
		return
	}

	projectIndex := make(map[string]int)
	projects := make([]ChatSidebarProjectGroup, 0)
	for _, row := range projectRows {
		if _, ok := allowed[uuidToString(row.AgentID)]; !ok {
			continue
		}
		projectID := uuidToString(row.GroupProjectID)
		idx, exists := projectIndex[projectID]
		if !exists {
			idx = len(projects)
			projectIndex[projectID] = idx
			projects = append(projects, ChatSidebarProjectGroup{
				Project: ChatSidebarProject{
					ID:     projectID,
					Title:  row.GroupProjectTitle,
					Icon:   textToPtr(row.GroupProjectIcon),
					Status: row.GroupProjectStatus,
				},
				Sessions: []ChatSessionResponse{},
			})
		}
		projects[idx].Sessions = append(projects[idx].Sessions, chatSessionRecentProjectRowToResponse(row))
	}

	loose := make([]ChatSessionResponse, 0, len(looseRows))
	for _, row := range looseRows {
		if _, ok := allowed[uuidToString(row.AgentID)]; !ok {
			continue
		}
		loose = append(loose, chatSessionRecentLooseRowToResponse(row))
	}

	writeJSON(w, http.StatusOK, ChatSidebarResponse{
		Projects: projects,
		Loose:    loose,
	})
}

func (h *Handler) loadChatSessionForUser(w http.ResponseWriter, r *http.Request, userID, workspaceID, sessionID string) (db.ChatSession, bool) {
	sessionUUID, ok := parseUUIDOrBadRequest(w, sessionID, "chat session id")
	if !ok {
		return db.ChatSession{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.ChatSession{}, false
	}
	session, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{
		ID:          sessionUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "chat session not found")
		return db.ChatSession{}, false
	}
	if uuidToString(session.CreatorID) != userID {
		writeError(w, http.StatusForbidden, "not your chat session")
		return db.ChatSession{}, false
	}
	return session, true
}

// gateChatSessionForUser combines the session ownership check with the
// private-agent access gate so a member who has lost access to the target
// agent (role downgrade, ownership transfer, agent flipped to private)
// cannot continue reading the chat transcript even though they remain the
// session creator. Returns ok=false after writing the error response.
func (h *Handler) gateChatSessionForUser(w http.ResponseWriter, r *http.Request, userID, workspaceID, sessionID string) (db.ChatSession, bool) {
	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return db.ChatSession{}, false
	}
	agent, err := h.Queries.GetAgent(r.Context(), session.AgentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return db.ChatSession{}, false
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	if !h.canAccessPrivateAgent(r.Context(), agent, actorType, actorID, workspaceID) {
		writeError(w, http.StatusForbidden, "you do not have access to this agent")
		return db.ChatSession{}, false
	}
	return session, true
}

func (h *Handler) GetChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, chatSessionToResponse(session))
}

type UpdateChatSessionRequest struct {
	Title               *string `json:"title"`
	DefaultRepositoryID *string `json:"default_repository_id"`
}

// UpdateChatSession updates user-editable fields on a chat session. The resume
// pointers (session_id / work_dir / runtime_id) are daemon-owned and stay off
// this surface.
func (h *Handler) UpdateChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	var req UpdateChatSessionRequest
	rawFields, err := decodeJSONBodyWithRawFields(r.Body, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	titleRaw, titleProvided := rawFields["title"]
	repoRaw, repoProvided := rawFields["default_repository_id"]
	if _, provided := rawFields["project_id"]; provided {
		writeError(w, http.StatusBadRequest, "project_id cannot be updated")
		return
	}
	if _, provided := rawFields["agent_id"]; provided {
		writeError(w, http.StatusBadRequest, "agent_id cannot be updated")
		return
	}
	if !titleProvided && !repoProvided {
		writeError(w, http.StatusBadRequest, "no supported fields provided")
		return
	}
	var title pgtype.Text
	if titleProvided {
		if string(titleRaw) == "null" || req.Title == nil {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		if len([]rune(trimmed)) > chatSessionTitleMaxLen {
			writeError(w, http.StatusBadRequest, "title is too long")
			return
		}
		title = pgtype.Text{String: trimmed, Valid: true}
	}

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	var defaultRepositoryID pgtype.UUID
	if repoProvided && string(repoRaw) != "null" && req.DefaultRepositoryID != nil && strings.TrimSpace(*req.DefaultRepositoryID) != "" {
		repoUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.DefaultRepositoryID), "default_repository_id")
		if !ok {
			return
		}
		workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
		if !ok {
			return
		}
		repo, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
			ID:          repoUUID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil || repo.Status == "archived" {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		defaultRepositoryID = repoUUID
	}

	updated, err := h.Queries.UpdateChatSessionFields(r.Context(), db.UpdateChatSessionFieldsParams{
		ID:                     session.ID,
		Title:                  title,
		SetDefaultRepositoryID: repoProvided,
		DefaultRepositoryID:    defaultRepositoryID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chat session")
		return
	}

	resolvedSessionID := uuidToString(updated.ID)
	h.publishChat(protocol.EventChatSessionUpdated, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionUpdatedPayload{
		ChatSessionID: resolvedSessionID,
		Title:         updated.Title,
		UpdatedAt:     timestampToString(updated.UpdatedAt),
	})

	writeJSON(w, http.StatusOK, chatSessionToResponse(updated))
}

// DeleteChatSession archives a chat session owned by the caller. The row lock,
// cancel, and archive run inside a single tx so a concurrent SendChatMessage
// cannot enqueue new work while the session is being removed from active
// lists. The row stays durable for proposal, issue, and output provenance.
func (h *Handler) DeleteChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	// FOR UPDATE on the chat_session row blocks any concurrent INSERT into
	// agent_task_queue that references it (the FK validation needs a
	// KEY SHARE lock). The blocked sender resumes after commit and then sees
	// status='archived' before it can enqueue durable work.
	if _, err := qtx.LockChatSessionForDelete(r.Context(), session.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already gone — treat as idempotent success.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lock chat session")
		return
	}

	cancelled, err := qtx.CancelAgentTasksByChatSession(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel chat session tasks")
		return
	}

	if err := qtx.ArchiveChatSession(r.Context(), session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive chat session")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Warn("commit chat session archive failed", "session_id", sessionID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit chat session archive")
		return
	}

	// Post-commit broadcasts. Subscribers should never observe events for a
	// tx that didn't actually persist.
	h.TaskService.BroadcastCancelledTasks(r.Context(), cancelled)

	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatSessionArchived, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionArchivedPayload{
		ChatSessionID: resolvedSessionID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Chat Messages
// ---------------------------------------------------------------------------

type SendChatMessageRequest struct {
	Content       string   `json:"content"`
	AttachmentIDs []string `json:"attachment_ids"`
}

type SendChatMessageResponse struct {
	MessageID string `json:"message_id"`
	TaskID    string `json:"task_id"`
	// CreatedAt anchors the chat StatusPill timer the instant the user
	// hits send. Without it the front-end falls back to its local clock
	// and the timer "snaps backwards" later when WS events deliver the
	// real created_at. Returning it here means the pill renders 0s from
	// the start with a stable anchor.
	CreatedAt string `json:"created_at"`
}

func (h *Handler) SendChatMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	var req SendChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Pre-validate attachment ids early so invalid input returns 400 before
	// any state mutation. The actual link runs after CreateChatMessage so we
	// have a message_id to back-fill into the attachment rows.
	attachmentIDs, ok := parseUUIDSliceOrBadRequest(w, req.AttachmentIDs, "attachment_ids")
	if !ok {
		return
	}

	// Load chat session and re-check the private-agent gate on every send.
	// The session's creator passed the gate at create time, but their
	// workspace role (or the agent's owner) may have changed since — keep
	// stale sessions from being a back-door into a private agent the user
	// can no longer reach. Agent senders bypass to preserve A2A collaboration.
	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}
	// New archive flow doesn't exist anymore, but legacy rows with
	// status='archived' may still be in the DB from before the feature
	// was removed. Refuse to enqueue new agent work for them — frontend
	// surfaces these as read-only.
	if session.Status != "active" {
		writeError(w, http.StatusBadRequest, "chat session is archived")
		return
	}

	// Create the user message first so the daemon can always find it.
	msg, err := h.Queries.CreateChatMessage(r.Context(), db.CreateChatMessageParams{
		ChatSessionID: session.ID,
		Role:          "user",
		Content:       req.Content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat message")
		return
	}

	if title := firstMessageChatTitle(req.Content); title != "" {
		updated, err := h.Queries.SetChatSessionFirstMessageTitle(r.Context(), db.SetChatSessionFirstMessageTitleParams{
			ID:    session.ID,
			Title: title,
		})
		if err == nil {
			resolvedSessionID := uuidToString(updated.ID)
			h.publishChat(protocol.EventChatSessionUpdated, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionUpdatedPayload{
				ChatSessionID: resolvedSessionID,
				Title:         updated.Title,
				UpdatedAt:     timestampToString(updated.UpdatedAt),
			})
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("failed to set first-message chat title", "session_id", sessionID, "error", err)
		}
	}

	// Back-fill chat_message_id on attachments that were uploaded against
	// this session while the user was composing. The query only touches rows
	// where chat_session_id matches AND chat_message_id IS NULL, so it cannot
	// rebind an attachment that already belongs to an earlier message.
	if len(attachmentIDs) > 0 {
		if err := h.Queries.LinkAttachmentsToChatMessage(r.Context(), db.LinkAttachmentsToChatMessageParams{
			ChatMessageID: msg.ID,
			ChatSessionID: session.ID,
			Column3:       attachmentIDs,
		}); err != nil {
			// Don't fail the send — the message content is already saved and
			// the attachments remain on the session (still downloadable).
			slog.Warn("link chat attachments failed", "error", err, "message_id", uuidToString(msg.ID))
		}
	}

	// Enqueue a chat task after the message exists.
	task, err := h.TaskService.EnqueueChatTask(r.Context(), session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue chat task: "+err.Error())
		return
	}

	// Touch session updated_at.
	if err := h.Queries.TouchChatSession(r.Context(), session.ID); err != nil {
		slog.Warn("failed to touch chat session", "session_id", sessionID, "error", err)
	}
	taskContext := h.TaskService.AnalyticsContextForTask(r.Context(), task)
	h.Analytics.Capture(analytics.ChatMessageSent(
		userID,
		workspaceID,
		uuidToString(session.ID),
		uuidToString(task.ID),
		uuidToString(session.AgentID),
		taskContext.RuntimeMode,
		taskContext.Provider,
	))

	// Broadcast the user message.
	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatMessage, workspaceID, "member", userID, resolvedSessionID, protocol.ChatMessagePayload{
		ChatSessionID: resolvedSessionID,
		MessageID:     uuidToString(msg.ID),
		Role:          "user",
		Content:       req.Content,
		TaskID:        uuidToString(task.ID),
		CreatedAt:     timestampToString(msg.CreatedAt),
	})

	writeJSON(w, http.StatusCreated, SendChatMessageResponse{
		MessageID: uuidToString(msg.ID),
		TaskID:    uuidToString(task.ID),
		CreatedAt: timestampToString(task.CreatedAt),
	})
}

func firstMessageChatTitle(content string) string {
	title := strings.Join(strings.Fields(content), " ")
	if title == "" {
		return ""
	}
	runes := []rune(title)
	if len(runes) > chatSessionFirstMessageTitleMaxLen {
		return string(runes[:chatSessionFirstMessageTitleMaxLen])
	}
	return title
}

func (h *Handler) ListChatMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	messages, err := h.Queries.ListChatMessages(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat messages")
		return
	}

	messageIDs := make([]pgtype.UUID, len(messages))
	for i, m := range messages {
		messageIDs[i] = m.ID
	}
	groupedAtt := h.groupChatMessageAttachments(r.Context(), workspaceID, messageIDs)

	resp := make([]ChatMessageResponse, len(messages))
	for i, m := range messages {
		resp[i] = chatMessageToResponse(m, groupedAtt[uuidToString(m.ID)])
	}
	writeJSON(w, http.StatusOK, resp)
}

// PendingChatTaskResponse is returned by GetPendingChatTask — either the
// current in-flight task's id/status, or an empty object when none is active.
// CreatedAt is the anchor the frontend uses to time the chat StatusPill
// (elapsed seconds = now - CreatedAt). It must come from the server because
// optimistic seeds don't have a real task created_at and the timer needs to
// survive refresh / reopen.
type PendingChatTaskResponse struct {
	TaskID    string `json:"task_id,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// MarkChatSessionRead clears the session's unread_since (→ has_unread=false)
// and broadcasts chat:session_read so other devices of the same user drop
// their badges.
func (h *Handler) MarkChatSessionRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	if err := h.Queries.MarkChatSessionRead(r.Context(), session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark session read")
		return
	}

	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatSessionRead, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionReadPayload{
		ChatSessionID: resolvedSessionID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// PendingChatTasksResponse is the aggregate view consumed by the FAB.
type PendingChatTasksResponse struct {
	Tasks []PendingChatTaskItem `json:"tasks"`
}

type PendingChatTaskItem struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	ChatSessionID string `json:"chat_session_id"`
}

// ListPendingChatTasks returns every in-flight chat task owned by the current
// user in this workspace. Drives the FAB's "running" indicator when the chat
// window is closed (no per-session query is subscribed). Tasks belonging to
// private agents the caller has lost access to are dropped from the response.
func (h *Handler) ListPendingChatTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	allowed, ok := h.accessibleAgentIDs(r.Context(), workspaceID, actorType, actorID, member.Role)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to resolve agent access")
		return
	}

	rows, err := h.Queries.ListPendingChatTasksByCreator(r.Context(), db.ListPendingChatTasksByCreatorParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatorID:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending chat tasks")
		return
	}

	// Map session → agent so we can filter without an N+1. The user's own
	// session list is small, so one extra query is cheaper than per-row
	// lookups.
	sessions, err := h.Queries.ListAllChatSessionsByCreator(r.Context(), db.ListAllChatSessionsByCreatorParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatorID:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve chat session agents")
		return
	}
	sessionAgent := make(map[string]string, len(sessions))
	for _, s := range sessions {
		sessionAgent[uuidToString(s.ID)] = uuidToString(s.AgentID)
	}

	items := make([]PendingChatTaskItem, 0, len(rows))
	for _, row := range rows {
		sessionID := uuidToString(row.ChatSessionID)
		agentID, hasAgent := sessionAgent[sessionID]
		if !hasAgent {
			continue
		}
		if _, ok := allowed[agentID]; !ok {
			continue
		}
		items = append(items, PendingChatTaskItem{
			TaskID:        uuidToString(row.TaskID),
			Status:        row.Status,
			ChatSessionID: sessionID,
		})
	}
	writeJSON(w, http.StatusOK, PendingChatTasksResponse{Tasks: items})
}

// GetPendingChatTask returns the most recent in-flight task (queued / dispatched
// / running) for a chat session. The frontend polls this on mount / session
// switch so pending UI state survives refresh and reopen.
func (h *Handler) GetPendingChatTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	task, err := h.Queries.GetPendingChatTask(r.Context(), session.ID)
	if err != nil {
		// No in-flight task — return an empty object, not an error.
		writeJSON(w, http.StatusOK, PendingChatTaskResponse{})
		return
	}

	writeJSON(w, http.StatusOK, PendingChatTaskResponse{
		TaskID:    uuidToString(task.ID),
		Status:    task.Status,
		CreatedAt: timestampToString(task.CreatedAt),
	})
}

func (h *Handler) ListChatIssueProposals(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	proposals, err := h.Queries.ListChatIssueProposalsBySession(r.Context(), db.ListChatIssueProposalsBySessionParams{
		WorkspaceID:   workspaceUUID,
		ChatSessionID: session.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issue proposals")
		return
	}
	items, err := h.Queries.ListChatIssueProposalItemsBySession(r.Context(), db.ListChatIssueProposalItemsBySessionParams{
		WorkspaceID:   workspaceUUID,
		ChatSessionID: session.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issue proposals")
		return
	}

	itemsByProposal := make(map[string][]ChatIssueProposalItemResponse, len(proposals))
	for _, item := range items {
		proposalID := uuidToString(item.ProposalID)
		itemsByProposal[proposalID] = append(itemsByProposal[proposalID], chatIssueProposalItemToResponse(item))
	}

	resp := make([]ChatIssueProposalResponse, 0, len(proposals))
	for _, proposal := range proposals {
		proposalID := uuidToString(proposal.ID)
		resp = append(resp, chatIssueProposalToResponse(proposal, itemsByProposal[proposalID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposals": resp, "total": len(resp)})
}

func (h *Handler) ListChatOutputs(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	rows, err := h.Queries.ListChatSessionOutputMetadata(r.Context(), db.ListChatSessionOutputMetadataParams{
		WorkspaceID:   workspaceUUID,
		ChatSessionID: session.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat outputs")
		return
	}
	prefix := h.getIssuePrefix(r.Context(), workspaceUUID)
	resp := chatOutputMetadataResponses(rows, prefix)
	writeJSON(w, http.StatusOK, map[string]any{"outputs": resp, "total": len(resp)})
}

// ---------------------------------------------------------------------------
// Task cancellation (user-facing, with ownership check)
// ---------------------------------------------------------------------------

// CancelTaskByUser cancels a task after verifying the requesting user owns
// the associated chat session or issue within the current workspace.
func (h *Handler) CancelTaskByUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	taskID := chi.URLParam(r, "taskId")
	taskUUID, ok := parseUUIDOrBadRequest(w, taskID, "task id")
	if !ok {
		return
	}

	task, err := h.Queries.GetAgentTask(r.Context(), taskUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Verify ownership: for chat tasks, check workspace + creator;
	// for issue tasks, verify the issue belongs to the current workspace.
	if task.ChatSessionID.Valid {
		cs, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{
			ID:          task.ChatSessionID,
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if uuidToString(cs.CreatorID) != userID {
			writeError(w, http.StatusForbidden, "not your task")
			return
		}
	} else if task.IssueID.Valid {
		issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
		if err != nil || uuidToString(issue.WorkspaceID) != workspaceID {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
	} else {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	cancelled, err := h.TaskService.CancelTask(r.Context(), taskUUID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, taskToResponse(*cancelled))
}

// ---------------------------------------------------------------------------
// Response types & helpers
// ---------------------------------------------------------------------------

type ChatSessionResponse struct {
	ID                  string          `json:"id"`
	WorkspaceID         string          `json:"workspace_id"`
	AgentID             string          `json:"agent_id"`
	CreatorID           string          `json:"creator_id"`
	Title               string          `json:"title"`
	Status              string          `json:"status"`
	DefaultRepositoryID *string         `json:"default_repository_id"`
	ProjectID           *string         `json:"project_id"`
	ProjectContextKind  string          `json:"project_context_kind"`
	ProjectSnapshot     json.RawMessage `json:"project_snapshot,omitempty"`
	TitleSource         string          `json:"title_source"`
	// Only populated by list endpoints — single-session fetches return false.
	HasUnread bool   `json:"has_unread"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ProjectContextSnapshot struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Title       string  `json:"title"`
	Icon        *string `json:"icon"`
	Status      string  `json:"status"`
	CapturedAt  string  `json:"captured_at"`
}

type ChatSidebarResponse struct {
	Projects []ChatSidebarProjectGroup `json:"projects"`
	Loose    []ChatSessionResponse     `json:"loose"`
}

type ChatSidebarProjectGroup struct {
	Project  ChatSidebarProject    `json:"project"`
	Sessions []ChatSessionResponse `json:"sessions"`
}

type ChatSidebarProject struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Icon   *string `json:"icon"`
	Status string  `json:"status"`
}

type ChatIssueProposalResponse struct {
	ID                  string                          `json:"id"`
	WorkspaceID         string                          `json:"workspace_id"`
	ChatSessionID       string                          `json:"chat_session_id"`
	SourceChatMessageID *string                         `json:"source_chat_message_id"`
	SourceTaskID        *string                         `json:"source_task_id"`
	ProposerAgentID     *string                         `json:"proposer_agent_id"`
	Title               string                          `json:"title"`
	Summary             *string                         `json:"summary"`
	Status              string                          `json:"status"`
	Items               []ChatIssueProposalItemResponse `json:"items"`
	CreatedAt           string                          `json:"created_at"`
	UpdatedAt           string                          `json:"updated_at"`
}

type ChatIssueProposalItemResponse struct {
	ID               string          `json:"id"`
	ProposalID       string          `json:"proposal_id"`
	Position         int32           `json:"position"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Priority         *string         `json:"priority"`
	Labels           json.RawMessage `json:"labels"`
	AssigneeType     *string         `json:"assignee_type"`
	AssigneeID       *string         `json:"assignee_id"`
	Status           string          `json:"status"`
	IssueID          *string         `json:"issue_id"`
	ApprovedSnapshot json.RawMessage `json:"approved_snapshot,omitempty"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

type ChatMessageResponse struct {
	ID            string  `json:"id"`
	ChatSessionID string  `json:"chat_session_id"`
	Role          string  `json:"role"`
	Content       string  `json:"content"`
	TaskID        *string `json:"task_id"`
	CreatedAt     string  `json:"created_at"`
	// FailureReason flags an assistant row synthesized by FailTask's chat
	// fallback. Front-end uses it to switch to the destructive bubble.
	FailureReason *string `json:"failure_reason"`
	// ElapsedMs is the wall-clock duration from task creation to terminal
	// state. Drives "Replied in 38s" / "Failed after 12s" captions.
	ElapsedMs *int64 `json:"elapsed_ms"`
	// Attachments linked to this message via chat_message_id. The chat
	// bubble renders file cards from these, and the daemon claim path
	// (daemon.go) pulls structured metadata from the same source so the
	// agent can `multica attachment download <id>` rather than guessing
	// from a markdown URL that may expire.
	Attachments []AttachmentResponse `json:"attachments,omitempty"`
}

func chatSessionToResponse(s db.ChatSession) ChatSessionResponse {
	return chatSessionResponseFromFields(
		s.ID,
		s.WorkspaceID,
		s.AgentID,
		s.CreatorID,
		s.Title,
		s.Status,
		s.DefaultRepositoryID,
		s.ProjectID,
		s.ProjectContextKind,
		s.ProjectSnapshot,
		s.TitleSource,
		false,
		s.CreatedAt,
		s.UpdatedAt,
	)
}

func chatSessionListFilteredRowToResponse(s db.ListChatSessionsByCreatorFilteredRow) ChatSessionResponse {
	return chatSessionResponseFromFields(
		s.ID,
		s.WorkspaceID,
		s.AgentID,
		s.CreatorID,
		s.Title,
		s.Status,
		s.DefaultRepositoryID,
		s.ProjectID,
		s.ProjectContextKind,
		s.ProjectSnapshot,
		s.TitleSource,
		s.HasUnread,
		s.CreatedAt,
		s.UpdatedAt,
	)
}

func chatSessionListAllFilteredRowToResponse(s db.ListAllChatSessionsByCreatorFilteredRow) ChatSessionResponse {
	return chatSessionResponseFromFields(
		s.ID,
		s.WorkspaceID,
		s.AgentID,
		s.CreatorID,
		s.Title,
		s.Status,
		s.DefaultRepositoryID,
		s.ProjectID,
		s.ProjectContextKind,
		s.ProjectSnapshot,
		s.TitleSource,
		s.HasUnread,
		s.CreatedAt,
		s.UpdatedAt,
	)
}

func chatSessionRecentProjectRowToResponse(s db.ListRecentProjectChatSessionsByCreatorRow) ChatSessionResponse {
	return chatSessionResponseFromFields(
		s.ID,
		s.WorkspaceID,
		s.AgentID,
		s.CreatorID,
		s.Title,
		s.Status,
		s.DefaultRepositoryID,
		s.ProjectID,
		s.ProjectContextKind,
		s.ProjectSnapshot,
		s.TitleSource,
		s.HasUnread,
		s.CreatedAt,
		s.UpdatedAt,
	)
}

func chatSessionRecentLooseRowToResponse(s db.ListRecentLooseChatSessionsByCreatorRow) ChatSessionResponse {
	return chatSessionResponseFromFields(
		s.ID,
		s.WorkspaceID,
		s.AgentID,
		s.CreatorID,
		s.Title,
		s.Status,
		s.DefaultRepositoryID,
		s.ProjectID,
		s.ProjectContextKind,
		s.ProjectSnapshot,
		s.TitleSource,
		s.HasUnread,
		s.CreatedAt,
		s.UpdatedAt,
	)
}

func chatSessionResponseFromFields(
	id pgtype.UUID,
	workspaceID pgtype.UUID,
	agentID pgtype.UUID,
	creatorID pgtype.UUID,
	title string,
	status string,
	defaultRepositoryID pgtype.UUID,
	projectID pgtype.UUID,
	projectContextKind string,
	projectSnapshot []byte,
	titleSource string,
	hasUnread bool,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) ChatSessionResponse {
	if projectContextKind == "" {
		projectContextKind = "loose"
	}
	if titleSource == "" {
		titleSource = "legacy"
	}
	return ChatSessionResponse{
		ID:                  uuidToString(id),
		WorkspaceID:         uuidToString(workspaceID),
		AgentID:             uuidToString(agentID),
		CreatorID:           uuidToString(creatorID),
		Title:               title,
		Status:              status,
		DefaultRepositoryID: uuidToPtr(defaultRepositoryID),
		ProjectID:           uuidToPtr(projectID),
		ProjectContextKind:  projectContextKind,
		ProjectSnapshot:     nullableJSONRawMessage(projectSnapshot),
		TitleSource:         titleSource,
		HasUnread:           hasUnread,
		CreatedAt:           timestampToString(createdAt),
		UpdatedAt:           timestampToString(updatedAt),
	}
}

func nullableJSONRawMessage(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func chatIssueProposalToResponse(row db.ChatIssueProposal, items []ChatIssueProposalItemResponse) ChatIssueProposalResponse {
	if items == nil {
		items = []ChatIssueProposalItemResponse{}
	}
	return ChatIssueProposalResponse{
		ID:                  uuidToString(row.ID),
		WorkspaceID:         uuidToString(row.WorkspaceID),
		ChatSessionID:       uuidToString(row.ChatSessionID),
		SourceChatMessageID: uuidToPtr(row.SourceChatMessageID),
		SourceTaskID:        uuidToPtr(row.SourceTaskID),
		ProposerAgentID:     uuidToPtr(row.ProposerAgentID),
		Title:               row.Title,
		Summary:             textToPtr(row.Summary),
		Status:              row.Status,
		Items:               items,
		CreatedAt:           timestampToString(row.CreatedAt),
		UpdatedAt:           timestampToString(row.UpdatedAt),
	}
}

func chatIssueProposalItemToResponse(row db.ChatIssueProposalItem) ChatIssueProposalItemResponse {
	return ChatIssueProposalItemResponse{
		ID:               uuidToString(row.ID),
		ProposalID:       uuidToString(row.ProposalID),
		Position:         row.Position,
		Title:            row.Title,
		Description:      row.Description,
		Priority:         textToPtr(row.Priority),
		Labels:           jsonArrayOrEmpty(row.Labels),
		AssigneeType:     textToPtr(row.AssigneeType),
		AssigneeID:       uuidToPtr(row.AssigneeID),
		Status:           row.Status,
		IssueID:          uuidToPtr(row.IssueID),
		ApprovedSnapshot: nullableJSONRawMessage(row.ApprovedSnapshot),
		CreatedAt:        timestampToString(row.CreatedAt),
		UpdatedAt:        timestampToString(row.UpdatedAt),
	}
}

func jsonArrayOrEmpty(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return json.RawMessage(raw)
}

func chatMessageToResponse(m db.ChatMessage, attachments []AttachmentResponse) ChatMessageResponse {
	return ChatMessageResponse{
		ID:            uuidToString(m.ID),
		ChatSessionID: uuidToString(m.ChatSessionID),
		Role:          m.Role,
		Content:       m.Content,
		TaskID:        uuidToPtr(m.TaskID),
		CreatedAt:     timestampToString(m.CreatedAt),
		FailureReason: textToPtr(m.FailureReason),
		ElapsedMs:     int8ToPtr(m.ElapsedMs),
		Attachments:   attachments,
	}
}
