package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WorkflowRunResponse struct {
	ID                   string                              `json:"id"`
	WorkspaceID          string                              `json:"workspace_id"`
	AgentTaskQueueID     string                              `json:"agent_task_queue_id"`
	IssueID              *string                             `json:"issue_id,omitempty"`
	ChatSessionID        *string                             `json:"chat_session_id,omitempty"`
	AutopilotRunID       *string                             `json:"autopilot_run_id,omitempty"`
	WorkflowDefinitionID *string                             `json:"workflow_definition_id,omitempty"`
	WorkflowRevisionID   *string                             `json:"workflow_revision_id,omitempty"`
	TriggerType          string                              `json:"trigger_type"`
	Snapshot             any                                 `json:"snapshot"`
	Status               string                              `json:"status"`
	StartedAt            *string                             `json:"started_at,omitempty"`
	CompletedAt          *string                             `json:"completed_at,omitempty"`
	CancelledAt          *string                             `json:"cancelled_at,omitempty"`
	CreatedAt            string                              `json:"created_at"`
	UpdatedAt            string                              `json:"updated_at"`
	Steps                []WorkflowStepRunResponse           `json:"steps,omitempty"`
	Artifacts            []WorkflowArtifactResponse          `json:"artifacts,omitempty"`
	Reviews              []WorkflowReviewResponse            `json:"reviews,omitempty"`
	QualityGateResults   []WorkflowQualityGateResultResponse `json:"quality_gate_results,omitempty"`
}

