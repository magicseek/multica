package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/mention"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type TaskService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Hub       *realtime.Hub
	Bus       *events.Bus
	Analytics analytics.Client
	Wakeup    TaskWakeupNotifier
	// EmptyClaim caches "this runtime has no queued task" so the daemon
	// poll path can skip a Postgres scan on the steady-state empty case.
	// Optional — a nil cache disables the fast path and every claim
	// goes through the DB. Wired in router.go from the shared Redis
	// client.
	EmptyClaim *EmptyClaimCache

	analyticsContextMu    sync.Mutex
	analyticsContextCache map[string]analytics.TaskContext
	analyticsContextOrder []string
}

type TaskWakeupNotifier interface {
	NotifyTaskAvailable(runtimeID, taskID string)
}

// triggerSummaryMaxLen caps the snapshot length so the row stays cheap to
// transmit (it ends up in every task list response). 200 is enough for a
// recognisable preview of a one-paragraph comment.
const triggerSummaryMaxLen = 200

const (
	ChatTaskKindNormal           = "normal"
	ChatTaskKindPlanLead         = "plan_lead"
	ChatTaskKindPlanConsultation = "plan_consultation"

	maxChatPlanConsultationWaves = 5

	TaskBundleMaxItems                 = 5
	TaskBundleDefaultItemBudgetSeconds = 2 * 3600
	TaskBundleBudgetBufferSeconds      = 30 * 60
	TaskBundleChangesetModePerIssue    = "per_issue"
	TaskBundleChangesetModeShared      = "shared"
)

var TaskBundleTerminalStatuses = map[string]bool{
	"completed":    true,
	"failed":       true,
	"blocked":      true,
	"input_needed": true,
	"cancelled":    true,
}

// truncateForSummary returns s shortened to maxRunes, with a trailing
// `…` when truncated. Operates on runes (not bytes) so multibyte characters
// — Chinese / emoji — count as one each. Strips surrounding whitespace
// first so a leading newline doesn't waste budget.
func truncateForSummary(s string, maxRunes int) string {
	// strings.Builder + Grow avoids the O(N²) realloc cycle of `+=` in
	// a loop. Grow uses byte length, which is an upper bound for the
	// rune-equivalent output (replacing \n/\r/\t with space is byte-equal
	// for ASCII whitespace), so we never reallocate.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	rs := []rune(strings.TrimSpace(b.String()))
	if len(rs) <= maxRunes {
		return string(rs)
	}
	return string(rs[:maxRunes]) + "…"
}

const taskAnalyticsContextCacheMax = 4096

// buildCommentTriggerSummary fetches the comment content and truncates
// it for storage on the task row. Returns an invalid pgtype.Text when
// the comment is missing (deleted / wrong workspace / etc) so the column
// stays NULL — front-end falls back to a structural label in that case.
func (s *TaskService) buildCommentTriggerSummary(ctx context.Context, commentID pgtype.UUID) pgtype.Text {
	if !commentID.Valid {
		return pgtype.Text{}
	}
	comment, err := s.Queries.GetComment(ctx, commentID)
	if err != nil {
		return pgtype.Text{}
	}
	summary := truncateForSummary(comment.Content, triggerSummaryMaxLen)
	if summary == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: summary, Valid: true}
}

func NewTaskService(q *db.Queries, tx TxStarter, hub *realtime.Hub, bus *events.Bus, wakeups ...TaskWakeupNotifier) *TaskService {
	var wakeup TaskWakeupNotifier
	if len(wakeups) > 0 {
		wakeup = wakeups[0]
	}
	return &TaskService{Queries: q, TxStarter: tx, Hub: hub, Bus: bus, Wakeup: wakeup}
}

var trivialDoneMarkers = []string{
	"done",
	"готово",
	"готова",
	"сделано",
	"完成",
	"完了",
}

func isTrivialDoneOutput(output string) bool {
	normalized := strings.TrimSpace(strings.ToLower(output))
	normalized = strings.Trim(normalized, ".!！。… ")
	for _, marker := range trivialDoneMarkers {
		if normalized == marker {
			return true
		}
	}
	return false
}

func (s *TaskService) captureTaskQueued(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskEvent(ctx, analytics.AgentTaskQueued(s.taskAnalyticsContext(ctx, task)))
}

func (s *TaskService) captureTaskDispatched(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskEvent(ctx, analytics.AgentTaskDispatched(s.taskAnalyticsContext(ctx, task)))
}

func (s *TaskService) AnalyticsContextForTask(ctx context.Context, task db.AgentTaskQueue) analytics.TaskContext {
	return s.taskAnalyticsContext(ctx, task)
}

func (s *TaskService) captureTaskStarted(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskEvent(ctx, analytics.AgentTaskStarted(s.taskAnalyticsContext(ctx, task)))
}

func (s *TaskService) captureTaskCompleted(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskEvent(ctx, analytics.AgentTaskCompleted(
		s.taskAnalyticsContext(ctx, task),
		taskDurationMS(task),
	))
}

func (s *TaskService) captureTaskFailed(ctx context.Context, task db.AgentTaskQueue) {
	failureReason := taskFailureReason(task)
	s.captureTaskEvent(ctx, analytics.AgentTaskFailed(
		s.taskAnalyticsContext(ctx, task),
		taskDurationMS(task),
		failureReason,
		taskErrorType(failureReason),
		s.willRetryTask(task),
	))
}

func (s *TaskService) captureTaskCancelled(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskEvent(ctx, analytics.AgentTaskCancelled(
		s.taskAnalyticsContext(ctx, task),
		taskDurationMS(task),
	))
}

func (s *TaskService) captureTaskEvent(ctx context.Context, event analytics.Event) {
	if s.Analytics == nil {
		return
	}
	if event.WorkspaceID == "" {
		return
	}
	s.Analytics.Capture(event)
}

func (s *TaskService) cachedTaskAnalyticsContext(task db.AgentTaskQueue) (analytics.TaskContext, bool) {
	key := taskAnalyticsContextKey(task)
	if key == "" {
		return analytics.TaskContext{}, false
	}
	s.analyticsContextMu.Lock()
	defer s.analyticsContextMu.Unlock()
	if s.analyticsContextCache == nil {
		return analytics.TaskContext{}, false
	}
	tc, ok := s.analyticsContextCache[key]
	return tc, ok
}

func (s *TaskService) storeTaskAnalyticsContext(task db.AgentTaskQueue, tc analytics.TaskContext) {
	if tc.WorkspaceID == "" {
		return
	}
	key := taskAnalyticsContextKey(task)
	if key == "" {
		return
	}
	s.analyticsContextMu.Lock()
	defer s.analyticsContextMu.Unlock()
	if s.analyticsContextCache == nil {
		s.analyticsContextCache = make(map[string]analytics.TaskContext)
	}
	if _, ok := s.analyticsContextCache[key]; !ok {
		s.analyticsContextOrder = append(s.analyticsContextOrder, key)
		if len(s.analyticsContextOrder) > taskAnalyticsContextCacheMax {
			oldest := s.analyticsContextOrder[0]
			s.analyticsContextOrder = s.analyticsContextOrder[1:]
			delete(s.analyticsContextCache, oldest)
		}
	}
	s.analyticsContextCache[key] = tc
}

func taskAnalyticsContextKey(task db.AgentTaskQueue) string {
	taskID := util.UUIDToString(task.ID)
	if taskID == "" {
		return ""
	}
	return strings.Join([]string{
		taskID,
		util.UUIDToString(task.RuntimeID),
		util.UUIDToString(task.IssueID),
		util.UUIDToString(task.ChatSessionID),
		util.UUIDToString(task.AutopilotRunID),
	}, "|")
}

func (s *TaskService) taskAnalyticsContext(ctx context.Context, task db.AgentTaskQueue) analytics.TaskContext {
	if tc, ok := s.cachedTaskAnalyticsContext(task); ok {
		return tc
	}
	tc := analytics.TaskContext{
		AgentID: util.UUIDToString(task.AgentID),
		TaskID:  util.UUIDToString(task.ID),
		Source:  analytics.SourceManual,
	}
	if task.IssueID.Valid {
		tc.IssueID = util.UUIDToString(task.IssueID)
	}
	if task.ChatSessionID.Valid {
		tc.ChatSessionID = util.UUIDToString(task.ChatSessionID)
		tc.Source = analytics.SourceChat
	}
	if task.AutopilotRunID.Valid {
		tc.AutopilotRunID = util.UUIDToString(task.AutopilotRunID)
		tc.Source = analytics.SourceAutopilot
	}

	if task.RuntimeID.Valid {
		if rt, err := s.Queries.GetAgentRuntime(ctx, task.RuntimeID); err == nil {
			tc.WorkspaceID = util.UUIDToString(rt.WorkspaceID)
			tc.RuntimeMode = rt.RuntimeMode
			tc.Provider = rt.Provider
		}
	}
	if tc.WorkspaceID == "" || tc.RuntimeMode == "" {
		if agent, err := s.Queries.GetAgent(ctx, task.AgentID); err == nil {
			if tc.WorkspaceID == "" {
				tc.WorkspaceID = util.UUIDToString(agent.WorkspaceID)
			}
			if tc.RuntimeMode == "" {
				tc.RuntimeMode = agent.RuntimeMode
			}
		}
	}

	if task.IssueID.Valid {
		if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
			tc.WorkspaceID = util.UUIDToString(issue.WorkspaceID)
			if issue.CreatorType == "member" {
				tc.UserID = util.UUIDToString(issue.CreatorID)
			}
			if issue.OriginType.Valid {
				switch issue.OriginType.String {
				case "autopilot":
					tc.Source = analytics.SourceAutopilot
					if ap, err := s.Queries.GetAutopilot(ctx, issue.OriginID); err == nil {
						if ap.CreatedByType == "member" {
							tc.UserID = util.UUIDToString(ap.CreatedByID)
						}
					}
				case "quick_create":
					tc.Source = analytics.SourceManual
				}
			}
		}
	}
	if task.ChatSessionID.Valid {
		if cs, err := s.Queries.GetChatSession(ctx, task.ChatSessionID); err == nil {
			tc.WorkspaceID = util.UUIDToString(cs.WorkspaceID)
			tc.UserID = util.UUIDToString(cs.CreatorID)
		}
	}
	if task.AutopilotRunID.Valid {
		if run, err := s.Queries.GetAutopilotRun(ctx, task.AutopilotRunID); err == nil {
			if ap, err := s.Queries.GetAutopilot(ctx, run.AutopilotID); err == nil {
				tc.WorkspaceID = util.UUIDToString(ap.WorkspaceID)
				if ap.CreatedByType == "member" {
					tc.UserID = util.UUIDToString(ap.CreatedByID)
				}
			}
		}
	}
	if qc, ok := s.parseQuickCreateContext(task); ok {
		tc.WorkspaceID = qc.WorkspaceID
		tc.UserID = qc.RequesterID
		tc.Source = analytics.SourceManual
	}
	s.storeTaskAnalyticsContext(task, tc)
	return tc
}

func taskDurationMS(task db.AgentTaskQueue) int64 {
	if !task.CompletedAt.Valid {
		return 0
	}
	start := task.CreatedAt
	if task.StartedAt.Valid {
		start = task.StartedAt
	} else if task.DispatchedAt.Valid {
		start = task.DispatchedAt
	}
	if !start.Valid {
		return 0
	}
	ms := task.CompletedAt.Time.Sub(start.Time).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func taskFailureReason(task db.AgentTaskQueue) string {
	if task.FailureReason.Valid && task.FailureReason.String != "" {
		return task.FailureReason.String
	}
	return "agent_error"
}

func taskErrorType(reason string) string {
	switch reason {
	case "runtime_offline", "runtime_recovery":
		return "runtime"
	case "timeout":
		return "timeout"
	case "iteration_limit", "agent_fallback_message":
		return "agent_output"
	case "cancelled", "user_cancelled":
		return "cancelled"
	default:
		return "agent_error"
	}
}

func (s *TaskService) willRetryTask(task db.AgentTaskQueue) bool {
	if task.TaskBundleID.Valid {
		return false
	}
	reason := taskFailureReason(task)
	if !retryableReasons[reason] {
		return false
	}
	if task.Attempt >= task.MaxAttempts {
		return false
	}
	if task.AutopilotRunID.Valid {
		return false
	}
	return task.IssueID.Valid || task.ChatSessionID.Valid
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the multica CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}
	return s.enqueueIssueTask(ctx, issue, commentID, false, pgtype.UUID{})
}

// EnqueueTaskForIssueByDelegatedUser creates an issue task while explicitly
// snapshotting the member identity that connector calls should act on behalf
// of. Comment-triggered tasks still prefer the comment author / inherited
// parent task identity.
func (s *TaskService) EnqueueTaskForIssueByDelegatedUser(ctx context.Context, issue db.Issue, delegatedUserID pgtype.UUID, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}
	return s.enqueueIssueTask(ctx, issue, commentID, false, delegatedUserID)
}

// enqueueIssueTask is the shared implementation behind EnqueueTaskForIssue
// and the manual rerun path. forceFreshSession=true marks the task so the
// daemon claim handler skips the (agent_id, issue_id) resume lookup — the
// user already judged the prior output bad, a fresh agent session is the
// expected behavior.
func (s *TaskService) enqueueIssueTask(ctx context.Context, issue db.Issue, triggerCommentID pgtype.UUID, forceFreshSession bool, delegatedUserFallback pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}

	agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "agent has no runtime")
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	workflowSnapshot, err := s.ResolveWorkflowSnapshotForIssueTask(ctx, issue, triggerCommentID)
	if err != nil {
		s.logWorkflowResolutionError(issue, err)
		return db.AgentTaskQueue{}, fmt.Errorf("resolve workflow snapshot: %w", err)
	}

	var task db.AgentTaskQueue
	triggerSummary := s.buildCommentTriggerSummary(ctx, triggerCommentID)
	connectorDelegatedUserID := s.resolveConnectorDelegatedUser(ctx, issue, issue.AssigneeID, triggerCommentID, delegatedUserFallback)
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		created, err := qtx.CreateAgentTask(ctx, db.CreateAgentTaskParams{
			AgentID:                  issue.AssigneeID,
			RuntimeID:                agent.RuntimeID,
			IssueID:                  issue.ID,
			Priority:                 priorityToInt(issue.Priority),
			TriggerCommentID:         triggerCommentID,
			TriggerSummary:           triggerSummary,
			ForceFreshSession:        pgtype.Bool{Bool: forceFreshSession, Valid: forceFreshSession},
			WorkflowDefinitionID:     workflowSnapshot.DefinitionID,
			WorkflowRevisionID:       workflowSnapshot.RevisionID,
			WorkflowSnapshot:         workflowSnapshot.SnapshotJSON,
			ConnectorDelegatedUserID: connectorDelegatedUserID,
		})
		if err != nil {
			return err
		}
		task = created
		return s.createWorkflowRunForTask(ctx, qtx, task)
	}); err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("task enqueued",
		"task_id", util.UUIDToString(task.ID),
		"issue_id", util.UUIDToString(issue.ID),
		"agent_id", util.UUIDToString(issue.AssigneeID),
		"force_fresh_session", forceFreshSession,
	)
	// Order matters: broadcast first, notify daemon second. notifyTaskAvailable
	// kicks an in-process channel that the daemon picks up over HTTP and
	// claims; the claim path then emits its own task:dispatch. Doing the
	// queued broadcast afterwards risks the dispatch event reaching clients
	// before the queued one (rare but unsafe-by-construction). Publishing
	// in the desired observe-order makes correctness independent of timing.
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
	return task, nil
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.enqueueMentionTask(ctx, issue, agentID, triggerCommentID, false, pgtype.UUID{})
}

