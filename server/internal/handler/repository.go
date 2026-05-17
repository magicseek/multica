package handler

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var validRepositorySourceStates = map[string]bool{
	"remote_git":    true,
	"local_dir":     true,
	"agent_managed": true,
	"local_git":     true,
}

var validRepositoryStatuses = map[string]bool{
	"initializing": true,
	"ready":        true,
	"error":        true,
	"archived":     true,
}

var validRepositoryBindingKinds = map[string]bool{
	"local_dir":          true,
	"daemon_workdir":     true,
	"git_cache_worktree": true,
}

var validRepositoryBindingStates = map[string]bool{
	"ready":        true,
	"missing":      true,
	"inaccessible": true,
	"initializing": true,
	"error":        true,
}

type RepositoryResponse struct {
	ID                  string          `json:"id"`
	WorkspaceID         string          `json:"workspace_id"`
	Name                string          `json:"name"`
	SourceState         string          `json:"source_state"`
	RemoteURL           *string         `json:"remote_url"`
	RemoteKey           *string         `json:"remote_key"`
	DefaultBranch       *string         `json:"default_branch"`
	LeadAgentID         *string         `json:"lead_agent_id"`
	CreatedBy           *string         `json:"created_by"`
	CreatedByAgentID    *string         `json:"created_by_agent_id"`
	Status              string          `json:"status"`
	Metadata            json.RawMessage `json:"metadata"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
	Compatibility       bool            `json:"compatibility,omitempty"`
	CompatibilitySource string          `json:"compatibility_source,omitempty"`
}

type RepositoryBindingResponse struct {
	ID               string          `json:"id"`
	RepositoryID     string          `json:"repository_id"`
	WorkspaceID      string          `json:"workspace_id"`
	OwnerUserID      *string         `json:"owner_user_id"`
	DaemonID         string          `json:"daemon_id"`
	RuntimeID        *string         `json:"runtime_id"`
	MachineLabel     string          `json:"machine_label"`
	BindingKind      string          `json:"binding_kind"`
	LocalPath        *string         `json:"local_path,omitempty"`
	State            string          `json:"state"`
	LastSeenAt       *string         `json:"last_seen_at"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
	LocalPathVisible bool            `json:"local_path_visible"`
}

type ProjectRepositoryResponse struct {
	ProjectID    string             `json:"project_id"`
	RepositoryID string             `json:"repository_id"`
	Role         string             `json:"role"`
	Position     int32              `json:"position"`
	CreatedAt    string             `json:"created_at"`
	Repository   RepositoryResponse `json:"repository"`
}

type RepositoryOperationResponse struct {
	ID              string                              `json:"id"`
	RepositoryID    string                              `json:"repository_id"`
	WorkspaceID     string                              `json:"workspace_id"`
	OperationType   string                              `json:"operation_type"`
	Status          string                              `json:"status"`
	RequestedByType string                              `json:"requested_by_type"`
	RequestedByID   *string                             `json:"requested_by_id"`
	TargetDaemonID  *string                             `json:"target_daemon_id"`
	TargetRuntimeID *string                             `json:"target_runtime_id"`
	BindingID       *string                             `json:"binding_id"`
	Request         json.RawMessage                     `json:"request"`
	Result          json.RawMessage                     `json:"result"`
	Error           *string                             `json:"error"`
	CreatedAt       string                              `json:"created_at"`
	UpdatedAt       string                              `json:"updated_at"`
	CompletedAt     *string                             `json:"completed_at"`
	Binding         *RepositoryOperationBindingResponse `json:"binding,omitempty"`
}

type RepositoryOperationBindingResponse struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	State     string  `json:"state"`
	DaemonID  string  `json:"daemon_id"`
	RuntimeID *string `json:"runtime_id,omitempty"`
	LocalPath string  `json:"local_path"`
}

type CreateRepositoryRequest struct {
	Name          string                          `json:"name"`
	SourceState   string                          `json:"source_state"`
	RemoteURL     *string                         `json:"remote_url"`
	DefaultBranch *string                         `json:"default_branch"`
	LeadAgentID   *string                         `json:"lead_agent_id"`
	Status        string                          `json:"status"`
	Metadata      *json.RawMessage                `json:"metadata"`
	Binding       *CreateRepositoryBindingRequest `json:"binding"`
}

type UpdateRepositoryRequest struct {
	Name          *string          `json:"name"`
	RemoteURL     *string          `json:"remote_url"`
	DefaultBranch *string          `json:"default_branch"`
	LeadAgentID   *string          `json:"lead_agent_id"`
	Status        *string          `json:"status"`
	Metadata      *json.RawMessage `json:"metadata"`
}

