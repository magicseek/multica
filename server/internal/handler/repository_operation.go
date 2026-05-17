package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

var validRepositoryOperationTypes = map[string]bool{
	"create_binding":  true,
	"init_git":        true,
	"publish_remote":  true,
	"refresh_binding": true,
}

var repositoryOperationTypesRequiringDaemon = map[string]bool{
	"create_binding":  true,
	"init_git":        true,
	"publish_remote":  true,
	"refresh_binding": true,
}

var repositoryOperationTypesRequiringBinding = map[string]bool{
	"init_git":        true,
	"publish_remote":  true,
	"refresh_binding": true,
}

type CreateRepositoryOperationRequest struct {
	OperationType   string           `json:"operation_type"`
	TargetDaemonID  *string          `json:"target_daemon_id"`
	TargetRuntimeID *string          `json:"target_runtime_id"`
	BindingID       *string          `json:"binding_id"`
	Request         *json.RawMessage `json:"request"`
}

type CompleteRepositoryOperationRequest struct {
	Result     *json.RawMessage                           `json:"result"`
	Binding    *CompleteRepositoryOperationBinding        `json:"binding"`
	Repository *CompleteRepositoryOperationRepositoryInfo `json:"repository"`
}

type CompleteRepositoryOperationBinding struct {
	RuntimeID    *string          `json:"runtime_id"`
	MachineLabel string           `json:"machine_label"`
	BindingKind  string           `json:"binding_kind"`
	LocalPath    string           `json:"local_path"`
	State        string           `json:"state"`
	Metadata     *json.RawMessage `json:"metadata"`
}

type CompleteRepositoryOperationRepositoryInfo struct {
	RemoteURL     *string          `json:"remote_url"`
	DefaultBranch *string          `json:"default_branch"`
	Metadata      *json.RawMessage `json:"metadata"`
}

type FailRepositoryOperationRequest struct {
	Error  string           `json:"error"`
	Result *json.RawMessage `json:"result"`
}

func normalizeRepositoryOperationType(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
}

func validateRepositoryOperationSource(opType, sourceState string) error {
	switch opType {
	case "init_git":
		if sourceState != "local_dir" && sourceState != "agent_managed" {
			return errors.New("init_git is only allowed for local_dir or agent_managed repositories")
		}
	case "publish_remote":
		if sourceState != "local_git" {
			return errors.New("publish_remote is only allowed for local_git repositories")
		}
	}
	return nil
}

func repositoryOperationObject(raw *json.RawMessage, fieldName string) (map[string]any, error) {
	if raw == nil || len(*raw) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(*raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New(fieldName + " must be a JSON object")
	}
	if containsPrivateRepositoryOperationValue(obj) {
		return nil, errors.New(fieldName + " must not include local paths")
	}
	return obj, nil
}

func repositoryOperationObjectBytes(raw *json.RawMessage, fieldName string) ([]byte, error) {
	obj, err := repositoryOperationObject(raw, fieldName)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}

func repositoryOperationObjectBytesFromMap(obj map[string]any) []byte {
	if obj == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func containsPrivateRepositoryOperationValue(v any) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			if isPrivateRepositoryOperationKey(key) || containsPrivateRepositoryOperationValue(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if containsPrivateRepositoryOperationValue(child) {
				return true
			}
		}
	case string:
		return looksLikeAbsoluteLocalPath(value)
	}
	return false
}

func isPrivateRepositoryOperationKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "localpath", "absolutepath", "workdir", "workdirectory":
		return true
	default:
		return false
	}
}

func looksLikeAbsoluteLocalPath(value string) bool {
	s := strings.TrimSpace(value)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~/") || s == "~" {
		return true
	}
	return len(s) >= 3 &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) &&
		s[1] == ':' &&
		(s[2] == '\\' || s[2] == '/')
}

func sanitizeRepositoryOperationRawForResponse(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return json.RawMessage("{}")
	}
	scrubRepositoryOperationPrivateValues(obj)
	data, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(data)
}

