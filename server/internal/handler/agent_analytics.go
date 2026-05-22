package handler

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	agentAnalyticsScopeProject = "project"
	agentAnalyticsScopeChat    = "chat"

	agentAnalyticsSourceAll    = "all"
	agentAnalyticsSourceIssues = "issues"
	agentAnalyticsSourceChats  = "chats"

	agentAnalyticsSortNewest   = "newest"
	agentAnalyticsSortDuration = "duration_desc"
	agentAnalyticsSortTokens   = "tokens_desc"

	agentAnalyticsDefaultLimit = 50
	agentAnalyticsMaxLimit     = 200
)

type AgentAnalyticsResponse struct {
	Window          AgentAnalyticsWindowResponse   `json:"window"`
	Summary         AgentAnalyticsSummaryResponse  `json:"summary"`
	PreviousSummary *AgentAnalyticsSummaryResponse `json:"previous_summary,omitempty"`
	Daily           []AgentAnalyticsDailyResponse  `json:"daily"`
	Agents          []AgentAnalyticsAgentResponse  `json:"agents"`
	Sources         []AgentAnalyticsSourceResponse `json:"sources,omitempty"`
	Runs            []AgentAnalyticsRunResponse    `json:"runs"`
	Pagination      AgentAnalyticsPagination       `json:"pagination"`
}

type AgentAnalyticsWindowResponse struct {
	Days        *int    `json:"days,omitempty"`
	Since       *string `json:"since,omitempty"`
	Before      *string `json:"before,omitempty"`
	HasPrevious bool    `json:"has_previous"`
}

type AgentAnalyticsSummaryResponse struct {
	TaskCount           int32                              `json:"task_count"`
	CompletedCount      int32                              `json:"completed_count"`
	FailedCount         int32                              `json:"failed_count"`
	CancelledCount      int32                              `json:"cancelled_count"`
	InputTokens         int64                              `json:"input_tokens"`
	OutputTokens        int64                              `json:"output_tokens"`
	CacheReadTokens     int64                              `json:"cache_read_tokens"`
	CacheWriteTokens    int64                              `json:"cache_write_tokens"`
	TotalTokens         int64                              `json:"total_tokens"`
	MedianCompletionMs  int64                              `json:"median_completion_ms"`
	P95CompletionMs     int64                              `json:"p95_completion_ms"`
	PromptCacheReadRate float64                            `json:"prompt_cache_read_rate"`
	PromptBytes         int64                              `json:"prompt_bytes"`
	MedianFirstTextMs   int64                              `json:"median_first_text_ms"`
	ToolUseCount        int64                              `json:"tool_use_count"`
	ToolResultBytes     int64                              `json:"tool_result_bytes"`
	ModelUsage          []AgentAnalyticsModelUsageResponse `json:"model_usage"`
}