// EnqueueTaskForSquadLeader is the leader-role variant of EnqueueTaskForMention.
// The resulting task carries is_leader_task=true so that downstream
// self-trigger guards can distinguish a comment posted while the agent was
// acting as the squad's leader (skip) from one posted while it was acting
// as a worker (do not skip). This matters for agents that are simultaneously
// the leader and a worker of the same squad — see migration 090.
func (s *TaskService) EnqueueTaskForSquadLeader(ctx context.Context, issue db.Issue, leaderID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.enqueueMentionTask(ctx, issue, leaderID, triggerCommentID, true, pgtype.UUID{})
}

func (s *TaskService) EnqueueTaskForSquadLeaderByDelegatedUser(ctx context.Context, issue db.Issue, leaderID pgtype.UUID, triggerCommentID pgtype.UUID, delegatedUserID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.enqueueMentionTask(ctx, issue, leaderID, triggerCommentID, true, delegatedUserID)
}

func (s *TaskService) enqueueMentionTask(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID, isLeader bool, delegatedUserFallback pgtype.UUID) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("mention task enqueue failed: agent has no runtime", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	workflowSnapshot, err := s.ResolveWorkflowSnapshotForIssueTask(ctx, issue, triggerCommentID)
	if err != nil {
		s.logWorkflowResolutionError(issue, err)
		return db.AgentTaskQueue{}, fmt.Errorf("resolve workflow snapshot: %w", err)
	}

	var task db.AgentTaskQueue
	triggerSummary := s.buildCommentTriggerSummary(ctx, triggerCommentID)
	connectorDelegatedUserID := s.resolveConnectorDelegatedUser(ctx, issue, agentID, triggerCommentID, delegatedUserFallback)
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		created, err := qtx.CreateAgentTask(ctx, db.CreateAgentTaskParams{
			AgentID:                  agentID,
			RuntimeID:                agent.RuntimeID,
			IssueID:                  issue.ID,
			Priority:                 priorityToInt(issue.Priority),
			TriggerCommentID:         triggerCommentID,
			TriggerSummary:           triggerSummary,
			IsLeaderTask:             pgtype.Bool{Bool: isLeader, Valid: isLeader},
			WorkflowDefinitionID:     workflowSnapshot.DefinitionID,
			WorkflowRevisionID:       workflowSnapshot.RevisionID,
			WorkflowSnapshot:         workflowSnapshot.SnapshotJSON,
			ConnectorDelegatedUserID: connectorDelegatedUserID,
		})
		if err != nil {
			return err
		}
		task = created
		return s.createWorkflowRunForTask(ctx, qtx, task)
	}); err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("mention task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "is_leader_task", isLeader)
	// See EnqueueTaskForIssue for ordering rationale.
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
	return task, nil
}

type CreateTaskBundleInput struct {
	WorkspaceID          pgtype.UUID
	AgentID              pgtype.UUID
	IssueIDs             []pgtype.UUID
	ChangesetMode        string
	RuntimeBudgetSeconds int32
	RerunOfBundleID      pgtype.UUID
	RerunScope           []byte
	CreatedBy            pgtype.UUID
}

type CreateTaskBundleResult struct {
	Bundle db.TaskBundle
	Items  []db.TaskBundleItem
	Task   db.AgentTaskQueue
}

type RerunTaskBundleInput struct {
	WorkspaceID pgtype.UUID
	BundleID    pgtype.UUID
	IssueIDs    []pgtype.UUID
	CreatedBy   pgtype.UUID
}

func (s *TaskService) CreateTaskBundle(ctx context.Context, input CreateTaskBundleInput) (*CreateTaskBundleResult, error) {
	if len(input.IssueIDs) == 0 {
		return nil, fmt.Errorf("at least one issue is required")
	}
	if len(input.IssueIDs) > TaskBundleMaxItems {
		return nil, fmt.Errorf("task bundle can include at most %d issues", TaskBundleMaxItems)
	}
	changesetMode := strings.TrimSpace(input.ChangesetMode)
	if changesetMode == "" {
		changesetMode = TaskBundleChangesetModePerIssue
	}
	if changesetMode != TaskBundleChangesetModePerIssue && changesetMode != TaskBundleChangesetModeShared {
		return nil, fmt.Errorf("invalid changeset_mode")
	}
	runtimeBudgetSeconds := input.RuntimeBudgetSeconds
	if runtimeBudgetSeconds <= 0 {
		runtimeBudgetSeconds = int32(len(input.IssueIDs)*TaskBundleDefaultItemBudgetSeconds + TaskBundleBudgetBufferSeconds)
	}

	seen := map[string]struct{}{}
	for _, issueID := range input.IssueIDs {
		key := util.UUIDToString(issueID)
		if key == "" {
			return nil, fmt.Errorf("invalid issue id")
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate issue id")
		}
		seen[key] = struct{}{}
	}

	var result CreateTaskBundleResult
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		agent, err := qtx.GetAgent(ctx, input.AgentID)
		if err != nil {
			return fmt.Errorf("load agent: %w", err)
		}
		if util.UUIDToString(agent.WorkspaceID) != util.UUIDToString(input.WorkspaceID) {
			return fmt.Errorf("agent is not in this workspace")
		}
		if agent.ArchivedAt.Valid {
			return fmt.Errorf("agent is archived")
		}
		if !agent.RuntimeID.Valid {
			return fmt.Errorf("agent has no runtime")
		}
		if !agent.RequestEfficientEnabled {
			return fmt.Errorf("agent does not have request-efficient mode enabled")
		}

		issues := make([]db.Issue, 0, len(input.IssueIDs))
		for _, issueID := range input.IssueIDs {
			issue, err := qtx.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{
				ID:          issueID,
				WorkspaceID: input.WorkspaceID,
			})
			if err != nil {
				return fmt.Errorf("load issue: %w", err)
			}
			if !s.issueCanRunInBundle(ctx, qtx, issue, agent) {
				return fmt.Errorf("issue %s is not assigned to this request-efficient agent or its led squad", util.UUIDToString(issue.ID))
			}
			if issue.Status == "done" || issue.Status == "cancelled" {
				return fmt.Errorf("issue %s is already terminal", util.UUIDToString(issue.ID))
			}
			hasActive, err := qtx.HasActiveTaskForIssue(ctx, issue.ID)
			if err != nil {
				return fmt.Errorf("check active task: %w", err)
			}
			if hasActive {
				return fmt.Errorf("issue %s already has an active task", util.UUIDToString(issue.ID))
			}
			issues = append(issues, issue)
		}

		rerunScope := input.RerunScope
		if len(rerunScope) == 0 {
			rerunScope = []byte("[]")
		}
		bundle, err := qtx.CreateTaskBundle(ctx, db.CreateTaskBundleParams{
			WorkspaceID:          input.WorkspaceID,
			AgentID:              agent.ID,
			RuntimeID:            agent.RuntimeID,
			ChangesetMode:        changesetMode,
			MaxItems:             TaskBundleMaxItems,
			RuntimeBudgetSeconds: runtimeBudgetSeconds,
			RerunOfBundleID:      input.RerunOfBundleID,
			RerunScope:           rerunScope,
			CreatedBy:            input.CreatedBy,
		})
		if err != nil {
			return fmt.Errorf("create bundle: %w", err)
		}

		task, err := qtx.CreateAgentTask(ctx, db.CreateAgentTaskParams{
			AgentID:      agent.ID,
			RuntimeID:    agent.RuntimeID,
			IssueID:      issues[0].ID,
			Priority:     priorityToInt(issues[0].Priority),
			TaskBundleID: bundle.ID,
		})
		if err != nil {
			return fmt.Errorf("create bundle execution task: %w", err)
		}
		if err := s.createWorkflowRunForTask(ctx, qtx, task); err != nil {
			return err
		}

		items := make([]db.TaskBundleItem, 0, len(issues))
		for i, issue := range issues {
			item, err := qtx.CreateTaskBundleItem(ctx, db.CreateTaskBundleItemParams{
				BundleID:        bundle.ID,
				IssueID:         issue.ID,
				Position:        int32(i + 1),
				OutputNamespace: fmt.Sprintf("bundle/%s/item-%02d-%s", util.UUIDToString(bundle.ID), i+1, util.UUIDToString(issue.ID)),
			})
			if err != nil {
				return fmt.Errorf("create bundle item: %w", err)
			}
			items = append(items, item)
		}

		result = CreateTaskBundleResult{
			Bundle: bundle,
			Items:  items,
			Task:   task,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, result.Task)
	s.NotifyTaskEnqueued(ctx, result.Task)
	return &result, nil
}

func (s *TaskService) issueCanRunInBundle(ctx context.Context, q *db.Queries, issue db.Issue, agent db.Agent) bool {
	if !issue.AssigneeID.Valid {
		return false
	}
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" {
		return util.UUIDToString(issue.AssigneeID) == util.UUIDToString(agent.ID)
	}
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "squad" {
		squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
			ID:          issue.AssigneeID,
			WorkspaceID: issue.WorkspaceID,
		})
		return err == nil && util.UUIDToString(squad.LeaderID) == util.UUIDToString(agent.ID)
	}
	return false
}

func (s *TaskService) RerunTaskBundle(ctx context.Context, input RerunTaskBundleInput) (*CreateTaskBundleResult, error) {
	bundle, err := s.Queries.GetTaskBundle(ctx, input.BundleID)
	if err != nil {
		return nil, fmt.Errorf("load bundle: %w", err)
	}
	if util.UUIDToString(bundle.WorkspaceID) != util.UUIDToString(input.WorkspaceID) {
		return nil, fmt.Errorf("task bundle is not in this workspace")
	}
	items, err := s.Queries.ListTaskBundleItems(ctx, bundle.ID)
	if err != nil {
		return nil, fmt.Errorf("list bundle items: %w", err)
	}

	selected := map[string]struct{}{}
	for _, issueID := range input.IssueIDs {
		key := util.UUIDToString(issueID)
		if key == "" {
			return nil, fmt.Errorf("invalid issue id")
		}
		selected[key] = struct{}{}
	}

	issueIDs := make([]pgtype.UUID, 0, len(items))
	rerunScope := make([]string, 0, len(items))
	for _, item := range items {
		issueID := util.UUIDToString(item.IssueID)
		if len(selected) > 0 {
			if _, ok := selected[issueID]; !ok {
				continue
			}
		} else if item.Status != "failed" && item.Status != "blocked" && item.Status != "input_needed" {
			continue
		}
		issueIDs = append(issueIDs, item.IssueID)
		rerunScope = append(rerunScope, issueID)
	}
	if len(issueIDs) == 0 {
		return nil, fmt.Errorf("no bundle items are eligible to rerun")
	}
	rerunScopeJSON, _ := json.Marshal(rerunScope)
	return s.CreateTaskBundle(ctx, CreateTaskBundleInput{
		WorkspaceID:     input.WorkspaceID,
		AgentID:         bundle.AgentID,
		IssueIDs:        issueIDs,
		ChangesetMode:   bundle.ChangesetMode,
		RerunOfBundleID: bundle.ID,
		RerunScope:      rerunScopeJSON,
		CreatedBy:       input.CreatedBy,
	})
}

type CheckpointTaskBundleItemInput struct {
	TaskID        pgtype.UUID
	ItemID        pgtype.UUID
	Status        string
	Result        []byte
	Error         string
	CheckpointSeq pgtype.Int4
}

type CheckpointTaskBundleItemResult struct {
	Bundle db.TaskBundle
	Item   db.TaskBundleItem
	Items  []db.TaskBundleItem
}

func (s *TaskService) CheckpointTaskBundleItem(ctx context.Context, input CheckpointTaskBundleItemInput) (*CheckpointTaskBundleItemResult, error) {
	status := strings.TrimSpace(input.Status)
	if !TaskBundleTerminalStatuses[status] {
		return nil, fmt.Errorf("invalid bundle item status")
	}

	var result CheckpointTaskBundleItemResult
	var issuesToBroadcast []db.Issue
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		task, err := qtx.GetAgentTask(ctx, input.TaskID)
		if err != nil {
			return fmt.Errorf("load task: %w", err)
		}
		if !task.TaskBundleID.Valid {
			return fmt.Errorf("task is not a task bundle execution")
		}
		existing, err := qtx.GetTaskBundleItem(ctx, input.ItemID)
		if err != nil {
			return fmt.Errorf("load bundle item: %w", err)
		}
		if util.UUIDToString(existing.BundleID) != util.UUIDToString(task.TaskBundleID) {
			return fmt.Errorf("bundle item does not belong to this task")
		}

		checkpointSeq := input.CheckpointSeq
		if !checkpointSeq.Valid {
			seq, err := qtx.GetLatestTaskMessageSeq(ctx, task.ID)
			if err != nil {
				return fmt.Errorf("load latest task message seq: %w", err)
			}
			checkpointSeq = pgtype.Int4{Int32: int32(seq), Valid: true}
		}
		item, err := qtx.CheckpointTaskBundleItem(ctx, db.CheckpointTaskBundleItemParams{
			ID:            existing.ID,
			Status:        status,
			Result:        input.Result,
			Error:         pgtype.Text{String: input.Error, Valid: input.Error != ""},
			CheckpointSeq: checkpointSeq,
		})
		if err != nil {
			return fmt.Errorf("checkpoint bundle item: %w", err)
		}
		result.Item = item
		if updated, err := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
			ID:     item.IssueID,
			Status: issueStatusForBundleItemStatus(status),
		}); err == nil {
			issuesToBroadcast = append(issuesToBroadcast, updated)
		} else {
			return fmt.Errorf("update checkpoint issue status: %w", err)
		}

		if next, err := qtx.StartNextTaskBundleItem(ctx, task.TaskBundleID); err == nil {
			if updated, updateErr := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				ID:     next.IssueID,
				Status: "in_progress",
			}); updateErr == nil {
				issuesToBroadcast = append(issuesToBroadcast, updated)
			} else {
				return fmt.Errorf("update next bundle issue status: %w", updateErr)
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("start next bundle item: %w", err)
		}

		if bundle, err := qtx.CompleteTaskBundleIfDone(ctx, task.TaskBundleID); err == nil {
			result.Bundle = bundle
		} else if errors.Is(err, pgx.ErrNoRows) {
			bundle, err := qtx.GetTaskBundle(ctx, task.TaskBundleID)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			result.Bundle = bundle
		} else {
			return fmt.Errorf("complete bundle: %w", err)
		}
		items, err := qtx.ListTaskBundleItems(ctx, task.TaskBundleID)
		if err != nil {
			return fmt.Errorf("list bundle items: %w", err)
		}
		result.Items = items
		return nil
	}); err != nil {
		return nil, err
	}
	for _, issue := range issuesToBroadcast {
		s.broadcastIssueUpdated(issue)
	}
	return &result, nil
}

