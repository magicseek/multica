package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/connectors"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ConnectorActionRequest struct {
	ProviderID        string          `json:"provider_id"`
	Capability        string          `json:"capability"`
	ProjectResourceID string          `json:"project_resource_id"`
	Input             json.RawMessage `json:"input"`
}

type ConnectorActionResponse struct {
	ProviderID        string `json:"provider_id"`
	Capability        string `json:"capability"`
	ProjectResourceID string `json:"project_resource_id"`
	Result            any    `json:"result"`
}

type connectorActionAuth struct {
	workspaceID pgtype.UUID
	taskID      pgtype.UUID
	agentID     pgtype.UUID
	delegatedID pgtype.UUID
	resource    db.ProjectResource
	credential  db.ConnectorCredential
	secret      string
	provider    connectors.ProviderDefinition
	settings    map[string]any
}

var semanticTaskBranchRE = regexp.MustCompile(`^(feat|fix|refactor|docs|test|chore|perf)/[A-Za-z0-9._/-]+$`)

func (h *Handler) RunConnectorAction(w http.ResponseWriter, r *http.Request) {
	var req ConnectorActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ProviderID = strings.TrimSpace(req.ProviderID)
	req.Capability = strings.TrimSpace(req.Capability)
	req.ProjectResourceID = strings.TrimSpace(req.ProjectResourceID)
	authz, ok := h.authorizeConnectorAction(w, r, req)
	if !ok {
		return
	}

	result, err := h.dispatchConnectorAction(r, authz, req)
	if err != nil {
		if connectors.IsAuthError(err) {
			_, _ = h.Queries.MarkConnectorCredentialInvalid(r.Context(), db.MarkConnectorCredentialInvalidParams{
				WorkspaceID: authz.workspaceID,
				ProviderID:  authz.provider.ID,
				OwnerUserID: authz.delegatedID,
			})
			h.auditConnectorAction(r, authz, req, "failed", "upstream authentication failed")
			writeError(w, http.StatusUnauthorized, "connector credential was rejected by upstream")
			return
		}
		status, reason := connectorActionHTTPError(err)
		h.auditConnectorAction(r, authz, req, "rejected", reason)
		writeError(w, status, reason)
		return
	}
	h.auditConnectorAction(r, authz, req, "succeeded", "")
	writeJSON(w, http.StatusOK, ConnectorActionResponse{
		ProviderID:        authz.provider.ID,
		Capability:        req.Capability,
		ProjectResourceID: uuidToString(authz.resource.ID),
		Result:            result,
	})
}

