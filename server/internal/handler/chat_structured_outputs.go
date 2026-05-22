package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	chatStructuredProposalTitleMaxLen       = 200
	chatStructuredProposalSummaryMaxLen     = 1000
	chatStructuredProposalItemTitleMaxLen   = 200
	chatStructuredProposalDescriptionMaxLen = 12000
)

var validChatProposalPriorities = map[string]bool{
	"urgent": true,
	"high":   true,
	"medium": true,
	"low":    true,
	"none":   true,
}

type TaskStructuredOutputsRequest struct {
	ChatSummary    *ChatSummaryManifestRequest        `json:"chat_summary"`
	IssueProposals *IssueProposalsManifestRequest     `json:"issue_proposals"`
	Outputs        *TaskOutputMetadataManifestRequest `json:"outputs"`
}

type ChatSummaryManifestRequest struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
}

type IssueProposalsManifestRequest struct {
	Version   int                                `json:"version"`
	Proposals []ChatIssueProposalManifestRequest `json:"proposals"`
}

type ChatIssueProposalManifestRequest struct {
	Title   string                                 `json:"title"`
	Summary *string                                `json:"summary"`
	Items   []ChatIssueProposalItemManifestRequest `json:"items"`
	Status  *json.RawMessage                       `json:"status"`
}

type ChatIssueProposalItemManifestRequest struct {
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Priority     *string          `json:"priority"`
	Labels       []string         `json:"labels"`
	AssigneeType *string          `json:"assignee_type"`
	AssigneeID   *string          `json:"assignee_id"`
	Status       *json.RawMessage `json:"status"`
}

func (h *Handler) processTaskStructuredOutputs(r *http.Request, task db.AgentTaskQueue, structured *TaskStructuredOutputsRequest) {
	if structured == nil || !task.ChatSessionID.Valid {
		if structured != nil && structured.Outputs != nil {
			h.processStructuredTaskOutputs(r, task, structured.Outputs)
		}
		return
	}

	if structured.ChatSummary != nil {
		h.processChatSummaryManifest(r, task, *structured.ChatSummary)
	}
	if structured.IssueProposals != nil {
		h.processIssueProposalsManifest(r, task, *structured.IssueProposals)
	}
	if structured.Outputs != nil {
		h.processStructuredTaskOutputs(r, task, structured.Outputs)
	}
}

func (h *Handler) processChatSummaryManifest(r *http.Request, task db.AgentTaskQueue, manifest ChatSummaryManifestRequest) {
	if manifest.Version != 1 {
		slog.Warn("chat summary manifest ignored: unsupported version", "task_id", uuidToString(task.ID), "version", manifest.Version)
		return
	}
	title := strings.TrimSpace(manifest.Title)
	if title == "" {
		slog.Warn("chat summary manifest ignored: title is required", "task_id", uuidToString(task.ID))
		return
	}
	title = truncateRunes(title, chatSessionTitleMaxLen)

	updated, err := h.Queries.SetChatSessionAgentSummaryTitle(r.Context(), db.SetChatSessionAgentSummaryTitleParams{
		ID:    task.ChatSessionID,
		Title: title,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("chat summary title update failed", "task_id", uuidToString(task.ID), "error", err)
		}
		return
	}

	sessionID := uuidToString(updated.ID)
	workspaceID := uuidToString(updated.WorkspaceID)
	h.publishChat(protocol.EventChatSessionUpdated, workspaceID, "daemon", "", sessionID, protocol.ChatSessionUpdatedPayload{
		ChatSessionID: sessionID,
		Title:         updated.Title,
		UpdatedAt:     timestampToString(updated.UpdatedAt),
	})
}