type CreateRepositoryBindingRequest struct {
	DaemonID     string           `json:"daemon_id"`
	RuntimeID    *string          `json:"runtime_id"`
	MachineLabel string           `json:"machine_label"`
	BindingKind  string           `json:"binding_kind"`
	LocalPath    string           `json:"local_path"`
	State        string           `json:"state"`
	Metadata     *json.RawMessage `json:"metadata"`
}

type preparedRepositoryBindingRequest struct {
	DaemonID     string
	RuntimeID    pgtype.UUID
	MachineLabel string
	BindingKind  string
	LocalPath    string
	State        string
	Metadata     []byte
}

type SetProjectRepositoriesRequest struct {
	Repositories []SetProjectRepositoryItem `json:"repositories"`
}

type SetProjectRepositoryItem struct {
	RepositoryID string `json:"repository_id"`
	Role         string `json:"role"`
	Position     *int32 `json:"position"`
}

func repositoryToResponse(repo db.Repository) RepositoryResponse {
	return RepositoryResponse{
		ID:               uuidToString(repo.ID),
		WorkspaceID:      uuidToString(repo.WorkspaceID),
		Name:             repo.Name,
		SourceState:      repo.SourceState,
		RemoteURL:        textToPtr(repo.RemoteUrl),
		RemoteKey:        textToPtr(repo.RemoteKey),
		DefaultBranch:    textToPtr(repo.DefaultBranch),
		LeadAgentID:      uuidToPtr(repo.LeadAgentID),
		CreatedBy:        uuidToPtr(repo.CreatedBy),
		CreatedByAgentID: uuidToPtr(repo.CreatedByAgentID),
		Status:           repo.Status,
		Metadata:         jsonObjectOrEmpty(repo.Metadata),
		CreatedAt:        timestampToString(repo.CreatedAt),
		UpdatedAt:        timestampToString(repo.UpdatedAt),
	}
}

func repositoryBindingToResponse(binding db.RepositoryBinding, viewerUserID, viewerDaemonID string) RepositoryBindingResponse {
	localPathVisible := false
	if viewerUserID != "" && uuidToString(binding.OwnerUserID) == viewerUserID {
		localPathVisible = true
	}
	if viewerDaemonID != "" && binding.DaemonID == viewerDaemonID {
		localPathVisible = true
	}
	var localPath *string
	metadata := json.RawMessage("{}")
	if localPathVisible {
		localPath = &binding.LocalPath
		metadata = jsonObjectOrEmpty(binding.Metadata)
	}
	return RepositoryBindingResponse{
		ID:               uuidToString(binding.ID),
		RepositoryID:     uuidToString(binding.RepositoryID),
		WorkspaceID:      uuidToString(binding.WorkspaceID),
		OwnerUserID:      uuidToPtr(binding.OwnerUserID),
		DaemonID:         binding.DaemonID,
		RuntimeID:        uuidToPtr(binding.RuntimeID),
		MachineLabel:     binding.MachineLabel,
		BindingKind:      binding.BindingKind,
		LocalPath:        localPath,
		State:            binding.State,
		LastSeenAt:       timestampToPtr(binding.LastSeenAt),
		Metadata:         metadata,
		CreatedAt:        timestampToString(binding.CreatedAt),
		UpdatedAt:        timestampToString(binding.UpdatedAt),
		LocalPathVisible: localPathVisible,
	}
}

func repositoryOperationToResponse(op db.RepositoryOperation) RepositoryOperationResponse {
	return RepositoryOperationResponse{
		ID:              uuidToString(op.ID),
		RepositoryID:    uuidToString(op.RepositoryID),
		WorkspaceID:     uuidToString(op.WorkspaceID),
		OperationType:   op.OperationType,
		Status:          op.Status,
		RequestedByType: op.RequestedByType,
		RequestedByID:   uuidToPtr(op.RequestedByID),
		TargetDaemonID:  textToPtr(op.TargetDaemonID),
		TargetRuntimeID: uuidToPtr(op.TargetRuntimeID),
		BindingID:       uuidToPtr(op.BindingID),
		Request:         sanitizeRepositoryOperationRawForResponse(op.Request),
		Result:          sanitizeRepositoryOperationRawForResponse(op.Result),
		Error:           repositoryOperationErrorPtr(op.Error),
		CreatedAt:       timestampToString(op.CreatedAt),
		UpdatedAt:       timestampToString(op.UpdatedAt),
		CompletedAt:     timestampToPtr(op.CompletedAt),
	}
}