func (h *Handler) authorizeConnectorAction(w http.ResponseWriter, r *http.Request, req ConnectorActionRequest) (connectorActionAuth, bool) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return connectorActionAuth{}, false
	}
	if h.cfg.ConnectorRegistry == nil || h.cfg.ConnectorClients == nil || h.cfg.ConnectorVault == nil {
		writeError(w, http.StatusNotFound, "connector provider not found")
		return connectorActionAuth{}, false
	}
	provider, ok := h.connectorProvider(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, "connector provider not found")
		return connectorActionAuth{}, false
	}
	if !providerHasCapability(provider, req.Capability) {
		writeError(w, http.StatusBadRequest, "connector capability is not supported")
		return connectorActionAuth{}, false
	}
	workspaceEnabled, ok := h.workspaceConnectorEnabled(r, workspaceID, provider, w)
	if !ok {
		return connectorActionAuth{}, false
	}
	if !workspaceEnabled {
		writeError(w, http.StatusForbidden, "connector provider is disabled for this workspace")
		return connectorActionAuth{}, false
	}
	settings, ok := h.workspaceConnectorSettings(r, workspaceID, provider, w)
	if !ok {
		return connectorActionAuth{}, false
	}

	agentID, ok := parseUUIDOrBadRequest(w, r.Header.Get("X-Agent-ID"), "agent id")
	if !ok {
		return connectorActionAuth{}, false
	}
	taskID, ok := parseUUIDOrBadRequest(w, r.Header.Get("X-Task-ID"), "task id")
	if !ok {
		return connectorActionAuth{}, false
	}
	agent, err := h.Queries.GetAgent(r.Context(), agentID)
	if err != nil || uuidToString(agent.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "agent is not in this workspace")
		return connectorActionAuth{}, false
	}
	task, err := h.Queries.GetAgentTask(r.Context(), taskID)
	if err != nil || uuidToString(task.AgentID) != uuidToString(agentID) {
		writeError(w, http.StatusForbidden, "task is not assigned to this agent")
		return connectorActionAuth{}, false
	}
	if task.Status != "running" && task.Status != "dispatched" {
		writeError(w, http.StatusConflict, "connector commands require an active task")
		return connectorActionAuth{}, false
	}
	if !task.ConnectorDelegatedUserID.Valid {
		writeError(w, http.StatusForbidden, "connector delegated user is missing for this task")
		return connectorActionAuth{}, false
	}

	resourceID, ok := parseUUIDOrBadRequest(w, req.ProjectResourceID, "project_resource_id")
	if !ok {
		return connectorActionAuth{}, false
	}
	resource, err := h.Queries.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{
		ID: resourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "project resource not found")
		return connectorActionAuth{}, false
	}
	if !providerSupportsResource(provider, resource.ResourceType) {
		writeError(w, http.StatusBadRequest, "project resource does not match connector provider")
		return connectorActionAuth{}, false
	}
	taskProjectID, err := h.projectIDForConnectorTask(r, task)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve task project")
		return connectorActionAuth{}, false
	}
	if !taskProjectID.Valid || uuidToString(taskProjectID) != uuidToString(resource.ProjectID) {
		writeError(w, http.StatusForbidden, "project resource is not attached to this task project")
		return connectorActionAuth{}, false
	}

	credential, err := h.Queries.GetConnectorCredential(r.Context(), db.GetConnectorCredentialParams{
		WorkspaceID: workspaceID,
		ProviderID:  provider.ID,
		OwnerUserID: task.ConnectorDelegatedUserID,
	})
	if err != nil || credential.Status != "valid" {
		writeError(w, http.StatusForbidden, "delegated user has no valid connector credential")
		return connectorActionAuth{}, false
	}
	secret, err := h.cfg.ConnectorVault.Decrypt(connectors.EncryptedSecret{
		Ciphertext: credential.EncryptedSecret,
		Nonce:      credential.SecretNonce,
		KeyID:      credential.KeyID,
	}, connectorCredentialAssociatedData(workspaceID, provider.ID, task.ConnectorDelegatedUserID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt connector credential")
		return connectorActionAuth{}, false
	}

	authz := connectorActionAuth{
		workspaceID: workspaceID,
		taskID:      taskID,
		agentID:     agentID,
		delegatedID: task.ConnectorDelegatedUserID,
		resource:    resource,
		credential:  credential,
		secret:      secret,
		provider:    provider,
		settings:    settings,
	}
	if capabilityIsWrite(provider, req.Capability) && !connectorWriteEnabled(authz) {
		writeError(w, http.StatusForbidden, "connector write policy does not allow this action")
		return connectorActionAuth{}, false
	}
	return authz, true
}

func (h *Handler) projectIDForConnectorTask(r *http.Request, task db.AgentTaskQueue) (pgtype.UUID, error) {
	if task.IssueID.Valid {
		issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
		if err != nil {
			return pgtype.UUID{}, err
		}
		return issue.ProjectID, nil
	}
	if task.ChatSessionID.Valid {
		session, err := h.Queries.GetChatSession(r.Context(), task.ChatSessionID)
		if err != nil {
			return pgtype.UUID{}, err
		}
		return session.ProjectID, nil
	}
	return pgtype.UUID{}, nil
}

func (h *Handler) workspaceConnectorEnabled(r *http.Request, workspaceID pgtype.UUID, provider connectors.ProviderDefinition, w http.ResponseWriter) (bool, bool) {
	row, err := h.Queries.GetWorkspaceConnector(r.Context(), db.GetWorkspaceConnectorParams{WorkspaceID: workspaceID, ProviderID: provider.ID})
	if err == nil {
		return row.Enabled, true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return true, true
	}
	writeError(w, http.StatusInternalServerError, "failed to load workspace connector")
	return false, false
}