func issueStatusForBundleItemStatus(status string) string {
	switch status {
	case "completed":
		return "in_review"
	case "cancelled":
		return "cancelled"
	default:
		return "blocked"
	}
}

func (s *TaskService) resolveConnectorDelegatedUser(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID, fallback pgtype.UUID) pgtype.UUID {
	if triggerCommentID.Valid {
		comment, err := s.Queries.GetComment(ctx, triggerCommentID)
		if err != nil {
			slog.Warn("connector delegated user: load trigger comment failed",
				"issue_id", util.UUIDToString(issue.ID),
				"trigger_comment_id", util.UUIDToString(triggerCommentID),
				"error", err,
			)
		} else {
			switch comment.AuthorType {
			case "member":
				if comment.AuthorID.Valid {
					return comment.AuthorID
				}
			case "agent":
				if comment.AuthorID.Valid {
					if inherited := s.latestConnectorDelegatedUser(ctx, issue.ID, comment.AuthorID); inherited.Valid {
						return inherited
					}
				}
			}
		}
	}

	if fallback.Valid {
		return fallback
	}
	if issue.CreatorType == "member" && issue.CreatorID.Valid {
		return issue.CreatorID
	}
	if agentID.Valid {
		if inherited := s.latestConnectorDelegatedUser(ctx, issue.ID, agentID); inherited.Valid {
			return inherited
		}
	}
	return pgtype.UUID{}
}

func (s *TaskService) latestConnectorDelegatedUser(ctx context.Context, issueID pgtype.UUID, agentID pgtype.UUID) pgtype.UUID {
	if !issueID.Valid || !agentID.Valid {
		return pgtype.UUID{}
	}
	delegatedUserID, err := s.Queries.GetLatestConnectorDelegatedUserForIssueAndAgent(ctx, db.GetLatestConnectorDelegatedUserForIssueAndAgentParams{
		IssueID: issueID,
		AgentID: agentID,
	})
	if err == nil {
		return delegatedUserID
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("connector delegated user: load latest task identity failed",
			"issue_id", util.UUIDToString(issueID),
			"agent_id", util.UUIDToString(agentID),
			"error", err,
		)
	}
	return pgtype.UUID{}
}

// QuickCreateContext is the JSON payload stored on a quick-create task's
// context column. The daemon detects this variant via Type == "quick_create"
// and switches to the quick-create prompt template; the completion path
// uses RequesterID + WorkspaceID to write the inbox notification.
//
// ProjectID is the optional project the user picked in the modal. When
// non-empty the daemon claim handler resolves the project's title +
// resources, and the prompt template instructs the agent to pass
// `--project <uuid>` so the new issue lands in that project.
//
// SquadID is non-empty when the user picked a squad (rather than an agent)
// in the modal. The task is still enqueued against the squad's leader
// agent (Queries.CreateQuickCreateTask is agent-scoped); SquadID is the
// hint the daemon claim handler uses to layer the squad-leader briefing
// onto the agent's Instructions, matching the behavior of issue-bound
// tasks assigned to the squad.
type QuickCreateContext struct {
	Type        string `json:"type"`
	Prompt      string `json:"prompt"`
	RequesterID string `json:"requester_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id,omitempty"`
	SquadID     string `json:"squad_id,omitempty"`
}

// QuickCreateContextType marks a task as a quick-create job.
const QuickCreateContextType = "quick_create"

// EnqueueQuickCreateTask creates a queued task that has no issue / chat /
// autopilot link — the user's natural-language prompt is stored in the
// task's context JSONB and the agent is expected to translate it into a
// `multica issue create` call. Pre-validates that the agent is reachable
// (not archived, has a runtime) so the API can reject up-front rather than
// queue a task no one will ever claim.
//
// projectID is optional (zero-valued pgtype.UUID when the user didn't pick
// one). The handler is responsible for validating it belongs to the same
// workspace before passing it in.
//
// squadID is non-empty (Valid) when the user picked a squad as the actor.
// The handler has already resolved it to the squad's leader agent for
// agentID; the squadID hint is stamped into the task context so the daemon
// claim handler can inject the squad-leader briefing on dispatch.
func (s *TaskService) EnqueueQuickCreateTask(ctx context.Context, workspaceID, requesterID pgtype.UUID, agentID, squadID pgtype.UUID, prompt string, projectID pgtype.UUID) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	payload := QuickCreateContext{
		Type:        QuickCreateContextType,
		Prompt:      prompt,
		RequesterID: util.UUIDToString(requesterID),
		WorkspaceID: util.UUIDToString(workspaceID),
	}
	if projectID.Valid {
		payload.ProjectID = util.UUIDToString(projectID)
	}
	if squadID.Valid {
		payload.SquadID = util.UUIDToString(squadID)
	}
	contextJSON, err := json.Marshal(payload)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("marshal quick-create context: %w", err)
	}

	task, err := s.Queries.CreateQuickCreateTask(ctx, db.CreateQuickCreateTaskParams{
		AgentID:                  agentID,
		RuntimeID:                agent.RuntimeID,
		Priority:                 priorityToInt("high"),
		Context:                  contextJSON,
		ConnectorDelegatedUserID: requesterID,
	})
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("create quick-create task: %w", err)
	}

	slog.Info("quick-create task enqueued",
		"task_id", util.UUIDToString(task.ID),
		"agent_id", util.UUIDToString(agentID),
		"squad_id", payload.SquadID,
		"requester_id", util.UUIDToString(requesterID),
		"workspace_id", util.UUIDToString(workspaceID),
		"project_id", payload.ProjectID,
	)
	// Match every other Enqueue* path: kick the daemon WS so the task
	// gets claimed promptly instead of waiting for the next 30 s poll
	// cycle. Without this the user perceives "quick create never
	// triggered" because the modal closes immediately and the task
	// sits in 'queued' until the next sleepWithContextOrWakeup tick.
	s.NotifyTaskEnqueued(ctx, task)
	return task, nil
}

// EnqueueChatTask creates a queued task for a chat session.
// Unlike issue tasks, chat tasks have no issue_id. triggerMessageID points at
// the exact user message that created this turn so daemon claim does not need
// to scan the whole transcript and guess "latest user message".
func (s *TaskService) EnqueueChatTask(ctx context.Context, chatSession db.ChatSession, triggerMessageID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueChatTaskForAgent(ctx, chatSession, chatSession.AgentID, triggerMessageID, pgtype.UUID{}, pgtype.UUID{}, ChatTaskKindNormal)
}

func (s *TaskService) EnqueuePlanLeadTask(ctx context.Context, chatSession db.ChatSession, planRun db.ChatPlanRun, triggerMessageID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueChatTaskForAgent(ctx, chatSession, planRun.LeadAgentID, triggerMessageID, planRun.ID, pgtype.UUID{}, ChatTaskKindPlanLead)
}

func (s *TaskService) EnqueuePlanConsultationTask(ctx context.Context, chatSession db.ChatSession, planRun db.ChatPlanRun, consultation db.ChatPlanConsultation) (db.AgentTaskQueue, error) {
	return s.EnqueueChatTaskForAgent(ctx, chatSession, consultation.TargetAgentID, consultation.RequestMessageID, planRun.ID, consultation.ID, ChatTaskKindPlanConsultation)
}

func (s *TaskService) EnqueueChatTaskForAgent(ctx context.Context, chatSession db.ChatSession, agentID pgtype.UUID, triggerMessageID, planRunID, consultationID pgtype.UUID, chatTaskKind string) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("chat task enqueue failed", "chat_session_id", util.UUIDToString(chatSession.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}
	if strings.TrimSpace(chatTaskKind) == "" {
		chatTaskKind = ChatTaskKindNormal
	}

	task, err := s.Queries.CreateChatTask(ctx, db.CreateChatTaskParams{
		AgentID:                  agentID,
		RuntimeID:                agent.RuntimeID,
		Priority:                 2, // medium priority for chat
		ChatSessionID:            chatSession.ID,
		TriggerChatMessageID:     triggerMessageID,
		ChatPlanRunID:            planRunID,
		ChatPlanConsultationID:   consultationID,
		ChatTaskKind:             chatTaskKind,
		ConnectorDelegatedUserID: chatSession.CreatorID,
	})
	if err != nil {
		slog.Error("chat task enqueue failed", "chat_session_id", util.UUIDToString(chatSession.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create chat task: %w", err)
	}

	slog.Info("chat task enqueued", "task_id", util.UUIDToString(task.ID), "chat_session_id", util.UUIDToString(chatSession.ID), "trigger_chat_message_id", util.UUIDToString(triggerMessageID), "agent_id", util.UUIDToString(agentID), "chat_task_kind", chatTaskKind)
	// See EnqueueTaskForIssue for ordering rationale.
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
	return task, nil
}

// CancelTasksForIssue cancels every active task on the issue, reconciles each
// affected agent's status, and broadcasts task:cancelled events so frontends
// clear their live cards.
//
// Before #1587 this path was "cancel rows and return" — issue-status flips
// (e.g. user marks the issue `done` or `cancelled` while a task is still
// running) left the agent stuck at status="working" indefinitely, requiring a
// manual `multica agent update <id> --status idle` to unwedge. Matches the
// pattern already used by CancelTask and RerunIssue.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	cancelled, err := s.Queries.CancelAgentTasksByIssue(ctx, issueID)
	if err != nil {
		return err
	}
	for _, t := range cancelled {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusCancelled); err != nil {
			slog.Warn("cancel issue tasks: update workflow run status failed", "task_id", util.UUIDToString(t.ID), "error", err)
		}
		s.captureTaskCancelled(ctx, t)
		s.ReconcileAgentStatus(ctx, t.AgentID)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
	}
	return nil
}

// CancelTasksForAgent cancels every active task belonging to an agent
// (queued + dispatched + running), reconciles the agent's status, and
// broadcasts task:cancelled events. Used by the agent-level "Cancel all
// tasks" action — same shape as CancelTasksForIssue but scoped on agent_id.
//
// Returns the cancelled rows so callers can report counts / log them.
func (s *TaskService) CancelTasksForAgent(ctx context.Context, agentID pgtype.UUID) ([]db.AgentTaskQueue, error) {
	cancelled, err := s.Queries.CancelAgentTasksByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for _, t := range cancelled {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusCancelled); err != nil {
			slog.Warn("cancel agent tasks: update workflow run status failed", "task_id", util.UUIDToString(t.ID), "error", err)
		}
		s.captureTaskCancelled(ctx, t)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
	}
	// Reconcile once after the loop — agent transitions from
	// working→available based on remaining task counts, no need to call
	// per row (the rows we just cancelled all belong to the same agent).
	s.ReconcileAgentStatus(ctx, agentID)
	return cancelled, nil
}

// CancelTasksByTriggerComment cancels active tasks whose trigger is the given
// comment. Called from DeleteComment so an agent does not run with the
// now-deleted content already embedded in its prompt. Must be invoked BEFORE
// the comment row is deleted because the FK ON DELETE SET NULL would
// otherwise nullify trigger_comment_id and we'd lose the ability to find
// the affected tasks.
func (s *TaskService) CancelTasksByTriggerComment(ctx context.Context, commentID pgtype.UUID) error {
	cancelled, err := s.Queries.CancelAgentTasksByTriggerComment(ctx, commentID)
	if err != nil {
		return err
	}
	for _, t := range cancelled {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusCancelled); err != nil {
			slog.Warn("cancel comment tasks: update workflow run status failed", "task_id", util.UUIDToString(t.ID), "error", err)
		}
		s.captureTaskCancelled(ctx, t)
		s.ReconcileAgentStatus(ctx, t.AgentID)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
	}
	return nil
}

// BroadcastCancelledTasks reconciles each affected agent's status and emits
// task:cancelled for every row. Callers must invoke this AFTER committing the
// cancellation so subscribers don't observe a "cancelled" event for a row
// that the tx might still roll back.
func (s *TaskService) BroadcastCancelledTasks(ctx context.Context, cancelled []db.AgentTaskQueue) {
	for _, t := range cancelled {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusCancelled); err != nil {
			slog.Warn("broadcast cancelled tasks: update workflow run status failed", "task_id", util.UUIDToString(t.ID), "error", err)
		}
		s.captureTaskCancelled(ctx, t)
		s.ReconcileAgentStatus(ctx, t.AgentID)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
	}
}

