package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowdefs"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WorkflowRevisionResponse struct {
	ID                   string                      `json:"id"`
	WorkflowDefinitionID string                      `json:"workflow_definition_id"`
	RevisionNumber       int32                       `json:"revision_number"`
	Status               string                      `json:"status"`
	Schema               any                         `json:"schema"`
	CreatedBy            *string                     `json:"created_by"`
	PublishedAt          *string                     `json:"published_at"`
	DeprecatedAt         *string                     `json:"deprecated_at"`
	CreatedAt            string                      `json:"created_at"`
	UpdatedAt            string                      `json:"updated_at"`
	Validation           *WorkflowValidationResponse `json:"validation,omitempty"`
}

type WorkflowValidationIssueResponse struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type WorkflowValidationResponse struct {
	Publishable bool                              `json:"publishable"`
	Issues      []WorkflowValidationIssueResponse `json:"issues"`
}

type WorkflowDefinitionResponse struct {
	ID                         string                    `json:"id"`
	WorkspaceID                string                    `json:"workspace_id"`
	Name                       string                    `json:"name"`
	Description                string                    `json:"description"`
	Origin                     string                    `json:"origin"`
	SystemKey                  *string                   `json:"system_key"`
	ForkedFromDefinitionID     *string                   `json:"forked_from_definition_id"`
	CurrentPublishedRevisionID *string                   `json:"current_published_revision_id"`
	PublishedRevision          *WorkflowRevisionResponse `json:"published_revision,omitempty"`
	DraftRevision              *WorkflowRevisionResponse `json:"draft_revision,omitempty"`
	CurrentRevision            *WorkflowRevisionResponse `json:"current_revision,omitempty"`
	HasUnpublishedChanges      bool                      `json:"has_unpublished_changes"`
	CreatedBy                  *string                   `json:"created_by"`
	ArchivedAt                 *string                   `json:"archived_at"`
	CreatedAt                  string                    `json:"created_at"`
	UpdatedAt                  string                    `json:"updated_at"`
}

type CreateWorkflowRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

type UpdateWorkflowRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type WorkflowSchemaRequest struct {
	Schema json.RawMessage `json:"schema"`
}

type ForkWorkflowRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WorkflowPreviewRequest struct {
	Schema           json.RawMessage `json:"schema"`
	IssueID          string          `json:"issue_id"`
	TriggerCommentID string          `json:"trigger_comment_id"`
}