func (h *Handler) workspaceConnectorSettings(r *http.Request, workspaceID pgtype.UUID, provider connectors.ProviderDefinition, w http.ResponseWriter) (map[string]any, bool) {
	settings := defaultWorkspaceConnectorSettings(provider)
	row, err := h.Queries.GetWorkspaceConnector(r.Context(), db.GetWorkspaceConnectorParams{WorkspaceID: workspaceID, ProviderID: provider.ID})
	if err == nil {
		if len(row.Settings) > 0 {
			_ = json.Unmarshal(row.Settings, &settings)
		}
		return settings, true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return settings, true
	}
	writeError(w, http.StatusInternalServerError, "failed to load workspace connector")
	return nil, false
}

func (h *Handler) dispatchConnectorAction(r *http.Request, authz connectorActionAuth, req ConnectorActionRequest) (any, error) {
	switch authz.provider.ID {
	case connectors.ProviderRingCentralGitLab:
		return h.dispatchGitLabConnectorAction(r, authz, req)
	case connectors.ProviderRingCentralJira:
		return h.dispatchJiraConnectorAction(r, authz, req)
	case connectors.ProviderRingCentralWiki:
		return h.dispatchWikiConnectorAction(r, authz, req)
	default:
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
}

func (h *Handler) dispatchGitLabConnectorAction(r *http.Request, authz connectorActionAuth, req ConnectorActionRequest) (any, error) {
	client, ok := h.connectorClientForSettings(authz.provider, authz.settings)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	gitlab, ok := client.(connectors.GitLabProvider)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	var ref ringCentralGitLabRepoRef
	if err := json.Unmarshal(authz.resource.ResourceRef, &ref); err != nil {
		return nil, connectorActionError{status: http.StatusBadRequest, reason: "invalid gitlab resource"}
	}
	input := map[string]any{}
	_ = json.Unmarshal(req.Input, &input)
	projectID := ref.ProjectID
	switch req.Capability {
	case "branch.list":
		return gitlab.ListBranches(r.Context(), authz.secret, connectors.GitLabListBranchesRequest{
			ProjectID: projectID,
			Search:    asString(input["search"]),
		})
	case "merge_request.list":
		return gitlab.ListMergeRequests(r.Context(), authz.secret, connectors.GitLabListMergeRequestsRequest{
			ProjectID: projectID,
			State:     asString(input["state"]),
		})
	case "branch.create":
		branch := strings.TrimSpace(asString(input["branch"]))
		sourceRef := strings.TrimSpace(asString(input["ref"]))
		if sourceRef == "" {
			sourceRef = ref.DefaultBranch
		}
		if err := validateGitLabWriteBranch(branch, ref.DefaultBranch); err != nil {
			return nil, err
		}
		if sourceRef == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "ref is required"}
		}
		return gitlab.CreateBranch(r.Context(), authz.secret, connectors.GitLabCreateBranchRequest{ProjectID: projectID, Branch: branch, Ref: sourceRef})
	case "commit.create":
		var payload struct {
			Branch        string                          `json:"branch"`
			CommitMessage string                          `json:"commit_message"`
			Actions       []connectors.GitLabCommitAction `json:"actions"`
		}
		if err := json.Unmarshal(req.Input, &payload); err != nil {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "invalid commit input"}
		}
		payload.Branch = strings.TrimSpace(payload.Branch)
		payload.CommitMessage = strings.TrimSpace(payload.CommitMessage)
		if err := validateGitLabWriteBranch(payload.Branch, ref.DefaultBranch); err != nil {
			return nil, err
		}
		if payload.CommitMessage == "" || len(payload.Actions) == 0 {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "commit_message and actions are required"}
		}
		branch, err := gitlab.GetBranch(r.Context(), authz.secret, connectors.GitLabGetBranchRequest{ProjectID: projectID, Branch: payload.Branch})
		if err == nil && (branch.Default || branch.Protected) {
			return nil, connectorActionError{status: http.StatusForbidden, reason: "cannot commit to default or protected branches"}
		}
		if err != nil && !errors.Is(err, connectors.ErrNotFound) {
			return nil, err
		}
		return gitlab.CreateCommit(r.Context(), authz.secret, connectors.GitLabCreateCommitRequest{
			ProjectID:     projectID,
			Branch:        payload.Branch,
			CommitMessage: payload.CommitMessage,
			Actions:       payload.Actions,
		})
	case "merge_request.create":
		var payload struct {
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
			Title        string `json:"title"`
			Description  string `json:"description"`
		}
		if err := json.Unmarshal(req.Input, &payload); err != nil {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "invalid merge request input"}
		}
		payload.SourceBranch = strings.TrimSpace(payload.SourceBranch)
		payload.TargetBranch = strings.TrimSpace(payload.TargetBranch)
		payload.Title = strings.TrimSpace(payload.Title)
		payload.Description = strings.TrimSpace(payload.Description)
		if payload.TargetBranch == "" {
			payload.TargetBranch = ref.DefaultBranch
		}
		if err := validateGitLabWriteBranch(payload.SourceBranch, ref.DefaultBranch); err != nil {
			return nil, err
		}
		if payload.TargetBranch == "" || payload.Title == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "target_branch and title are required"}
		}
		if payload.SourceBranch == payload.TargetBranch {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "source_branch and target_branch must differ"}
		}
		return gitlab.CreateMergeRequest(r.Context(), authz.secret, connectors.GitLabCreateMergeRequestRequest{
			ProjectID:    projectID,
			SourceBranch: payload.SourceBranch,
			TargetBranch: payload.TargetBranch,
			Title:        payload.Title,
			Description:  payload.Description,
			RemoveSource: false,
		})
	default:
		return nil, connectorActionError{status: http.StatusBadRequest, reason: "connector capability is not supported"}
	}
}

