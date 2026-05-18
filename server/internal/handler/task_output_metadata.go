package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	maxTaskOutputMetadataItems = 100
	maxTaskOutputPathLength    = 2048
	maxTaskOutputFilenameLen   = 255
)

var validTaskOutputMetadataKinds = map[string]bool{
	"source":   true,
	"doc":      true,
	"artifact": true,
	"log":      true,
	"report":   true,
	"unknown":  true,
}

type TaskOutputMetadataResponse struct {
	ID                    string          `json:"id"`
	WorkspaceID           string          `json:"workspace_id"`
	RepositoryID          *string         `json:"repository_id"`
	TaskID                string          `json:"task_id"`
	RelativePath          string          `json:"relative_path"`
	Filename              string          `json:"filename"`
	Kind                  string          `json:"kind"`
	SizeBytes             *int64          `json:"size_bytes"`
	MimeType              *string         `json:"mime_type"`
	Metadata              json.RawMessage `json:"metadata"`
	CreatedAt             string          `json:"created_at"`
	SourceType            *string         `json:"source_type,omitempty"`
	SourceIssueID         *string         `json:"source_issue_id,omitempty"`
	SourceIssueIdentifier *string         `json:"source_issue_identifier,omitempty"`
	SourceIssueTitle      *string         `json:"source_issue_title,omitempty"`
}

type TaskOutputMetadataManifestRequest struct {
	Outputs []TaskOutputMetadataManifestItem `json:"outputs"`
}

type TaskOutputMetadataManifestItem struct {
	RepositoryID *string          `json:"repository_id"`
	RelativePath string           `json:"relative_path"`
	Filename     *string          `json:"filename"`
	Kind         string           `json:"kind"`
	SizeBytes    *int64           `json:"size_bytes"`
	Size         *int64           `json:"size"`
	MimeType     *string          `json:"mime_type"`
	Metadata     *json.RawMessage `json:"metadata"`
}

func taskOutputMetadataToResponse(row db.TaskOutputMetadatum) TaskOutputMetadataResponse {
	return TaskOutputMetadataResponse{
		ID:           uuidToString(row.ID),
		WorkspaceID:  uuidToString(row.WorkspaceID),
		RepositoryID: uuidToPtr(row.RepositoryID),
		TaskID:       uuidToString(row.TaskID),
		RelativePath: row.RelativePath,
		Filename:     row.Filename,
		Kind:         row.Kind,
		SizeBytes:    int8ToPtr(row.SizeBytes),
		MimeType:     textToPtr(row.MimeType),
		Metadata:     jsonObjectOrEmpty(row.Metadata),
		CreatedAt:    timestampToString(row.CreatedAt),
	}
}

func taskOutputMetadataResponses(rows []db.TaskOutputMetadatum) []TaskOutputMetadataResponse {
	out := make([]TaskOutputMetadataResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskOutputMetadataToResponse(row))
	}
	return out
}

func chatOutputMetadataToResponse(row db.ListChatSessionOutputMetadataRow, issuePrefix string) TaskOutputMetadataResponse {
	resp := TaskOutputMetadataResponse{
		ID:           uuidToString(row.ID),
		WorkspaceID:  uuidToString(row.WorkspaceID),
		RepositoryID: uuidToPtr(row.RepositoryID),
		TaskID:       uuidToString(row.TaskID),
		RelativePath: row.RelativePath,
		Filename:     row.Filename,
		Kind:         row.Kind,
		SizeBytes:    int8ToPtr(row.SizeBytes),
		MimeType:     textToPtr(row.MimeType),
		Metadata:     jsonObjectOrEmpty(row.Metadata),
		CreatedAt:    timestampToString(row.CreatedAt),
		SourceType:   &row.SourceType,
	}
	if row.SourceIssueID.Valid {
		resp.SourceIssueID = uuidToPtr(row.SourceIssueID)
	}
	if row.SourceIssueTitle.Valid {
		resp.SourceIssueTitle = &row.SourceIssueTitle.String
	}
	if row.SourceIssueNumber.Valid {
		identifier := fmt.Sprintf("%s-%d", issuePrefix, row.SourceIssueNumber.Int32)
		resp.SourceIssueIdentifier = &identifier
	}
	return resp
}

func chatOutputMetadataResponses(rows []db.ListChatSessionOutputMetadataRow, issuePrefix string) []TaskOutputMetadataResponse {
	out := make([]TaskOutputMetadataResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, chatOutputMetadataToResponse(row, issuePrefix))
	}
	return out
}