func scrubRepositoryOperationPrivateValues(v any) {
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			if isPrivateRepositoryOperationKey(key) {
				value[key] = "[REDACTED LOCAL PATH]"
				continue
			}
			if s, ok := child.(string); ok && looksLikeAbsoluteLocalPath(s) {
				value[key] = "[REDACTED LOCAL PATH]"
				continue
			}
			scrubRepositoryOperationPrivateValues(child)
		}
	case []any:
		for i, child := range value {
			if s, ok := child.(string); ok && looksLikeAbsoluteLocalPath(s) {
				value[i] = "[REDACTED LOCAL PATH]"
				continue
			}
			scrubRepositoryOperationPrivateValues(child)
		}
	}
}

func sanitizeRepositoryOperationError(raw string) string {
	s := strings.TrimSpace(redact.Text(raw))
	if s == "" {
		return "repository operation failed"
	}
	fields := strings.Fields(s)
	for i, field := range fields {
		trimmed := strings.Trim(field, ".,;:()[]{}\"'")
		if looksLikeAbsoluteLocalPath(trimmed) {
			fields[i] = strings.Replace(field, trimmed, "[REDACTED LOCAL PATH]", 1)
		}
	}
	return strings.Join(fields, " ")
}

func repositoryOperationErrorPtr(errText pgtype.Text) *string {
	if !errText.Valid {
		return nil
	}
	s := sanitizeRepositoryOperationError(errText.String)
	return &s
}

func (h *Handler) CreateRepositoryOperation(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	repo, ok := h.loadRepositoryRow(w, r, workspaceID, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	var req CreateRepositoryOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	operationType := normalizeRepositoryOperationType(chi.URLParam(r, "operationType"))
	if operationType == "" {
		operationType = normalizeRepositoryOperationType(req.OperationType)
	}
	if !validRepositoryOperationTypes[operationType] {
		writeError(w, http.StatusBadRequest, "invalid operation_type")
		return
	}
	if err := validateRepositoryOperationSource(operationType, repo.SourceState); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	requestBytes, err := repositoryOperationObjectBytes(req.Request, "request")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if operationType == "publish_remote" {
		var obj map[string]any
		_ = json.Unmarshal(requestBytes, &obj)
		if rawURL, _ := obj["remote_url"].(string); strings.TrimSpace(rawURL) != "" && !isValidGitRepoURL(strings.TrimSpace(rawURL)) {
			writeError(w, http.StatusBadRequest, "request.remote_url must be a valid http(s) or ssh git URL")
			return
		}
	}

	targetDaemonID := ""
	if req.TargetDaemonID != nil {
		targetDaemonID = strings.TrimSpace(*req.TargetDaemonID)
	}
	var targetRuntimeID pgtype.UUID
	if req.TargetRuntimeID != nil && strings.TrimSpace(*req.TargetRuntimeID) != "" {
		rtUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.TargetRuntimeID), "target_runtime_id")
		if !ok {
			return
		}
		rt, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{
			ID:          rtUUID,
			WorkspaceID: member.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "target runtime not found")
			return
		}
		targetRuntimeID = rtUUID
		if targetDaemonID == "" && rt.DaemonID.Valid {
			targetDaemonID = rt.DaemonID.String
		}
		if targetDaemonID != "" && rt.DaemonID.Valid && rt.DaemonID.String != targetDaemonID {
			writeError(w, http.StatusBadRequest, "target_runtime_id does not belong to target_daemon_id")
			return
		}
	}

	var bindingID pgtype.UUID
	if req.BindingID != nil && strings.TrimSpace(*req.BindingID) != "" {
		bindingUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.BindingID), "binding_id")
		if !ok {
			return
		}
		binding, err := h.Queries.GetRepositoryBindingInWorkspace(r.Context(), db.GetRepositoryBindingInWorkspaceParams{
			ID:           bindingUUID,
			RepositoryID: repo.ID,
			WorkspaceID:  member.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "repository binding not found")
			return
		}
		bindingID = bindingUUID
		if targetDaemonID == "" {
			targetDaemonID = binding.DaemonID
		}
		if targetDaemonID != binding.DaemonID {
			writeError(w, http.StatusBadRequest, "binding_id does not belong to target_daemon_id")
			return
		}
		if targetRuntimeID.Valid && binding.RuntimeID.Valid && targetRuntimeID != binding.RuntimeID {
			writeError(w, http.StatusBadRequest, "binding_id does not belong to target_runtime_id")
			return
		}
		if !targetRuntimeID.Valid && binding.RuntimeID.Valid {
			targetRuntimeID = binding.RuntimeID
		}
	}
	if repositoryOperationTypesRequiringBinding[operationType] && !bindingID.Valid {
		writeError(w, http.StatusBadRequest, "binding_id is required for "+operationType)
		return
	}
	if repositoryOperationTypesRequiringDaemon[operationType] && targetDaemonID == "" {
		writeError(w, http.StatusBadRequest, "target_daemon_id is required for "+operationType)
		return
	}

	op, err := h.Queries.CreateRepositoryOperation(r.Context(), db.CreateRepositoryOperationParams{
		RepositoryID:    repo.ID,
		WorkspaceID:     member.WorkspaceID,
		OperationType:   operationType,
		Status:          "queued",
		RequestedByType: "member",
		Request:         requestBytes,
		RequestedByID:   parseUUID(userID),
		TargetDaemonID:  pgtype.Text{String: targetDaemonID, Valid: targetDaemonID != ""},
		TargetRuntimeID: targetRuntimeID,
		BindingID:       bindingID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create repository operation")
		return
	}
	writeJSON(w, http.StatusCreated, repositoryOperationToResponse(op))
}

