package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type TaskBundleItemResponse struct {
	ID              string  `json:"id"`
	BundleID        string  `json:"bundle_id"`
	IssueID         string  `json:"issue_id"`
	Position        int32   `json:"position"`
	Status          string  `json:"status"`
	OutputNamespace string  `json:"output_namespace"`
	CheckpointSeq   *int32  `json:"checkpoint_seq,omitempty"`
	Result          any     `json:"result,omitempty"`
	Error           string  `json:"error,omitempty"`
	StartedAt       *string `json:"started_at,omitempty"`
	CompletedAt     *string `json:"completed_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type TaskBundleResponse struct {
	ID                   string                   `json:"id"`
	WorkspaceID          string                   `json:"workspace_id"`
	AgentID              string                   `json:"agent_id"`
	RuntimeID            string                   `json:"runtime_id"`
	Status               string                   `json:"status"`
	ChangesetMode        string                   `json:"changeset_mode"`
	MaxItems             int32                    `json:"max_items"`
	RuntimeBudgetSeconds int32                    `json:"runtime_budget_seconds"`
	RerunOfBundleID      *string                  `json:"rerun_of_bundle_id,omitempty"`
	RerunScope           json.RawMessage          `json:"rerun_scope"`
	CreatedBy            *string                  `json:"created_by,omitempty"`
	CreatedAt            string                   `json:"created_at"`
	UpdatedAt            string                   `json:"updated_at"`
	CompletedAt          *string                  `json:"completed_at,omitempty"`
	Items                []TaskBundleItemResponse `json:"items"`
	Task                 *AgentTaskResponse       `json:"task,omitempty"`
}

type CreateTaskBundleRequest struct {
	AgentID              string   `json:"agent_id"`
	IssueIDs             []string `json:"issue_ids"`
	ChangesetMode        string   `json:"changeset_mode"`
	RuntimeBudgetSeconds int32    `json:"runtime_budget_seconds"`
	RerunOfBundleID      string   `json:"rerun_of_bundle_id"`
	RerunScope           []string `json:"rerun_scope"`
}

type RerunTaskBundleRequest struct {
	IssueIDs []string `json:"issue_ids"`
}

type CheckpointTaskBundleItemRequest struct {
	ItemID        string `json:"item_id"`
	Status        string `json:"status"`
	Result        any    `json:"result"`
	Error         string `json:"error"`
	CheckpointSeq *int32 `json:"checkpoint_seq"`
}

func taskBundleItemToResponse(item db.TaskBundleItem) TaskBundleItemResponse {
	var result any
	if item.Result != nil {
		_ = json.Unmarshal(item.Result, &result)
	}
	var checkpointSeq *int32
	if item.CheckpointSeq.Valid {
		value := item.CheckpointSeq.Int32
		checkpointSeq = &value
	}
	return TaskBundleItemResponse{
		ID:              uuidToString(item.ID),
		BundleID:        uuidToString(item.BundleID),
		IssueID:         uuidToString(item.IssueID),
		Position:        item.Position,
		Status:          item.Status,
		OutputNamespace: item.OutputNamespace,
		CheckpointSeq:   checkpointSeq,
		Result:          result,
		Error:           item.Error.String,
		StartedAt:       timestampToPtr(item.StartedAt),
		CompletedAt:     timestampToPtr(item.CompletedAt),
		CreatedAt:       timestampToString(item.CreatedAt),
		UpdatedAt:       timestampToString(item.UpdatedAt),
	}
}

func taskBundleToResponse(bundle db.TaskBundle, items []db.TaskBundleItem, task *db.AgentTaskQueue) TaskBundleResponse {
	itemResponses := make([]TaskBundleItemResponse, len(items))
	for i, item := range items {
		itemResponses[i] = taskBundleItemToResponse(item)
	}
	var taskResp *AgentTaskResponse
	if task != nil {
		resp := taskToResponse(*task)
		taskResp = &resp
	}
	return TaskBundleResponse{
		ID:                   uuidToString(bundle.ID),
		WorkspaceID:          uuidToString(bundle.WorkspaceID),
		AgentID:              uuidToString(bundle.AgentID),
		RuntimeID:            uuidToString(bundle.RuntimeID),
		Status:               bundle.Status,
		ChangesetMode:        bundle.ChangesetMode,
		MaxItems:             bundle.MaxItems,
		RuntimeBudgetSeconds: bundle.RuntimeBudgetSeconds,
		RerunOfBundleID:      uuidToPtr(bundle.RerunOfBundleID),
		RerunScope:           json.RawMessage(bundle.RerunScope),
		CreatedBy:            uuidToPtr(bundle.CreatedBy),
		CreatedAt:            timestampToString(bundle.CreatedAt),
		UpdatedAt:            timestampToString(bundle.UpdatedAt),
		CompletedAt:          timestampToPtr(bundle.CompletedAt),
		Items:                itemResponses,
		Task:                 taskResp,
	}
}

func (h *Handler) populateTaskBundleResponse(ctx context.Context, resp *AgentTaskResponse, task db.AgentTaskQueue) {
	if !task.TaskBundleID.Valid {
		return
	}
	bundle, err := h.Queries.GetTaskBundle(ctx, task.TaskBundleID)
	if err != nil {
		return
	}
	items, err := h.Queries.ListTaskBundleItems(ctx, bundle.ID)
	if err != nil {
		return
	}
	bundleResp := taskBundleToResponse(bundle, items, &task)
	resp.TaskBundle = &bundleResp
}

func (h *Handler) taskToResponseWithBundle(ctx context.Context, task db.AgentTaskQueue) AgentTaskResponse {
	resp := taskToResponse(task)
	h.populateTaskBundleResponse(ctx, &resp, task)
	return resp
}

func (h *Handler) CreateTaskBundle(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateTaskBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.AgentID) == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}
	wsID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	issueIDs := make([]pgtype.UUID, 0, len(req.IssueIDs))
	for _, rawID := range req.IssueIDs {
		issueID, ok := parseUUIDOrBadRequest(w, rawID, "issue_id")
		if !ok {
			return
		}
		issueIDs = append(issueIDs, issueID)
	}
	var rerunOf pgtype.UUID
	if strings.TrimSpace(req.RerunOfBundleID) != "" {
		rerunOf, ok = parseUUIDOrBadRequest(w, req.RerunOfBundleID, "rerun_of_bundle_id")
		if !ok {
			return
		}
	}
	rerunScope, _ := json.Marshal(req.RerunScope)

	created, err := h.TaskService.CreateTaskBundle(r.Context(), service.CreateTaskBundleInput{
		WorkspaceID:          wsID,
		AgentID:              agentID,
		IssueIDs:             issueIDs,
		ChangesetMode:        req.ChangesetMode,
		RuntimeBudgetSeconds: req.RuntimeBudgetSeconds,
		RerunOfBundleID:      rerunOf,
		RerunScope:           rerunScope,
		CreatedBy:            parseUUID(creatorID),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := taskBundleToResponse(created.Bundle, created.Items, &created.Task)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) RerunTaskBundle(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	bundleID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "bundle_id")
	if !ok {
		return
	}
	wsID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	var req RerunTaskBundleRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	issueIDs := make([]pgtype.UUID, 0, len(req.IssueIDs))
	for _, rawID := range req.IssueIDs {
		issueID, ok := parseUUIDOrBadRequest(w, rawID, "issue_id")
		if !ok {
			return
		}
		issueIDs = append(issueIDs, issueID)
	}

	created, err := h.TaskService.RerunTaskBundle(r.Context(), service.RerunTaskBundleInput{
		WorkspaceID: wsID,
		BundleID:    bundleID,
		IssueIDs:    issueIDs,
		CreatedBy:   parseUUID(creatorID),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := taskBundleToResponse(created.Bundle, created.Items, &created.Task)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) ListTaskBundlesByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}
	bundles, err := h.Queries.ListTaskBundlesByIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task bundles")
		return
	}
	resp := make([]TaskBundleResponse, 0, len(bundles))
	for _, bundle := range bundles {
		items, err := h.Queries.ListTaskBundleItems(r.Context(), bundle.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list task bundle items")
			return
		}
		var task *db.AgentTaskQueue
		if row, err := h.Queries.GetTaskForBundle(r.Context(), bundle.ID); err == nil {
			task = &row
		} else if !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "failed to load task bundle task")
			return
		}
		resp = append(resp, taskBundleToResponse(bundle, items, task))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CheckpointTaskBundleItem(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, ok := h.requireDaemonTaskAccess(w, r, taskID)
	if !ok {
		return
	}
	var req CheckpointTaskBundleItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	itemID, ok := parseUUIDOrBadRequest(w, req.ItemID, "item_id")
	if !ok {
		return
	}
	resultJSON, err := json.Marshal(req.Result)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid result")
		return
	}
	var checkpointSeq pgtype.Int4
	if req.CheckpointSeq != nil {
		checkpointSeq = pgtype.Int4{Int32: *req.CheckpointSeq, Valid: true}
	}
	checkpoint, err := h.TaskService.CheckpointTaskBundleItem(r.Context(), service.CheckpointTaskBundleItemInput{
		TaskID:        task.ID,
		ItemID:        itemID,
		Status:        req.Status,
		Result:        resultJSON,
		Error:         req.Error,
		CheckpointSeq: checkpointSeq,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := taskBundleToResponse(checkpoint.Bundle, checkpoint.Items, &task)
	writeJSON(w, http.StatusOK, resp)
}