func (s *TaskService) CaptureCancelledTasks(ctx context.Context, cancelled []db.AgentTaskQueue) {
	for _, t := range cancelled {
		s.captureTaskCancelled(ctx, t)
	}
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	err := s.runInTx(ctx, func(qtx *db.Queries) error {
		t, err := qtx.CancelAgentTask(ctx, taskID)
		if err != nil {
			return err
		}
		task = t
		return s.setWorkflowRunStatusForTask(ctx, qtx, task.ID, workflowRunStatusCancelled)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, err := s.Queries.GetAgentTask(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("cancel task: %w", err)
		}
		return &existing, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	s.captureTaskCancelled(ctx, task)

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	return s.claimTask(ctx, agentID, pgtype.UUID{})
}

func (s *TaskService) claimTask(ctx context.Context, agentID pgtype.UUID, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	start := time.Now()
	var (
		outcome                                                              = "unknown"
		getAgentMs, countRunningMs, claimAgentMs, updateStatusMs, dispatchMs int64
	)
	defer func() {
		s.maybeLogClaimSlow(agentID, outcome, start, getAgentMs, countRunningMs, claimAgentMs, updateStatusMs, dispatchMs)
	}()

	t0 := start
	agent, err := s.Queries.GetAgent(ctx, agentID)
	getAgentMs = time.Since(t0).Milliseconds()
	if err != nil {
		outcome = "error_get_agent"
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	t0 = time.Now()
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	countRunningMs = time.Since(t0).Milliseconds()
	if err != nil {
		outcome = "error_count_running"
		return nil, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		slog.Debug("task claim: no capacity", "agent_id", util.UUIDToString(agentID), "running", running, "max", agent.MaxConcurrentTasks)
		outcome = "no_capacity"
		return nil, nil // No capacity
	}

	t0 = time.Now()
	var task db.AgentTaskQueue
	if runtimeID.Valid {
		task, err = s.Queries.ClaimAgentTaskForRuntime(ctx, db.ClaimAgentTaskForRuntimeParams{
			AgentID:   agentID,
			RuntimeID: runtimeID,
		})
	} else {
		task, err = s.Queries.ClaimAgentTask(ctx, agentID)
	}
	claimAgentMs = time.Since(t0).Milliseconds()
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			outcome = "no_tasks"
			return nil, nil // No tasks available
		}
		outcome = "error_claim"
		return nil, fmt.Errorf("claim task: %w", err)
	}

	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "agent_id", util.UUIDToString(agentID))
	s.captureTaskDispatched(ctx, task)

	// Refresh agent status from active tasks. This avoids a stale unconditional
	// working write racing after a just-cancelled claim.
	t0 = time.Now()
	s.ReconcileAgentStatus(ctx, agentID)
	updateStatusMs = time.Since(t0).Milliseconds()

	// Broadcast task:dispatch. ResolveTaskWorkspaceID inside this path can
	// re-query issue/chat_session/autopilot_run, so it can also be a real
	// contributor to claim latency.
	t0 = time.Now()
	s.broadcastTaskDispatch(ctx, task)
	dispatchMs = time.Since(t0).Milliseconds()

	outcome = "claimed"
	return &task, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
//
// Empty-claim fast path: when EmptyClaim is configured and a recent
// check verified the runtime had no queued tasks, returns immediately
// without touching Postgres. The cache is invalidated synchronously on
// every enqueue (notifyTaskAvailable), so a queued task becomes
// claimable on the next call rather than waiting for the TTL.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	start := time.Now()
	var (
		outcome          = "no_task"
		listMs, loopMs   int64
		listCount, tried int
		claimedFlag      bool
	)
	defer func() {
		totalMs := time.Since(start).Milliseconds()
		if totalMs < 300 {
			return
		}
		slog.Info("claim_for_runtime slow",
			"runtime_id", util.UUIDToString(runtimeID),
			"outcome", outcome,
			"total_ms", totalMs,
			"list_pending_ms", listMs,
			"list_pending_count", listCount,
			"agents_tried", tried,
			"claim_loop_ms", loopMs,
			"claimed", claimedFlag,
		)
	}()

	runtimeKey := util.UUIDToString(runtimeID)
	if s.EmptyClaim.IsEmpty(ctx, runtimeKey) {
		outcome = "empty_cache_hit"
		return nil, nil
	}

	// Sample the invalidation version BEFORE the SELECT. If a
	// concurrent enqueue Bumps between this read and the post-SELECT
	// MarkEmpty, the next IsEmpty will see the empty key tagged with
	// a stale version and reject it — closing the race that would
	// otherwise stall the just-queued task until the empty key's TTL
	// expired.
	preSelectVersion := s.EmptyClaim.CurrentVersion(ctx, runtimeKey)

	t0 := time.Now()
	tasks, err := s.Queries.ListQueuedClaimCandidatesByRuntime(ctx, runtimeID)
	listMs = time.Since(t0).Milliseconds()
	listCount = len(tasks)
	if err != nil {
		outcome = "error_list"
		return nil, fmt.Errorf("list queued claim candidates: %w", err)
	}

	if len(tasks) == 0 {
		s.EmptyClaim.MarkEmpty(ctx, runtimeKey, preSelectVersion)
		outcome = "empty_db"
		return nil, nil
	}

	loopStart := time.Now()
	triedAgents := map[string]struct{}{}
	var claimed *db.AgentTaskQueue
	for _, candidate := range tasks {
		agentKey := util.UUIDToString(candidate.AgentID)
		if _, seen := triedAgents[agentKey]; seen {
			continue
		}
		triedAgents[agentKey] = struct{}{}
		tried++

		task, err := s.claimTask(ctx, candidate.AgentID, runtimeID)
		if err != nil {
			loopMs = time.Since(loopStart).Milliseconds()
			outcome = "error_claim"
			return nil, err
		}
		if task != nil {
			claimed = task
			break
		}
	}
	loopMs = time.Since(loopStart).Milliseconds()
	if claimed != nil {
		claimedFlag = true
		outcome = "claimed"
	}

	return claimed, nil
}

// maybeLogClaimSlow emits one structured log per ClaimTask call when its total
// latency exceeds 300ms, so the prod tail can be diagnosed without flooding
// logs at normal poll rates. Called via defer so it captures the full path
// including post-claim updateAgentStatus / broadcastTaskDispatch (both of
// which can hit the DB) and any error exit.
func (s *TaskService) maybeLogClaimSlow(agentID pgtype.UUID, outcome string, start time.Time, getAgentMs, countRunningMs, claimAgentMs, updateStatusMs, dispatchMs int64) {
	totalMs := time.Since(start).Milliseconds()
	if totalMs < 300 {
		return
	}
	slog.Info("claim_task slow",
		"agent_id", util.UUIDToString(agentID),
		"outcome", outcome,
		"total_ms", totalMs,
		"get_agent_ms", getAgentMs,
		"count_running_ms", countRunningMs,
		"claim_agent_ms", claimAgentMs,
		"update_status_ms", updateStatusMs,
		"dispatch_ms", dispatchMs,
	)
}

// StartTask transitions a dispatched task to running.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) StartTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	var issuesToBroadcast []db.Issue
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		t, err := qtx.StartAgentTask(ctx, taskID)
		if err != nil {
			return err
		}
		task = t
		if task.TaskBundleID.Valid {
			if _, err := qtx.StartTaskBundle(ctx, task.TaskBundleID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("start task bundle: %w", err)
			}
			item, err := qtx.StartNextTaskBundleItem(ctx, task.TaskBundleID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("start first bundle item: %w", err)
			}
			if err == nil {
				updated, updateErr := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
					ID:     item.IssueID,
					Status: "in_progress",
				})
				if updateErr != nil {
					return fmt.Errorf("update first bundle issue status: %w", updateErr)
				}
				issuesToBroadcast = append(issuesToBroadcast, updated)
			}
		}
		if err := s.setWorkflowRunStatusForTask(ctx, qtx, task.ID, workflowRunStatusRunning); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	s.captureTaskStarted(ctx, task)
	for _, issue := range issuesToBroadcast {
		s.broadcastIssueUpdated(issue)
	}
	return &task, nil
}

// CompleteTask marks a task as completed.
// Issue status is NOT changed here — the agent manages it via the CLI.
//
// For chat tasks, CompleteAgentTask and the chat_session resume-pointer
// update run in a single transaction. This closes a race where the next
// queued chat message could be claimed in the window between the task
// flipping to 'completed' and chat_session.session_id being refreshed,
// causing the new task to resume against a stale (or NULL) session.
func (s *TaskService) CompleteTask(ctx context.Context, taskID pgtype.UUID, result []byte, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	var issuesToBroadcast []db.Issue
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		t, err := qtx.CompleteAgentTask(ctx, db.CompleteAgentTaskParams{
			ID:        taskID,
			Result:    result,
			SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
			WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
		})
		if err != nil {
			return err
		}
		task = t

		if t.ChatSessionID.Valid {
			// Pin the chat_session's runtime_id alongside the session_id so the
			// next claim can apply the runtime-guard. Both fields move together:
			// when there's no session_id to record, leave runtime_id untouched
			// (NULL → COALESCE keeps the existing value).
			var sessionRuntimeID pgtype.UUID
			if sessionID != "" {
				sessionRuntimeID = t.RuntimeID
			}
			// COALESCE in SQL guarantees empty inputs don't wipe the
			// existing resume pointer; we still surface DB errors.
			if err := qtx.UpdateChatSessionSession(ctx, db.UpdateChatSessionSessionParams{
				ID:        t.ChatSessionID,
				SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
				WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
				RuntimeID: sessionRuntimeID,
			}); err != nil {
				return fmt.Errorf("update chat session resume pointer: %w", err)
			}
		}
		if err := s.setWorkflowRunStatusForTask(ctx, qtx, t.ID, workflowRunStatusCompleted); err != nil {
			return err
		}
		if t.TaskBundleID.Valid {
			unfinished, err := qtx.FinishUnfinishedTaskBundleItems(ctx, db.FinishUnfinishedTaskBundleItemsParams{
				BundleID: t.TaskBundleID,
				Status:   "blocked",
				Error:    pgtype.Text{String: "bundle execution ended before this item was checkpointed", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("finish unfinished bundle items: %w", err)
			}
			for _, item := range unfinished {
				updated, updateErr := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
					ID:     item.IssueID,
					Status: "blocked",
				})
				if updateErr != nil {
					return fmt.Errorf("update unfinished bundle issue status: %w", updateErr)
				}
				issuesToBroadcast = append(issuesToBroadcast, updated)
			}
			if _, err := qtx.CompleteTaskBundleIfDone(ctx, t.TaskBundleID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("complete task bundle: %w", err)
			}
		}
		return nil
	}); err != nil {
		// When parallel agents race, a task may already be completed,
		// cancelled, or failed by the time this call runs. The UPDATE
		// … WHERE status = 'running' returns no rows in that case.
		// Treat it as an idempotent success — same pattern as CancelTask.
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			if errors.Is(err, pgx.ErrNoRows) {
				slog.Info("complete task: already finalized",
					"task_id", util.UUIDToString(taskID),
					"current_status", existing.Status,
					"agent_id", util.UUIDToString(existing.AgentID),
				)
				if existing.Status == "waiting" && (sessionID != "" || workDir != "") {
					if pinErr := s.Queries.UpdateAgentTaskSession(ctx, db.UpdateAgentTaskSessionParams{
						ID:        taskID,
						SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
						WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
					}); pinErr != nil {
						slog.Warn("complete waiting task: pin session failed",
							"task_id", util.UUIDToString(taskID),
							"error", pinErr,
						)
					}
				}
				return &existing, nil
			}
			slog.Warn("complete task failed",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"chat_session_id", util.UUIDToString(existing.ChatSessionID),
				"agent_id", util.UUIDToString(existing.AgentID),
				"error", err,
			)
		} else {
			slog.Warn("complete task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	s.captureTaskCompleted(ctx, task)
	for _, issue := range issuesToBroadcast {
		s.broadcastIssueUpdated(issue)
	}

	// Invariant: every completed issue task must have at least one agent
	// comment on the issue, so the user always sees something when a run
	// ends. If the agent posted a comment during execution (result, progress
	// ping, or CLI reply), HasAgentCommentedSince returns true and we skip.
	// Otherwise, synthesize one from the final output. For comment-triggered
	// tasks, TriggerCommentID threads the fallback under the original comment;
	// for assignment-triggered tasks it is NULL and the fallback is top-level.
	// Chat tasks have no IssueID and are handled separately below.
	if task.IssueID.Valid {
		suppressNoActionComment, err := HasSquadLeaderNoActionEvaluationForTask(ctx, s.Queries, task)
		if err != nil {
			slog.Warn("checking squad leader no_action evaluation failed",
				"task_id", util.UUIDToString(task.ID),
				"issue_id", util.UUIDToString(task.IssueID),
				"agent_id", util.UUIDToString(task.AgentID),
				"error", err,
			)
		}
		agentCommented, _ := s.Queries.HasAgentCommentedSince(ctx, db.HasAgentCommentedSinceParams{
			IssueID:  task.IssueID,
			AuthorID: task.AgentID,
			Since:    task.StartedAt,
		})
		if !suppressNoActionComment && !agentCommented {
			var payload protocol.TaskCompletedPayload
			if err := json.Unmarshal(result, &payload); err == nil {
				if payload.Output != "" {
					// Match the CLI's --content / --description behavior: agents that
					// emit literal `\n` 4-char sequences (Python/JSON-style) get them
					// decoded into real newlines before the comment hits the DB. See
					// util.UnescapeBackslashEscapes for the exact contract.
					body := util.UnescapeBackslashEscapes(payload.Output)
					if task.TriggerCommentID.Valid && isTrivialDoneOutput(body) {
						slog.Warn("suppressing trivial comment-trigger fallback output",
							"task_id", util.UUIDToString(task.ID),
							"issue_id", util.UUIDToString(task.IssueID),
							"agent_id", util.UUIDToString(task.AgentID),
						)
					} else {
						s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(body), "comment", task.TriggerCommentID)
					}
				}
			}
		}
	}

	// Quick-create tasks: locate the issue the agent just created and push
	// an inbox confirmation to the requester. The agent has no issue / chat
	// link, so the regular completion paths above don't apply. We find the
	// new issue by querying for the most recent issue this agent created in
	// the requester's workspace since the task started — more robust than
	// parsing the agent's stdout for an identifier.
	if qc, ok := s.parseQuickCreateContext(task); ok {
		s.notifyQuickCreateCompleted(ctx, task, qc)
	}

	// For chat tasks, save assistant reply and broadcast chat:done. The
	// resume pointer was already persisted inside the transaction above.
	if task.ChatSessionID.Valid {
		var assistantMsg *db.ChatMessage
		var payload protocol.TaskCompletedPayload
		if err := json.Unmarshal(result, &payload); err == nil && payload.Output != "" {
			// Same unescape as the issue-comment path above: literal `\n` from
			// agent stdout becomes a real newline so the chat panel renders
			// paragraph breaks instead of one wall of prose.
			body := util.UnescapeBackslashEscapes(payload.Output)
			row, err := s.Queries.CreateChatMessage(ctx, db.CreateChatMessageParams{
				ChatSessionID:  task.ChatSessionID,
				Role:           "assistant",
				Content:        redact.Text(body),
				TaskID:         task.ID,
				ElapsedMs:      computeChatElapsedMs(task),
				AuthorType:     "agent",
				AuthorAgentID:  task.AgentID,
				PlanRunID:      task.ChatPlanRunID,
				ConsultationID: task.ChatPlanConsultationID,
			})
			if err != nil {
				slog.Error("failed to save assistant chat message", "task_id", util.UUIDToString(task.ID), "error", err)
			} else {
				assistantMsg = &row
				// Event-driven unread: stamp unread_since on the first unread
				// assistant message. No-op if the session already has unread.
				// If the user is actively viewing the session, the frontend's
				// auto-mark-read effect will clear this within a tick.
				if err := s.Queries.SetUnreadSinceIfNull(ctx, task.ChatSessionID); err != nil {
					slog.Warn("failed to set unread_since", "chat_session_id", util.UUIDToString(task.ChatSessionID), "error", err)
				}
				if err := s.handleChatPlanTaskCompleted(ctx, task, row, body); err != nil {
					slog.Warn("chat plan completion handling failed",
						"task_id", util.UUIDToString(task.ID),
						"chat_plan_run_id", util.UUIDToString(task.ChatPlanRunID),
						"chat_plan_consultation_id", util.UUIDToString(task.ChatPlanConsultationID),
						"error", err,
					)
				}
				if !task.ChatPlanRunID.Valid {
					if err := s.handleOrdinaryChatTaskCompleted(ctx, task, row, body); err != nil {
						slog.Warn("chat routing completion handling failed",
							"task_id", util.UUIDToString(task.ID),
							"chat_session_id", util.UUIDToString(task.ChatSessionID),
							"error", err,
						)
					}
				}
			}
		}
		s.broadcastChatDone(ctx, task, assistantMsg)
	}

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task)

	return &task, nil
}