func (h *Handler) taskWorkspaceUUIDForResponse(w http.ResponseWriter, r *http.Request, task db.AgentTaskQueue) (pgtype.UUID, string, bool) {
	workspaceID := h.TaskService.ResolveTaskWorkspaceID(r.Context(), task)
	if workspaceID == "" {
		writeError(w, http.StatusNotFound, "task not found")
		return pgtype.UUID{}, "", false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return pgtype.UUID{}, "", false
	}
	return workspaceUUID, workspaceID, true
}

func (h *Handler) ListTaskOutputMetadata(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
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
	workspaceUUID, workspaceID, ok := h.taskWorkspaceUUIDForResponse(w, r, task)
	if !ok {
		return
	}
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	if task.ChatSessionID.Valid {
		if _, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, uuidToString(task.ChatSessionID)); !ok {
			return
		}
	}
	rows, err := h.Queries.ListTaskOutputMetadata(r.Context(), db.ListTaskOutputMetadataParams{
		TaskID:      taskUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task output metadata")
		return
	}
	resp := taskOutputMetadataResponses(rows)
	writeJSON(w, http.StatusOK, map[string]any{"outputs": resp, "total": len(resp)})
}

func (h *Handler) UploadTaskOutputMetadata(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, ok := h.requireDaemonTaskAccess(w, r, taskID)
	if !ok {
		return
	}
	workspaceUUID, workspaceID, ok := h.taskWorkspaceUUIDForResponse(w, r, task)
	if !ok {
		return
	}

	var req TaskOutputMetadataManifestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Outputs) > maxTaskOutputMetadataItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("outputs is limited to %d items", maxTaskOutputMetadataItems))
		return
	}

	rows, err := h.replaceTaskOutputMetadata(r, workspaceUUID, parseUUID(taskID), req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errTaskOutputMetadataInvalid) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	resp := taskOutputMetadataResponses(rows)
	chatSessionID := h.chatSessionIDForTaskOutputEvent(r.Context(), workspaceUUID, task)
	h.publishTask(protocol.EventTaskOutputsUpdated, workspaceID, "daemon", "", taskID, map[string]any{
		"task_id":         taskID,
		"issue_id":        uuidToString(task.IssueID),
		"chat_session_id": chatSessionID,
		"count":           len(resp),
	})
	writeJSON(w, http.StatusOK, map[string]any{"outputs": resp, "total": len(resp)})
}

func (h *Handler) chatSessionIDForTaskOutputEvent(ctx context.Context, workspaceUUID pgtype.UUID, task db.AgentTaskQueue) string {
	if task.ChatSessionID.Valid {
		return uuidToString(task.ChatSessionID)
	}
	if !task.IssueID.Valid {
		return ""
	}
	issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{
		ID:          task.IssueID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		return ""
	}
	if !issue.OriginType.Valid || issue.OriginType.String != "chat_session" || !issue.OriginID.Valid {
		return ""
	}
	return uuidToString(issue.OriginID)
}

var errTaskOutputMetadataInvalid = errors.New("invalid task output metadata")