func (h *Handler) dispatchJiraConnectorAction(r *http.Request, authz connectorActionAuth, req ConnectorActionRequest) (any, error) {
	client, ok := h.connectorClientForSettings(authz.provider, authz.settings)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	jira, ok := client.(connectors.JiraProvider)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	input := map[string]any{}
	_ = json.Unmarshal(req.Input, &input)
	switch req.Capability {
	case "project.list":
		return jira.ListProjects(r.Context(), authz.secret)
	case "issue.search":
		jql := strings.TrimSpace(asString(input["jql"]))
		maxResults := int(asFloat(input["max_results"]))
		if authz.resource.ResourceType == connectors.ResourceRingCentralJiraProject {
			var ref ringCentralJiraProjectRef
			_ = json.Unmarshal(authz.resource.ResourceRef, &ref)
			jql = scopedJiraJQL(ref.ProjectKey, jql)
		}
		if jql == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "jql is required"}
		}
		return jira.SearchIssues(r.Context(), authz.secret, connectors.JiraSearchIssuesRequest{JQL: jql, MaxResults: maxResults})
	case "issue.read":
		key := strings.TrimSpace(asString(input["issue_key"]))
		if authz.resource.ResourceType == connectors.ResourceRingCentralJiraIssue {
			var ref ringCentralJiraIssueRef
			_ = json.Unmarshal(authz.resource.ResourceRef, &ref)
			key = ref.IssueKey
		}
		if key == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "issue_key is required"}
		}
		return jira.GetIssue(r.Context(), authz.secret, key)
	default:
		return nil, connectorActionError{status: http.StatusBadRequest, reason: "connector capability is not supported"}
	}
}

func (h *Handler) dispatchWikiConnectorAction(r *http.Request, authz connectorActionAuth, req ConnectorActionRequest) (any, error) {
	client, ok := h.connectorClientForSettings(authz.provider, authz.settings)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	wiki, ok := client.(connectors.WikiProvider)
	if !ok {
		return nil, connectorActionError{status: http.StatusNotFound, reason: "connector provider not found"}
	}
	input := map[string]any{}
	_ = json.Unmarshal(req.Input, &input)
	switch req.Capability {
	case "space.search":
		query := strings.TrimSpace(asString(input["query"]))
		if authz.resource.ResourceType == connectors.ResourceRingCentralWikiSpace {
			var ref ringCentralWikiSpaceRef
			_ = json.Unmarshal(authz.resource.ResourceRef, &ref)
			query = ref.SpaceKey
		}
		return wiki.SearchSpaces(r.Context(), authz.secret, connectors.WikiSearchSpacesRequest{Query: query, Limit: int(asFloat(input["limit"]))})
	case "page.search":
		cql := strings.TrimSpace(asString(input["cql"]))
		if authz.resource.ResourceType == connectors.ResourceRingCentralWikiSpace {
			var ref ringCentralWikiSpaceRef
			_ = json.Unmarshal(authz.resource.ResourceRef, &ref)
			cql = scopedWikiCQL(ref.SpaceKey, cql)
		}
		if cql == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "cql is required"}
		}
		return wiki.SearchPages(r.Context(), authz.secret, connectors.WikiSearchPagesRequest{CQL: cql, Limit: int(asFloat(input["limit"]))})
	case "page.read":
		pageID := strings.TrimSpace(asString(input["page_id"]))
		if authz.resource.ResourceType == connectors.ResourceRingCentralWikiPage {
			var ref ringCentralWikiPageRef
			_ = json.Unmarshal(authz.resource.ResourceRef, &ref)
			pageID = ref.PageID
		}
		if pageID == "" {
			return nil, connectorActionError{status: http.StatusBadRequest, reason: "page_id is required"}
		}
		return wiki.GetPage(r.Context(), authz.secret, pageID)
	default:
		return nil, connectorActionError{status: http.StatusBadRequest, reason: "connector capability is not supported"}
	}
}

