package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type UpdateChatIssueProposalItemRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Priority     *string  `json:"priority"`
	Labels       []string `json:"labels"`
	AssigneeType *string  `json:"assignee_type"`
	AssigneeID   *string  `json:"assignee_id"`
}

type ApproveChatIssueProposalRequest struct {
	ItemIDs []string `json:"item_ids"`
}

type ApproveChatIssueProposalResponse struct {
	Issues   []IssueResponse           `json:"issues"`
	Proposal ChatIssueProposalResponse `json:"proposal"`
}

type ChatSessionIssuesResponse struct {
	Issues []IssueResponse `json:"issues"`
	Total  int             `json:"total"`
}

func (h *Handler) ListChatIssues(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.Queries.ListChatSessionIssues(r.Context(), db.ListChatSessionIssuesParams{
		WorkspaceID: workspaceUUID,
		OriginID:    session.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat issues")
		return
	}
	prefix := h.getIssuePrefix(r.Context(), workspaceUUID)
	resp := make([]IssueResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, issueToResponse(row, prefix))
	}
	writeJSON(w, http.StatusOK, ChatSessionIssuesResponse{Issues: resp, Total: len(resp)})
}

func (h *Handler) UpdateChatIssueProposalItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	proposalID := chi.URLParam(r, "proposalId")
	itemID := chi.URLParam(r, "itemId")

	proposal, session, ok := h.loadChatIssueProposalForUser(w, r, userID, workspaceID, proposalID)
	if !ok {
		return
	}
	itemUUID, ok := parseUUIDOrBadRequest(w, itemID, "item id")
	if !ok {
		return
	}

	var req UpdateChatIssueProposalItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	prepared, ok := h.prepareProposalItemUpdate(w, r, workspaceID, req)
	if !ok {
		return
	}

	item, err := h.Queries.UpdateChatIssueProposalItemDraft(r.Context(), db.UpdateChatIssueProposalItemDraftParams{
		ID:           itemUUID,
		ProposalID:   proposal.ID,
		Title:        prepared.Title,
		Description:  prepared.Description,
		Priority:     ptrToText(prepared.Priority),
		Labels:       prepared.Labels,
		AssigneeType: ptrToText(prepared.AssigneeType),
		AssigneeID:   prepared.AssigneeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "proposal item cannot be edited")
			return
		}
		slog.Warn("update chat issue proposal item failed", "proposal_id", proposalID, "item_id", itemID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update proposal item")
		return
	}

	h.publishChatIssueProposalUpdate(workspaceID, session.ID, proposal.ID)
	writeJSON(w, http.StatusOK, chatIssueProposalItemToResponse(item))
}

func (h *Handler) RestoreChatIssueProposalItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	proposalID := chi.URLParam(r, "proposalId")
	itemID := chi.URLParam(r, "itemId")

	proposal, session, ok := h.loadChatIssueProposalForUser(w, r, userID, workspaceID, proposalID)
	if !ok {
		return
	}
	itemUUID, ok := parseUUIDOrBadRequest(w, itemID, "item id")
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore proposal item")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	if _, err := qtx.RestoreChatIssueProposalItem(r.Context(), db.RestoreChatIssueProposalItemParams{
		ID:         itemUUID,
		ProposalID: proposal.ID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "proposal item cannot be restored")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to restore proposal item")
		return
	}
	updatedProposal, items, err := h.refreshChatIssueProposalStatus(r, qtx, proposal.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore proposal item")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore proposal item")
		return
	}

	h.publishChatIssueProposalUpdate(workspaceID, session.ID, proposal.ID)
	writeJSON(w, http.StatusOK, chatIssueProposalToResponse(updatedProposal, proposalItemResponses(items)))
}

func (h *Handler) DismissChatIssueProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	proposalID := chi.URLParam(r, "proposalId")

	proposal, session, ok := h.loadChatIssueProposalForUser(w, r, userID, workspaceID, proposalID)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss proposal")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	if err := qtx.SkipPendingChatIssueProposalItems(r.Context(), proposal.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss proposal")
		return
	}
	updatedProposal, items, err := h.refreshChatIssueProposalStatus(r, qtx, proposal.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss proposal")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss proposal")
		return
	}

	h.publishChatIssueProposalUpdate(workspaceID, session.ID, proposal.ID)
	writeJSON(w, http.StatusOK, chatIssueProposalToResponse(updatedProposal, proposalItemResponses(items)))
}