func (h *Handler) repositoryOperationDaemonTarget(w http.ResponseWriter, r *http.Request) (pgtype.UUID, string, pgtype.UUID, bool) {
	if runtimeID := strings.TrimSpace(r.URL.Query().Get("runtime_id")); runtimeID != "" {
		runtime, ok := h.requireDaemonRuntimeAccess(w, r, runtimeID)
		if !ok {
			return pgtype.UUID{}, "", pgtype.UUID{}, false
		}
		daemonID := ""
		if runtime.DaemonID.Valid {
			daemonID = runtime.DaemonID.String
		}
		if daemonID == "" {
			writeError(w, http.StatusBadRequest, "runtime has no daemon_id")
			return pgtype.UUID{}, "", pgtype.UUID{}, false
		}
		if ctxDaemonID := middleware.DaemonIDFromContext(r.Context()); ctxDaemonID != "" && ctxDaemonID != daemonID {
			writeError(w, http.StatusNotFound, "not found")
			return pgtype.UUID{}, "", pgtype.UUID{}, false
		}
		return runtime.WorkspaceID, daemonID, runtime.ID, true
	}

	workspaceID := middleware.DaemonWorkspaceIDFromContext(r.Context())
	daemonID := middleware.DaemonIDFromContext(r.Context())
	if workspaceID == "" || daemonID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return pgtype.UUID{}, "", pgtype.UUID{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return pgtype.UUID{}, "", pgtype.UUID{}, false
	}
	return workspaceUUID, daemonID, pgtype.UUID{}, true
}