func repositoryOperationToDaemonResponse(ctx context.Context, q *db.Queries, op db.RepositoryOperation, daemonID string, runtimeID pgtype.UUID) RepositoryOperationResponse {
	resp := repositoryOperationToResponse(op)
	if !op.BindingID.Valid {
		return resp
	}
	binding, err := q.GetRepositoryBindingInWorkspace(ctx, db.GetRepositoryBindingInWorkspaceParams{
		ID:           op.BindingID,
		RepositoryID: op.RepositoryID,
		WorkspaceID:  op.WorkspaceID,
	})
	if err != nil || !repositoryOperationBindingPathVisible(op, binding, daemonID, runtimeID) {
		return resp
	}
	resp.Binding = &RepositoryOperationBindingResponse{
		ID:        uuidToString(binding.ID),
		Kind:      binding.BindingKind,
		State:     binding.State,
		DaemonID:  binding.DaemonID,
		RuntimeID: uuidToPtr(binding.RuntimeID),
		LocalPath: binding.LocalPath,
	}
	return resp
}

func repositoryOperationBindingPathVisible(op db.RepositoryOperation, binding db.RepositoryBinding, daemonID string, runtimeID pgtype.UUID) bool {
	if strings.TrimSpace(daemonID) == "" || binding.DaemonID != daemonID {
		return false
	}
	if !op.TargetDaemonID.Valid || op.TargetDaemonID.String != daemonID {
		return false
	}
	if op.TargetRuntimeID.Valid && (!runtimeID.Valid || op.TargetRuntimeID != runtimeID) {
		return false
	}
	if binding.RuntimeID.Valid && (!runtimeID.Valid || binding.RuntimeID != runtimeID) {
		return false
	}
	return strings.TrimSpace(binding.LocalPath) != ""
}

func jsonObjectOrEmpty(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(raw)
}

func jsonObjectBytes(raw *json.RawMessage) ([]byte, error) {
	if raw == nil || len(*raw) == 0 {
		return []byte("{}"), nil
	}
	var obj map[string]any
	if err := json.Unmarshal(*raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("metadata must be a JSON object")
	}
	return []byte(*raw), nil
}

func nullableTextFromPtr(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func validateRepositorySourceState(state string) bool {
	return validRepositorySourceStates[state]
}

func validateRepositoryStatus(status string) bool {
	return validRepositoryStatuses[status]
}

func normalizeRepositoryRemoteKey(raw string) string {
	s := strings.TrimSpace(strings.TrimRight(raw, "/"))
	if s == "" {
		return ""
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		return strings.ToLower(u.Host) + "/" + normalizeRepositoryPath(u.Path)
	}
	host, repoPath := splitSCPStyleGitRemote(s)
	if host != "" && repoPath != "" {
		return strings.ToLower(host) + "/" + normalizeRepositoryPath(repoPath)
	}
	return normalizeRepositoryPath(s)
}

func splitSCPStyleGitRemote(raw string) (host, repoPath string) {
	s := raw
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	colon := strings.Index(s, ":")
	if colon <= 0 || colon == len(s)-1 {
		return "", ""
	}
	return s[:colon], s[colon+1:]
}

func normalizeRepositoryPath(raw string) string {
	p := strings.TrimSpace(strings.Trim(raw, "/"))
	p = strings.TrimSuffix(p, ".git")
	return strings.ToLower(p)
}

func repositoryNameFromRemoteURL(raw string) string {
	key := normalizeRepositoryRemoteKey(raw)
	if key == "" {
		return "Repository"
	}
	parts := strings.Split(key, "/")
	name := parts[len(parts)-1]
	if name == "" {
		return "Repository"
	}
	return name
}

func compatibilityRepositoryID(workspaceID, remoteKey string) string {
	sum := sha1.Sum([]byte("multica:repository-compat:" + workspaceID + ":" + remoteKey))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func compatibilityRepositoryFromURL(workspaceID, remoteURL, source string) (RepositoryResponse, string, bool) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return RepositoryResponse{}, "", false
	}
	remoteKey := normalizeRepositoryRemoteKey(remoteURL)
	if remoteKey == "" {
		remoteKey = strings.ToLower(remoteURL)
	}
	return RepositoryResponse{
		ID:                  compatibilityRepositoryID(workspaceID, remoteKey),
		WorkspaceID:         workspaceID,
		Name:                repositoryNameFromRemoteURL(remoteURL),
		SourceState:         "remote_git",
		RemoteURL:           &remoteURL,
		RemoteKey:           &remoteKey,
		Status:              "ready",
		Metadata:            json.RawMessage("{}"),
		Compatibility:       true,
		CompatibilitySource: source,
	}, remoteKey, true
}