func (h *Handler) ApproveChatIssueProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	proposalID := chi.URLParam(r, "proposalId")

	proposal, session, ok := h.loadChatIssueProposalForUser(w, r, userID, workspaceID, proposalID)
	if !ok {
		return
	}
	var req ApproveChatIssueProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	selectedIDs, ok := parseUUIDSliceOrBadRequest(w, req.ItemIDs, "item_ids")
	if !ok {
		return
	}
	if len(selectedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "item_ids must include at least one item")
		return
	}

	result, status, message, err := h.approveChatIssueProposalTransaction(r, workspaceID, userID, proposal, session, selectedIDs)
	if err != nil {
		slog.Warn("approve chat issue proposal failed", "proposal_id", proposalID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to approve proposal")
		return
	}
	if status != 0 {
		writeError(w, status, message)
		return
	}

	for _, issue := range result.Issues {
		h.publish(protocol.EventIssueCreated, workspaceID, "member", userID, map[string]any{"issue": issue})
		if issue.Labels != nil {
			h.publish(protocol.EventIssueLabelsChanged, workspaceID, "member", userID, map[string]any{
				"issue_id": issue.ID,
				"labels":   *issue.Labels,
			})
		}
	}
	h.publishChatIssueProposalUpdate(workspaceID, session.ID, proposal.ID)
	h.publishChat(protocol.EventChatIssuesUpdated, workspaceID, "member", userID, uuidToString(session.ID), map[string]any{
		"chat_session_id": uuidToString(session.ID),
		"count":           len(result.Issues),
	})
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) approveChatIssueProposalTransaction(
	r *http.Request,
	workspaceID string,
	userID string,
	proposal db.ChatIssueProposal,
	session db.ChatSession,
	selectedIDs []pgtype.UUID,
) (ApproveChatIssueProposalResponse, int, string, error) {
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return ApproveChatIssueProposalResponse{}, 0, "", err
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	items, err := qtx.ListChatIssueProposalItemsByProposalForUpdate(r.Context(), proposal.ID)
	if err != nil {
		return ApproveChatIssueProposalResponse{}, 0, "", err
	}
	selected := make(map[string]pgtype.UUID, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[uuidToString(id)] = id
	}
	selectedItems := make([]db.ChatIssueProposalItem, 0, len(selectedIDs))
	for _, item := range items {
		key := uuidToString(item.ID)
		if _, ok := selected[key]; !ok {
			continue
		}
		if item.Status != "pending" {
			return ApproveChatIssueProposalResponse{}, http.StatusBadRequest, "selected items must be pending", nil
		}
		if strings.TrimSpace(item.Title) == "" {
			return ApproveChatIssueProposalResponse{}, http.StatusBadRequest, "selected item title is required", nil
		}
		if item.AssigneeType.Valid || item.AssigneeID.Valid {
			if status, msg := h.validateAssigneePair(r.Context(), r, workspaceID, item.AssigneeType, item.AssigneeID); status != 0 {
				return ApproveChatIssueProposalResponse{}, status, msg, nil
			}
		}
		if _, err := proposalItemLabelNames(item); err != nil {
			return ApproveChatIssueProposalResponse{}, http.StatusBadRequest, err.Error(), nil
		}
		selectedItems = append(selectedItems, item)
		delete(selected, key)
	}
	if len(selected) > 0 {
		return ApproveChatIssueProposalResponse{}, http.StatusBadRequest, "item_ids contains an item outside this proposal", nil
	}

	workspaceUUID := parseUUID(workspaceID)
	projectID := pgtype.UUID{}
	if session.ProjectContextKind == "project" && session.ProjectID.Valid {
		projectID = session.ProjectID
	}
	prefix := h.getIssuePrefix(r.Context(), workspaceUUID)
	created := make([]IssueResponse, 0, len(selectedItems))
	for _, item := range selectedItems {
		issueNumber, err := qtx.IncrementIssueCounter(r.Context(), workspaceUUID)
		if err != nil {
			return ApproveChatIssueProposalResponse{}, 0, "", err
		}
		issue, err := qtx.CreateIssueWithOrigin(r.Context(), db.CreateIssueWithOriginParams{
			WorkspaceID:   workspaceUUID,
			Title:         item.Title,
			Description:   pgtype.Text{String: item.Description, Valid: strings.TrimSpace(item.Description) != ""},
			Status:        "backlog",
			Priority:      proposalItemPriority(item),
			AssigneeType:  item.AssigneeType,
			AssigneeID:    item.AssigneeID,
			CreatorType:   "member",
			CreatorID:     parseUUID(userID),
			ParentIssueID: pgtype.UUID{},
			Position:      0,
			DueDate:       pgtype.Timestamptz{},
			Number:        issueNumber,
			ProjectID:     projectID,
			OriginType:    pgtype.Text{String: "chat_session", Valid: true},
			OriginID:      session.ID,
		})
		if err != nil {
			return ApproveChatIssueProposalResponse{}, 0, "", err
		}
		labels, err := h.attachProposalLabelsToIssue(r, qtx, workspaceUUID, issue.ID, item)
		if err != nil {
			return ApproveChatIssueProposalResponse{}, 0, "", err
		}
		snapshot, err := approvedIssueProposalSnapshot(item)
		if err != nil {
			return ApproveChatIssueProposalResponse{}, 0, "", err
		}
		if _, err := qtx.MarkChatIssueProposalItemCreated(r.Context(), db.MarkChatIssueProposalItemCreatedParams{
			ID:               item.ID,
			ProposalID:       proposal.ID,
			IssueID:          issue.ID,
			ApprovedSnapshot: snapshot,
		}); err != nil {
			return ApproveChatIssueProposalResponse{}, 0, "", err
		}
		resp := issueToResponse(issue, prefix)
		if labels != nil {
			labelResp := labelsToResponse(labels)
			resp.Labels = &labelResp
		}
		created = append(created, resp)
	}
	if err := qtx.MarkUnselectedPendingChatIssueProposalItemsSkipped(r.Context(), db.MarkUnselectedPendingChatIssueProposalItemsSkippedParams{
		ProposalID: proposal.ID,
		ItemIds:    selectedIDs,
	}); err != nil {
		return ApproveChatIssueProposalResponse{}, 0, "", err
	}
	updatedProposal, updatedItems, err := h.refreshChatIssueProposalStatus(r, qtx, proposal.ID)
	if err != nil {
		return ApproveChatIssueProposalResponse{}, 0, "", err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return ApproveChatIssueProposalResponse{}, 0, "", err
	}

	return ApproveChatIssueProposalResponse{
		Issues:   created,
		Proposal: chatIssueProposalToResponse(updatedProposal, proposalItemResponses(updatedItems)),
	}, 0, "", nil
}