// FailTask marks a task as failed.
// Assignment issue failures are moved to blocked when no retry is pending.
//
// sessionID/workDir are optional: when the agent established a real session
// before failing (e.g. crashed mid-conversation, was cancelled, or hit a
// tool error), the daemon should pass them so we can preserve the resume
// pointer on both the task row and the chat_session — otherwise the next
// chat turn would silently start a brand-new session and lose memory.
//
// failureReason is a coarse classifier consumed by the auto-retry path.
// Pass "" when unknown (treated as 'agent_error').
func (s *TaskService) FailTask(ctx context.Context, taskID pgtype.UUID, errMsg, sessionID, workDir, failureReason string) (*db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	var bundleIssuesToBroadcast []db.Issue
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		t, err := qtx.FailAgentTask(ctx, db.FailAgentTaskParams{
			ID:            taskID,
			Error:         pgtype.Text{String: errMsg, Valid: true},
			FailureReason: pgtype.Text{String: failureReason, Valid: failureReason != ""},
			SessionID:     pgtype.Text{String: sessionID, Valid: sessionID != ""},
			WorkDir:       pgtype.Text{String: workDir, Valid: workDir != ""},
		})
		if err != nil {
			return err
		}
		task = t

		if t.ChatSessionID.Valid {
			// Pin the chat_session's runtime_id alongside the session_id so the
			// next claim can apply the runtime-guard. Both fields move together:
			// when there's no session_id to record, leave runtime_id untouched
			// (NULL → COALESCE keeps the existing value).
			var sessionRuntimeID pgtype.UUID
			if sessionID != "" {
				sessionRuntimeID = t.RuntimeID
			}
			if err := qtx.UpdateChatSessionSession(ctx, db.UpdateChatSessionSessionParams{
				ID:        t.ChatSessionID,
				SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
				WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
				RuntimeID: sessionRuntimeID,
			}); err != nil {
				return fmt.Errorf("update chat session resume pointer: %w", err)
			}
		}
		if err := s.setWorkflowRunStatusForTask(ctx, qtx, t.ID, workflowRunStatusFailed); err != nil {
			return err
		}
		if t.TaskBundleID.Valid {
			unfinished, err := qtx.FinishUnfinishedTaskBundleItems(ctx, db.FinishUnfinishedTaskBundleItemsParams{
				BundleID: t.TaskBundleID,
				Status:   "failed",
				Error:    pgtype.Text{String: errMsg, Valid: errMsg != ""},
			})
			if err != nil {
				return fmt.Errorf("fail unfinished bundle items: %w", err)
			}
			for _, item := range unfinished {
				updated, updateErr := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
					ID:     item.IssueID,
					Status: "blocked",
				})
				if updateErr != nil {
					return fmt.Errorf("update failed bundle issue status: %w", updateErr)
				}
				bundleIssuesToBroadcast = append(bundleIssuesToBroadcast, updated)
			}
			if _, err := qtx.FailTaskBundle(ctx, t.TaskBundleID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("fail task bundle: %w", err)
			}
		}
		return nil
	}); err != nil {
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			if errors.Is(err, pgx.ErrNoRows) {
				slog.Info("fail task: already finalized",
					"task_id", util.UUIDToString(taskID),
					"current_status", existing.Status,
					"agent_id", util.UUIDToString(existing.AgentID),
				)
				return &existing, nil
			}
			slog.Warn("fail task failed",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"chat_session_id", util.UUIDToString(existing.ChatSessionID),
				"agent_id", util.UUIDToString(existing.AgentID),
				"error", err,
			)
		} else {
			slog.Warn("fail task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg, "failure_reason", failureReason)
	s.captureTaskFailed(ctx, task)
	for _, issue := range bundleIssuesToBroadcast {
		s.broadcastIssueUpdated(issue)
	}

	// Auto-retry eligible failures (orphan, timeout, runtime_offline,
	// runtime_recovery). The helper itself enforces attempt < max_attempts
	// and only triggers for issue/chat tasks.
	retried, _ := s.MaybeRetryFailedTask(ctx, task)

	// Skip the per-failure system comment when we'll immediately retry —
	// the new task will surface its own status to the user, and we don't
	// want to spam the issue with "task timed out" messages on every
	// daemon hiccup.
	if task.IssueID.Valid && retried == nil {
		s.markIssueBlockedAfterTaskFailure(ctx, task)
	}
	if errMsg != "" && task.IssueID.Valid && retried == nil {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}

	// Mirror the issue fallback for chat tasks: write an assistant
	// chat_message tagged with the daemon-reported failure_reason so the
	// conversation history shows what happened. Skip when auto-retry is
	// pending (the new attempt will write its own outcome) — same guard as
	// the issue path above.
	if task.ChatSessionID.Valid && retried == nil {
		if _, err := s.Queries.CreateChatMessage(ctx, db.CreateChatMessageParams{
			ChatSessionID:  task.ChatSessionID,
			Role:           "assistant",
			Content:        redact.Text(errMsg),
			TaskID:         pgtype.UUID{Bytes: task.ID.Bytes, Valid: true},
			FailureReason:  pgtype.Text{String: failureReason, Valid: failureReason != ""},
			ElapsedMs:      computeChatElapsedMs(task),
			AuthorType:     "agent",
			AuthorAgentID:  task.AgentID,
			PlanRunID:      task.ChatPlanRunID,
			ConsultationID: task.ChatPlanConsultationID,
		}); err != nil {
			slog.Error("failed to save failure chat message",
				"task_id", util.UUIDToString(task.ID),
				"chat_session_id", util.UUIDToString(task.ChatSessionID),
				"error", err)
		} else if err := s.Queries.SetUnreadSinceIfNull(ctx, task.ChatSessionID); err != nil {
			slog.Warn("failed to set unread_since on failure",
				"chat_session_id", util.UUIDToString(task.ChatSessionID),
				"error", err)
		}
		if task.ChatPlanConsultationID.Valid {
			s.handleChatPlanConsultationFailed(ctx, task)
		}
	}

	// Quick-create tasks: push a failure inbox notification to the
	// requester so they can either retry or fall back to the advanced form
	// without losing their original prompt. Skipped when an auto-retry is
	// pending — the new attempt will write its own outcome.
	if retried == nil {
		if qc, ok := s.parseQuickCreateContext(task); ok {
			s.notifyQuickCreateFailed(ctx, task, qc, errMsg)
		}
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)

	return &task, nil
}

func (s *TaskService) markIssueBlockedAfterTaskFailure(ctx context.Context, task db.AgentTaskQueue) {
	if !task.IssueID.Valid || task.TriggerCommentID.Valid || task.ChatSessionID.Valid {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		slog.Warn("mark failed task issue blocked: load issue failed",
			"issue_id", util.UUIDToString(task.IssueID),
			"task_id", util.UUIDToString(task.ID),
			"error", err,
		)
		return
	}
	switch issue.Status {
	case "blocked", "done", "cancelled":
		return
	}
	updated, err := s.Queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
		ID:     task.IssueID,
		Status: "blocked",
	})
	if err != nil {
		slog.Warn("mark failed task issue blocked: update issue failed",
			"issue_id", util.UUIDToString(task.IssueID),
			"task_id", util.UUIDToString(task.ID),
			"error", err,
		)
		return
	}
	s.broadcastIssueUpdated(updated)
}

// retryableReasons enumerates failure reasons that the auto-retry path is
// allowed to act on. Agent-side errors (compile failures, model rejections,
// etc.) are intentionally excluded — those are real problems that the user
// should see, not infrastructure flakiness.
var retryableReasons = map[string]bool{
	"runtime_offline":  true,
	"runtime_recovery": true,
	"timeout":          true,
}

// MaybeRetryFailedTask spawns a fresh queued attempt for a recently-failed
// task when the failure was infrastructure-shaped (daemon crash, runtime
// went offline, dispatch/run timeout) and the task hasn't exhausted its
// max_attempts budget. The child task inherits agent/runtime/issue/chat
// links and the parent's session_id/work_dir so the agent can resume the
// conversation when the backend supports it. Returns the new task, or nil
// when no retry was created.
//
// Autopilot tasks are NOT auto-retried here; the autopilot scheduler owns
// its own re-run cadence and we don't want to double-fire it.
func (s *TaskService) MaybeRetryFailedTask(ctx context.Context, parent db.AgentTaskQueue) (*db.AgentTaskQueue, error) {
	if parent.Status != "failed" {
		return nil, nil
	}
	reason := ""
	if parent.FailureReason.Valid {
		reason = parent.FailureReason.String
	}
	if !retryableReasons[reason] {
		return nil, nil
	}
	if parent.Attempt >= parent.MaxAttempts {
		slog.Info("task auto-retry skipped: budget exhausted",
			"task_id", util.UUIDToString(parent.ID),
			"attempt", parent.Attempt,
			"max_attempts", parent.MaxAttempts,
		)
		return nil, nil
	}
	if parent.AutopilotRunID.Valid {
		// Autopilot has its own retry semantics; do not double-trigger.
		return nil, nil
	}
	if parent.ChatPlanConsultationID.Valid || parent.ChatTaskKind == ChatTaskKindPlanConsultation {
		// Consultations are bounded by design. A missing helper reply should
		// resume the lead with the gap recorded instead of retrying forever.
		return nil, nil
	}
	if !parent.IssueID.Valid && !parent.ChatSessionID.Valid {
		return nil, nil
	}

	var child db.AgentTaskQueue
	if err := s.runInTx(ctx, func(qtx *db.Queries) error {
		created, err := qtx.CreateRetryTask(ctx, parent.ID)
		if err != nil {
			return err
		}
		child = created
		return s.createWorkflowRunForTask(ctx, qtx, child)
	}); err != nil {
		slog.Warn("task auto-retry failed",
			"parent_task_id", util.UUIDToString(parent.ID),
			"reason", reason,
			"error", err,
		)
		return nil, err
	}
	slog.Info("task auto-retry enqueued",
		"parent_task_id", util.UUIDToString(parent.ID),
		"child_task_id", util.UUIDToString(child.ID),
		"reason", reason,
		"attempt", child.Attempt,
		"max_attempts", child.MaxAttempts,
	)
	// Retry creates a fresh queued row, same status transition (∅ → queued)
	// as EnqueueTaskFor*. Broadcast queued first, then notify the daemon —
	// see EnqueueTaskForIssue for ordering rationale.
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, child)
	s.NotifyTaskEnqueued(ctx, child)
	return &child, nil
}

// RerunIssue creates a fresh queued task for the agent currently assigned
// to the issue. Used by the manual rerun endpoint.
//
// The new task is flagged force_fresh_session=true so the daemon starts a
// clean agent session instead of resuming the prior (agent_id, issue_id)
// session. A user clicking rerun has just judged the prior output bad —
// resuming the same conversation would replay the same poisoned state.
// Auto-retry of an orphaned mid-flight failure (HandleFailedTasks →
// MaybeRetryFailedTask → CreateRetryTask) does NOT take this path, so
// MUL-1128's mid-flight resume contract is preserved.
//
// Only tasks belonging to the issue's current assignee are cancelled.
// Tasks owned by other agents on the same issue (e.g. a parallel
// @-mention agent) are left alone — rerun must not collateral-cancel
// them.
func (s *TaskService) RerunIssue(ctx context.Context, issueID pgtype.UUID, triggerCommentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("load issue: %w", err)
	}

	// Determine the target agent for the rerun.
	var agentID pgtype.UUID
	switch {
	case issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid:
		agentID = issue.AssigneeID
	case issue.AssigneeType.String == "squad" && issue.AssigneeID.Valid:
		squad, err := s.Queries.GetSquad(ctx, issue.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("issue is assigned to a squad but squad not found")
		}
		agentID = squad.LeaderID
	default:
		return nil, fmt.Errorf("issue is not assigned to an agent or squad")
	}

	// Cancel only the target agent's active/queued tasks on this issue.
	cancelled, err := s.Queries.CancelAgentTasksByIssueAndAgent(ctx, db.CancelAgentTasksByIssueAndAgentParams{
		IssueID: issueID,
		AgentID: agentID,
	})
	if err != nil {
		slog.Warn("rerun: cancel prior tasks failed",
			"issue_id", util.UUIDToString(issueID),
			"agent_id", util.UUIDToString(issue.AssigneeID),
			"error", err,
		)
	}
	for _, t := range cancelled {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusCancelled); err != nil {
			slog.Warn("rerun: update cancelled workflow run status failed",
				"task_id", util.UUIDToString(t.ID),
				"error", err,
			)
		}
		s.captureTaskCancelled(ctx, t)
		s.ReconcileAgentStatus(ctx, t.AgentID)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
	}

	delegatedUserID := s.latestConnectorDelegatedUser(ctx, issue.ID, agentID)
	task, err := s.enqueueRerunTask(ctx, issue, agentID, triggerCommentID, delegatedUserID)
	if err != nil {
		return nil, err
	}
	slog.Info("issue rerun enqueued",
		"task_id", util.UUIDToString(task.ID),
		"issue_id", util.UUIDToString(issueID),
		"agent_id", util.UUIDToString(agentID),
		"cancelled_prior", len(cancelled),
	)
	return &task, nil
}

// enqueueRerunTask enqueues a fresh task for the given agent on the issue.
// For agent-assigned issues it uses enqueueIssueTask (which reads AssigneeID);
// for squad-assigned issues the rerun targets the squad leader and is flagged
// as a leader task so the self-trigger guard treats it correctly.
func (s *TaskService) enqueueRerunTask(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID, delegatedUserID pgtype.UUID) (db.AgentTaskQueue, error) {
	if issue.AssigneeType.String == "agent" {
		return s.enqueueIssueTask(ctx, issue, triggerCommentID, true, delegatedUserID)
	}
	return s.EnqueueTaskForSquadLeaderByDelegatedUser(ctx, issue, agentID, triggerCommentID, delegatedUserID)
}