func (h *Handler) compatibilityRepositories(ctx context.Context, workspaceUUID pgtype.UUID, seen map[string]struct{}) []RepositoryResponse {
	workspaceID := uuidToString(workspaceUUID)
	var out []RepositoryResponse
	addURL := func(rawURL, source string) {
		repo, remoteKey, ok := compatibilityRepositoryFromURL(workspaceID, rawURL, source)
		if !ok {
			return
		}
		if _, exists := seen[remoteKey]; exists {
			return
		}
		seen[remoteKey] = struct{}{}
		out = append(out, repo)
	}

	if ws, err := h.Queries.GetWorkspace(ctx, workspaceUUID); err == nil {
		for _, repo := range parseWorkspaceRepos(ws.Repos) {
			addURL(repo.URL, "workspace.repos")
		}
	}
	if urls, err := h.Queries.ListGithubProjectResourceURLsInWorkspace(ctx, workspaceUUID); err == nil {
		for _, rawURL := range urls {
			addURL(rawURL, "project_resource.github_repo")
		}
	}
	return out
}

func (h *Handler) compatibilityRepositoriesForProject(ctx context.Context, workspaceID string, projectID pgtype.UUID, seen map[string]struct{}) []ProjectRepositoryResponse {
	urls, err := h.Queries.ListGithubProjectResourceURLsForProject(ctx, projectID)
	if err != nil {
		return nil
	}
	out := make([]ProjectRepositoryResponse, 0, len(urls))
	for _, rawURL := range urls {
		repo, remoteKey, ok := compatibilityRepositoryFromURL(workspaceID, rawURL, "project_resource.github_repo")
		if !ok {
			continue
		}
		if _, exists := seen[remoteKey]; exists {
			continue
		}
		seen[remoteKey] = struct{}{}
		position := int32(len(out))
		role := "secondary"
		if len(out) == 0 {
			role = "primary"
		}
		out = append(out, ProjectRepositoryResponse{
			ProjectID:    uuidToString(projectID),
			RepositoryID: repo.ID,
			Role:         role,
			Position:     position,
			Repository:   repo,
		})
	}
	return out
}

func (h *Handler) loadRepositoryRow(w http.ResponseWriter, r *http.Request, workspaceID, repoID string) (db.Repository, bool) {
	repoUUID, ok := parseUUIDOrBadRequest(w, repoID, "repository id")
	if !ok {
		return db.Repository{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.Repository{}, false
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
		ID:          repoUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return db.Repository{}, false
	}
	return repo, true
}

func (h *Handler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	repositories, err := h.Queries.ListRepositories(r.Context(), member.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}
	resp := make([]RepositoryResponse, 0, len(repositories))
	seen := make(map[string]struct{}, len(repositories))
	for _, repo := range repositories {
		item := repositoryToResponse(repo)
		resp = append(resp, item)
		if repo.RemoteKey.Valid && repo.RemoteKey.String != "" {
			seen[repo.RemoteKey.String] = struct{}{}
		}
	}
	resp = append(resp, h.compatibilityRepositories(r.Context(), member.WorkspaceID, seen)...)
	writeJSON(w, http.StatusOK, map[string]any{"repositories": resp, "total": len(resp)})
}

func (h *Handler) GetRepository(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	repoID := chi.URLParam(r, "id")
	repoUUID, ok := parseUUIDOrBadRequest(w, repoID, "repository id")
	if !ok {
		return
	}
	if repo, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
		ID:          repoUUID,
		WorkspaceID: member.WorkspaceID,
	}); err == nil {
		writeJSON(w, http.StatusOK, repositoryToResponse(repo))
		return
	}

	seen := map[string]struct{}{}
	for _, repo := range h.compatibilityRepositories(r.Context(), member.WorkspaceID, seen) {
		if repo.ID == repoID {
			writeJSON(w, http.StatusOK, repo)
			return
		}
	}
	writeError(w, http.StatusNotFound, "repository not found")
}