func (h *Handler) loadChatIssueProposalForUser(w http.ResponseWriter, r *http.Request, userID, workspaceID, proposalID string) (db.ChatIssueProposal, db.ChatSession, bool) {
	proposalUUID, ok := parseUUIDOrBadRequest(w, proposalID, "proposal id")
	if !ok {
		return db.ChatIssueProposal{}, db.ChatSession{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.ChatIssueProposal{}, db.ChatSession{}, false
	}
	proposal, err := h.Queries.GetChatIssueProposal(r.Context(), proposalUUID)
	if err != nil || proposal.WorkspaceID != workspaceUUID {
		writeError(w, http.StatusNotFound, "proposal not found")
		return db.ChatIssueProposal{}, db.ChatSession{}, false
	}
	session, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{
		ID:          proposal.ChatSessionID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil || uuidToString(session.CreatorID) != userID {
		writeError(w, http.StatusNotFound, "proposal not found")
		return db.ChatIssueProposal{}, db.ChatSession{}, false
	}
	gatedSession, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, uuidToString(session.ID))
	if !ok {
		return db.ChatIssueProposal{}, db.ChatSession{}, false
	}
	return proposal, gatedSession, true
}

func (h *Handler) prepareProposalItemUpdate(w http.ResponseWriter, r *http.Request, workspaceID string, req UpdateChatIssueProposalItemRequest) (preparedChatIssueProposalItem, bool) {
	item := ChatIssueProposalItemManifestRequest{
		Title:        req.Title,
		Description:  req.Description,
		Priority:     req.Priority,
		Labels:       req.Labels,
		AssigneeType: req.AssigneeType,
		AssigneeID:   req.AssigneeID,
	}
	prepared, err := h.prepareChatIssueProposalItem(r, workspaceID, 0, 0, item)
	if err != nil {
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "proposals[0].items[0]."))
		return preparedChatIssueProposalItem{}, false
	}
	return prepared, true
}

func (h *Handler) refreshChatIssueProposalStatus(r *http.Request, qtx *db.Queries, proposalID pgtype.UUID) (db.ChatIssueProposal, []db.ChatIssueProposalItem, error) {
	items, err := qtx.ListChatIssueProposalItemsByProposal(r.Context(), proposalID)
	if err != nil {
		return db.ChatIssueProposal{}, nil, err
	}
	status := chatIssueProposalStatusFromItems(items)
	proposal, err := qtx.SetChatIssueProposalStatus(r.Context(), db.SetChatIssueProposalStatusParams{
		ID:     proposalID,
		Status: status,
	})
	if err != nil {
		return db.ChatIssueProposal{}, nil, err
	}
	return proposal, items, nil
}