// HandleFailedTasks runs the post-failure side effects for a batch of
// freshly-failed tasks: optional auto-retry, task:failed event broadcast,
// agent status reconciliation, and (when an assignment issue has no remaining
// active task and isn't being retried) marking the issue blocked.
//
// All callers that surface a task as failed — sweepers, FailTask,
// recover-orphans — funnel through here so the same UI-consistency
// guarantees apply on every code path.
func (s *TaskService) HandleFailedTasks(ctx context.Context, tasks []db.AgentTaskQueue) int {
	if len(tasks) == 0 {
		return 0
	}

	affectedAgents := make(map[string]pgtype.UUID)
	processedIssues := make(map[string]bool)
	retriedIssues := make(map[string]bool)
	retried := 0

	for _, t := range tasks {
		if err := s.setWorkflowRunStatusForTask(ctx, s.Queries, t.ID, workflowRunStatusFailed); err != nil {
			slog.Warn("handle failed tasks: update workflow run status failed",
				"task_id", util.UUIDToString(t.ID),
				"error", err,
			)
		}
		if t.TaskBundleID.Valid {
			unfinished, err := s.Queries.FinishUnfinishedTaskBundleItems(ctx, db.FinishUnfinishedTaskBundleItemsParams{
				BundleID: t.TaskBundleID,
				Status:   "failed",
				Error:    pgtype.Text{String: taskFailureReason(t), Valid: true},
			})
			if err != nil {
				slog.Warn("handle failed tasks: fail bundle items failed",
					"task_id", util.UUIDToString(t.ID),
					"task_bundle_id", util.UUIDToString(t.TaskBundleID),
					"error", err,
				)
			}
			for _, item := range unfinished {
				updated, updateErr := s.Queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
					ID:     item.IssueID,
					Status: "blocked",
				})
				if updateErr != nil {
					slog.Warn("handle failed tasks: mark bundled issue blocked failed",
						"issue_id", util.UUIDToString(item.IssueID),
						"task_bundle_id", util.UUIDToString(t.TaskBundleID),
						"error", updateErr,
					)
				} else {
					s.broadcastIssueUpdated(updated)
				}
			}
			if _, err := s.Queries.FailTaskBundle(ctx, t.TaskBundleID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				slog.Warn("handle failed tasks: fail bundle failed",
					"task_id", util.UUIDToString(t.ID),
					"task_bundle_id", util.UUIDToString(t.TaskBundleID),
					"error", err,
				)
			}
		}
		// Auto-retry first so the issue stays in_progress rather than
		// flapping todo → in_progress within a tick.
		if child, _ := s.MaybeRetryFailedTask(ctx, t); child != nil {
			retried++
			if t.IssueID.Valid {
				retriedIssues[util.UUIDToString(t.IssueID)] = true
			}
		}
		if t.ChatPlanConsultationID.Valid {
			s.handleChatPlanConsultationFailed(ctx, t)
		}

		failureReason := "agent_error"
		if t.FailureReason.Valid && t.FailureReason.String != "" {
			failureReason = t.FailureReason.String
		}
		s.captureTaskFailed(ctx, t)

		workspaceID := ""
		if t.IssueID.Valid {
			if issue, err := s.Queries.GetIssue(ctx, t.IssueID); err == nil {
				workspaceID = util.UUIDToString(issue.WorkspaceID)
				// Mark stuck in_progress assignment issues blocked only when
				// no other active task exists for the issue and no retry was
				// just enqueued.
				issueKey := util.UUIDToString(t.IssueID)
				if !t.TriggerCommentID.Valid && !t.ChatSessionID.Valid && issue.Status == "in_progress" && !processedIssues[issueKey] && !retriedIssues[issueKey] {
					processedIssues[issueKey] = true
					hasActive, checkErr := s.Queries.HasActiveTaskForIssue(ctx, t.IssueID)
					if checkErr != nil {
						slog.Warn("handle failed tasks: active check failed",
							"issue_id", issueKey,
							"error", checkErr,
						)
					} else if !hasActive {
						updated, updateErr := s.Queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
							ID:     t.IssueID,
							Status: "blocked",
						})
						if updateErr != nil {
							slog.Warn("handle failed tasks: mark issue blocked failed",
								"issue_id", issueKey,
								"error", updateErr,
							)
						} else {
							s.broadcastIssueUpdated(updated)
						}
					}
				}
			}
		}
		if workspaceID == "" {
			workspaceID = s.ResolveTaskWorkspaceID(ctx, t)
		}

		if workspaceID != "" {
			s.Bus.Publish(events.Event{
				Type:        protocol.EventTaskFailed,
				WorkspaceID: workspaceID,
				ActorType:   "system",
				Payload: map[string]any{
					"task_id":        util.UUIDToString(t.ID),
					"agent_id":       util.UUIDToString(t.AgentID),
					"issue_id":       util.UUIDToString(t.IssueID),
					"status":         "failed",
					"failure_reason": failureReason,
				},
			})
		}

		affectedAgents[util.UUIDToString(t.AgentID)] = t.AgentID
	}

	for _, agentID := range affectedAgents {
		s.ReconcileAgentStatus(ctx, agentID)
	}
	return retried
}

var chatPlanAgentMentionRe = regexp.MustCompile(`mention://agent/([0-9a-fA-F-]{36})`)

func (s *TaskService) handleChatPlanTaskCompleted(ctx context.Context, task db.AgentTaskQueue, assistantMsg db.ChatMessage, output string) error {
	if !task.ChatPlanRunID.Valid {
		return nil
	}
	if task.ChatPlanConsultationID.Valid || task.ChatTaskKind == ChatTaskKindPlanConsultation {
		if !task.ChatPlanConsultationID.Valid {
			return nil
		}
		run, err := s.Queries.GetChatPlanRun(ctx, task.ChatPlanRunID)
		if err != nil {
			return err
		}
		if s.agentMentioned(ctx, output, run.LeadAgentID) {
			if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, assistantMsg.ID, "agent", run.LeadAgentID, run.LeadAgentID, "explicit_mention", "routed", pgtype.UUID{}, "", ""); err != nil {
				slog.Warn("chat plan helper lead edge create failed",
					"chat_plan_run_id", util.UUIDToString(run.ID),
					"message_id", util.UUIDToString(assistantMsg.ID),
					"error", err,
				)
			}
			if _, err := s.Queries.MarkChatPlanConsultationResponded(ctx, db.MarkChatPlanConsultationRespondedParams{
				ID:                task.ChatPlanConsultationID,
				ResponseMessageID: assistantMsg.ID,
			}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("mark consultation responded: %w", err)
			}
		} else if _, err := s.Queries.MarkChatPlanConsultationFailed(ctx, db.MarkChatPlanConsultationFailedParams{
			ID:     task.ChatPlanConsultationID,
			Status: "failed",
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("mark consultation missing lead mention: %w", err)
		} else {
			if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, assistantMsg.ID, "agent", run.LeadAgentID, run.LeadAgentID, "explicit_mention", "blocked", pgtype.UUID{}, "missing_lead_mention", "Helper reply must mention the lead agent before control returns to the lead."); err != nil {
				slog.Warn("chat plan missing lead edge create failed",
					"chat_plan_run_id", util.UUIDToString(run.ID),
					"message_id", util.UUIDToString(assistantMsg.ID),
					"error", err,
				)
			}
			s.broadcastChatPlanRunsUpdated(ctx, run)
			return nil
		}
		s.broadcastChatPlanRunsUpdated(ctx, run)
		return s.resumePlanLeadIfConsultationsClosed(ctx, task.ChatPlanRunID, assistantMsg.ID)
	}
	if task.ChatTaskKind != ChatTaskKindPlanLead {
		return nil
	}
	return s.enqueueConsultationsFromLeadMessage(ctx, task, assistantMsg, output)
}

func (s *TaskService) enqueueConsultationsFromLeadMessage(ctx context.Context, task db.AgentTaskQueue, assistantMsg db.ChatMessage, output string) error {
	run, err := s.Queries.GetChatPlanRun(ctx, task.ChatPlanRunID)
	if err != nil {
		return err
	}
	if run.ActorType != "squad" || !run.ActorID.Valid {
		return nil
	}
	if util.UUIDToString(run.LeadAgentID) != util.UUIDToString(task.AgentID) {
		return nil
	}

	session, err := s.Queries.GetChatSession(ctx, run.ChatSessionID)
	if err != nil {
		return err
	}
	eligible, err := s.planConsultationHelperSet(ctx, run)
	if err != nil {
		return err
	}
	if len(eligible) == 0 {
		return nil
	}

	mentioned := s.planLeadMentionedHelpers(ctx, output, eligible)
	createdAny := false
	closedAny := false
	limitReached := run.ConsultationWaveCount >= maxChatPlanConsultationWaves
	for _, agentID := range mentioned {
		key := util.UUIDToString(agentID)
		if _, ok := eligible[key]; !ok {
			if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, assistantMsg.ID, "agent", agentID, agentID, "explicit_mention", "blocked", pgtype.UUID{}, "out_of_scope", "Lead can only consult agents in the selected squad."); err != nil {
				slog.Warn("chat plan out-of-scope edge create failed",
					"plan_run_id", util.UUIDToString(run.ID),
					"target_agent_id", key,
					"error", err,
				)
			}
			continue
		}
		if limitReached {
			if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, assistantMsg.ID, "agent", agentID, agentID, "explicit_mention", "skipped", pgtype.UUID{}, "consultation_limit_reached", "Plan squad consultation is limited to 5 waves; the lead should summarize the current consensus."); err != nil {
				slog.Warn("chat plan limit edge create failed",
					"plan_run_id", util.UUIDToString(run.ID),
					"target_agent_id", key,
					"error", err,
				)
			}
			continue
		}
		consultation, err := s.Queries.CreateChatPlanConsultation(ctx, db.CreateChatPlanConsultationParams{
			PlanRunID:        run.ID,
			RequesterAgentID: run.LeadAgentID,
			TargetAgentID:    agentID,
			RequestMessageID: assistantMsg.ID,
		})
		if err != nil {
			slog.Warn("chat plan consultation create failed",
				"plan_run_id", util.UUIDToString(run.ID),
				"target_agent_id", key,
				"error", err,
			)
			continue
		}
		if consultation.TaskID.Valid || consultation.Status == "responded" || consultation.Status == "failed" || consultation.Status == "timed_out" || consultation.Status == "skipped" {
			continue
		}
		helperTask, err := s.EnqueuePlanConsultationTask(ctx, session, run, consultation)
		if err != nil {
			slog.Warn("chat plan consultation enqueue failed",
				"plan_run_id", util.UUIDToString(run.ID),
				"consultation_id", util.UUIDToString(consultation.ID),
				"target_agent_id", key,
				"error", err,
			)
			if _, markErr := s.Queries.MarkChatPlanConsultationFailed(ctx, db.MarkChatPlanConsultationFailedParams{
				ID:     consultation.ID,
				Status: "failed",
			}); markErr != nil && !errors.Is(markErr, pgx.ErrNoRows) {
				slog.Warn("chat plan consultation mark failed after enqueue error failed",
					"consultation_id", util.UUIDToString(consultation.ID),
					"error", markErr,
				)
			}
			closedAny = true
			continue
		}
		if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, assistantMsg.ID, "agent", agentID, agentID, "explicit_mention", "routed", helperTask.ID, "", ""); err != nil {
			slog.Warn("chat plan helper edge create failed",
				"plan_run_id", util.UUIDToString(run.ID),
				"consultation_id", util.UUIDToString(consultation.ID),
				"task_id", util.UUIDToString(helperTask.ID),
				"error", err,
			)
		}
		if _, err := s.Queries.SetChatPlanConsultationTask(ctx, db.SetChatPlanConsultationTaskParams{
			ID:     consultation.ID,
			TaskID: helperTask.ID,
		}); err != nil {
			slog.Warn("chat plan consultation task link failed",
				"plan_run_id", util.UUIDToString(run.ID),
				"consultation_id", util.UUIDToString(consultation.ID),
				"task_id", util.UUIDToString(helperTask.ID),
				"error", err,
			)
		}
		createdAny = true
	}
	if createdAny {
		updated, err := s.Queries.IncrementChatPlanRunConsultationWaveCount(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("increment plan consultation wave count: %w", err)
		}
		updated, err = s.Queries.UpdateChatPlanRunStatus(ctx, db.UpdateChatPlanRunStatusParams{
			ID:     run.ID,
			Status: "consulting",
		})
		if err != nil {
			return fmt.Errorf("mark plan consulting: %w", err)
		}
		s.broadcastChatPlanRunsUpdated(ctx, updated)
	} else if closedAny {
		return s.resumePlanLeadIfConsultationsClosed(ctx, run.ID, pgtype.UUID{})
	}
	return nil
}

func (s *TaskService) planConsultationHelperSet(ctx context.Context, run db.ChatPlanRun) (map[string]pgtype.UUID, error) {
	members, err := s.Queries.ListSquadMembers(ctx, run.ActorID)
	if err != nil {
		return nil, err
	}
	eligible := make(map[string]pgtype.UUID)
	leadID := util.UUIDToString(run.LeadAgentID)
	for _, member := range members {
		if member.MemberType != "agent" {
			continue
		}
		memberID := util.UUIDToString(member.MemberID)
		if memberID == "" || memberID == leadID {
			continue
		}
		agent, err := s.Queries.GetAgent(ctx, member.MemberID)
		if err != nil || agent.ArchivedAt.Valid {
			continue
		}
		eligible[memberID] = member.MemberID
	}
	return eligible, nil
}

func parseAgentMentions(content string) []pgtype.UUID {
	matches := chatPlanAgentMentionRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]pgtype.UUID, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id, err := util.ParseUUID(match[1])
		if err != nil {
			continue
		}
		key := util.UUIDToString(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	return out
}

func agentMentioned(content string, agentID pgtype.UUID) bool {
	if !agentID.Valid {
		return false
	}
	target := util.UUIDToString(agentID)
	for _, mentioned := range parseAgentMentions(content) {
		if util.UUIDToString(mentioned) == target {
			return true
		}
	}
	return false
}

func (s *TaskService) agentMentioned(ctx context.Context, content string, agentID pgtype.UUID) bool {
	if agentMentioned(content, agentID) {
		return true
	}
	if !agentID.Valid || !strings.Contains(content, "@") {
		return false
	}
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil || agent.ArchivedAt.Valid {
		return false
	}
	return hasPlainAgentMention(content, agent.Name)
}

func (s *TaskService) planLeadMentionedHelpers(ctx context.Context, content string, eligible map[string]pgtype.UUID) []pgtype.UUID {
	mentioned := parseAgentMentions(content)
	if len(eligible) == 0 || !strings.Contains(content, "@") {
		return mentioned
	}
	seen := make(map[string]struct{}, len(mentioned)+len(eligible))
	for _, agentID := range mentioned {
		seen[util.UUIDToString(agentID)] = struct{}{}
	}
	for key, agentID := range eligible {
		if _, ok := seen[key]; ok {
			continue
		}
		agent, err := s.Queries.GetAgent(ctx, agentID)
		if err != nil || agent.ArchivedAt.Valid {
			continue
		}
		if hasPlainAgentMention(content, agent.Name) {
			mentioned = append(mentioned, agentID)
			seen[key] = struct{}{}
		}
	}
	return mentioned
}

func hasPlainAgentMention(content, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	needle := "@" + strings.ToLower(name)
	haystack := strings.ToLower(content)
	for start := 0; start < len(haystack); {
		idx := strings.Index(haystack[start:], needle)
		if idx < 0 {
			return false
		}
		pos := start + idx
		if pos > 0 {
			before, _ := utf8.DecodeLastRuneInString(haystack[:pos])
			if isPlainMentionNameContinue(before) {
				start = pos + 1
				continue
			}
		}
		after := pos + len(needle)
		if after >= len(haystack) {
			return true
		}
		r, _ := utf8.DecodeRuneInString(haystack[after:])
		if !isPlainMentionNameContinue(r) {
			return true
		}
		start = after
	}
	return false
}

func isPlainMentionNameContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

type chatDirectedStateCandidate struct {
	RecipientType   string  `json:"recipient_type"`
	RecipientID     string  `json:"recipient_id"`
	ResolvedAgentID *string `json:"resolved_agent_id,omitempty"`
}

type serviceChatRoutingTarget struct {
	RecipientType   string
	RecipientID     pgtype.UUID
	ResolvedAgentID pgtype.UUID
	Status          string
	WarningCode     string
	WarningMessage  string
}