func (h *Handler) ClaimRepositoryOperation(w http.ResponseWriter, r *http.Request) {
	workspaceID, daemonID, runtimeID, ok := h.repositoryOperationDaemonTarget(w, r)
	if !ok {
		return
	}
	op, err := h.Queries.ClaimRepositoryOperationForDaemon(r.Context(), db.ClaimRepositoryOperationForDaemonParams{
		WorkspaceID:     workspaceID,
		TargetDaemonID:  pgtype.Text{String: daemonID, Valid: true},
		TargetRuntimeID: runtimeID,
	})
	if err != nil {
		if isNotFound(err) {
			writeJSON(w, http.StatusOK, map[string]any{"operation": nil})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to claim repository operation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operation": repositoryOperationToResponse(op)})
}

func (h *Handler) repositoryOperationForDaemon(w http.ResponseWriter, r *http.Request) (db.RepositoryOperation, pgtype.UUID, string, bool) {
	opID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "operationId"), "repository operation id")
	if !ok {
		return db.RepositoryOperation{}, pgtype.UUID{}, "", false
	}
	workspaceID, daemonID, runtimeID, ok := h.repositoryOperationDaemonTarget(w, r)
	if !ok {
		return db.RepositoryOperation{}, pgtype.UUID{}, "", false
	}
	op, err := h.Queries.GetRepositoryOperationForDaemon(r.Context(), db.GetRepositoryOperationForDaemonParams{
		ID:             opID,
		WorkspaceID:    workspaceID,
		TargetDaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository operation not found")
		return db.RepositoryOperation{}, pgtype.UUID{}, "", false
	}
	if runtimeID.Valid && op.TargetRuntimeID.Valid && op.TargetRuntimeID != runtimeID {
		writeError(w, http.StatusNotFound, "repository operation not found")
		return db.RepositoryOperation{}, pgtype.UUID{}, "", false
	}
	return op, workspaceID, daemonID, true
}

func (h *Handler) StartRepositoryOperation(w http.ResponseWriter, r *http.Request) {
	op, workspaceID, daemonID, ok := h.repositoryOperationForDaemon(w, r)
	if !ok {
		return
	}
	if op.Status != "queued" && op.Status != "running" {
		writeError(w, http.StatusConflict, "repository operation is already terminal")
		return
	}
	updated, err := h.Queries.MarkRepositoryOperationRunning(r.Context(), db.MarkRepositoryOperationRunningParams{
		ID:             op.ID,
		WorkspaceID:    workspaceID,
		TargetDaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start repository operation")
		return
	}
	writeJSON(w, http.StatusOK, repositoryOperationToResponse(updated))
}

func decodeRepositoryOperationCompleteRequest(r *http.Request) (CompleteRepositoryOperationRequest, error) {
	var req CompleteRepositoryOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return req, err
	}
	return req, nil
}

func (h *Handler) CompleteRepositoryOperation(w http.ResponseWriter, r *http.Request) {
	op, workspaceID, daemonID, ok := h.repositoryOperationForDaemon(w, r)
	if !ok {
		return
	}
	if op.Status != "running" {
		writeError(w, http.StatusConflict, "repository operation is not running")
		return
	}
	req, err := decodeRepositoryOperationCompleteRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resultObj, err := repositoryOperationObject(req.Result, "result")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	op, err = qtx.GetRepositoryOperationForDaemon(r.Context(), db.GetRepositoryOperationForDaemonParams{
		ID:             op.ID,
		WorkspaceID:    workspaceID,
		TargetDaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository operation not found")
		return
	}
	if op.Status != "running" {
		writeError(w, http.StatusConflict, "repository operation is not running")
		return
	}
	repo, err := qtx.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
		ID:          op.RepositoryID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	var bindingID pgtype.UUID
	bindingID = op.BindingID
	var bindingEvent map[string]any
	var repoEvent *RepositoryResponse
	var publishedEvent map[string]any

	if req.Binding != nil {
		if op.OperationType != "create_binding" {
			writeError(w, http.StatusBadRequest, "binding result is only valid for create_binding operations")
			return
		}
		created, err := h.createRepositoryBindingFromOperation(r, qtx, repo, op, daemonID, req.Binding)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bindingID = created.ID
		resultObj["binding_id"] = uuidToString(created.ID)
		resultObj["binding_kind"] = created.BindingKind
		resultObj["binding_state"] = created.State
		resp := repositoryBindingToResponse(created, "", "")
		bindingEvent = map[string]any{
			"repository_id": uuidToString(repo.ID),
			"binding": map[string]any{
				"id":                 resp.ID,
				"repository_id":      resp.RepositoryID,
				"workspace_id":       resp.WorkspaceID,
				"owner_user_id":      resp.OwnerUserID,
				"daemon_id":          resp.DaemonID,
				"runtime_id":         resp.RuntimeID,
				"machine_label":      resp.MachineLabel,
				"binding_kind":       resp.BindingKind,
				"state":              resp.State,
				"last_seen_at":       resp.LastSeenAt,
				"metadata":           json.RawMessage("{}"),
				"created_at":         resp.CreatedAt,
				"updated_at":         resp.UpdatedAt,
				"local_path_visible": false,
			},
		}
	}

	if op.OperationType == "init_git" || op.OperationType == "publish_remote" || (op.OperationType == "create_binding" && repo.Status == "initializing") {
		updated, err := h.applyRepositoryOperationCompletion(r, qtx, repo, op.OperationType, req.Repository, resultObj)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if updated.ID.Valid {
			resp := repositoryToResponse(updated)
			repoEvent = &resp
			if op.OperationType == "publish_remote" {
				publishedEvent = map[string]any{
					"repository_id":         resp.ID,
					"workspace_id":          resp.WorkspaceID,
					"name":                  resp.Name,
					"source_state":          resp.SourceState,
					"remote_url":            resp.RemoteURL,
					"default_branch":        resp.DefaultBranch,
					"published_by":          map[string]any{"type": "daemon", "id": daemonID},
					"previous_source_state": repo.SourceState,
				}
			}
		}
	}

	completed, err := qtx.CompleteRepositoryOperation(r.Context(), db.CompleteRepositoryOperationParams{
		ID:             op.ID,
		WorkspaceID:    workspaceID,
		TargetDaemonID: pgtype.Text{String: daemonID, Valid: true},
		Result:         repositoryOperationObjectBytesFromMap(resultObj),
		BindingID:      bindingID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "failed to complete repository operation")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit repository operation")
		return
	}

	workspaceIDString := uuidToString(workspaceID)
	if bindingEvent != nil {
		h.publish(protocol.EventRepositoryBindingUpdated, workspaceIDString, "daemon", daemonID, bindingEvent)
	}
	if repoEvent != nil {
		h.publish(protocol.EventRepositoryUpdated, workspaceIDString, "daemon", daemonID, map[string]any{"repository": *repoEvent})
	}
	if publishedEvent != nil {
		h.publish(protocol.EventRepositoryPublished, workspaceIDString, "daemon", daemonID, publishedEvent)
	}
	writeJSON(w, http.StatusOK, repositoryOperationToResponse(completed))
}

func (h *Handler) createRepositoryBindingFromOperation(r *http.Request, qtx *db.Queries, repo db.Repository, op db.RepositoryOperation, daemonID string, req *CompleteRepositoryOperationBinding) (db.RepositoryBinding, error) {
	localPath := strings.TrimSpace(req.LocalPath)
	if localPath == "" {
		return db.RepositoryBinding{}, errors.New("binding.local_path is required")
	}
	bindingKind := strings.TrimSpace(req.BindingKind)
	if bindingKind == "" {
		bindingKind = "daemon_workdir"
	}
	if !validRepositoryBindingKinds[bindingKind] {
		return db.RepositoryBinding{}, errors.New("invalid binding.binding_kind")
	}
	state := strings.TrimSpace(req.State)
	if state == "" {
		state = "ready"
	}
	if !validRepositoryBindingStates[state] {
		return db.RepositoryBinding{}, errors.New("invalid binding.state")
	}
	machineLabel := strings.TrimSpace(req.MachineLabel)
	if machineLabel == "" {
		machineLabel = daemonID
	}
	metadata, err := jsonObjectBytes(req.Metadata)
	if err != nil {
		return db.RepositoryBinding{}, errors.New("binding.metadata must be a JSON object")
	}

	runtimeID := op.TargetRuntimeID
	if req.RuntimeID != nil && strings.TrimSpace(*req.RuntimeID) != "" {
		rtUUID, err := parseOperationUUID(strings.TrimSpace(*req.RuntimeID))
		if err != nil {
			return db.RepositoryBinding{}, errors.New("invalid binding.runtime_id")
		}
		rt, err := qtx.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{
			ID:          rtUUID,
			WorkspaceID: repo.WorkspaceID,
		})
		if err != nil {
			return db.RepositoryBinding{}, errors.New("binding.runtime_id not found")
		}
		if rt.DaemonID.Valid && rt.DaemonID.String != daemonID {
			return db.RepositoryBinding{}, errors.New("binding.runtime_id does not belong to target daemon")
		}
		if runtimeID.Valid && runtimeID != rtUUID {
			return db.RepositoryBinding{}, errors.New("binding.runtime_id does not match operation target_runtime_id")
		}
		runtimeID = rtUUID
	}

	var ownerUserID pgtype.UUID
	if op.RequestedByType == "member" {
		ownerUserID = op.RequestedByID
	}
	return qtx.CreateRepositoryBinding(r.Context(), db.CreateRepositoryBindingParams{
		RepositoryID: repo.ID,
		WorkspaceID:  repo.WorkspaceID,
		DaemonID:     daemonID,
		MachineLabel: machineLabel,
		BindingKind:  bindingKind,
		LocalPath:    localPath,
		State:        state,
		Metadata:     metadata,
		OwnerUserID:  ownerUserID,
		RuntimeID:    runtimeID,
		LastSeenAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
}

func parseOperationUUID(raw string) (pgtype.UUID, error) {
	u := pgtype.UUID{}
	err := u.Scan(raw)
	return u, err
}

func (h *Handler) applyRepositoryOperationCompletion(r *http.Request, qtx *db.Queries, repo db.Repository, opType string, result *CompleteRepositoryOperationRepositoryInfo, resultObj map[string]any) (db.Repository, error) {
	params := db.UpdateRepositoryParams{
		ID:            repo.ID,
		WorkspaceID:   repo.WorkspaceID,
		Name:          pgtype.Text{String: repo.Name, Valid: true},
		SourceState:   pgtype.Text{String: repo.SourceState, Valid: true},
		RemoteUrl:     repo.RemoteUrl,
		RemoteKey:     repo.RemoteKey,
		DefaultBranch: repo.DefaultBranch,
		LeadAgentID:   repo.LeadAgentID,
		Status:        pgtype.Text{String: "ready", Valid: true},
		Metadata:      repo.Metadata,
	}
	switch opType {
	case "init_git":
		params.SourceState = pgtype.Text{String: "local_git", Valid: true}
		if result != nil && result.DefaultBranch != nil {
			params.DefaultBranch = nullableTextFromPtr(result.DefaultBranch)
			if params.DefaultBranch.Valid {
				resultObj["default_branch"] = params.DefaultBranch.String
			}
		}
		if result != nil && result.Metadata != nil {
			metadata, err := repositoryOperationObjectBytes(result.Metadata, "repository.metadata")
			if err != nil {
				return db.Repository{}, err
			}
			params.Metadata = metadata
		}
	case "publish_remote":
		remoteURL := ""
		if result != nil && result.RemoteURL != nil {
			remoteURL = strings.TrimSpace(*result.RemoteURL)
		}
		if remoteURL == "" {
			if rawURL, _ := resultObj["remote_url"].(string); strings.TrimSpace(rawURL) != "" {
				remoteURL = strings.TrimSpace(rawURL)
			}
		}
		if remoteURL == "" {
			return db.Repository{}, errors.New("repository.remote_url is required for publish_remote")
		}
		if !isValidGitRepoURL(remoteURL) {
			return db.Repository{}, errors.New("repository.remote_url must be a valid http(s) or ssh git URL")
		}
		params.SourceState = pgtype.Text{String: "remote_git", Valid: true}
		params.RemoteUrl = pgtype.Text{String: remoteURL, Valid: true}
		params.RemoteKey = pgtype.Text{String: normalizeRepositoryRemoteKey(remoteURL), Valid: true}
		resultObj["remote_url"] = remoteURL
		if result != nil && result.DefaultBranch != nil {
			params.DefaultBranch = nullableTextFromPtr(result.DefaultBranch)
			if params.DefaultBranch.Valid {
				resultObj["default_branch"] = params.DefaultBranch.String
			}
		}
	case "create_binding":
		// Successful create_binding moves an initializing agent-managed repo to ready.
	default:
		return db.Repository{}, nil
	}
	return qtx.UpdateRepository(r.Context(), params)
}

func (h *Handler) FailRepositoryOperation(w http.ResponseWriter, r *http.Request) {
	op, workspaceID, daemonID, ok := h.repositoryOperationForDaemon(w, r)
	if !ok {
		return
	}
	if op.Status != "running" {
		writeError(w, http.StatusConflict, "repository operation is not running")
		return
	}
	var req FailRepositoryOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resultBytes, err := repositoryOperationObjectBytes(req.Result, "result")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	failed, err := h.Queries.FailRepositoryOperation(r.Context(), db.FailRepositoryOperationParams{
		ID:             op.ID,
		WorkspaceID:    workspaceID,
		TargetDaemonID: pgtype.Text{String: daemonID, Valid: true},
		Result:         resultBytes,
		Error:          pgtype.Text{String: sanitizeRepositoryOperationError(req.Error), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusConflict, "failed to fail repository operation")
		return
	}
	writeJSON(w, http.StatusOK, repositoryOperationToResponse(failed))
}