func chatIssueProposalStatusFromItems(items []db.ChatIssueProposalItem) string {
	hasPending := false
	hasCreated := false
	hasSkipped := false
	for _, item := range items {
		switch item.Status {
		case "created":
			hasCreated = true
		case "skipped":
			hasSkipped = true
		default:
			hasPending = true
		}
	}
	switch {
	case hasCreated && (hasPending || hasSkipped):
		return "partially_accepted"
	case hasCreated:
		return "accepted"
	case hasPending:
		return "pending"
	case hasSkipped:
		return "dismissed"
	default:
		return "pending"
	}
}

func proposalItemPriority(item db.ChatIssueProposalItem) string {
	if item.Priority.Valid && strings.TrimSpace(item.Priority.String) != "" {
		return item.Priority.String
	}
	return "none"
}

const chatIssueProposalDefaultLabelColor = "#3b82f6"

func proposalItemLabelNames(item db.ChatIssueProposalItem) ([]string, error) {
	var raw []string
	if len(item.Labels) > 0 {
		if err := json.Unmarshal(item.Labels, &raw); err != nil {
			return nil, fmt.Errorf("decode proposal labels: %w", err)
		}
	}
	names := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, label := range raw {
		if strings.TrimSpace(label) == "" {
			continue
		}
		name, err := validateLabelName(label)
		if err != nil {
			return nil, fmt.Errorf("proposal label %q is invalid: %w", label, err)
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

func (h *Handler) attachProposalLabelsToIssue(
	r *http.Request,
	qtx *db.Queries,
	workspaceID pgtype.UUID,
	issueID pgtype.UUID,
	item db.ChatIssueProposalItem,
) ([]db.IssueLabel, error) {
	names, err := proposalItemLabelNames(item)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	for _, name := range names {
		label, err := qtx.GetLabelByName(r.Context(), db.GetLabelByNameParams{
			WorkspaceID: workspaceID,
			Name:        name,
		})
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			label, err = qtx.CreateLabel(r.Context(), db.CreateLabelParams{
				WorkspaceID: workspaceID,
				Name:        name,
				Color:       chatIssueProposalDefaultLabelColor,
			})
			if err != nil {
				if !isUniqueViolation(err) {
					return nil, err
				}
				label, err = qtx.GetLabelByName(r.Context(), db.GetLabelByNameParams{
					WorkspaceID: workspaceID,
					Name:        name,
				})
				if err != nil {
					return nil, err
				}
			}
		}
		if err := qtx.AttachLabelToIssue(r.Context(), db.AttachLabelToIssueParams{
			IssueID:     issueID,
			LabelID:     label.ID,
			WorkspaceID: workspaceID,
		}); err != nil {
			return nil, err
		}
	}
	return qtx.ListLabelsByIssue(r.Context(), db.ListLabelsByIssueParams{
		IssueID:     issueID,
		WorkspaceID: workspaceID,
	})
}

func approvedIssueProposalSnapshot(item db.ChatIssueProposalItem) ([]byte, error) {
	labels, err := proposalItemLabelNames(item)
	if err != nil {
		return nil, err
	}
	snapshot := map[string]any{
		"title":       item.Title,
		"description": item.Description,
		"priority":    proposalItemPriority(item),
		"labels":      labels,
	}
	if item.AssigneeType.Valid {
		snapshot["assignee_type"] = item.AssigneeType.String
	}
	if item.AssigneeID.Valid {
		snapshot["assignee_id"] = uuidToString(item.AssigneeID)
	}
	return json.Marshal(snapshot)
}

func proposalItemResponses(items []db.ChatIssueProposalItem) []ChatIssueProposalItemResponse {
	out := make([]ChatIssueProposalItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, chatIssueProposalItemToResponse(item))
	}
	return out
}

func (h *Handler) publishChatIssueProposalUpdate(workspaceID string, sessionID, proposalID pgtype.UUID) {
	sessionIDStr := uuidToString(sessionID)
	h.publishChat(protocol.EventChatIssueProposalsUpdated, workspaceID, "member", "", sessionIDStr, map[string]any{
		"chat_session_id": sessionIDStr,
		"proposal_id":     uuidToString(proposalID),
	})
}