type WorkflowStepRunResponse struct {
	ID               string  `json:"id"`
	WorkflowRunID    string  `json:"workflow_run_id"`
	StepDefinitionID string  `json:"step_definition_id"`
	Title            string  `json:"title"`
	OrderIndex       int32   `json:"order_index"`
	Required         bool    `json:"required"`
	Status           string  `json:"status"`
	ExecutionKind    string  `json:"execution_kind"`
	Attempt          int32   `json:"attempt"`
	DependsOnStepIDs any     `json:"depends_on_step_ids"`
	ArtifactInputs   any     `json:"artifact_inputs"`
	Snapshot         any     `json:"snapshot"`
	StartedAt        *string `json:"started_at,omitempty"`
	CompletedAt      *string `json:"completed_at,omitempty"`
	Error            *string `json:"error,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type WorkflowArtifactResponse struct {
	ID                   string  `json:"id"`
	WorkflowRunID        string  `json:"workflow_run_id"`
	WorkflowStepRunID    string  `json:"workflow_step_run_id"`
	LogicalName          string  `json:"logical_name"`
	Version              int32   `json:"version"`
	ContentKind          string  `json:"content_kind"`
	ContentText          *string `json:"content_text,omitempty"`
	ContentJSON          any     `json:"content_json,omitempty"`
	ProducerType         string  `json:"producer_type"`
	ProducerID           *string `json:"producer_id,omitempty"`
	SupersedesArtifactID *string `json:"supersedes_artifact_id,omitempty"`
	CreatedAt            string  `json:"created_at"`
}

type WorkflowReviewResponse struct {
	ID                 string  `json:"id"`
	WorkflowRunID      string  `json:"workflow_run_id"`
	WorkflowStepRunID  *string `json:"workflow_step_run_id,omitempty"`
	WorkflowArtifactID *string `json:"workflow_artifact_id,omitempty"`
	Status             string  `json:"status"`
	ReviewerID         *string `json:"reviewer_id,omitempty"`
	DecisionNotes      *string `json:"decision_notes,omitempty"`
	ReviewedAt         *string `json:"reviewed_at,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type WorkflowQualityGateResultResponse struct {
	ID                 string  `json:"id"`
	WorkflowRunID      string  `json:"workflow_run_id"`
	WorkflowStepRunID  string  `json:"workflow_step_run_id"`
	WorkflowArtifactID *string `json:"workflow_artifact_id,omitempty"`
	Status             string  `json:"status"`
	Blocking           bool    `json:"blocking"`
	ProducerType       string  `json:"producer_type"`
	ProducerID         *string `json:"producer_id,omitempty"`
	ReportText         *string `json:"report_text,omitempty"`
	ReportJSON         any     `json:"report_json,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

type workflowStepMutationRequest struct {
	Reason string `json:"reason"`
}

type workflowArtifactRequest struct {
	LogicalName string          `json:"logical_name"`
	Name        string          `json:"name"`
	ContentKind string          `json:"content_kind"`
	ContentText string          `json:"content_text"`
	ContentJSON json.RawMessage `json:"content_json"`
}

type workflowQualityGateRequest struct {
	ArtifactID string          `json:"artifact_id"`
	Status     string          `json:"status"`
	Blocking   bool            `json:"blocking"`
	ReportText string          `json:"report_text"`
	ReportJSON json.RawMessage `json:"report_json"`
}

type workflowReviewDecisionRequest struct {
	Notes string `json:"notes"`
}

func workflowRunToResponse(
	run db.WorkflowRun,
	steps []db.WorkflowStepRun,
	artifacts []db.WorkflowArtifact,
	reviews []db.WorkflowReview,
	quality []db.WorkflowQualityGateResult,
) WorkflowRunResponse {
	resp := WorkflowRunResponse{
		ID:                   uuidToString(run.ID),
		WorkspaceID:          uuidToString(run.WorkspaceID),
		AgentTaskQueueID:     uuidToString(run.AgentTaskQueueID),
		IssueID:              uuidToPtr(run.IssueID),
		ChatSessionID:        uuidToPtr(run.ChatSessionID),
		AutopilotRunID:       uuidToPtr(run.AutopilotRunID),
		WorkflowDefinitionID: uuidToPtr(run.WorkflowDefinitionID),
		WorkflowRevisionID:   uuidToPtr(run.WorkflowRevisionID),
		TriggerType:          run.TriggerType,
		Snapshot:             decodeWorkflowSchema(run.Snapshot),
		Status:               run.Status,
		StartedAt:            timestampToPtr(run.StartedAt),
		CompletedAt:          timestampToPtr(run.CompletedAt),
		CancelledAt:          timestampToPtr(run.CancelledAt),
		CreatedAt:            timestampToString(run.CreatedAt),
		UpdatedAt:            timestampToString(run.UpdatedAt),
	}
	if steps != nil {
		resp.Steps = make([]WorkflowStepRunResponse, len(steps))
		for i, step := range steps {
			resp.Steps[i] = workflowStepRunToResponseForRun(run, step)
		}
	}
	if artifacts != nil {
		resp.Artifacts = make([]WorkflowArtifactResponse, len(artifacts))
		for i, artifact := range artifacts {
			resp.Artifacts[i] = workflowArtifactToResponse(artifact)
		}
	}
	if reviews != nil {
		resp.Reviews = make([]WorkflowReviewResponse, len(reviews))
		for i, review := range reviews {
			resp.Reviews[i] = workflowReviewToResponse(review)
		}
	}
	if quality != nil {
		resp.QualityGateResults = make([]WorkflowQualityGateResultResponse, len(quality))
		for i, result := range quality {
			resp.QualityGateResults[i] = workflowQualityGateResultToResponse(result)
		}
	}
	return resp
}

func workflowStepRunToResponse(step db.WorkflowStepRun) WorkflowStepRunResponse {
	return WorkflowStepRunResponse{
		ID:               uuidToString(step.ID),
		WorkflowRunID:    uuidToString(step.WorkflowRunID),
		StepDefinitionID: step.StepDefinitionID,
		Title:            step.Title,
		OrderIndex:       step.OrderIndex,
		Required:         step.Required,
		Status:           step.Status,
		ExecutionKind:    step.ExecutionKind,
		Attempt:          step.Attempt,
		DependsOnStepIDs: decodeWorkflowSchema(step.DependsOnStepIds),
		ArtifactInputs:   decodeWorkflowSchema(step.ArtifactInputs),
		Snapshot:         decodeWorkflowSchema(step.Snapshot),
		StartedAt:        timestampToPtr(step.StartedAt),
		CompletedAt:      timestampToPtr(step.CompletedAt),
		Error:            textToPtr(step.Error),
		CreatedAt:        timestampToString(step.CreatedAt),
		UpdatedAt:        timestampToString(step.UpdatedAt),
	}
}

func workflowStepRunToResponseForRun(run db.WorkflowRun, step db.WorkflowStepRun) WorkflowStepRunResponse {
	resp := workflowStepRunToResponse(step)
	if run.Status == "completed" && !workflowStepResponseIsSuccessfulTerminal(resp.Status) {
		resp.Status = "completed"
		if resp.CompletedAt == nil {
			resp.CompletedAt = timestampToPtr(run.CompletedAt)
		}
	}
	return resp
}

func workflowStepResponseIsSuccessfulTerminal(status string) bool {
	return status == "completed" || status == "skipped"
}

func workflowArtifactToResponse(artifact db.WorkflowArtifact) WorkflowArtifactResponse {
	return WorkflowArtifactResponse{
		ID:                   uuidToString(artifact.ID),
		WorkflowRunID:        uuidToString(artifact.WorkflowRunID),
		WorkflowStepRunID:    uuidToString(artifact.WorkflowStepRunID),
		LogicalName:          artifact.LogicalName,
		Version:              artifact.Version,
		ContentKind:          artifact.ContentKind,
		ContentText:          textToPtr(artifact.ContentText),
		ContentJSON:          optionalDecodedJSON(artifact.ContentJson),
		ProducerType:         artifact.ProducerType,
		ProducerID:           uuidToPtr(artifact.ProducerID),
		SupersedesArtifactID: uuidToPtr(artifact.SupersedesArtifactID),
		CreatedAt:            timestampToString(artifact.CreatedAt),
	}
}

func workflowReviewToResponse(review db.WorkflowReview) WorkflowReviewResponse {
	return WorkflowReviewResponse{
		ID:                 uuidToString(review.ID),
		WorkflowRunID:      uuidToString(review.WorkflowRunID),
		WorkflowStepRunID:  uuidToPtr(review.WorkflowStepRunID),
		WorkflowArtifactID: uuidToPtr(review.WorkflowArtifactID),
		Status:             review.Status,
		ReviewerID:         uuidToPtr(review.ReviewerID),
		DecisionNotes:      textToPtr(review.DecisionNotes),
		ReviewedAt:         timestampToPtr(review.ReviewedAt),
		CreatedAt:          timestampToString(review.CreatedAt),
		UpdatedAt:          timestampToString(review.UpdatedAt),
	}
}

func workflowQualityGateResultToResponse(result db.WorkflowQualityGateResult) WorkflowQualityGateResultResponse {
	return WorkflowQualityGateResultResponse{
		ID:                 uuidToString(result.ID),
		WorkflowRunID:      uuidToString(result.WorkflowRunID),
		WorkflowStepRunID:  uuidToString(result.WorkflowStepRunID),
		WorkflowArtifactID: uuidToPtr(result.WorkflowArtifactID),
		Status:             result.Status,
		Blocking:           result.Blocking,
		ProducerType:       result.ProducerType,
		ProducerID:         uuidToPtr(result.ProducerID),
		ReportText:         textToPtr(result.ReportText),
		ReportJSON:         optionalDecodedJSON(result.ReportJson),
		CreatedAt:          timestampToString(result.CreatedAt),
	}
}

func optionalDecodedJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return decodeWorkflowSchema(raw)
}

func (h *Handler) ListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	if issueID := strings.TrimSpace(r.URL.Query().Get("issue_id")); issueID != "" {
		issue, ok := h.loadIssueForUser(w, r, issueID)
		if !ok {
			return
		}
		runs, err := h.Queries.ListWorkflowRunsByIssue(r.Context(), issue.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list workflow runs")
			return
		}
		resp := make([]WorkflowRunResponse, len(runs))
		for i, run := range runs {
			resp[i] = workflowRunToResponse(run, nil, nil, nil, nil)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if taskID := strings.TrimSpace(r.URL.Query().Get("task_id")); taskID != "" {
		taskUUID, ok := parseUUIDOrBadRequest(w, taskID, "task_id")
		if !ok {
			return
		}
		run, err := h.Queries.GetWorkflowRunByTask(r.Context(), taskUUID)
		if err != nil {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		if _, ok := h.requireWorkspaceMember(w, r, uuidToString(run.WorkspaceID), "workflow run not found"); !ok {
			return
		}
		writeJSON(w, http.StatusOK, []WorkflowRunResponse{workflowRunToResponse(run, nil, nil, nil, nil)})
		return
	}

	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	if chatSessionID := strings.TrimSpace(r.URL.Query().Get("chat_session_id")); chatSessionID != "" {
		chatUUID, ok := parseUUIDOrBadRequest(w, chatSessionID, "chat_session_id")
		if !ok {
			return
		}
		runs, err := h.Queries.ListWorkflowRunsByChatSession(r.Context(), db.ListWorkflowRunsByChatSessionParams{
			ChatSessionID: chatUUID,
			WorkspaceID:   wsUUID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list workflow runs")
			return
		}
		resp := make([]WorkflowRunResponse, len(runs))
		for i, run := range runs {
			resp[i] = workflowRunToResponse(run, nil, nil, nil, nil)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if autopilotRunID := strings.TrimSpace(r.URL.Query().Get("autopilot_run_id")); autopilotRunID != "" {
		runUUID, ok := parseUUIDOrBadRequest(w, autopilotRunID, "autopilot_run_id")
		if !ok {
			return
		}
		runs, err := h.Queries.ListWorkflowRunsByAutopilotRun(r.Context(), db.ListWorkflowRunsByAutopilotRunParams{
			AutopilotRunID: runUUID,
			WorkspaceID:    wsUUID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list workflow runs")
			return
		}
		resp := make([]WorkflowRunResponse, len(runs))
		for i, run := range runs {
			resp[i] = workflowRunToResponse(run, nil, nil, nil, nil)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	writeError(w, http.StatusBadRequest, "issue_id, task_id, chat_session_id, or autopilot_run_id is required")
}

func (h *Handler) GetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.workflowRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	resp, ok := h.workflowRunDetailResponse(w, r, run)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CancelWorkflowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.workflowRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if _, err := h.TaskService.CancelTask(r.Context(), run.AgentTaskQueueID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.Queries.GetWorkflowRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow run")
		return
	}
	resp, ok := h.workflowRunDetailResponse(w, r, updated)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) RerunWorkflowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.workflowRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if !run.IssueID.Valid {
		writeError(w, http.StatusBadRequest, "workflow run is not linked to an issue")
		return
	}
	task, err := h.TaskService.RerunIssue(r.Context(), run.IssueID, pgtype.UUID{})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, taskToResponse(*task))
}

func (h *Handler) GetWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(step))
}

func (h *Handler) StartWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	updated, err := h.TaskService.StartWorkflowStepRun(r.Context(), step.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) CompleteWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	updated, err := h.TaskService.CompleteWorkflowStepRun(r.Context(), step.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) CompleteManualWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	updated, err := h.TaskService.CompleteManualWorkflowStepRun(r.Context(), step.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) FailWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req workflowStepMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.TaskService.FailWorkflowStepRun(r.Context(), step.ID, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) PauseWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req workflowStepMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.TaskService.PauseWorkflowStepRun(r.Context(), step.ID, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) RetryWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	updated, err := h.TaskService.RetryWorkflowStepRun(r.Context(), step.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflowStepRunToResponse(updated))
}

func (h *Handler) SkipWorkflowStepRun(w http.ResponseWriter, r *http.Request) {
	step, _, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	updated, err := h.TaskService.SkipWorkflowStepRun(r.Context(), step.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowStepRunToResponse(updated))
}

func (h *Handler) CreateWorkflowArtifact(w http.ResponseWriter, r *http.Request) {
	step, run, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req workflowArtifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.LogicalName)
	if name == "" {
		name = strings.TrimSpace(req.Name)
	}
	actorType, actorID := h.resolveActor(r, userID, uuidToString(run.WorkspaceID))
	producerID, ok := parseUUIDOrBadRequest(w, actorID, "producer id")
	if !ok {
		return
	}
	artifact, err := h.TaskService.SaveWorkflowArtifact(r.Context(), step.ID, name, req.ContentKind, req.ContentText, req.ContentJSON, actorType, producerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflowArtifactToResponse(artifact))
}

func (h *Handler) ReportWorkflowQualityGate(w http.ResponseWriter, r *http.Request) {
	step, run, ok := h.workflowStepRunWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req workflowQualityGateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var artifactID pgtype.UUID
	if strings.TrimSpace(req.ArtifactID) != "" {
		var parsed bool
		artifactID, parsed = parseUUIDOrBadRequest(w, req.ArtifactID, "artifact_id")
		if !parsed {
			return
		}
	}
	actorType, actorID := h.resolveActor(r, userID, uuidToString(run.WorkspaceID))
	producerID, ok := parseUUIDOrBadRequest(w, actorID, "producer id")
	if !ok {
		return
	}
	result, err := h.TaskService.ReportWorkflowQualityGate(r.Context(), step.ID, artifactID, req.Status, req.Blocking, req.ReportText, req.ReportJSON, actorType, producerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflowQualityGateResultToResponse(result))
}

func (h *Handler) ApproveWorkflowReview(w http.ResponseWriter, r *http.Request) {
	h.decideWorkflowReview(w, r, "approved")
}

func (h *Handler) RejectWorkflowReview(w http.ResponseWriter, r *http.Request) {
	h.decideWorkflowReview(w, r, "rejected")
}

func (h *Handler) decideWorkflowReview(w http.ResponseWriter, r *http.Request, status string) {
	review, _, ok := h.workflowReviewWithAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	reviewerID, ok := parseUUIDOrBadRequest(w, userID, "reviewer id")
	if !ok {
		return
	}
	var req workflowReviewDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.TaskService.DecideWorkflowReview(r.Context(), review.ID, reviewerID, status, req.Notes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowReviewToResponse(updated))
}

func (h *Handler) workflowRunWithAccess(w http.ResponseWriter, r *http.Request, id string) (db.WorkflowRun, bool) {
	runID, ok := parseUUIDOrBadRequest(w, id, "workflow run id")
	if !ok {
		return db.WorkflowRun{}, false
	}
	run, err := h.Queries.GetWorkflowRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load workflow run")
		}
		return db.WorkflowRun{}, false
	}
	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(run.WorkspaceID), "workflow run not found"); !ok {
		return db.WorkflowRun{}, false
	}
	return run, true
}

func (h *Handler) workflowStepRunWithAccess(w http.ResponseWriter, r *http.Request, id string) (db.WorkflowStepRun, db.WorkflowRun, bool) {
	stepID, ok := parseUUIDOrBadRequest(w, id, "workflow step run id")
	if !ok {
		return db.WorkflowStepRun{}, db.WorkflowRun{}, false
	}
	step, err := h.Queries.GetWorkflowStepRun(r.Context(), stepID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow step run not found")
		return db.WorkflowStepRun{}, db.WorkflowRun{}, false
	}
	run, err := h.Queries.GetWorkflowRun(r.Context(), step.WorkflowRunID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow run not found")
		return db.WorkflowStepRun{}, db.WorkflowRun{}, false
	}
	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(run.WorkspaceID), "workflow step run not found"); !ok {
		return db.WorkflowStepRun{}, db.WorkflowRun{}, false
	}
	return step, run, true
}

func (h *Handler) workflowReviewWithAccess(w http.ResponseWriter, r *http.Request, id string) (db.WorkflowReview, db.WorkflowRun, bool) {
	reviewID, ok := parseUUIDOrBadRequest(w, id, "workflow review id")
	if !ok {
		return db.WorkflowReview{}, db.WorkflowRun{}, false
	}
	review, err := h.Queries.GetWorkflowReview(r.Context(), reviewID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow review not found")
		return db.WorkflowReview{}, db.WorkflowRun{}, false
	}
	run, err := h.Queries.GetWorkflowRun(r.Context(), review.WorkflowRunID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow run not found")
		return db.WorkflowReview{}, db.WorkflowRun{}, false
	}
	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(run.WorkspaceID), "workflow review not found"); !ok {
		return db.WorkflowReview{}, db.WorkflowRun{}, false
	}
	return review, run, true
}

func (h *Handler) workflowRunDetailResponse(w http.ResponseWriter, r *http.Request, run db.WorkflowRun) (WorkflowRunResponse, bool) {
	steps, err := h.Queries.ListWorkflowStepRunsByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow steps")
		return WorkflowRunResponse{}, false
	}
	artifacts, err := h.Queries.ListWorkflowArtifactsByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow artifacts")
		return WorkflowRunResponse{}, false
	}
	reviews, err := h.Queries.ListWorkflowReviewsByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow reviews")
		return WorkflowRunResponse{}, false
	}
	quality, err := h.Queries.ListWorkflowQualityGateResultsByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow quality gate results")
		return WorkflowRunResponse{}, false
	}
	return workflowRunToResponse(run, steps, artifacts, reviews, quality), true
}