func decodeWorkflowSchema(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func workflowRevisionToResponse(rev db.WorkflowRevision) WorkflowRevisionResponse {
	resp := workflowRevisionToResponseWithoutValidation(rev)
	if rev.Status == "draft" {
		resp.Validation = workflowValidationToResponse(workflowdefs.ValidateSchema(rev.Schema))
	}
	return resp
}

func workflowRevisionToResponseWithoutValidation(rev db.WorkflowRevision) WorkflowRevisionResponse {
	return WorkflowRevisionResponse{
		ID:                   uuidToString(rev.ID),
		WorkflowDefinitionID: uuidToString(rev.WorkflowDefinitionID),
		RevisionNumber:       rev.RevisionNumber,
		Status:               rev.Status,
		Schema:               decodeWorkflowSchema(rev.Schema),
		CreatedBy:            uuidToPtr(rev.CreatedBy),
		PublishedAt:          timestampToPtr(rev.PublishedAt),
		DeprecatedAt:         timestampToPtr(rev.DeprecatedAt),
		CreatedAt:            timestampToString(rev.CreatedAt),
		UpdatedAt:            timestampToString(rev.UpdatedAt),
	}
}

func workflowValidationToResponse(result workflowdefs.ValidationResult) *WorkflowValidationResponse {
	issues := make([]WorkflowValidationIssueResponse, len(result.Issues))
	for i, issue := range result.Issues {
		issues[i] = WorkflowValidationIssueResponse{
			Code:     issue.Code,
			Message:  issue.Message,
			Severity: issue.Severity,
		}
	}
	return &WorkflowValidationResponse{
		Publishable: result.Publishable,
		Issues:      issues,
	}
}

func currentWorkflowRevisionResponse(
	defID pgtype.UUID,
	id pgtype.UUID,
	revisionNumber pgtype.Int4,
	status pgtype.Text,
	schema []byte,
	createdBy pgtype.UUID,
	publishedAt, deprecatedAt, createdAt, updatedAt pgtype.Timestamptz,
) *WorkflowRevisionResponse {
	if !id.Valid {
		return nil
	}
	number := int32(0)
	if revisionNumber.Valid {
		number = revisionNumber.Int32
	}
	return &WorkflowRevisionResponse{
		ID:                   uuidToString(id),
		WorkflowDefinitionID: uuidToString(defID),
		RevisionNumber:       number,
		Status:               status.String,
		Schema:               decodeWorkflowSchema(schema),
		CreatedBy:            uuidToPtr(createdBy),
		PublishedAt:          timestampToPtr(publishedAt),
		DeprecatedAt:         timestampToPtr(deprecatedAt),
		CreatedAt:            timestampToString(createdAt),
		UpdatedAt:            timestampToString(updatedAt),
	}
}

func workflowDetailRowToResponse(row db.GetWorkflowDefinitionWithCurrentRevisionRow) WorkflowDefinitionResponse {
	published := currentWorkflowRevisionResponse(
		row.ID,
		row.CurrentRevisionID,
		row.CurrentRevisionNumber,
		row.CurrentRevisionStatus,
		row.CurrentRevisionSchema,
		row.CurrentRevisionCreatedBy,
		row.CurrentRevisionPublishedAt,
		row.CurrentRevisionDeprecatedAt,
		row.CurrentRevisionCreatedAt,
		row.CurrentRevisionUpdatedAt,
	)
	return WorkflowDefinitionResponse{
		ID:                         uuidToString(row.ID),
		WorkspaceID:                uuidToString(row.WorkspaceID),
		Name:                       row.Name,
		Description:                row.Description,
		Origin:                     row.Origin,
		SystemKey:                  textToPtr(row.SystemKey),
		ForkedFromDefinitionID:     uuidToPtr(row.ForkedFromDefinitionID),
		CurrentPublishedRevisionID: uuidToPtr(row.CurrentPublishedRevisionID),
		PublishedRevision:          published,
		CurrentRevision:            published,
		CreatedBy:                  uuidToPtr(row.CreatedBy),
		ArchivedAt:                 timestampToPtr(row.ArchivedAt),
		CreatedAt:                  timestampToString(row.CreatedAt),
		UpdatedAt:                  timestampToString(row.UpdatedAt),
	}
}

func workflowListRowToResponse(row db.ListWorkflowDefinitionsByWorkspaceRow) WorkflowDefinitionResponse {
	published := currentWorkflowRevisionResponse(
		row.ID,
		row.CurrentRevisionID,
		row.CurrentRevisionNumber,
		row.CurrentRevisionStatus,
		row.CurrentRevisionSchema,
		row.CurrentRevisionCreatedBy,
		row.CurrentRevisionPublishedAt,
		row.CurrentRevisionDeprecatedAt,
		row.CurrentRevisionCreatedAt,
		row.CurrentRevisionUpdatedAt,
	)
	return WorkflowDefinitionResponse{
		ID:                         uuidToString(row.ID),
		WorkspaceID:                uuidToString(row.WorkspaceID),
		Name:                       row.Name,
		Description:                row.Description,
		Origin:                     row.Origin,
		SystemKey:                  textToPtr(row.SystemKey),
		ForkedFromDefinitionID:     uuidToPtr(row.ForkedFromDefinitionID),
		CurrentPublishedRevisionID: uuidToPtr(row.CurrentPublishedRevisionID),
		PublishedRevision:          published,
		CurrentRevision:            published,
		CreatedBy:                  uuidToPtr(row.CreatedBy),
		ArchivedAt:                 timestampToPtr(row.ArchivedAt),
		CreatedAt:                  timestampToString(row.CreatedAt),
		UpdatedAt:                  timestampToString(row.UpdatedAt),
	}
}

func workflowApplicabilityRowToResponse(row db.ListWorkflowDefinitionsByApplicabilityRow) WorkflowDefinitionResponse {
	published := &WorkflowRevisionResponse{
		ID:                   uuidToString(row.CurrentRevisionID),
		WorkflowDefinitionID: uuidToString(row.ID),
		RevisionNumber:       row.CurrentRevisionNumber,
		Status:               row.CurrentRevisionStatus,
		Schema:               decodeWorkflowSchema(row.CurrentRevisionSchema),
		CreatedBy:            uuidToPtr(row.CurrentRevisionCreatedBy),
		PublishedAt:          timestampToPtr(row.CurrentRevisionPublishedAt),
		DeprecatedAt:         timestampToPtr(row.CurrentRevisionDeprecatedAt),
		CreatedAt:            timestampToString(row.CurrentRevisionCreatedAt),
		UpdatedAt:            timestampToString(row.CurrentRevisionUpdatedAt),
	}
	return WorkflowDefinitionResponse{
		ID:                         uuidToString(row.ID),
		WorkspaceID:                uuidToString(row.WorkspaceID),
		Name:                       row.Name,
		Description:                row.Description,
		Origin:                     row.Origin,
		SystemKey:                  textToPtr(row.SystemKey),
		ForkedFromDefinitionID:     uuidToPtr(row.ForkedFromDefinitionID),
		CurrentPublishedRevisionID: uuidToPtr(row.CurrentPublishedRevisionID),
		PublishedRevision:          published,
		CurrentRevision:            published,
		CreatedBy:                  uuidToPtr(row.CreatedBy),
		ArchivedAt:                 timestampToPtr(row.ArchivedAt),
		CreatedAt:                  timestampToString(row.CreatedAt),
		UpdatedAt:                  timestampToString(row.UpdatedAt),
	}
}

func (h *Handler) attachWorkflowDraft(r *http.Request, workspaceID pgtype.UUID, resp *WorkflowDefinitionResponse) error {
	draft, err := h.Queries.GetLatestDraftWorkflowRevision(r.Context(), db.GetLatestDraftWorkflowRevisionParams{
		ID:          parseUUID(resp.ID),
		WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	draftResp := workflowRevisionToResponse(draft)
	resp.DraftRevision = &draftResp
	resp.HasUnpublishedChanges = true
	return nil
}

func (h *Handler) ensureWorkflowSeeds(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID) bool {
	if _, err := h.TaskService.EnsureSystemWorkflowDefinitions(r.Context(), workspaceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to seed system workflows")
		return false
	}
	return true
}

func workflowSchemaHasApplicability(raw []byte, applicability string) bool {
	var schema workflowdefs.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return false
	}
	for _, item := range schema.Applicability {
		if item == applicability {
			return true
		}
	}
	return false
}

func (h *Handler) validateWorkflowDefinitionForUse(w http.ResponseWriter, r *http.Request, workspaceID, workflowID pgtype.UUID, applicability, field string) bool {
	if !workflowID.Valid {
		return true
	}
	row, err := h.Queries.GetWorkflowDefinitionWithCurrentRevision(r.Context(), db.GetWorkflowDefinitionWithCurrentRevisionParams{
		ID:          workflowID,
		WorkspaceID: workspaceID,
	})
	if err != nil || row.ArchivedAt.Valid || !row.CurrentRevisionID.Valid {
		writeError(w, http.StatusBadRequest, field+" must reference an active published workflow")
		return false
	}
	if !workflowSchemaHasApplicability(row.CurrentRevisionSchema, applicability) {
		writeError(w, http.StatusBadRequest, field+" is not applicable to "+applicability+" tasks")
		return false
	}
	return true
}

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok || !h.ensureWorkflowSeeds(w, r, wsUUID) {
		return
	}

	applicability := strings.TrimSpace(r.URL.Query().Get("applicability"))
	if applicability != "" {
		rows, err := h.Queries.ListWorkflowDefinitionsByApplicability(r.Context(), db.ListWorkflowDefinitionsByApplicabilityParams{
			WorkspaceID:   wsUUID,
			Applicability: applicability,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list workflows")
			return
		}
		resp := make([]WorkflowDefinitionResponse, len(rows))
		for i, row := range rows {
			resp[i] = workflowApplicabilityRowToResponse(row)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rows, err := h.Queries.ListWorkflowDefinitionsByWorkspace(r.Context(), db.ListWorkflowDefinitionsByWorkspaceParams{
		WorkspaceID:     wsUUID,
		IncludeArchived: r.URL.Query().Get("include_archived") == "true",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}
	resp := make([]WorkflowDefinitionResponse, len(rows))
	for i, row := range rows {
		resp[i] = workflowListRowToResponse(row)
		if err := h.attachWorkflowDraft(r, wsUUID, &resp[i]); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load workflow drafts")
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	row, err := h.Queries.GetWorkflowDefinitionWithCurrentRevision(r.Context(), db.GetWorkflowDefinitionWithCurrentRevisionParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	resp := workflowDetailRowToResponse(row)
	if err := h.attachWorkflowDraft(r, wsUUID, &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow draft")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok || !h.ensureWorkflowSeeds(w, r, wsUUID) {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	schema, _, err := workflowdefs.NormalizeDraftSchema(req.Schema, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	def, err := h.Queries.CreateUserWorkflowDefinition(r.Context(), db.CreateUserWorkflowDefinitionParams{
		WorkspaceID: wsUUID,
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}
	if _, err := h.Queries.CreateWorkflowRevision(r.Context(), db.CreateWorkflowRevisionParams{
		WorkflowDefinitionID: def.ID,
		RevisionNumber:       1,
		Status:               "draft",
		Schema:               schema,
		CreatedBy:            parseUUID(userID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow revision")
		return
	}
	h.writeWorkflowDetail(w, r, http.StatusCreated, def.ID, wsUUID)
}

func (h *Handler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	var req UpdateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	def, err := h.Queries.GetWorkflowDefinitionInWorkspace(r.Context(), db.GetWorkflowDefinitionInWorkspaceParams{ID: idUUID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	if def.Origin == "system_seeded" {
		writeError(w, http.StatusForbidden, "system workflows are read-only; fork before editing")
		return
	}
	params := db.UpdateWorkflowDefinitionMetadataParams{ID: idUUID, WorkspaceID: wsUUID}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: strings.TrimSpace(*req.Description), Valid: true}
	}
	if _, err := h.Queries.UpdateWorkflowDefinitionMetadata(r.Context(), params); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workflow")
		return
	}
	h.writeWorkflowDetail(w, r, http.StatusOK, idUUID, wsUUID)
}

func (h *Handler) CreateWorkflowDraft(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	def, ok := h.editableWorkflow(w, r, idUUID, wsUUID)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	if _, err := h.Queries.GetLatestDraftWorkflowRevision(r.Context(), db.GetLatestDraftWorkflowRevisionParams{ID: idUUID, WorkspaceID: wsUUID}); err == nil {
		h.writeWorkflowDetail(w, r, http.StatusOK, idUUID, wsUUID)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to load workflow draft")
		return
	}
	current, err := h.Queries.GetCurrentWorkflowRevision(r.Context(), db.GetCurrentWorkflowRevisionParams{ID: idUUID, WorkspaceID: wsUUID})
	baseSchema := workflowdefs.DefaultUserSchema(def.Name, def.Description)
	if err == nil {
		baseSchema = current.Schema
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to load workflow revision")
		return
	}
	next, err := h.Queries.GetNextWorkflowRevisionNumber(r.Context(), def.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow draft")
		return
	}
	if _, err := h.Queries.CreateWorkflowRevision(r.Context(), db.CreateWorkflowRevisionParams{
		WorkflowDefinitionID: def.ID,
		RevisionNumber:       next,
		Status:               "draft",
		Schema:               baseSchema,
		CreatedBy:            parseUUID(userID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow draft")
		return
	}
	h.writeWorkflowDetail(w, r, http.StatusOK, idUUID, wsUUID)
}

func (h *Handler) UpdateWorkflowDraft(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	def, ok := h.editableWorkflow(w, r, idUUID, wsUUID)
	if !ok {
		return
	}
	var req WorkflowSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	schema, _, err := workflowdefs.NormalizeDraftSchema(req.Schema, def.Name, def.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	draft, err := h.Queries.GetLatestDraftWorkflowRevision(r.Context(), db.GetLatestDraftWorkflowRevisionParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		next, nextErr := h.Queries.GetNextWorkflowRevisionNumber(r.Context(), def.ID)
		if nextErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to update workflow draft")
			return
		}
		draft, err = h.Queries.CreateWorkflowRevision(r.Context(), db.CreateWorkflowRevisionParams{
			WorkflowDefinitionID: def.ID,
			RevisionNumber:       next,
			Status:               "draft",
			Schema:               schema,
			CreatedBy:            parseUUID(userID),
		})
	} else if err == nil {
		draft, err = h.Queries.UpdateWorkflowRevisionSchema(r.Context(), db.UpdateWorkflowRevisionSchemaParams{
			ID:     draft.ID,
			Schema: schema,
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workflow draft")
		return
	}
	writeJSON(w, http.StatusOK, workflowRevisionToResponse(draft))
}

func (h *Handler) DeleteWorkflowDraft(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	if _, ok := h.editableWorkflow(w, r, idUUID, wsUUID); !ok {
		return
	}
	draft, err := h.Queries.GetLatestDraftWorkflowRevision(r.Context(), db.GetLatestDraftWorkflowRevisionParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow draft")
		return
	}
	if err := h.Queries.DeleteWorkflowDraftRevision(r.Context(), draft.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discard workflow draft")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PublishWorkflow(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	def, ok := h.editableWorkflow(w, r, idUUID, wsUUID)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req WorkflowSchemaRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	var rev db.WorkflowRevision
	var err error
	if len(req.Schema) > 0 {
		schema, normalizeErr := workflowdefs.NormalizeSchema(req.Schema, def.Name, def.Description)
		if normalizeErr != nil {
			writeError(w, http.StatusBadRequest, normalizeErr.Error())
			return
		}
		next, nextErr := h.Queries.GetNextWorkflowRevisionNumber(r.Context(), def.ID)
		if nextErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish workflow")
			return
		}
		rev, err = h.Queries.CreateWorkflowRevision(r.Context(), db.CreateWorkflowRevisionParams{
			WorkflowDefinitionID: def.ID,
			RevisionNumber:       next,
			Status:               "published",
			Schema:               schema,
			CreatedBy:            parseUUID(userID),
		})
	} else {
		rev, err = h.Queries.GetLatestDraftWorkflowRevision(r.Context(), db.GetLatestDraftWorkflowRevisionParams{ID: idUUID, WorkspaceID: wsUUID})
		if err == nil {
			schema, normalizeErr := workflowdefs.NormalizeSchema(rev.Schema, def.Name, def.Description)
			if normalizeErr != nil {
				writeError(w, http.StatusBadRequest, normalizeErr.Error())
				return
			}
			rev, err = h.Queries.UpdateWorkflowRevisionSchema(r.Context(), db.UpdateWorkflowRevisionSchemaParams{
				ID:     rev.ID,
				Schema: schema,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update workflow draft")
				return
			}
			rev, err = h.Queries.PublishWorkflowRevision(r.Context(), rev.ID)
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "no draft or schema to publish")
		return
	}
	if err := h.Queries.DeprecateOtherPublishedWorkflowRevisions(r.Context(), db.DeprecateOtherPublishedWorkflowRevisionsParams{
		WorkflowDefinitionID: def.ID,
		ID:                   rev.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deprecate old workflow revisions")
		return
	}
	name, description := workflowdefs.SchemaMetadata(rev.Schema, def.Name, def.Description)
	if _, err := h.Queries.UpdateWorkflowDefinitionMetadata(r.Context(), db.UpdateWorkflowDefinitionMetadataParams{
		ID:          def.ID,
		WorkspaceID: wsUUID,
		Name:        pgtype.Text{String: name, Valid: true},
		Description: pgtype.Text{String: description, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workflow metadata")
		return
	}
	if _, err := h.Queries.SetWorkflowCurrentPublishedRevision(r.Context(), db.SetWorkflowCurrentPublishedRevisionParams{
		ID:                         def.ID,
		CurrentPublishedRevisionID: rev.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set current workflow revision")
		return
	}
	h.writeWorkflowDetail(w, r, http.StatusOK, idUUID, wsUUID)
}

func (h *Handler) ForkWorkflow(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	row, err := h.Queries.GetWorkflowDefinitionWithCurrentRevision(r.Context(), db.GetWorkflowDefinitionWithCurrentRevisionParams{ID: idUUID, WorkspaceID: wsUUID})
	if err != nil || !row.CurrentRevisionID.Valid {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForkWorkflowRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = row.Name + " copy"
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = row.Description
	}
	def, err := h.Queries.CreateUserWorkflowDefinition(r.Context(), db.CreateUserWorkflowDefinitionParams{
		WorkspaceID:            wsUUID,
		Name:                   name,
		Description:            description,
		CreatedBy:              parseUUID(userID),
		ForkedFromDefinitionID: row.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fork workflow")
		return
	}
	if _, err := h.Queries.CreateWorkflowRevision(r.Context(), db.CreateWorkflowRevisionParams{
		WorkflowDefinitionID: def.ID,
		RevisionNumber:       1,
		Status:               "draft",
		Schema:               row.CurrentRevisionSchema,
		CreatedBy:            parseUUID(userID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fork workflow revision")
		return
	}
	h.writeWorkflowDetail(w, r, http.StatusCreated, def.ID, wsUUID)
}

func (h *Handler) PreviewWorkflow(w http.ResponseWriter, r *http.Request) {
	var req WorkflowPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	schema, _, err := workflowdefs.NormalizeDraftSchema(req.Schema, "Preview workflow", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rendered := workflowdefs.Render(schema, workflowdefs.RenderContext{
		IssueID:          strings.TrimSpace(req.IssueID),
		TriggerCommentID: strings.TrimSpace(req.TriggerCommentID),
	})
	writeJSON(w, http.StatusOK, rendered)
}

func (h *Handler) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	idUUID, wsUUID, ok := h.workflowIDAndWorkspace(w, r)
	if !ok {
		return
	}
	def, err := h.Queries.GetWorkflowDefinitionInWorkspace(r.Context(), db.GetWorkflowDefinitionInWorkspaceParams{ID: idUUID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	if def.Origin == "system_seeded" {
		writeError(w, http.StatusForbidden, "system workflows cannot be archived")
		return
	}
	if !def.CurrentPublishedRevisionID.Valid {
		if err := h.Queries.DeleteDraftOnlyWorkflowDefinition(r.Context(), db.DeleteDraftOnlyWorkflowDefinitionParams{
			ID:          idUUID,
			WorkspaceID: wsUUID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete workflow")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := h.Queries.ArchiveWorkflowDefinition(r.Context(), db.ArchiveWorkflowDefinitionParams{ID: idUUID, WorkspaceID: wsUUID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive workflow")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) workflowIDAndWorkspace(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return idUUID, wsUUID, true
}

func (h *Handler) editableWorkflowWithCurrent(w http.ResponseWriter, r *http.Request, id, workspaceID pgtype.UUID) (db.WorkflowDefinition, db.WorkflowRevision, bool) {
	def, ok := h.editableWorkflow(w, r, id, workspaceID)
	if !ok {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, false
	}
	current, err := h.Queries.GetCurrentWorkflowRevision(r.Context(), db.GetCurrentWorkflowRevisionParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		writeError(w, http.StatusBadRequest, "workflow has no published revision")
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, false
	}
	return def, current, true
}

func (h *Handler) editableWorkflow(w http.ResponseWriter, r *http.Request, id, workspaceID pgtype.UUID) (db.WorkflowDefinition, bool) {
	def, err := h.Queries.GetWorkflowDefinitionInWorkspace(r.Context(), db.GetWorkflowDefinitionInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return db.WorkflowDefinition{}, false
	}
	if def.Origin == "system_seeded" {
		writeError(w, http.StatusForbidden, "system workflows are read-only; fork before editing")
		return db.WorkflowDefinition{}, false
	}
	return def, true
}

func (h *Handler) writeWorkflowDetail(w http.ResponseWriter, r *http.Request, status int, id, workspaceID pgtype.UUID) {
	row, err := h.Queries.GetWorkflowDefinitionWithCurrentRevision(r.Context(), db.GetWorkflowDefinitionWithCurrentRevisionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	resp := workflowDetailRowToResponse(row)
	if err := h.attachWorkflowDraft(r, workspaceID, &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow draft")
		return
	}
	writeJSON(w, status, resp)
}