type AgentAnalyticsDailyResponse struct {
	Date             string `json:"date"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	TaskCount        int32  `json:"task_count"`
	CompletedCount   int32  `json:"completed_count"`
	FailedCount      int32  `json:"failed_count"`
	CancelledCount   int32  `json:"cancelled_count"`
}

type AgentAnalyticsAgentResponse struct {
	AgentID            string                             `json:"agent_id"`
	AgentName          string                             `json:"agent_name"`
	TaskCount          int32                              `json:"task_count"`
	CompletedCount     int32                              `json:"completed_count"`
	FailedCount        int32                              `json:"failed_count"`
	CancelledCount     int32                              `json:"cancelled_count"`
	InputTokens        int64                              `json:"input_tokens"`
	OutputTokens       int64                              `json:"output_tokens"`
	CacheReadTokens    int64                              `json:"cache_read_tokens"`
	CacheWriteTokens   int64                              `json:"cache_write_tokens"`
	TotalTokens        int64                              `json:"total_tokens"`
	MedianCompletionMs int64                              `json:"median_completion_ms"`
	ModelUsage         []AgentAnalyticsModelUsageResponse `json:"model_usage"`
}

type AgentAnalyticsSourceResponse struct {
	SourceType         string                             `json:"source_type"`
	SourceID           *string                            `json:"source_id,omitempty"`
	SourceTitle        *string                            `json:"source_title,omitempty"`
	IssueIdentifier    *string                            `json:"issue_identifier,omitempty"`
	TaskCount          int32                              `json:"task_count"`
	CompletedCount     int32                              `json:"completed_count"`
	FailedCount        int32                              `json:"failed_count"`
	CancelledCount     int32                              `json:"cancelled_count"`
	InputTokens        int64                              `json:"input_tokens"`
	OutputTokens       int64                              `json:"output_tokens"`
	CacheReadTokens    int64                              `json:"cache_read_tokens"`
	CacheWriteTokens   int64                              `json:"cache_write_tokens"`
	TotalTokens        int64                              `json:"total_tokens"`
	MedianCompletionMs int64                              `json:"median_completion_ms"`
	ModelUsage         []AgentAnalyticsModelUsageResponse `json:"model_usage"`
}

type AgentAnalyticsModelUsageResponse struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	TaskCount        int32  `json:"task_count"`
}

type AgentAnalyticsRunResponse struct {
	TaskID               string                             `json:"task_id"`
	AgentID              string                             `json:"agent_id"`
	AgentName            string                             `json:"agent_name"`
	Status               string                             `json:"status"`
	SourceType           string                             `json:"source_type"`
	IssueID              *string                            `json:"issue_id,omitempty"`
	IssueIdentifier      *string                            `json:"issue_identifier,omitempty"`
	IssueTitle           *string                            `json:"issue_title,omitempty"`
	ChatSessionID        *string                            `json:"chat_session_id,omitempty"`
	ChatTitle            *string                            `json:"chat_title,omitempty"`
	AutopilotRunID       *string                            `json:"autopilot_run_id,omitempty"`
	WorkflowDefinitionID *string                            `json:"workflow_definition_id,omitempty"`
	WorkflowRunID        *string                            `json:"workflow_run_id,omitempty"`
	CreatedAt            *string                            `json:"created_at,omitempty"`
	DispatchedAt         *string                            `json:"dispatched_at,omitempty"`
	StartedAt            *string                            `json:"started_at,omitempty"`
	CompletedAt          *string                            `json:"completed_at,omitempty"`
	TotalDurationMs      int64                              `json:"total_duration_ms"`
	QueueMs              int64                              `json:"queue_ms"`
	StartupMs            int64                              `json:"startup_ms"`
	ExecutionMs          int64                              `json:"execution_ms"`
	InputTokens          int64                              `json:"input_tokens"`
	OutputTokens         int64                              `json:"output_tokens"`
	CacheReadTokens      int64                              `json:"cache_read_tokens"`
	CacheWriteTokens     int64                              `json:"cache_write_tokens"`
	TotalTokens          int64                              `json:"total_tokens"`
	ModelUsage           []AgentAnalyticsModelUsageResponse `json:"model_usage"`
	Tracing              AgentAnalyticsTracingResponse      `json:"tracing"`
}

type AgentAnalyticsTracingResponse struct {
	PromptBytes                int64 `json:"prompt_bytes"`
	SystemPromptBytes          int64 `json:"system_prompt_bytes"`
	ChatMessageBytes           int64 `json:"chat_message_bytes"`
	ChatAttachmentCount        int64 `json:"chat_attachment_count"`
	AgentInstructionsBytes     int64 `json:"agent_instructions_bytes"`
	AgentSkillCount            int64 `json:"agent_skill_count"`
	AgentSkillBytes            int64 `json:"agent_skill_bytes"`
	RepoCount                  int64 `json:"repo_count"`
	RepositoryCount            int64 `json:"repository_count"`
	ProjectResourceCount       int64 `json:"project_resource_count"`
	WorkflowSnapshotBytes      int64 `json:"workflow_snapshot_bytes"`
	WorkflowStepSnapshotBytes  int64 `json:"workflow_step_snapshot_bytes"`
	AutopilotDescriptionBytes  int64 `json:"autopilot_description_bytes"`
	AutopilotPayloadBytes      int64 `json:"autopilot_payload_bytes"`
	QuickCreatePromptBytes     int64 `json:"quick_create_prompt_bytes"`
	ExecEnvMs                  int64 `json:"exec_env_ms"`
	RuntimeConfigMs            int64 `json:"runtime_config_ms"`
	BackendCreateMs            int64 `json:"backend_create_ms"`
	AgentRunMs                 int64 `json:"agent_run_ms"`
	DaemonRunMs                int64 `json:"daemon_run_ms"`
	FirstEventMs               int64 `json:"first_event_ms"`
	FirstTextMs                int64 `json:"first_text_ms"`
	FirstToolUseMs             int64 `json:"first_tool_use_ms"`
	FirstToolResultMs          int64 `json:"first_tool_result_ms"`
	TaskMessageTextCount       int64 `json:"task_message_text_count"`
	TaskMessageThinkingCount   int64 `json:"task_message_thinking_count"`
	TaskMessageToolUseCount    int64 `json:"task_message_tool_use_count"`
	TaskMessageToolResultCount int64 `json:"task_message_tool_result_count"`
	TaskMessageErrorCount      int64 `json:"task_message_error_count"`
	AssistantTextBytes         int64 `json:"assistant_text_bytes"`
	ThinkingBytes              int64 `json:"thinking_bytes"`
	ToolInputBytes             int64 `json:"tool_input_bytes"`
	ToolResultBytes            int64 `json:"tool_result_bytes"`
	AgentResultOutputBytes     int64 `json:"agent_result_output_bytes"`
}

type AgentAnalyticsPagination struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
	Total  int32 `json:"total"`
}

type agentAnalyticsWindow struct {
	days              *int
	hasSince          bool
	since             pgtype.Timestamptz
	hasBefore         bool
	before            pgtype.Timestamptz
	hasPrevious       bool
	previousHasSince  bool
	previousSince     pgtype.Timestamptz
	previousHasBefore bool
	previousBefore    pgtype.Timestamptz
}

// GetAgentAnalyticsRuns returns scoped task-usage analytics for project and
// chat views. Cost is intentionally not computed here; the response preserves
// provider/model token dimensions so the frontend can reuse its pricing table.
func (h *Handler) GetAgentAnalyticsRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope != agentAnalyticsScopeProject && scope != agentAnalyticsScopeChat {
		writeError(w, http.StatusBadRequest, "invalid scope")
		return
	}
	scopeRaw := strings.TrimSpace(r.URL.Query().Get("scope_id"))
	if scopeRaw == "" {
		writeError(w, http.StatusBadRequest, "scope_id is required")
		return
	}
	scopeID, ok := parseUUIDOrBadRequest(w, scopeRaw, "scope_id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	switch scope {
	case agentAnalyticsScopeProject:
		if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID:          scopeID,
			WorkspaceID: workspaceUUID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	case agentAnalyticsScopeChat:
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		if _, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, scopeRaw); !ok {
			return
		}
	}

	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if source == "" {
		source = agentAnalyticsSourceAll
	}
	if source != agentAnalyticsSourceAll && source != agentAnalyticsSourceIssues && source != agentAnalyticsSourceChats {
		writeError(w, http.StatusBadRequest, "invalid source")
		return
	}

	window, ok := parseAgentAnalyticsWindow(w, r, scope)
	if !ok {
		return
	}
	limit, offset, ok := parseAgentAnalyticsPagination(w, r)
	if !ok {
		return
	}
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortBy == "" {
		sortBy = agentAnalyticsSortNewest
	}
	if sortBy != agentAnalyticsSortNewest && sortBy != agentAnalyticsSortDuration && sortBy != agentAnalyticsSortTokens {
		writeError(w, http.StatusBadRequest, "invalid sort")
		return
	}

	usageParams := db.ListAgentAnalyticsUsageRowsParams{
		WorkspaceID: workspaceUUID,
		HasSince:    window.hasSince,
		Since:       window.since,
		HasBefore:   window.hasBefore,
		Before:      window.before,
		Scope:       scope,
		ScopeID:     scopeID,
		Source:      source,
	}
	usageRows, err := h.Queries.ListAgentAnalyticsUsageRows(r.Context(), usageParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list analytics usage")
		return
	}

	var previousSummary *AgentAnalyticsSummaryResponse
	if window.hasPrevious {
		prevRows, err := h.Queries.ListAgentAnalyticsUsageRows(r.Context(), db.ListAgentAnalyticsUsageRowsParams{
			WorkspaceID: workspaceUUID,
			HasSince:    window.previousHasSince,
			Since:       window.previousSince,
			HasBefore:   window.previousHasBefore,
			Before:      window.previousBefore,
			Scope:       scope,
			ScopeID:     scopeID,
			Source:      source,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list previous analytics usage")
			return
		}
		prevAggregate := buildAgentAnalyticsAggregate(prevRows)
		previousSummary = &prevAggregate.summary
	}

	total, err := h.Queries.CountAgentAnalyticsRuns(r.Context(), db.CountAgentAnalyticsRunsParams{
		WorkspaceID: workspaceUUID,
		HasSince:    window.hasSince,
		Since:       window.since,
		HasBefore:   window.hasBefore,
		Before:      window.before,
		Scope:       scope,
		ScopeID:     scopeID,
		Source:      source,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count analytics runs")
		return
	}
	runRows, err := h.Queries.ListAgentAnalyticsRuns(r.Context(), db.ListAgentAnalyticsRunsParams{
		Sort:        sortBy,
		OffsetCount: offset,
		LimitCount:  limit,
		WorkspaceID: workspaceUUID,
		HasSince:    window.hasSince,
		Since:       window.since,
		HasBefore:   window.hasBefore,
		Before:      window.before,
		Scope:       scope,
		ScopeID:     scopeID,
		Source:      source,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list analytics runs")
		return
	}

	aggregate := buildAgentAnalyticsAggregate(usageRows)
	writeJSON(w, http.StatusOK, AgentAnalyticsResponse{
		Window:          window.toResponse(),
		Summary:         aggregate.summary,
		PreviousSummary: previousSummary,
		Daily:           aggregate.daily,
		Agents:          aggregate.agents,
		Sources:         aggregate.sources,
		Runs:            agentAnalyticsRunsToResponse(runRows),
		Pagination: AgentAnalyticsPagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

func parseAgentAnalyticsWindow(w http.ResponseWriter, r *http.Request, scope string) (agentAnalyticsWindow, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("days"))
	if raw == "" && scope == agentAnalyticsScopeProject {
		raw = "30"
	}
	if raw == "" || raw == "all" {
		return agentAnalyticsWindow{}, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 || days > 365 {
		writeError(w, http.StatusBadRequest, "invalid days")
		return agentAnalyticsWindow{}, false
	}
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -days)
	previousSince := now.AddDate(0, 0, -2*days)
	return agentAnalyticsWindow{
		days:              &days,
		hasSince:          true,
		since:             pgtype.Timestamptz{Time: since, Valid: true},
		hasPrevious:       true,
		previousHasSince:  true,
		previousSince:     pgtype.Timestamptz{Time: previousSince, Valid: true},
		previousHasBefore: true,
		previousBefore:    pgtype.Timestamptz{Time: since, Valid: true},
	}, true
}

func parseAgentAnalyticsPagination(w http.ResponseWriter, r *http.Request) (int32, int32, bool) {
	limit := agentAnalyticsDefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return 0, 0, false
		}
		limit = parsed
	}
	if limit > agentAnalyticsMaxLimit {
		limit = agentAnalyticsMaxLimit
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return 0, 0, false
		}
		offset = parsed
	}
	return int32(limit), int32(offset), true
}

func (w agentAnalyticsWindow) toResponse() AgentAnalyticsWindowResponse {
	resp := AgentAnalyticsWindowResponse{Days: w.days, HasPrevious: w.hasPrevious}
	if w.hasSince {
		resp.Since = timestampToPtr(w.since)
	}
	if w.hasBefore {
		resp.Before = timestampToPtr(w.before)
	}
	return resp
}

type agentAnalyticsAggregate struct {
	summary AgentAnalyticsSummaryResponse
	daily   []AgentAnalyticsDailyResponse
	agents  []AgentAnalyticsAgentResponse
	sources []AgentAnalyticsSourceResponse
}

type agentAnalyticsModelBucket struct {
	row      AgentAnalyticsModelUsageResponse
	taskSeen map[string]struct{}
}

type agentAnalyticsDailyBucket struct {
	row      AgentAnalyticsDailyResponse
	taskSeen map[string]struct{}
}

type agentAnalyticsAgentBucket struct {
	row       AgentAnalyticsAgentResponse
	taskSeen  map[string]struct{}
	durations []int64
	models    map[string]*agentAnalyticsModelBucket
}

type agentAnalyticsSourceBucket struct {
	row       AgentAnalyticsSourceResponse
	taskSeen  map[string]struct{}
	durations []int64
	models    map[string]*agentAnalyticsModelBucket
}

type agentAnalyticsTaskInfo struct {
	status          string
	durationMs      int64
	promptBytes     int64
	firstTextMs     int64
	toolUseCount    int64
	toolResultBytes int64
}

func buildAgentAnalyticsAggregate(rows []db.ListAgentAnalyticsUsageRowsRow) agentAnalyticsAggregate {
	summary := AgentAnalyticsSummaryResponse{}
	summaryModels := map[string]*agentAnalyticsModelBucket{}
	dailyBuckets := map[string]*agentAnalyticsDailyBucket{}
	agentBuckets := map[string]*agentAnalyticsAgentBucket{}
	sourceBuckets := map[string]*agentAnalyticsSourceBucket{}
	taskSeen := map[string]struct{}{}
	completionDurations := make([]int64, 0)
	firstTextDurations := make([]int64, 0)

	for _, row := range rows {
		taskKey := uuidToString(row.TaskID)
		tokens := tokenTotal(row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens)

		summary.InputTokens += row.InputTokens
		summary.OutputTokens += row.OutputTokens
		summary.CacheReadTokens += row.CacheReadTokens
		summary.CacheWriteTokens += row.CacheWriteTokens
		summary.TotalTokens += tokens
		addModelUsage(summaryModels, row.Provider, row.Model, taskKey, row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens)

		date := row.Date.Time.Format("2006-01-02")
		dailyKey := date + "\x00" + row.Provider + "\x00" + row.Model
		dailyBucket := dailyBuckets[dailyKey]
		if dailyBucket == nil {
			dailyBucket = &agentAnalyticsDailyBucket{
				row: AgentAnalyticsDailyResponse{
					Date:     date,
					Provider: row.Provider,
					Model:    row.Model,
				},
				taskSeen: map[string]struct{}{},
			}
			dailyBuckets[dailyKey] = dailyBucket
		}
		addDailyUsage(dailyBucket, row, taskKey, tokens)

		agentKey := uuidToString(row.AgentID)
		agentBucket := agentBuckets[agentKey]
		if agentBucket == nil {
			agentBucket = &agentAnalyticsAgentBucket{
				row: AgentAnalyticsAgentResponse{
					AgentID:   agentKey,
					AgentName: row.AgentName,
				},
				taskSeen: map[string]struct{}{},
				models:   map[string]*agentAnalyticsModelBucket{},
			}
			agentBuckets[agentKey] = agentBucket
		}
		addAgentUsage(agentBucket, row, taskKey, tokens)

		sourceKey, sourceID, sourceTitle, issueIdentifier := sourceIdentity(row)
		sourceBucket := sourceBuckets[sourceKey]
		if sourceBucket == nil {
			sourceBucket = &agentAnalyticsSourceBucket{
				row: AgentAnalyticsSourceResponse{
					SourceType:      row.SourceType,
					SourceID:        sourceID,
					SourceTitle:     sourceTitle,
					IssueIdentifier: issueIdentifier,
				},
				taskSeen: map[string]struct{}{},
				models:   map[string]*agentAnalyticsModelBucket{},
			}
			sourceBuckets[sourceKey] = sourceBucket
		}
		addSourceUsage(sourceBucket, row, taskKey, tokens)

		if _, seen := taskSeen[taskKey]; seen {
			continue
		}
		taskSeen[taskKey] = struct{}{}
		info := taskInfoFromUsageRow(row)
		summary.TaskCount++
		applyStatusCounts(info.status, &summary.CompletedCount, &summary.FailedCount, &summary.CancelledCount)
		summary.PromptBytes += info.promptBytes
		summary.ToolUseCount += info.toolUseCount
		summary.ToolResultBytes += info.toolResultBytes
		if info.status == "completed" && info.durationMs > 0 {
			completionDurations = append(completionDurations, info.durationMs)
		}
		if info.firstTextMs > 0 {
			firstTextDurations = append(firstTextDurations, info.firstTextMs)
		}

		countTaskForAgent(agentBucket, info)
		countTaskForSource(sourceBucket, info)
	}

	summary.MedianCompletionMs = medianInt64(completionDurations)
	summary.P95CompletionMs = percentileInt64(completionDurations, 0.95)
	summary.MedianFirstTextMs = medianInt64(firstTextDurations)
	summary.PromptCacheReadRate = cacheReadRate(summary.CacheReadTokens, summary.InputTokens, summary.CacheWriteTokens)
	summary.ModelUsage = modelUsageRows(summaryModels)

	return agentAnalyticsAggregate{
		summary: summary,
		daily:   dailyRows(dailyBuckets),
		agents:  agentRows(agentBuckets),
		sources: sourceRows(sourceBuckets),
	}
}

func addDailyUsage(bucket *agentAnalyticsDailyBucket, row db.ListAgentAnalyticsUsageRowsRow, taskKey string, total int64) {
	bucket.row.InputTokens += row.InputTokens
	bucket.row.OutputTokens += row.OutputTokens
	bucket.row.CacheReadTokens += row.CacheReadTokens
	bucket.row.CacheWriteTokens += row.CacheWriteTokens
	bucket.row.TotalTokens += total
	if _, seen := bucket.taskSeen[taskKey]; seen {
		return
	}
	bucket.taskSeen[taskKey] = struct{}{}
	bucket.row.TaskCount++
	applyStatusCounts(row.Status, &bucket.row.CompletedCount, &bucket.row.FailedCount, &bucket.row.CancelledCount)
}

func addAgentUsage(bucket *agentAnalyticsAgentBucket, row db.ListAgentAnalyticsUsageRowsRow, taskKey string, total int64) {
	bucket.row.InputTokens += row.InputTokens
	bucket.row.OutputTokens += row.OutputTokens
	bucket.row.CacheReadTokens += row.CacheReadTokens
	bucket.row.CacheWriteTokens += row.CacheWriteTokens
	bucket.row.TotalTokens += total
	addModelUsage(bucket.models, row.Provider, row.Model, taskKey, row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens)
}

func addSourceUsage(bucket *agentAnalyticsSourceBucket, row db.ListAgentAnalyticsUsageRowsRow, taskKey string, total int64) {
	bucket.row.InputTokens += row.InputTokens
	bucket.row.OutputTokens += row.OutputTokens
	bucket.row.CacheReadTokens += row.CacheReadTokens
	bucket.row.CacheWriteTokens += row.CacheWriteTokens
	bucket.row.TotalTokens += total
	addModelUsage(bucket.models, row.Provider, row.Model, taskKey, row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens)
}

func countTaskForAgent(bucket *agentAnalyticsAgentBucket, info agentAnalyticsTaskInfo) {
	bucket.row.TaskCount++
	applyStatusCounts(info.status, &bucket.row.CompletedCount, &bucket.row.FailedCount, &bucket.row.CancelledCount)
	if info.status == "completed" && info.durationMs > 0 {
		bucket.durations = append(bucket.durations, info.durationMs)
	}
}

func countTaskForSource(bucket *agentAnalyticsSourceBucket, info agentAnalyticsTaskInfo) {
	bucket.row.TaskCount++
	applyStatusCounts(info.status, &bucket.row.CompletedCount, &bucket.row.FailedCount, &bucket.row.CancelledCount)
	if info.status == "completed" && info.durationMs > 0 {
		bucket.durations = append(bucket.durations, info.durationMs)
	}
}

func addModelUsage(
	buckets map[string]*agentAnalyticsModelBucket,
	provider string,
	model string,
	taskKey string,
	inputTokens int64,
	outputTokens int64,
	cacheReadTokens int64,
	cacheWriteTokens int64,
) {
	key := provider + "\x00" + model
	bucket := buckets[key]
	if bucket == nil {
		bucket = &agentAnalyticsModelBucket{
			row: AgentAnalyticsModelUsageResponse{
				Provider: provider,
				Model:    model,
			},
			taskSeen: map[string]struct{}{},
		}
		buckets[key] = bucket
	}
	bucket.row.InputTokens += inputTokens
	bucket.row.OutputTokens += outputTokens
	bucket.row.CacheReadTokens += cacheReadTokens
	bucket.row.CacheWriteTokens += cacheWriteTokens
	bucket.row.TotalTokens += tokenTotal(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
	if _, seen := bucket.taskSeen[taskKey]; !seen {
		bucket.taskSeen[taskKey] = struct{}{}
		bucket.row.TaskCount++
	}
}

func taskInfoFromUsageRow(row db.ListAgentAnalyticsUsageRowsRow) agentAnalyticsTaskInfo {
	duration := durationMillis(row.StartedAt, row.CompletedAt)
	if duration == 0 {
		duration = durationMillis(row.CreatedAt, row.CompletedAt)
	}
	return agentAnalyticsTaskInfo{
		status:          row.Status,
		durationMs:      duration,
		promptBytes:     row.PromptBytes,
		firstTextMs:     row.FirstTextMs,
		toolUseCount:    row.TaskMessageToolUseCount,
		toolResultBytes: row.ToolResultBytes,
	}
}

func sourceIdentity(row db.ListAgentAnalyticsUsageRowsRow) (string, *string, *string, *string) {
	switch row.SourceType {
	case "issue":
		id := uuidToPtr(row.IssueID)
		title := textToPtr(row.IssueTitle)
		identifier := row.IssueIdentifier
		return "issue:" + stringValue(id), id, title, stringPtrIfNotEmpty(identifier)
	case "chat":
		id := uuidToPtr(row.ChatSessionID)
		title := textToPtr(row.ChatTitle)
		return "chat:" + stringValue(id), id, title, nil
	default:
		return "task:", nil, nil, nil
	}
}

func dailyRows(buckets map[string]*agentAnalyticsDailyBucket) []AgentAnalyticsDailyResponse {
	rows := make([]AgentAnalyticsDailyResponse, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, bucket.row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date == rows[j].Date {
			return rows[i].TotalTokens > rows[j].TotalTokens
		}
		return rows[i].Date < rows[j].Date
	})
	return rows
}

func agentRows(buckets map[string]*agentAnalyticsAgentBucket) []AgentAnalyticsAgentResponse {
	rows := make([]AgentAnalyticsAgentResponse, 0, len(buckets))
	for _, bucket := range buckets {
		bucket.row.MedianCompletionMs = medianInt64(bucket.durations)
		bucket.row.ModelUsage = modelUsageRows(bucket.models)
		rows = append(rows, bucket.row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	return rows
}

func sourceRows(buckets map[string]*agentAnalyticsSourceBucket) []AgentAnalyticsSourceResponse {
	rows := make([]AgentAnalyticsSourceResponse, 0, len(buckets))
	for _, bucket := range buckets {
		bucket.row.MedianCompletionMs = medianInt64(bucket.durations)
		bucket.row.ModelUsage = modelUsageRows(bucket.models)
		rows = append(rows, bucket.row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	return rows
}

func modelUsageRows(buckets map[string]*agentAnalyticsModelBucket) []AgentAnalyticsModelUsageResponse {
	rows := make([]AgentAnalyticsModelUsageResponse, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, bucket.row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	return rows
}

func applyStatusCounts(status string, completed *int32, failed *int32, cancelled *int32) {
	switch status {
	case "completed":
		(*completed)++
	case "failed":
		(*failed)++
	case "cancelled":
		(*cancelled)++
	}
}

func agentAnalyticsRunsToResponse(rows []db.ListAgentAnalyticsRunsRow) []AgentAnalyticsRunResponse {
	resp := make([]AgentAnalyticsRunResponse, len(rows))
	for i, row := range rows {
		modelUsage := decodeAgentAnalyticsModelUsage(row.ModelUsage)
		resp[i] = AgentAnalyticsRunResponse{
			TaskID:               uuidToString(row.TaskID),
			AgentID:              uuidToString(row.AgentID),
			AgentName:            row.AgentName,
			Status:               row.Status,
			SourceType:           row.SourceType,
			IssueID:              uuidToPtr(row.IssueID),
			IssueIdentifier:      stringPtrIfNotEmpty(row.IssueIdentifier),
			IssueTitle:           textToPtr(row.IssueTitle),
			ChatSessionID:        uuidToPtr(row.ChatSessionID),
			ChatTitle:            textToPtr(row.ChatTitle),
			AutopilotRunID:       uuidToPtr(row.AutopilotRunID),
			WorkflowDefinitionID: uuidToPtr(row.WorkflowDefinitionID),
			WorkflowRunID:        uuidToPtr(row.WorkflowRunID),
			CreatedAt:            timestampToPtr(row.CreatedAt),
			DispatchedAt:         timestampToPtr(row.DispatchedAt),
			StartedAt:            timestampToPtr(row.StartedAt),
			CompletedAt:          timestampToPtr(row.CompletedAt),
			TotalDurationMs:      row.TotalDurationMs,
			QueueMs:              row.QueueMs,
			StartupMs:            row.StartupMs,
			ExecutionMs:          row.ExecutionMs,
			InputTokens:          row.InputTokens,
			OutputTokens:         row.OutputTokens,
			CacheReadTokens:      row.CacheReadTokens,
			CacheWriteTokens:     row.CacheWriteTokens,
			TotalTokens:          tokenTotal(row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens),
			ModelUsage:           modelUsage,
			Tracing: AgentAnalyticsTracingResponse{
				PromptBytes:                row.PromptBytes,
				SystemPromptBytes:          row.SystemPromptBytes,
				ChatMessageBytes:           row.ChatMessageBytes,
				ChatAttachmentCount:        row.ChatAttachmentCount,
				AgentInstructionsBytes:     row.AgentInstructionsBytes,
				AgentSkillCount:            row.AgentSkillCount,
				AgentSkillBytes:            row.AgentSkillBytes,
				RepoCount:                  row.RepoCount,
				RepositoryCount:            row.RepositoryCount,
				ProjectResourceCount:       row.ProjectResourceCount,
				WorkflowSnapshotBytes:      row.WorkflowSnapshotBytes,
				WorkflowStepSnapshotBytes:  row.WorkflowStepSnapshotBytes,
				AutopilotDescriptionBytes:  row.AutopilotDescriptionBytes,
				AutopilotPayloadBytes:      row.AutopilotPayloadBytes,
				QuickCreatePromptBytes:     row.QuickCreatePromptBytes,
				ExecEnvMs:                  row.ExecEnvMs,
				RuntimeConfigMs:            row.RuntimeConfigMs,
				BackendCreateMs:            row.BackendCreateMs,
				AgentRunMs:                 row.AgentRunMs,
				DaemonRunMs:                row.DaemonRunMs,
				FirstEventMs:               row.FirstEventMs,
				FirstTextMs:                row.FirstTextMs,
				FirstToolUseMs:             row.FirstToolUseMs,
				FirstToolResultMs:          row.FirstToolResultMs,
				TaskMessageTextCount:       row.TaskMessageTextCount,
				TaskMessageThinkingCount:   row.TaskMessageThinkingCount,
				TaskMessageToolUseCount:    row.TaskMessageToolUseCount,
				TaskMessageToolResultCount: row.TaskMessageToolResultCount,
				TaskMessageErrorCount:      row.TaskMessageErrorCount,
				AssistantTextBytes:         row.AssistantTextBytes,
				ThinkingBytes:              row.ThinkingBytes,
				ToolInputBytes:             row.ToolInputBytes,
				ToolResultBytes:            row.ToolResultBytes,
				AgentResultOutputBytes:     row.AgentResultOutputBytes,
			},
		}
	}
	return resp
}

func decodeAgentAnalyticsModelUsage(value any) []AgentAnalyticsModelUsageResponse {
	if value == nil {
		return []AgentAnalyticsModelUsageResponse{}
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return []AgentAnalyticsModelUsageResponse{}
		}
		data = marshaled
	}
	var rows []AgentAnalyticsModelUsageResponse
	if err := json.Unmarshal(data, &rows); err != nil {
		return []AgentAnalyticsModelUsageResponse{}
	}
	for i := range rows {
		rows[i].TotalTokens = tokenTotal(rows[i].InputTokens, rows[i].OutputTokens, rows[i].CacheReadTokens, rows[i].CacheWriteTokens)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	return rows
}

func tokenTotal(input int64, output int64, cacheRead int64, cacheWrite int64) int64 {
	return input + output + cacheRead + cacheWrite
}

func cacheReadRate(cacheRead int64, input int64, cacheWrite int64) float64 {
	totalPromptSideTokens := input + cacheRead + cacheWrite
	if totalPromptSideTokens <= 0 {
		return 0
	}
	return float64(cacheRead) / float64(totalPromptSideTokens)
}

func durationMillis(start pgtype.Timestamptz, end pgtype.Timestamptz) int64 {
	if !start.Valid || !end.Valid {
		return 0
	}
	ms := end.Time.Sub(start.Time).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func percentileInt64(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func stringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