func validateGitLabWriteBranch(branch, defaultBranch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return connectorActionError{status: http.StatusBadRequest, reason: "branch is required"}
	}
	if branch == strings.TrimSpace(defaultBranch) {
		return connectorActionError{status: http.StatusForbidden, reason: "cannot write to default branch"}
	}
	if strings.Contains(branch, "..") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || !semanticTaskBranchRE.MatchString(branch) {
		return connectorActionError{status: http.StatusForbidden, reason: "branch must use a semantic task prefix"}
	}
	return nil
}

func scopedJiraJQL(projectKey, jql string) string {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return jql
	}
	if strings.TrimSpace(jql) == "" {
		return "project = " + projectKey
	}
	return "project = " + projectKey + " AND (" + jql + ")"
}

func scopedWikiCQL(spaceKey, cql string) string {
	spaceKey = strings.TrimSpace(spaceKey)
	if spaceKey == "" {
		return cql
	}
	if strings.TrimSpace(cql) == "" {
		return `space = "` + spaceKey + `"`
	}
	return `space = "` + spaceKey + `" AND (` + cql + `)`
}

func providerHasCapability(provider connectors.ProviderDefinition, capability string) bool {
	return slicesContainsCapability(provider.Capabilities, capability)
}

func capabilityIsWrite(provider connectors.ProviderDefinition, capability string) bool {
	for _, candidate := range provider.Capabilities {
		if candidate.ID == capability {
			return candidate.Write
		}
	}
	return false
}

func slicesContainsCapability(capabilities []connectors.Capability, capability string) bool {
	for _, candidate := range capabilities {
		if candidate.ID == capability {
			return true
		}
	}
	return false
}

func providerSupportsResource(provider connectors.ProviderDefinition, resourceType string) bool {
	for _, candidate := range provider.ResourceTypes {
		if candidate == resourceType {
			return true
		}
	}
	return false
}

func connectorWriteEnabled(authz connectorActionAuth) bool {
	return authz.provider.ID == connectors.ProviderRingCentralGitLab &&
		asString(authz.settings["remote_write_policy"]) == "merge_request_preparation"
}

func (h *Handler) auditConnectorAction(r *http.Request, authz connectorActionAuth, req ConnectorActionRequest, status, reason string) {
	metadata, _ := json.Marshal(map[string]any{
		"resource_type": authz.resource.ResourceType,
	})
	var reasonText pgtype.Text
	if reason != "" {
		reasonText = pgtype.Text{String: reason, Valid: true}
	}
	_, _ = h.Queries.CreateConnectorCapabilityAuditEvent(r.Context(), db.CreateConnectorCapabilityAuditEventParams{
		WorkspaceID:       authz.workspaceID,
		ProviderID:        authz.provider.ID,
		Capability:        req.Capability,
		TaskID:            authz.taskID,
		AgentID:           authz.agentID,
		DelegatedUserID:   authz.delegatedID,
		ProjectResourceID: authz.resource.ID,
		Status:            status,
		Reason:            reasonText,
		Metadata:          metadata,
	})
}

type connectorActionError struct {
	status int
	reason string
}

func (e connectorActionError) Error() string { return e.reason }

func connectorActionHTTPError(err error) (int, string) {
	var actionErr connectorActionError
	if errors.As(err, &actionErr) {
		return actionErr.status, actionErr.reason
	}
	if errors.Is(err, connectors.ErrNotFound) {
		return http.StatusNotFound, "connector upstream resource not found"
	}
	return http.StatusBadGateway, "connector upstream request failed"
}

func asFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