func (h *Handler) processIssueProposalsManifest(r *http.Request, task db.AgentTaskQueue, manifest IssueProposalsManifestRequest) {
	if manifest.Version != 1 {
		slog.Warn("issue proposals manifest ignored: unsupported version", "task_id", uuidToString(task.ID), "version", manifest.Version)
		return
	}
	if len(manifest.Proposals) == 0 {
		return
	}

	workspaceID := h.TaskService.ResolveTaskWorkspaceID(r.Context(), task)
	if workspaceID == "" {
		slog.Warn("issue proposals manifest ignored: task workspace not found", "task_id", uuidToString(task.ID))
		return
	}
	workspaceUUID, err := parseStructuredUUID(workspaceID)
	if err != nil {
		slog.Warn("issue proposals manifest ignored: invalid workspace", "task_id", uuidToString(task.ID), "workspace_id", workspaceID)
		return
	}

	assistantMessageID := h.assistantChatMessageIDForTask(r, task)
	prepared, err := h.prepareChatIssueProposals(r, workspaceID, manifest.Proposals)
	if err != nil {
		slog.Warn("issue proposals manifest ignored", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		slog.Warn("issue proposals persist failed: start tx", "task_id", uuidToString(task.ID), "error", err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	existingItems, err := qtx.ListChatIssueProposalItemsForTaskForUpdate(r.Context(), db.ListChatIssueProposalItemsForTaskForUpdateParams{
		ChatSessionID: task.ChatSessionID,
		SourceTaskID:  task.ID,
	})
	if err != nil {
		slog.Warn("issue proposals replace failed: lock existing proposals", "task_id", uuidToString(task.ID), "error", err)
		return
	}
	for _, existing := range existingItems {
		if existing.ProposalStatus != "pending" || existing.ItemStatus != "pending" {
			slog.Warn(
				"issue proposals manifest ignored: existing proposal already acted on",
				"task_id", uuidToString(task.ID),
				"proposal_status", existing.ProposalStatus,
				"item_status", existing.ItemStatus,
			)
			return
		}
	}

	if err := qtx.DeleteChatIssueProposalsForTask(r.Context(), db.DeleteChatIssueProposalsForTaskParams{
		ChatSessionID: task.ChatSessionID,
		SourceTaskID:  task.ID,
	}); err != nil {
		slog.Warn("issue proposals replace failed", "task_id", uuidToString(task.ID), "error", err)
		return
	}
	if err := qtx.SupersedePendingChatIssueProposalsForSession(r.Context(), db.SupersedePendingChatIssueProposalsForSessionParams{
		ChatSessionID: task.ChatSessionID,
		SourceTaskID:  task.ID,
	}); err != nil {
		slog.Warn("issue proposals supersede failed", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	itemCount := 0
	for _, proposal := range prepared {
		row, err := qtx.CreateChatIssueProposal(r.Context(), db.CreateChatIssueProposalParams{
			WorkspaceID:         workspaceUUID,
			ChatSessionID:       task.ChatSessionID,
			Title:               proposal.Title,
			SourceChatMessageID: assistantMessageID,
			SourceTaskID:        task.ID,
			ProposerAgentID:     task.AgentID,
			Summary:             ptrToText(proposal.Summary),
		})
		if err != nil {
			slog.Warn("issue proposal persist failed", "task_id", uuidToString(task.ID), "error", err)
			return
		}
		for _, item := range proposal.Items {
			if _, err := qtx.CreateChatIssueProposalItem(r.Context(), db.CreateChatIssueProposalItemParams{
				ProposalID:   row.ID,
				Position:     item.Position,
				Title:        item.Title,
				Description:  item.Description,
				Priority:     ptrToText(item.Priority),
				Labels:       item.Labels,
				AssigneeType: ptrToText(item.AssigneeType),
				AssigneeID:   item.AssigneeID,
			}); err != nil {
				slog.Warn("issue proposal item persist failed", "task_id", uuidToString(task.ID), "error", err)
				return
			}
			itemCount++
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		slog.Warn("issue proposals persist failed: commit", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	sessionID := uuidToString(task.ChatSessionID)
	h.publishChat(protocol.EventChatIssueProposalsUpdated, workspaceID, "daemon", "", sessionID, map[string]any{
		"chat_session_id": sessionID,
		"task_id":         uuidToString(task.ID),
		"count":           len(prepared),
		"item_count":      itemCount,
	})
}

func (h *Handler) processStructuredTaskOutputs(r *http.Request, task db.AgentTaskQueue, manifest *TaskOutputMetadataManifestRequest) {
	workspaceID := h.TaskService.ResolveTaskWorkspaceID(r.Context(), task)
	if workspaceID == "" {
		slog.Warn("structured task outputs ignored: task workspace not found", "task_id", uuidToString(task.ID))
		return
	}
	workspaceUUID, err := parseStructuredUUID(workspaceID)
	if err != nil {
		slog.Warn("structured task outputs ignored: invalid workspace", "task_id", uuidToString(task.ID), "workspace_id", workspaceID)
		return
	}
	rows, err := h.replaceTaskOutputMetadata(r, workspaceUUID, task.ID, *manifest)
	if err != nil {
		slog.Warn("structured task outputs ignored", "task_id", uuidToString(task.ID), "error", err)
		return
	}
	chatSessionID := h.chatSessionIDForTaskOutputEvent(r.Context(), workspaceUUID, task)
	h.publishTask(protocol.EventTaskOutputsUpdated, workspaceID, "daemon", "", uuidToString(task.ID), map[string]any{
		"task_id":         uuidToString(task.ID),
		"issue_id":        uuidToString(task.IssueID),
		"chat_session_id": chatSessionID,
		"count":           len(rows),
	})
}

func (h *Handler) assistantChatMessageIDForTask(r *http.Request, task db.AgentTaskQueue) pgtype.UUID {
	row, err := h.Queries.GetAssistantChatMessageByTask(r.Context(), db.GetAssistantChatMessageByTaskParams{
		ChatSessionID: task.ChatSessionID,
		TaskID:        task.ID,
	})
	if err != nil {
		return pgtype.UUID{}
	}
	return row.ID
}

type preparedChatIssueProposal struct {
	Title   string
	Summary *string
	Items   []preparedChatIssueProposalItem
}

type preparedChatIssueProposalItem struct {
	Position     int32
	Title        string
	Description  string
	Priority     *string
	Labels       []byte
	AssigneeType *string
	AssigneeID   pgtype.UUID
}

func (h *Handler) prepareChatIssueProposals(
	r *http.Request,
	workspaceID string,
	proposals []ChatIssueProposalManifestRequest,
) ([]preparedChatIssueProposal, error) {
	out := make([]preparedChatIssueProposal, 0, len(proposals))
	for i, proposal := range proposals {
		title := strings.TrimSpace(proposal.Title)
		if title == "" {
			return nil, fmt.Errorf("proposals[%d].title is required", i)
		}
		title = truncateRunes(title, chatStructuredProposalTitleMaxLen)
		var summary *string
		if proposal.Summary != nil {
			value := truncateRunes(strings.TrimSpace(*proposal.Summary), chatStructuredProposalSummaryMaxLen)
			if value != "" {
				summary = &value
			}
		}
		if len(proposal.Items) == 0 {
			return nil, fmt.Errorf("proposals[%d].items must include at least one item", i)
		}
		items := make([]preparedChatIssueProposalItem, 0, len(proposal.Items))
		for j, item := range proposal.Items {
			prepared, err := h.prepareChatIssueProposalItem(r, workspaceID, i, j, item)
			if err != nil {
				return nil, err
			}
			items = append(items, prepared)
		}
		out = append(out, preparedChatIssueProposal{
			Title:   title,
			Summary: summary,
			Items:   items,
		})
	}
	return out, nil
}

func (h *Handler) prepareChatIssueProposalItem(
	r *http.Request,
	workspaceID string,
	proposalIndex int,
	itemIndex int,
	item ChatIssueProposalItemManifestRequest,
) (preparedChatIssueProposalItem, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].title is required", proposalIndex, itemIndex)
	}
	title = truncateRunes(title, chatStructuredProposalItemTitleMaxLen)
	description := truncateRunes(strings.TrimSpace(item.Description), chatStructuredProposalDescriptionMaxLen)

	var priority *string
	if item.Priority != nil {
		value := strings.TrimSpace(*item.Priority)
		if value != "" {
			if !validChatProposalPriorities[value] {
				return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].priority is invalid", proposalIndex, itemIndex)
			}
			priority = &value
		}
	}

	labels, err := json.Marshal(item.Labels)
	if err != nil {
		return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].labels is invalid", proposalIndex, itemIndex)
	}
	if item.Labels == nil {
		labels = []byte("[]")
	}

	var assigneeType *string
	var assigneeID pgtype.UUID
	if item.AssigneeID != nil && strings.TrimSpace(*item.AssigneeID) != "" {
		parsed, err := parseStructuredUUID(strings.TrimSpace(*item.AssigneeID))
		if err != nil {
			return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].assignee_id is invalid", proposalIndex, itemIndex)
		}
		if item.AssigneeType == nil || strings.TrimSpace(*item.AssigneeType) == "" {
			return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].assignee_type is required with assignee_id", proposalIndex, itemIndex)
		}
		value := strings.TrimSpace(*item.AssigneeType)
		if value != "member" && value != "agent" {
			return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d].assignee_type must be 'member' or 'agent'", proposalIndex, itemIndex)
		}
		status, msg := h.validateAssigneePair(
			r.Context(),
			r,
			workspaceID,
			pgtype.Text{String: value, Valid: true},
			parsed,
		)
		if status != 0 {
			return preparedChatIssueProposalItem{}, fmt.Errorf("proposals[%d].items[%d]: %s", proposalIndex, itemIndex, msg)
		}
		assigneeType = &value
		assigneeID = parsed
	}
	return preparedChatIssueProposalItem{
		Position:     int32(itemIndex),
		Title:        title,
		Description:  description,
		Priority:     priority,
		Labels:       labels,
		AssigneeType: assigneeType,
		AssigneeID:   assigneeID,
	}, nil
}

func parseStructuredUUID(value string) (pgtype.UUID, error) {
	return util.ParseUUID(value)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