func (h *Handler) replaceTaskOutputMetadata(r *http.Request, workspaceUUID, taskUUID pgtype.UUID, req TaskOutputMetadataManifestRequest) ([]db.TaskOutputMetadatum, error) {
	if len(req.Outputs) > maxTaskOutputMetadataItems {
		return nil, fmt.Errorf("%w: outputs is limited to %d items", errTaskOutputMetadataInvalid, maxTaskOutputMetadataItems)
	}

	prepared := make([]db.CreateTaskOutputMetadataParams, 0, len(req.Outputs))
	for i, item := range req.Outputs {
		params, err := h.prepareTaskOutputMetadataParams(r, workspaceUUID, taskUUID, item)
		if err != nil {
			return nil, fmt.Errorf("%w: outputs[%d]: %s", errTaskOutputMetadataInvalid, i, err.Error())
		}
		prepared = append(prepared, params)
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to start task output metadata transaction: %w", err)
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)
	if err := qtx.DeleteTaskOutputMetadataForTask(r.Context(), db.DeleteTaskOutputMetadataForTaskParams{
		TaskID:      taskUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return nil, fmt.Errorf("failed to replace task output metadata: %w", err)
	}

	rows := make([]db.TaskOutputMetadatum, 0, len(prepared))
	for _, params := range prepared {
		row, err := qtx.CreateTaskOutputMetadata(r.Context(), params)
		if err != nil {
			return nil, fmt.Errorf("failed to create task output metadata: %w", err)
		}
		rows = append(rows, row)
	}
	if err := tx.Commit(r.Context()); err != nil {
		return nil, fmt.Errorf("failed to commit task output metadata: %w", err)
	}
	return rows, nil
}

func (h *Handler) prepareTaskOutputMetadataParams(r *http.Request, workspaceID, taskID pgtype.UUID, item TaskOutputMetadataManifestItem) (db.CreateTaskOutputMetadataParams, error) {
	relativePath, err := normalizeTaskOutputRelativePath(item.RelativePath)
	if err != nil {
		return db.CreateTaskOutputMetadataParams{}, err
	}
	filename := strings.TrimSpace(path.Base(relativePath))
	if item.Filename != nil {
		filename = strings.TrimSpace(*item.Filename)
	}
	if filename == "" || filename == "." || filename == ".." {
		return db.CreateTaskOutputMetadataParams{}, errors.New("filename is required")
	}
	if len(filename) > maxTaskOutputFilenameLen {
		return db.CreateTaskOutputMetadataParams{}, fmt.Errorf("filename is limited to %d characters", maxTaskOutputFilenameLen)
	}

	kind := strings.TrimSpace(item.Kind)
	if kind == "" {
		kind = "unknown"
	}
	if !validTaskOutputMetadataKinds[kind] {
		return db.CreateTaskOutputMetadataParams{}, errors.New("invalid kind")
	}

	var repositoryID pgtype.UUID
	if item.RepositoryID != nil && strings.TrimSpace(*item.RepositoryID) != "" {
		repoUUID, err := parseTaskOutputUUID(*item.RepositoryID)
		if err != nil {
			return db.CreateTaskOutputMetadataParams{}, errors.New("invalid repository_id")
		}
		if _, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
			ID:          repoUUID,
			WorkspaceID: workspaceID,
		}); err != nil {
			return db.CreateTaskOutputMetadataParams{}, errors.New("repository not found")
		}
		repositoryID = repoUUID
	}

	var sizeBytes pgtype.Int8
	size := item.SizeBytes
	if size == nil {
		size = item.Size
	}
	if size != nil {
		if *size < 0 {
			return db.CreateTaskOutputMetadataParams{}, errors.New("size_bytes must be non-negative")
		}
		sizeBytes = pgtype.Int8{Int64: *size, Valid: true}
	}

	var mimeType pgtype.Text
	if item.MimeType != nil {
		mime := strings.TrimSpace(*item.MimeType)
		if mime != "" {
			mimeType = pgtype.Text{String: mime, Valid: true}
		}
	}

	metadata, err := repositoryOperationObjectBytes(item.Metadata, "metadata")
	if err != nil {
		return db.CreateTaskOutputMetadataParams{}, err
	}

	return db.CreateTaskOutputMetadataParams{
		WorkspaceID:  workspaceID,
		RepositoryID: repositoryID,
		TaskID:       taskID,
		RelativePath: relativePath,
		Filename:     filename,
		Kind:         kind,
		SizeBytes:    sizeBytes,
		MimeType:     mimeType,
		Metadata:     metadata,
	}, nil
}

func normalizeTaskOutputRelativePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("relative_path is required")
	}
	if len(value) > maxTaskOutputPathLength {
		return "", fmt.Errorf("relative_path is limited to %d characters", maxTaskOutputPathLength)
	}
	if strings.Contains(value, "\x00") {
		return "", errors.New("relative_path contains an invalid character")
	}
	if strings.Contains(value, "\\") {
		return "", errors.New("relative_path must use forward slashes")
	}
	if path.IsAbs(value) || strings.HasPrefix(value, "~/") || value == "~" || looksLikeWindowsDrivePath(value) {
		return "", errors.New("relative_path must be repository-relative")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("relative_path must not traverse parent directories")
	}
	return cleaned, nil
}

func looksLikeWindowsDrivePath(value string) bool {
	return len(value) >= 2 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':'
}

func parseTaskOutputUUID(raw string) (pgtype.UUID, error) {
	var out pgtype.UUID
	err := out.Scan(strings.TrimSpace(raw))
	return out, err
}