func (s *TaskService) handleOrdinaryChatTaskCompleted(ctx context.Context, task db.AgentTaskQueue, assistantMsg db.ChatMessage, output string) error {
	session, err := s.Queries.GetChatSession(ctx, task.ChatSessionID)
	if err != nil {
		return err
	}
	targets := s.resolveOrdinaryChatAgentMentions(ctx, session.WorkspaceID, task.AgentID, output)
	if len(targets) == 0 {
		return s.upsertChatDirectedActive(ctx, session, assistantMsg.ID, task.AgentID)
	}

	routable := make([]serviceChatRoutingTarget, 0, len(targets))
	for _, target := range targets {
		if target.Status != "routed" || !target.ResolvedAgentID.Valid {
			if err := s.createChatRoutingEdge(ctx, session.WorkspaceID, session.ID, assistantMsg.ID, target.RecipientType, target.RecipientID, target.ResolvedAgentID, "explicit_mention", target.Status, pgtype.UUID{}, target.WarningCode, target.WarningMessage); err != nil {
				slog.Warn("ordinary chat blocked edge create failed",
					"message_id", util.UUIDToString(assistantMsg.ID),
					"recipient_id", util.UUIDToString(target.RecipientID),
					"error", err,
				)
			}
			continue
		}
		child, err := s.EnqueueChatTaskForAgent(ctx, session, target.ResolvedAgentID, assistantMsg.ID, pgtype.UUID{}, pgtype.UUID{}, ChatTaskKindNormal)
		if err != nil {
			if edgeErr := s.createChatRoutingEdge(ctx, session.WorkspaceID, session.ID, assistantMsg.ID, target.RecipientType, target.RecipientID, target.ResolvedAgentID, "explicit_mention", "failed", pgtype.UUID{}, "enqueue_failed", err.Error()); edgeErr != nil {
				slog.Warn("ordinary chat failed edge create failed",
					"message_id", util.UUIDToString(assistantMsg.ID),
					"recipient_id", util.UUIDToString(target.RecipientID),
					"error", edgeErr,
				)
			}
			continue
		}
		if err := s.createChatRoutingEdge(ctx, session.WorkspaceID, session.ID, assistantMsg.ID, target.RecipientType, target.RecipientID, target.ResolvedAgentID, "explicit_mention", "routed", child.ID, "", ""); err != nil {
			slog.Warn("ordinary chat edge create failed",
				"message_id", util.UUIDToString(assistantMsg.ID),
				"task_id", util.UUIDToString(child.ID),
				"error", err,
			)
		}
		routable = append(routable, target)
	}

	switch len(routable) {
	case 0:
		return s.upsertChatDirectedActive(ctx, session, assistantMsg.ID, task.AgentID)
	case 1:
		return s.upsertChatDirectedActive(ctx, session, assistantMsg.ID, routable[0].ResolvedAgentID)
	default:
		return s.upsertChatDirectedAmbiguous(ctx, session, assistantMsg.ID, routable)
	}
}

func (s *TaskService) resolveOrdinaryChatAgentMentions(ctx context.Context, workspaceID, authorAgentID pgtype.UUID, output string) []serviceChatRoutingTarget {
	mentions := util.ParseMentions(output)
	targets := make([]serviceChatRoutingTarget, 0, len(mentions))
	for _, mention := range mentions {
		if mention.Type != "agent" && mention.Type != "squad" {
			continue
		}
		recipientID, err := util.ParseUUID(mention.ID)
		if err != nil {
			continue
		}
		target := serviceChatRoutingTarget{
			RecipientType: mention.Type,
			RecipientID:   recipientID,
			Status:        "routed",
		}
		switch mention.Type {
		case "agent":
			if util.UUIDToString(recipientID) == util.UUIDToString(authorAgentID) {
				target.ResolvedAgentID = recipientID
				target.Status = "skipped"
				target.WarningCode = "self_mention"
				target.WarningMessage = "Agent mention points back to the same agent."
				targets = append(targets, target)
				continue
			}
			agent, err := s.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
				ID:          recipientID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				target.Status = "blocked"
				target.WarningCode = "recipient_not_found"
				target.WarningMessage = "Mentioned agent was not found in this workspace."
			} else if agent.ArchivedAt.Valid {
				target.Status = "blocked"
				target.WarningCode = "recipient_archived"
				target.WarningMessage = "Mentioned agent is archived."
			} else {
				target.ResolvedAgentID = recipientID
			}
		case "squad":
			squad, err := s.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
				ID:          recipientID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				target.Status = "blocked"
				target.WarningCode = "recipient_not_found"
				target.WarningMessage = "Mentioned squad was not found in this workspace."
			} else if squad.ArchivedAt.Valid {
				target.Status = "blocked"
				target.WarningCode = "recipient_archived"
				target.WarningMessage = "Mentioned squad is archived."
			} else if util.UUIDToString(squad.LeaderID) == util.UUIDToString(authorAgentID) {
				target.ResolvedAgentID = squad.LeaderID
				target.Status = "skipped"
				target.WarningCode = "self_mention"
				target.WarningMessage = "Squad mention resolves back to the same lead agent."
			} else {
				target.ResolvedAgentID = squad.LeaderID
			}
		}
		targets = append(targets, target)
	}
	return targets
}

func (s *TaskService) createChatRoutingEdge(ctx context.Context, workspaceID, chatSessionID, messageID pgtype.UUID, recipientType string, recipientID, resolvedAgentID pgtype.UUID, source, status string, taskID pgtype.UUID, warningCode, warningMessage string) error {
	if source == "" {
		source = "explicit_mention"
	}
	if status == "" {
		status = "routed"
	}
	_, err := s.Queries.CreateChatMessageRecipient(ctx, db.CreateChatMessageRecipientParams{
		WorkspaceID:     workspaceID,
		ChatSessionID:   chatSessionID,
		MessageID:       messageID,
		RecipientType:   recipientType,
		RecipientID:     recipientID,
		ResolvedAgentID: resolvedAgentID,
		Source:          source,
		Status:          status,
		RecipientTaskID: taskID,
		WarningCode:     warningCode,
		WarningMessage:  warningMessage,
	})
	return err
}

func (s *TaskService) upsertChatDirectedActive(ctx context.Context, session db.ChatSession, messageID, activeAgentID pgtype.UUID) error {
	_, err := s.Queries.UpsertChatSessionDirectedState(ctx, db.UpsertChatSessionDirectedStateParams{
		ChatSessionID:       session.ID,
		WorkspaceID:         session.WorkspaceID,
		State:               "active",
		ActiveRecipientType: pgtype.Text{String: "agent", Valid: true},
		ActiveRecipientID:   activeAgentID,
		ActiveMessageID:     messageID,
		CandidateRecipients: []byte("[]"),
	})
	return err
}

func (s *TaskService) upsertChatDirectedAmbiguous(ctx context.Context, session db.ChatSession, messageID pgtype.UUID, targets []serviceChatRoutingTarget) error {
	candidates := make([]chatDirectedStateCandidate, 0, len(targets))
	for _, target := range targets {
		candidate := chatDirectedStateCandidate{
			RecipientType: target.RecipientType,
			RecipientID:   util.UUIDToString(target.RecipientID),
		}
		if target.ResolvedAgentID.Valid {
			resolved := util.UUIDToString(target.ResolvedAgentID)
			candidate.ResolvedAgentID = &resolved
		}
		candidates = append(candidates, candidate)
	}
	raw, _ := json.Marshal(candidates)
	_, err := s.Queries.UpsertChatSessionDirectedState(ctx, db.UpsertChatSessionDirectedStateParams{
		ChatSessionID:       session.ID,
		WorkspaceID:         session.WorkspaceID,
		State:               "ambiguous",
		ActiveRecipientType: pgtype.Text{},
		ActiveRecipientID:   pgtype.UUID{},
		ActiveMessageID:     messageID,
		CandidateRecipients: raw,
	})
	return err
}

func (s *TaskService) handleChatPlanConsultationFailed(ctx context.Context, task db.AgentTaskQueue) {
	reason := "failed"
	if task.FailureReason.Valid && task.FailureReason.String == "timeout" {
		reason = "timeout"
	}
	if _, err := s.Queries.MarkChatPlanConsultationFailedByTask(ctx, db.MarkChatPlanConsultationFailedByTaskParams{
		TaskID:        task.ID,
		FailureReason: reason,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("chat plan consultation failure mark failed",
			"task_id", util.UUIDToString(task.ID),
			"chat_plan_consultation_id", util.UUIDToString(task.ChatPlanConsultationID),
			"error", err,
		)
	}
	if task.ChatPlanRunID.Valid {
		if err := s.resumePlanLeadIfConsultationsClosed(ctx, task.ChatPlanRunID, pgtype.UUID{}); err != nil {
			slog.Warn("chat plan lead resume after consultation failure failed",
				"task_id", util.UUIDToString(task.ID),
				"chat_plan_run_id", util.UUIDToString(task.ChatPlanRunID),
				"error", err,
			)
		}
	}
}

func (s *TaskService) resumePlanLeadIfConsultationsClosed(ctx context.Context, planRunID, triggerMessageID pgtype.UUID) error {
	open, err := s.Queries.CountOpenChatPlanConsultations(ctx, planRunID)
	if err != nil {
		return err
	}
	if open > 0 {
		return nil
	}
	run, err := s.Queries.GetChatPlanRun(ctx, planRunID)
	if err != nil {
		return err
	}
	switch run.Status {
	case "cancelled", "failed", "completed":
		return nil
	}
	session, err := s.Queries.GetChatSession(ctx, run.ChatSessionID)
	if err != nil {
		return err
	}
	updated, err := s.Queries.UpdateChatPlanRunStatus(ctx, db.UpdateChatPlanRunStatusParams{
		ID:     run.ID,
		Status: "brainstorming",
	})
	if err != nil {
		return err
	}
	s.broadcastChatPlanRunsUpdated(ctx, updated)
	task, err := s.EnqueuePlanLeadTask(ctx, session, updated, triggerMessageID)
	if err != nil {
		return err
	}
	if triggerMessageID.Valid {
		if err := s.createChatRoutingEdge(ctx, run.WorkspaceID, run.ChatSessionID, triggerMessageID, "agent", run.LeadAgentID, run.LeadAgentID, "explicit_mention", "routed", task.ID, "", ""); err != nil {
			slog.Warn("chat plan lead resume edge create failed",
				"chat_plan_run_id", util.UUIDToString(run.ID),
				"trigger_message_id", util.UUIDToString(triggerMessageID),
				"task_id", util.UUIDToString(task.ID),
				"error", err,
			)
		}
	}
	if err := s.upsertChatDirectedActive(ctx, session, triggerMessageID, run.LeadAgentID); err != nil {
		slog.Warn("chat plan lead directed state update failed",
			"chat_plan_run_id", util.UUIDToString(run.ID),
			"error", err,
		)
	}
	return nil
}

func (s *TaskService) broadcastChatPlanRunsUpdated(ctx context.Context, run db.ChatPlanRun) {
	workspaceID := util.UUIDToString(run.WorkspaceID)
	sessionID := util.UUIDToString(run.ChatSessionID)
	if workspaceID == "" || sessionID == "" || s.Bus == nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:          protocol.EventChatPlanRunsUpdated,
		WorkspaceID:   workspaceID,
		ActorType:     "system",
		ActorID:       "",
		ChatSessionID: sessionID,
		Payload: protocol.ChatPlanRunsUpdatedPayload{
			ChatSessionID: sessionID,
			PlanRunID:     util.UUIDToString(run.ID),
		},
	})
}