func (h *Handler) prepareRepositoryBindingRequest(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, userID string, req CreateRepositoryBindingRequest) (preparedRepositoryBindingRequest, bool) {
	req.DaemonID = strings.TrimSpace(req.DaemonID)
	req.MachineLabel = strings.TrimSpace(req.MachineLabel)
	req.BindingKind = strings.TrimSpace(req.BindingKind)
	req.LocalPath = strings.TrimSpace(req.LocalPath)
	req.State = strings.TrimSpace(req.State)
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return preparedRepositoryBindingRequest{}, false
	}
	if req.MachineLabel == "" {
		req.MachineLabel = req.DaemonID
	}
	if req.BindingKind == "" {
		req.BindingKind = "local_dir"
	}
	if !validRepositoryBindingKinds[req.BindingKind] {
		writeError(w, http.StatusBadRequest, "invalid binding_kind")
		return preparedRepositoryBindingRequest{}, false
	}
	if req.LocalPath == "" {
		writeError(w, http.StatusBadRequest, "local_path is required")
		return preparedRepositoryBindingRequest{}, false
	}
	if req.State == "" {
		req.State = "initializing"
	}
	if !validRepositoryBindingStates[req.State] {
		writeError(w, http.StatusBadRequest, "invalid state")
		return preparedRepositoryBindingRequest{}, false
	}
	var runtimeID pgtype.UUID
	if req.RuntimeID != nil && strings.TrimSpace(*req.RuntimeID) != "" {
		rtUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.RuntimeID), "runtime_id")
		if !ok {
			return preparedRepositoryBindingRequest{}, false
		}
		rt, err := h.Queries.GetAgentRuntime(r.Context(), rtUUID)
		if err != nil || uuidToString(rt.WorkspaceID) != uuidToString(workspaceID) {
			writeError(w, http.StatusNotFound, "runtime not found")
			return preparedRepositoryBindingRequest{}, false
		}
		if uuidToString(rt.OwnerID) != userID {
			writeError(w, http.StatusForbidden, "runtime is not owned by the current user")
			return preparedRepositoryBindingRequest{}, false
		}
		if !rt.DaemonID.Valid || strings.TrimSpace(rt.DaemonID.String) == "" {
			writeError(w, http.StatusBadRequest, "runtime has no daemon_id")
			return preparedRepositoryBindingRequest{}, false
		}
		if strings.TrimSpace(rt.DaemonID.String) != req.DaemonID {
			writeError(w, http.StatusBadRequest, "daemon_id must match runtime daemon_id")
			return preparedRepositoryBindingRequest{}, false
		}
		runtimeID = rtUUID
	}
	metadata, err := jsonObjectBytes(req.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, "metadata must be a JSON object")
		return preparedRepositoryBindingRequest{}, false
	}
	return preparedRepositoryBindingRequest{
		DaemonID:     req.DaemonID,
		RuntimeID:    runtimeID,
		MachineLabel: req.MachineLabel,
		BindingKind:  req.BindingKind,
		LocalPath:    req.LocalPath,
		State:        req.State,
		Metadata:     metadata,
	}, true
}