// runInTx executes fn inside a single DB transaction. If TxStarter is nil
// (e.g. some tests construct TaskService directly), fn runs against the
// regular Queries handle without transactional guarantees.
func (s *TaskService) runInTx(ctx context.Context, fn func(*db.Queries) error) error {
	if s.TxStarter == nil {
		return fn(s.Queries)
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, taskID string, workspaceID string, summary string, step, total int) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		TaskID:      taskID,
		Payload: protocol.TaskProgressPayload{
			TaskID:  taskID,
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
}

// ReconcileAgentStatus refreshes agent status from the current active task set.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	agent, err := s.Queries.RefreshAgentStatusFromTasks(ctx, agentID)
	if err != nil {
		return
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", agent.Status)
	s.publishAgentStatus(agent)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.publishAgentStatus(agent)
}

func (s *TaskService) publishAgentStatus(agent db.Agent) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// computeChatElapsedMs returns the wall-clock duration from task creation
// (user hit send) to terminal state (completed/failed). Stored on the
// assistant chat_message so the UI can render "Replied in 38s" /
// "Failed after 12s". Uses created_at — not started_at — because users
// experience total wait time, including queue + dispatch, not just the
// daemon's actual run time.
func computeChatElapsedMs(task db.AgentTaskQueue) pgtype.Int8 {
	if !task.CompletedAt.Valid || !task.CreatedAt.Valid {
		return pgtype.Int8{}
	}
	ms := task.CompletedAt.Time.Sub(task.CreatedAt.Time).Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return pgtype.Int8{Int64: ms, Valid: true}
}

func priorityToInt(p string) int32 {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// NotifyTaskEnqueued is the cross-package shim for callers outside
// TaskService (e.g. AutopilotService.dispatchRunOnly) that insert a
// row into agent_task_queue directly. Invalidates the empty-claim
// cache and kicks the daemon WS so the new task is claimed without
// waiting for the next poll.
func (s *TaskService) NotifyTaskEnqueued(ctx context.Context, task db.AgentTaskQueue) {
	s.captureTaskQueued(ctx, task)
	s.notifyTaskAvailable(task)
}

// notifyTaskAvailable runs after a task has been inserted: bumps the
// runtime's invalidation version so any in-flight claim that is about
// to write an "empty" verdict will have it rejected on read, then
// kicks the daemon WS so the daemon claims without waiting for its
// next poll. Order matters — Bump must happen before the wakeup,
// otherwise the wakeup-driven claim could read the still-current
// empty verdict and return null.
func (s *TaskService) notifyTaskAvailable(task db.AgentTaskQueue) {
	if !task.RuntimeID.Valid {
		return
	}
	runtimeKey := util.UUIDToString(task.RuntimeID)
	// Use a background context: the cache bump / wakeup must outlive
	// the request that created the task, otherwise an early client
	// disconnect could leave the empty verdict in place and stall the
	// just-queued task until the TTL expires. The cache itself bounds
	// every Redis call with a short timeout so a wedged Redis cannot
	// block enqueue.
	s.EmptyClaim.Bump(context.Background(), runtimeKey)
	if s.Wakeup == nil {
		return
	}
	s.Wakeup.NotifyTaskAvailable(runtimeKey, util.UUIDToString(task.ID))
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue) {
	var payload map[string]any
	if task.Context != nil {
		json.Unmarshal(task.Context, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)
	payload["issue_id"] = util.UUIDToString(task.IssueID)
	payload["agent_id"] = util.UUIDToString(task.AgentID)
	// chat_session_id is the routing key the chat window uses to writethrough
	// `chatKeys.pendingTask` to status="running" the moment the daemon claims
	// the task. Without it the pill stays stuck at "Queued" until completion.
	if task.ChatSessionID.Valid {
		payload["chat_session_id"] = util.UUIDToString(task.ChatSessionID)
	}

	workspaceID := s.ResolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := s.ResolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	payload := map[string]any{
		"task_id":  util.UUIDToString(task.ID),
		"agent_id": util.UUIDToString(task.AgentID),
		"issue_id": util.UUIDToString(task.IssueID),
		"status":   task.Status,
	}
	if task.ChatSessionID.Valid {
		payload["chat_session_id"] = util.UUIDToString(task.ChatSessionID)
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

// ResolveTaskWorkspaceID determines the workspace ID for a task.
// For issue tasks, it comes from the issue. For chat tasks, from the chat session.
// For autopilot tasks, from the autopilot via its run.
// Returns "" when none of the links resolve — callers treat that as "not found".
func (s *TaskService) ResolveTaskWorkspaceID(ctx context.Context, task db.AgentTaskQueue) string {
	if task.IssueID.Valid {
		if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
			return util.UUIDToString(issue.WorkspaceID)
		}
	}
	if task.ChatSessionID.Valid {
		if cs, err := s.Queries.GetChatSession(ctx, task.ChatSessionID); err == nil {
			return util.UUIDToString(cs.WorkspaceID)
		}
	}
	if task.AutopilotRunID.Valid {
		if run, err := s.Queries.GetAutopilotRun(ctx, task.AutopilotRunID); err == nil {
			if ap, err := s.Queries.GetAutopilot(ctx, run.AutopilotID); err == nil {
				return util.UUIDToString(ap.WorkspaceID)
			}
		}
	}
	// Quick-create tasks have no issue / chat / autopilot link — workspace
	// lives in the context JSONB. Returning "" here is what blocked
	// requireDaemonTaskAccess (404 on /start, /progress, /complete, /fail
	// for the daemon) and silently dropped task:dispatch / task:completed
	// broadcasts, which is why quick-create tasks appeared stuck queued.
	if qc, ok := s.parseQuickCreateContext(task); ok {
		return qc.WorkspaceID
	}
	return ""
}

func (s *TaskService) broadcastChatDone(ctx context.Context, task db.AgentTaskQueue, msg *db.ChatMessage) {
	workspaceID := s.ResolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	payload := protocol.ChatDonePayload{
		ChatSessionID: util.UUIDToString(task.ChatSessionID),
		TaskID:        util.UUIDToString(task.ID),
	}
	if msg != nil {
		payload.MessageID = util.UUIDToString(msg.ID)
		payload.Content = msg.Content
		payload.AuthorType = msg.AuthorType
		payload.AuthorMemberID = util.UUIDToString(msg.AuthorMemberID)
		payload.AuthorAgentID = util.UUIDToString(msg.AuthorAgentID)
		payload.PlanRunID = util.UUIDToString(msg.PlanRunID)
		payload.ConsultationID = util.UUIDToString(msg.ConsultationID)
		payload.ReplyToMessageID = util.UUIDToString(msg.ReplyToMessageID)
		if msg.CreatedAt.Valid {
			payload.CreatedAt = msg.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if msg.ElapsedMs.Valid {
			payload.ElapsedMs = msg.ElapsedMs.Int64
		}
	}
	s.Bus.Publish(events.Event{
		Type:          protocol.EventChatDone,
		WorkspaceID:   workspaceID,
		ActorType:     "system",
		ActorID:       "",
		ChatSessionID: util.UUIDToString(task.ChatSessionID),
		Payload:       payload,
	})
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(workspaceID pgtype.UUID) string {
	ws, err := s.Queries.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Resolve thread root: if parentID points to a reply (has its own parent),
	// use that parent instead so the comment lands in the top-level thread.
	// rootComment captures the root row so we can auto-unresolve it after the
	// reply is committed (see AutoUnresolveThreadOnReply).
	var rootComment *db.Comment
	if parentID.Valid {
		if parent, err := s.Queries.GetComment(ctx, parentID); err == nil {
			if parent.ParentID.Valid {
				if root, err := s.Queries.GetComment(ctx, parent.ParentID); err == nil {
					rootComment = &root
					parentID = root.ID
				}
			} else {
				rootComment = &parent
			}
		}
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issueID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    agentID,
		Content:     content,
		Type:        commentType,
		ParentID:    parentID,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
	s.AutoUnresolveThreadOnReply(ctx, rootComment, util.UUIDToString(issue.WorkspaceID), "agent", util.UUIDToString(agentID))
}

// AutoUnresolveThreadOnReply clears resolved_at on the thread root when a
// reply lands in a resolved thread, and broadcasts comment:unresolved. Shared
// between the user-facing Handler.CreateComment path and the agent-facing
// TaskService.createAgentComment path so the resolved-then-replied state can
// never desync (one of the bugs Emacs flagged on PR #2300). Errors are logged
// — the reply itself already committed, the desync is recoverable on next read.
func (s *TaskService) AutoUnresolveThreadOnReply(ctx context.Context, parent *db.Comment, workspaceID, actorType, actorID string) {
	if parent == nil || !parent.ResolvedAt.Valid {
		return
	}
	updated, err := s.Queries.UnresolveComment(ctx, parent.ID)
	if err != nil {
		slog.Warn("auto-unresolve on reply failed", "error", err, "comment_id", util.UUIDToString(parent.ID))
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentUnresolved,
		WorkspaceID: workspaceID,
		ActorType:   actorType,
		ActorID:     actorID,
		Payload: map[string]any{
			"comment": map[string]any{
				"id":               util.UUIDToString(updated.ID),
				"issue_id":         util.UUIDToString(updated.IssueID),
				"author_type":      updated.AuthorType,
				"author_id":        util.UUIDToString(updated.AuthorID),
				"content":          updated.Content,
				"type":             updated.Type,
				"parent_id":        util.UUIDToPtr(updated.ParentID),
				"created_at":       util.TimestampToString(updated.CreatedAt),
				"updated_at":       util.TimestampToString(updated.UpdatedAt),
				"resolved_at":      util.TimestampToPtr(updated.ResolvedAt),
				"resolved_by_type": util.TextToPtr(updated.ResolvedByType),
				"resolved_by_id":   util.UUIDToPtr(updated.ResolvedByID),
			},
		},
	})
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	return map[string]any{
		"id":              util.UUIDToString(issue.ID),
		"workspace_id":    util.UUIDToString(issue.WorkspaceID),
		"number":          issue.Number,
		"identifier":      issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":           issue.Title,
		"description":     util.TextToPtr(issue.Description),
		"status":          issue.Status,
		"priority":        issue.Priority,
		"assignee_type":   util.TextToPtr(issue.AssigneeType),
		"assignee_id":     util.UUIDToPtr(issue.AssigneeID),
		"creator_type":    issue.CreatorType,
		"creator_id":      util.UUIDToString(issue.CreatorID),
		"parent_issue_id": util.UUIDToPtr(issue.ParentIssueID),
		"position":        issue.Position,
		"due_date":        util.TimestampToPtr(issue.DueDate),
		"created_at":      util.TimestampToString(issue.CreatedAt),
		"updated_at":      util.TimestampToString(issue.UpdatedAt),
	}
}

// parseQuickCreateContext returns the quick-create payload if the task's
// context JSONB contains type == "quick_create"; otherwise the bool is
// false so callers can short-circuit. Tasks linked to an issue / chat /
// autopilot are never quick-create even if they happen to carry a
// context blob, so those are filtered up front.
func (s *TaskService) parseQuickCreateContext(task db.AgentTaskQueue) (QuickCreateContext, bool) {
	if task.IssueID.Valid || task.ChatSessionID.Valid || task.AutopilotRunID.Valid {
		return QuickCreateContext{}, false
	}
	if len(task.Context) == 0 {
		return QuickCreateContext{}, false
	}
	var qc QuickCreateContext
	if err := json.Unmarshal(task.Context, &qc); err != nil {
		return QuickCreateContext{}, false
	}
	if qc.Type != QuickCreateContextType {
		return QuickCreateContext{}, false
	}
	return qc, true
}

// notifyQuickCreateCompleted writes a success inbox notification to the
// requester pointing at the issue the agent just created. The issue is
// stamped with origin_type=quick_create + origin_id=<task_id> by the
// daemon-injected MULTICA_QUICK_CREATE_TASK_ID env var, so this lookup is
// deterministic — robust against the same agent creating other issues in
// parallel (e.g. assignment task running while max_concurrent_tasks > 1
// permits another quick-create alongside it).
func (s *TaskService) notifyQuickCreateCompleted(ctx context.Context, task db.AgentTaskQueue, qc QuickCreateContext) {
	requesterID, err := util.ParseUUID(qc.RequesterID)
	if err != nil {
		slog.Warn("quick-create completion: invalid requester id", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	workspaceID, err := util.ParseUUID(qc.WorkspaceID)
	if err != nil {
		slog.Warn("quick-create completion: invalid workspace id", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	issue, err := s.Queries.GetIssueByOrigin(ctx, db.GetIssueByOriginParams{
		WorkspaceID: workspaceID,
		OriginType:  pgtype.Text{String: "quick_create", Valid: true},
		OriginID:    task.ID,
	})
	if err != nil {
		// No issue created — agent ran to completion but the CLI call must
		// have failed. Surface as a failure inbox so the user sees something.
		slog.Warn("quick-create completion: no issue found, writing failure inbox",
			"task_id", util.UUIDToString(task.ID),
			"agent_id", util.UUIDToString(task.AgentID),
			"workspace_id", qc.WorkspaceID,
		)
		s.notifyQuickCreateFailed(ctx, task, qc, "agent finished without creating an issue")
		return
	}

	// Link the new issue back to this task so subsequent reads of the task
	// (Activity tab, Recent work, etc.) render it as a normal issue task
	// (kind = "direct") instead of staying on the "Creating issue" active-
	// wording label. Best-effort: a write failure here doesn't block the
	// inbox notification, which is the more important signal to the user.
	if err := s.Queries.LinkTaskToIssue(ctx, db.LinkTaskToIssueParams{
		ID:      task.ID,
		IssueID: issue.ID,
	}); err != nil {
		slog.Warn("quick-create completion: link task→issue failed",
			"task_id", util.UUIDToString(task.ID),
			"issue_id", util.UUIDToString(issue.ID),
			"error", err,
		)
	}

	// Subscribe the requester so they receive notifications for follow-up
	// comments and updates. The DB row's creator_type/creator_id is the
	// agent (it ran the CLI), but the human who triggered the quick-create
	// is the semantic creator from a UX perspective — without this they
	// only see the one-shot completion inbox and miss everything after.
	// Best-effort: log on failure but don't block the inbox notification.
	if err := s.Queries.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{
		IssueID:  issue.ID,
		UserType: "member",
		UserID:   requesterID,
		Reason:   "creator",
	}); err != nil {
		slog.Warn("quick-create completion: subscribe requester failed",
			"task_id", util.UUIDToString(task.ID),
			"issue_id", util.UUIDToString(issue.ID),
			"requester_id", qc.RequesterID,
			"error", err,
		)
	} else {
		s.Bus.Publish(events.Event{
			Type:        protocol.EventSubscriberAdded,
			WorkspaceID: qc.WorkspaceID,
			ActorType:   "agent",
			ActorID:     util.UUIDToString(task.AgentID),
			Payload: map[string]any{
				"issue_id":  util.UUIDToString(issue.ID),
				"user_type": "member",
				"user_id":   qc.RequesterID,
				"reason":    "creator",
			},
		})
	}
	prefix := s.getIssuePrefix(workspaceID)
	identifier := fmt.Sprintf("%s-%d", prefix, issue.Number)
	details, _ := json.Marshal(map[string]any{
		"task_id":         util.UUIDToString(task.ID),
		"agent_id":        util.UUIDToString(task.AgentID),
		"issue_id":        util.UUIDToString(issue.ID),
		"identifier":      identifier,
		"original_prompt": qc.Prompt,
	})
	item, err := s.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
		WorkspaceID:   workspaceID,
		RecipientType: "member",
		RecipientID:   requesterID,
		Type:          "quick_create_done",
		Severity:      "info",
		IssueID:       issue.ID,
		Title:         issue.Title,
		Body:          pgtype.Text{},
		ActorType:     pgtype.Text{String: "agent", Valid: true},
		ActorID:       task.AgentID,
		Details:       details,
	})
	if err != nil {
		slog.Error("quick-create completion: inbox write failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	s.publishQuickCreateInbox(item, qc.WorkspaceID, util.UUIDToString(task.AgentID), issue.Status)
}

// notifyQuickCreateFailed writes a failure inbox notification carrying the
// original prompt + agent ID so the frontend can render an "Edit as
// advanced form" entry that pre-fills the legacy create-issue modal
// without asking the user to retype.
func (s *TaskService) notifyQuickCreateFailed(ctx context.Context, task db.AgentTaskQueue, qc QuickCreateContext, errMsg string) {
	requesterID, err := util.ParseUUID(qc.RequesterID)
	if err != nil {
		return
	}
	workspaceID, err := util.ParseUUID(qc.WorkspaceID)
	if err != nil {
		return
	}
	if errMsg == "" {
		errMsg = "Quick create did not finish successfully"
	}
	details, _ := json.Marshal(map[string]any{
		"task_id":         util.UUIDToString(task.ID),
		"agent_id":        util.UUIDToString(task.AgentID),
		"original_prompt": qc.Prompt,
		"error":           redact.Text(errMsg),
	})
	item, err := s.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
		WorkspaceID:   workspaceID,
		RecipientType: "member",
		RecipientID:   requesterID,
		Type:          "quick_create_failed",
		Severity:      "action_required",
		IssueID:       pgtype.UUID{},
		Title:         "Quick create failed",
		Body:          pgtype.Text{String: redact.Text(errMsg), Valid: true},
		ActorType:     pgtype.Text{String: "agent", Valid: true},
		ActorID:       task.AgentID,
		Details:       details,
	})
	if err != nil {
		slog.Error("quick-create failure: inbox write failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	s.publishQuickCreateInbox(item, qc.WorkspaceID, util.UUIDToString(task.AgentID), "")
}

// publishQuickCreateInbox emits the WS event so the requester's inbox list
// updates immediately. Mirrors the payload shape used by the other inbox
// listeners (notification_listeners.go).
func (s *TaskService) publishQuickCreateInbox(item db.InboxItem, workspaceID, agentID, issueStatus string) {
	resp := map[string]any{
		"id":             util.UUIDToString(item.ID),
		"workspace_id":   util.UUIDToString(item.WorkspaceID),
		"recipient_type": item.RecipientType,
		"recipient_id":   util.UUIDToString(item.RecipientID),
		"type":           item.Type,
		"severity":       item.Severity,
		"issue_id":       util.UUIDToPtr(item.IssueID),
		"title":          item.Title,
		"body":           util.TextToPtr(item.Body),
		"read":           item.Read,
		"archived":       item.Archived,
		"created_at":     util.TimestampToString(item.CreatedAt),
		"actor_type":     util.TextToPtr(item.ActorType),
		"actor_id":       util.UUIDToPtr(item.ActorID),
		"details":        json.RawMessage(item.Details),
		"issue_status":   issueStatus,
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventInboxNew,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload:     map[string]any{"item": resp},
	})
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var rc any
	if a.RuntimeConfig != nil {
		json.Unmarshal(a.RuntimeConfig, &rc)
	}
	return map[string]any{
		"id":                   util.UUIDToString(a.ID),
		"workspace_id":         util.UUIDToString(a.WorkspaceID),
		"runtime_id":           util.UUIDToString(a.RuntimeID),
		"name":                 a.Name,
		"description":          a.Description,
		"avatar_url":           util.TextToPtr(a.AvatarUrl),
		"runtime_mode":         a.RuntimeMode,
		"runtime_config":       rc,
		"visibility":           a.Visibility,
		"status":               a.Status,
		"max_concurrent_tasks": a.MaxConcurrentTasks,
		"owner_id":             util.UUIDToPtr(a.OwnerID),
		"skills":               []any{},
		"created_at":           util.TimestampToString(a.CreatedAt),
		"updated_at":           util.TimestampToString(a.UpdatedAt),
		"archived_at":          util.TimestampToPtr(a.ArchivedAt),
		"archived_by":          util.UUIDToPtr(a.ArchivedBy),
	}
}