func (h *Handler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.SourceState = strings.TrimSpace(req.SourceState)
	if req.SourceState == "" {
		if req.RemoteURL != nil && strings.TrimSpace(*req.RemoteURL) != "" {
			req.SourceState = "remote_git"
		} else {
			req.SourceState = "agent_managed"
		}
	}
	if !validateRepositorySourceState(req.SourceState) {
		writeError(w, http.StatusBadRequest, "invalid source_state")
		return
	}

	var remoteURL pgtype.Text
	var remoteKey pgtype.Text
	if req.RemoteURL != nil {
		urlValue := strings.TrimSpace(*req.RemoteURL)
		if urlValue != "" {
			if !isValidGitRepoURL(urlValue) {
				writeError(w, http.StatusBadRequest, "remote_url must be a valid http(s) or ssh git URL")
				return
			}
			remoteURL = pgtype.Text{String: urlValue, Valid: true}
			remoteKey = pgtype.Text{String: normalizeRepositoryRemoteKey(urlValue), Valid: true}
			if req.Name == "" {
				req.Name = repositoryNameFromRemoteURL(urlValue)
			}
		}
	}
	if req.SourceState == "remote_git" && !remoteURL.Valid {
		writeError(w, http.StatusBadRequest, "remote_url is required for remote_git repositories")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "ready"
		if req.SourceState == "agent_managed" {
			status = "initializing"
		}
	}
	if !validateRepositoryStatus(status) {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	metadata, err := jsonObjectBytes(req.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, "metadata must be a JSON object")
		return
	}
	var localBinding *preparedRepositoryBindingRequest
	if req.Binding != nil {
		if req.SourceState != "local_dir" {
			writeError(w, http.StatusBadRequest, "binding is only valid for local_dir repositories")
			return
		}
		prepared, ok := h.prepareRepositoryBindingRequest(w, r, member.WorkspaceID, userID, *req.Binding)
		if !ok {
			return
		}
		localBinding = &prepared
	}

	var leadAgentID pgtype.UUID
	var leadAgent db.Agent
	if req.LeadAgentID != nil && strings.TrimSpace(*req.LeadAgentID) != "" {
		leadUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.LeadAgentID), "lead_agent_id")
		if !ok {
			return
		}
		agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
			ID:          leadUUID,
			WorkspaceID: member.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "lead agent not found")
			return
		}
		leadAgent = agent
		leadAgentID = leadUUID
	}

	var createBindingOperation bool
	var targetDaemonID pgtype.Text
	var targetRuntimeID pgtype.UUID
	if req.SourceState == "agent_managed" && leadAgentID.Valid {
		if !leadAgent.RuntimeID.Valid {
			writeError(w, http.StatusBadRequest, "lead agent runtime is required for agent_managed repositories")
			return
		}
		runtime, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{
			ID:          leadAgent.RuntimeID,
			WorkspaceID: member.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "lead agent runtime not found")
			return
		}
		if !runtime.DaemonID.Valid || strings.TrimSpace(runtime.DaemonID.String) == "" {
			writeError(w, http.StatusBadRequest, "lead agent runtime has no daemon_id")
			return
		}
		createBindingOperation = true
		targetDaemonID = pgtype.Text{String: strings.TrimSpace(runtime.DaemonID.String), Valid: true}
		targetRuntimeID = runtime.ID
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start repository transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	repo, err := qtx.CreateRepository(r.Context(), db.CreateRepositoryParams{
		WorkspaceID:   member.WorkspaceID,
		Name:          req.Name,
		SourceState:   req.SourceState,
		Status:        status,
		Metadata:      metadata,
		RemoteUrl:     remoteURL,
		RemoteKey:     remoteKey,
		DefaultBranch: nullableTextFromPtr(req.DefaultBranch),
		LeadAgentID:   leadAgentID,
		CreatedBy:     parseUUID(userID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "repository already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create repository")
		return
	}
	if createBindingOperation {
		if _, err := qtx.CreateRepositoryOperation(r.Context(), db.CreateRepositoryOperationParams{
			RepositoryID:    repo.ID,
			WorkspaceID:     member.WorkspaceID,
			OperationType:   "create_binding",
			Status:          "queued",
			RequestedByType: "member",
			RequestedByID:   parseUUID(userID),
			TargetDaemonID:  targetDaemonID,
			TargetRuntimeID: targetRuntimeID,
			Request:         []byte(`{"reason":"agent_managed_start"}`),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create repository binding operation")
			return
		}
	}
	var createdLocalBinding db.RepositoryBinding
	var hasCreatedLocalBinding bool
	if localBinding != nil {
		binding, err := qtx.CreateRepositoryBinding(r.Context(), db.CreateRepositoryBindingParams{
			RepositoryID: repo.ID,
			WorkspaceID:  member.WorkspaceID,
			DaemonID:     localBinding.DaemonID,
			MachineLabel: localBinding.MachineLabel,
			BindingKind:  localBinding.BindingKind,
			LocalPath:    localBinding.LocalPath,
			State:        localBinding.State,
			Metadata:     localBinding.Metadata,
			OwnerUserID:  parseUUID(userID),
			RuntimeID:    localBinding.RuntimeID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create repository binding")
			return
		}
		createdLocalBinding = binding
		hasCreatedLocalBinding = true
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit repository")
		return
	}
	resp := repositoryToResponse(repo)
	h.publish(protocol.EventRepositoryCreated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{"repository": resp})
	if hasCreatedLocalBinding {
		bindingResp := repositoryBindingToResponse(createdLocalBinding, userID, "")
		h.publish(protocol.EventRepositoryBindingUpdated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{
			"repository_id": uuidToString(repo.ID),
			"binding": map[string]any{
				"id":                 bindingResp.ID,
				"repository_id":      bindingResp.RepositoryID,
				"workspace_id":       bindingResp.WorkspaceID,
				"owner_user_id":      bindingResp.OwnerUserID,
				"daemon_id":          bindingResp.DaemonID,
				"runtime_id":         bindingResp.RuntimeID,
				"machine_label":      bindingResp.MachineLabel,
				"binding_kind":       bindingResp.BindingKind,
				"state":              bindingResp.State,
				"last_seen_at":       bindingResp.LastSeenAt,
				"metadata":           json.RawMessage("{}"),
				"created_at":         bindingResp.CreatedAt,
				"updated_at":         bindingResp.UpdatedAt,
				"local_path_visible": false,
			},
		})
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateRepository(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateRepositoryRequest
	rawFields, err := decodeJSONBodyWithRawFields(r.Body, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateRepositoryParams{
		ID:            repo.ID,
		WorkspaceID:   member.WorkspaceID,
		Name:          pgtype.Text{String: repo.Name, Valid: true},
		SourceState:   pgtype.Text{String: repo.SourceState, Valid: true},
		RemoteUrl:     repo.RemoteUrl,
		RemoteKey:     repo.RemoteKey,
		DefaultBranch: repo.DefaultBranch,
		LeadAgentID:   repo.LeadAgentID,
		Status:        pgtype.Text{String: repo.Status, Valid: true},
		Metadata:      repo.Metadata,
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if _, exists := rawFields["remote_url"]; exists {
		if req.RemoteURL == nil || strings.TrimSpace(*req.RemoteURL) == "" {
			if repo.SourceState == "remote_git" {
				writeError(w, http.StatusBadRequest, "remote_url is required for remote_git repositories")
				return
			}
			params.RemoteUrl = pgtype.Text{}
			params.RemoteKey = pgtype.Text{}
		} else {
			urlValue := strings.TrimSpace(*req.RemoteURL)
			if !isValidGitRepoURL(urlValue) {
				writeError(w, http.StatusBadRequest, "remote_url must be a valid http(s) or ssh git URL")
				return
			}
			params.RemoteUrl = pgtype.Text{String: urlValue, Valid: true}
			params.RemoteKey = pgtype.Text{String: normalizeRepositoryRemoteKey(urlValue), Valid: true}
		}
	}
	if _, exists := rawFields["default_branch"]; exists {
		params.DefaultBranch = nullableTextFromPtr(req.DefaultBranch)
	}
	if _, exists := rawFields["lead_agent_id"]; exists {
		params.LeadAgentID = pgtype.UUID{}
		if req.LeadAgentID != nil && strings.TrimSpace(*req.LeadAgentID) != "" {
			leadUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.LeadAgentID), "lead_agent_id")
			if !ok {
				return
			}
			if _, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
				ID:          leadUUID,
				WorkspaceID: member.WorkspaceID,
			}); err != nil {
				writeError(w, http.StatusNotFound, "lead agent not found")
				return
			}
			params.LeadAgentID = leadUUID
		}
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !validateRepositoryStatus(status) {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if _, exists := rawFields["metadata"]; exists {
		metadata, err := jsonObjectBytes(req.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, "metadata must be a JSON object")
			return
		}
		params.Metadata = metadata
	}

	updated, err := h.Queries.UpdateRepository(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "repository already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update repository")
		return
	}
	resp := repositoryToResponse(updated)
	h.publish(protocol.EventRepositoryUpdated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{"repository": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
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
	archived, err := h.Queries.ArchiveRepository(r.Context(), db.ArchiveRepositoryParams{
		ID:          repo.ID,
		WorkspaceID: member.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive repository")
		return
	}
	h.publish(protocol.EventRepositoryUpdated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{"repository": repositoryToResponse(archived)})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListRepositoryBindings(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	repo, ok := h.loadRepositoryRow(w, r, workspaceID, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	bindings, err := h.Queries.ListRepositoryBindings(r.Context(), db.ListRepositoryBindingsParams{
		RepositoryID: repo.ID,
		WorkspaceID:  member.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repository bindings")
		return
	}
	userID := requestUserID(r)
	resp := make([]RepositoryBindingResponse, len(bindings))
	for i, binding := range bindings {
		resp[i] = repositoryBindingToResponse(binding, userID, "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"bindings": resp, "total": len(resp)})
}

func (h *Handler) CreateRepositoryBinding(w http.ResponseWriter, r *http.Request) {
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

	var req CreateRepositoryBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	prepared, ok := h.prepareRepositoryBindingRequest(w, r, member.WorkspaceID, userID, req)
	if !ok {
		return
	}

	binding, err := h.Queries.CreateRepositoryBinding(r.Context(), db.CreateRepositoryBindingParams{
		RepositoryID: repo.ID,
		WorkspaceID:  member.WorkspaceID,
		DaemonID:     prepared.DaemonID,
		MachineLabel: prepared.MachineLabel,
		BindingKind:  prepared.BindingKind,
		LocalPath:    prepared.LocalPath,
		State:        prepared.State,
		Metadata:     prepared.Metadata,
		OwnerUserID:  parseUUID(userID),
		RuntimeID:    prepared.RuntimeID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create repository binding")
		return
	}
	resp := repositoryBindingToResponse(binding, userID, "")
	h.publish(protocol.EventRepositoryBindingUpdated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{
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
	})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) DeleteRepositoryBinding(w http.ResponseWriter, r *http.Request) {
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
	bindingID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "bindingId"), "binding id")
	if !ok {
		return
	}
	binding, err := h.Queries.GetRepositoryBindingInWorkspace(r.Context(), db.GetRepositoryBindingInWorkspaceParams{
		ID:           bindingID,
		RepositoryID: repo.ID,
		WorkspaceID:  member.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository binding not found")
		return
	}
	if uuidToString(binding.OwnerUserID) != userID && !roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "not your repository binding")
		return
	}
	if err := h.Queries.DeleteRepositoryBinding(r.Context(), db.DeleteRepositoryBindingParams{
		ID:           binding.ID,
		RepositoryID: repo.ID,
		WorkspaceID:  member.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete repository binding")
		return
	}
	h.publish(protocol.EventRepositoryBindingUpdated, uuidToString(member.WorkspaceID), "member", userID, map[string]any{
		"repository_id": uuidToString(repo.ID),
		"binding_id":    uuidToString(binding.ID),
		"deleted":       true,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListProjectRepositories(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	refs, err := h.Queries.ListProjectRepositoryRefs(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project repositories")
		return
	}
	resp := make([]ProjectRepositoryResponse, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		repo, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
			ID:          ref.RepositoryID,
			WorkspaceID: project.WorkspaceID,
		})
		if err != nil {
			continue
		}
		repoResp := repositoryToResponse(repo)
		if repo.RemoteKey.Valid && repo.RemoteKey.String != "" {
			seen[repo.RemoteKey.String] = struct{}{}
		}
		resp = append(resp, ProjectRepositoryResponse{
			ProjectID:    uuidToString(ref.ProjectID),
			RepositoryID: uuidToString(ref.RepositoryID),
			Role:         ref.Role,
			Position:     ref.Position,
			CreatedAt:    timestampToString(ref.CreatedAt),
			Repository:   repoResp,
		})
	}
	resp = append(resp, h.compatibilityRepositoriesForProject(r.Context(), uuidToString(project.WorkspaceID), project.ID, seen)...)
	writeJSON(w, http.StatusOK, map[string]any{"repositories": resp, "total": len(resp)})
}

func (h *Handler) SetProjectRepositories(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req SetProjectRepositoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	seenRepos := map[string]struct{}{}
	primaryCount := 0
	prepared := make([]db.CreateProjectRepositoryRefParams, 0, len(req.Repositories))
	for i, item := range req.Repositories {
		repoID := strings.TrimSpace(item.RepositoryID)
		if repoID == "" {
			writeError(w, http.StatusBadRequest, "repositories[].repository_id is required")
			return
		}
		if _, exists := seenRepos[repoID]; exists {
			writeError(w, http.StatusBadRequest, "duplicate repository_id")
			return
		}
		seenRepos[repoID] = struct{}{}
		repoUUID, ok := parseUUIDOrBadRequest(w, repoID, "repository_id")
		if !ok {
			return
		}
		if _, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{
			ID:          repoUUID,
			WorkspaceID: project.WorkspaceID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = "secondary"
			if i == 0 {
				role = "primary"
			}
		}
		if role != "primary" && role != "secondary" {
			writeError(w, http.StatusBadRequest, "invalid repository role")
			return
		}
		if role == "primary" {
			primaryCount++
		}
		position := int32(i)
		if item.Position != nil {
			position = *item.Position
		}
		prepared = append(prepared, db.CreateProjectRepositoryRefParams{
			ProjectID:    project.ID,
			RepositoryID: repoUUID,
			WorkspaceID:  project.WorkspaceID,
			Role:         role,
			Position:     position,
		})
	}
	if primaryCount > 1 {
		writeError(w, http.StatusBadRequest, "only one primary repository is allowed")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if err := qtx.DeleteProjectRepositoryRefs(r.Context(), project.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project repositories")
		return
	}
	for _, params := range prepared {
		if _, err := qtx.CreateProjectRepositoryRef(r.Context(), params); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update project repositories")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit project repositories")
		return
	}
	h.publish(protocol.EventProjectUpdated, uuidToString(project.WorkspaceID), "member", userID, map[string]any{
		"project_id": uuidToString(project.ID),
	})
	h.ListProjectRepositories(w, r)
}

func (h *Handler) ListRepositoryOperations(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	repo, ok := h.loadRepositoryRow(w, r, workspaceID, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	operations, err := h.Queries.ListRepositoryOperations(r.Context(), db.ListRepositoryOperationsParams{
		RepositoryID: repo.ID,
		WorkspaceID:  member.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repository operations")
		return
	}
	resp := make([]RepositoryOperationResponse, len(operations))
	for i, op := range operations {
		resp[i] = repositoryOperationToResponse(op)
	}
	writeJSON(w, http.StatusOK, map[string]any{"operations": resp, "total": len(resp)})
}
