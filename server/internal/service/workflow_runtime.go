package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/mention"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	workflowRunStatusQueued    = "queued"
	workflowRunStatusRunning   = "running"
	workflowRunStatusWaiting   = "waiting"
	workflowRunStatusBlocked   = "blocked"
	workflowRunStatusFailed    = "failed"
	workflowRunStatusCompleted = "completed"
	workflowRunStatusCancelled = "cancelled"

	workflowStepStatusPending         = "pending"
	workflowStepStatusReady           = "ready"
	workflowStepStatusRunning         = "running"
	workflowStepStatusWaitingInput    = "waiting_input"
	workflowStepStatusWaitingManual   = "waiting_manual"
	workflowStepStatusWaitingExternal = "waiting_external"
	workflowStepStatusWaitingReview   = "waiting_review"
	workflowStepStatusWaitingQuality  = "waiting_quality"
	workflowStepStatusPaused          = "paused"
	workflowStepStatusBlocked         = "blocked"
	workflowStepStatusFailed          = "failed"
	workflowStepStatusCompleted       = "completed"
	workflowStepStatusSkipped         = "skipped"

	workflowStepExecutionAgent    = "agent"
	workflowStepExecutionManual   = "manual"
	workflowStepExecutionExternal = "external"

	workflowArtifactKindText     = "text"
	workflowArtifactKindMarkdown = "markdown"
	workflowArtifactKindJSON     = "json"

	workflowQualityStatusPass    = "pass"
	workflowQualityStatusFail    = "fail"
	workflowQualityStatusWarning = "warning"

	workflowReviewStatusApproved = "approved"
	workflowReviewStatusRejected = "rejected"

	workflowInputRequestStatusRequested = "requested"
	workflowInputRequestStatusAnswered  = "answered"
	workflowInputRequestStatusCancelled = "cancelled"
)

type workflowRuntimeSchema struct {
	Steps []json.RawMessage `json:"steps"`
}

type workflowRuntimeStep struct {
	ID             string                       `json:"id"`
	Name           string                       `json:"name,omitempty"`
	Title          string                       `json:"title,omitempty"`
	Order          int32                        `json:"order,omitempty"`
	Required       *bool                        `json:"required,omitempty"`
	DependsOn      []string                     `json:"depends_on,omitempty"`
	Execution      workflowRuntimeStepExecution `json:"execution,omitempty"`
	Artifact       workflowRuntimeStepArtifact  `json:"artifact,omitempty"`
	InputArtifacts json.RawMessage              `json:"input_artifacts,omitempty"`
	Review         workflowRuntimeStepReview    `json:"review,omitempty"`
	ReviewRequired *bool                        `json:"review_required,omitempty"`
	QualityGate    workflowRuntimeStepQuality   `json:"quality_gate,omitempty"`
	InputRequests  workflowRuntimeInputRequests `json:"input_requests,omitempty"`
}

type workflowRuntimeStepExecution struct {
	Kind string `json:"kind,omitempty"`
}

type workflowRuntimeStepArtifact struct {
	Inputs json.RawMessage `json:"inputs,omitempty"`
}

type workflowRuntimeStepReview struct {
	Required bool `json:"required,omitempty"`
}

type workflowRuntimeStepQuality struct {
	Blocking bool `json:"blocking,omitempty"`
}

type workflowRuntimeInputRequests struct {
	Allowed        bool   `json:"allowed,omitempty"`
	MaxRounds      int32  `json:"max_rounds,omitempty"`
	QuestionPolicy string `json:"question_policy,omitempty"`
}

type workflowInitialStepRun struct {
	StepDefinitionID string
	Title            string
	OrderIndex       int32
	Required         bool
	Status           string
	ExecutionKind    string
	DependsOnStepIDs []byte
	ArtifactInputs   []byte
	Snapshot         []byte
	Attempt          int32
}

type WorkflowArtifactDiff struct {
	LogicalName   string
	BaseVersion   int32
	TargetVersion int32
	ContentKind   string
	UnifiedDiff   string
	Summary       *string
}

func (s *TaskService) createWorkflowRunForTask(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	if len(task.WorkflowSnapshot) == 0 {
		return nil
	}

	var snapshot WorkflowSnapshot
	if err := json.Unmarshal(task.WorkflowSnapshot, &snapshot); err != nil {
		return fmt.Errorf("parse workflow snapshot: %w", err)
	}

	workspaceID, err := s.workflowRunWorkspaceID(ctx, q, task)
	if err != nil {
		return err
	}

	run, err := q.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:          workspaceID,
		AgentTaskQueueID:     task.ID,
		IssueID:              task.IssueID,
		ChatSessionID:        task.ChatSessionID,
		AutopilotRunID:       task.AutopilotRunID,
		WorkflowDefinitionID: task.WorkflowDefinitionID,
		WorkflowRevisionID:   task.WorkflowRevisionID,
		TriggerType:          strings.TrimSpace(snapshot.TriggerType),
		Snapshot:             task.WorkflowSnapshot,
		Status:               workflowRunStatusQueued,
	})
	if err != nil {
		return fmt.Errorf("create workflow run: %w", err)
	}

	stepRuns, err := workflowInitialStepRunsFromSnapshot(task.WorkflowSnapshot)
	if err != nil {
		return err
	}
	for _, stepRun := range stepRuns {
		if _, err := q.CreateWorkflowStepRun(ctx, db.CreateWorkflowStepRunParams{
			WorkflowRunID:    run.ID,
			StepDefinitionID: stepRun.StepDefinitionID,
			Title:            stepRun.Title,
			OrderIndex:       stepRun.OrderIndex,
			Required:         stepRun.Required,
			Status:           stepRun.Status,
			ExecutionKind:    stepRun.ExecutionKind,
			DependsOnStepIds: stepRun.DependsOnStepIDs,
			ArtifactInputs:   stepRun.ArtifactInputs,
			Snapshot:         stepRun.Snapshot,
			Attempt:          stepRun.Attempt,
		}); err != nil {
			return fmt.Errorf("create workflow step run %q: %w", stepRun.StepDefinitionID, err)
		}
	}

	return nil
}

func (s *TaskService) workflowRunWorkspaceID(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (pgtype.UUID, error) {
	if task.IssueID.Valid {
		issue, err := q.GetIssue(ctx, task.IssueID)
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("load workflow run issue workspace: %w", err)
		}
		return issue.WorkspaceID, nil
	}
	if task.ChatSessionID.Valid {
		session, err := q.GetChatSession(ctx, task.ChatSessionID)
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("load workflow run chat workspace: %w", err)
		}
		return session.WorkspaceID, nil
	}
	if task.AutopilotRunID.Valid {
		run, err := q.GetAutopilotRun(ctx, task.AutopilotRunID)
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("load workflow run autopilot: %w", err)
		}
		autopilot, err := q.GetAutopilot(ctx, run.AutopilotID)
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("load workflow run autopilot workspace: %w", err)
		}
		return autopilot.WorkspaceID, nil
	}

	agent, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("load workflow run agent workspace: %w", err)
	}
	return agent.WorkspaceID, nil
}

func workflowInitialStepRunsFromSnapshot(raw []byte) ([]workflowInitialStepRun, error) {
	var snapshot WorkflowSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("parse workflow snapshot: %w", err)
	}
	if len(snapshot.Schema) == 0 || string(snapshot.Schema) == "null" {
		return nil, nil
	}

	var schema workflowRuntimeSchema
	if err := json.Unmarshal(snapshot.Schema, &schema); err != nil {
		return nil, fmt.Errorf("parse workflow snapshot schema: %w", err)
	}

	stepRuns := make([]workflowInitialStepRun, 0, len(schema.Steps))
	for i, rawStep := range schema.Steps {
		var step workflowRuntimeStep
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return nil, fmt.Errorf("parse workflow step %d: %w", i+1, err)
		}

		stepID := strings.TrimSpace(step.ID)
		if stepID == "" {
			return nil, fmt.Errorf("workflow step %d id is required", i+1)
		}

		dependsOn := normalizedStringList(step.DependsOn)
		dependsJSON, err := marshalJSONArray(dependsOn)
		if err != nil {
			return nil, fmt.Errorf("marshal step %q dependencies: %w", stepID, err)
		}

		artifactInputs := normalizedRawJSONArray(step.Artifact.Inputs)
		if len(artifactInputs) == 0 {
			artifactInputs = normalizedRawJSONArray(step.InputArtifacts)
		}
		if len(artifactInputs) == 0 {
			artifactInputs = []byte("[]")
		}

		executionKind := normalizeWorkflowStepExecutionKind(step.Execution.Kind)
		stepRuns = append(stepRuns, workflowInitialStepRun{
			StepDefinitionID: stepID,
			Title:            workflowStepTitle(step),
			OrderIndex:       workflowStepOrder(step.Order, i),
			Required:         workflowStepRequired(step.Required),
			Status:           initialWorkflowStepStatus(dependsOn, executionKind),
			ExecutionKind:    executionKind,
			DependsOnStepIDs: dependsJSON,
			ArtifactInputs:   artifactInputs,
			Snapshot:         rawStep,
			Attempt:          1,
		})
	}
	return stepRuns, nil
}

func (s *TaskService) setWorkflowRunStatusForTask(ctx context.Context, q *db.Queries, taskID pgtype.UUID, status string) error {
	run, err := q.UpdateWorkflowRunStatusByTask(ctx, db.UpdateWorkflowRunStatusByTaskParams{
		AgentTaskQueueID: taskID,
		Status:           status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("update workflow run status: %w", err)
	}
	if status == workflowRunStatusCompleted {
		if err := completeUnfinishedWorkflowStepRuns(ctx, q, run.ID); err != nil {
			return err
		}
	}
	return nil
}

func completeUnfinishedWorkflowStepRuns(ctx context.Context, q *db.Queries, runID pgtype.UUID) error {
	steps, err := q.ListWorkflowStepRunsByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("list workflow step runs for completion: %w", err)
	}
	for _, step := range steps {
		if workflowStepIsSuccessfulTerminal(step.Status) {
			continue
		}
		if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusCompleted,
		}); err != nil {
			return fmt.Errorf("complete workflow step %q with task: %w", step.StepDefinitionID, err)
		}
	}
	return nil
}

func (s *TaskService) CompleteWorkflowStepRun(ctx context.Context, stepID pgtype.UUID) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		updated, err := s.completeWorkflowStepRunInTx(ctx, q, step)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	return out, err
}

func (s *TaskService) completeWorkflowStepRunInTx(ctx context.Context, q *db.Queries, step db.WorkflowStepRun) (db.WorkflowStepRun, error) {
	if workflowStepIsTerminal(step.Status) || step.Status == workflowStepStatusWaitingReview {
		return step, nil
	}
	if hasBlockingQualityFailure(ctx, q, step.ID) {
		return db.WorkflowStepRun{}, fmt.Errorf("workflow step has a blocking quality failure")
	}
	if workflowStepRequiresReview(step.Snapshot) {
		updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusWaitingReview,
		})
		if err != nil {
			return db.WorkflowStepRun{}, fmt.Errorf("mark workflow step waiting for review: %w", err)
		}
		if _, err := q.CreateWorkflowReview(ctx, db.CreateWorkflowReviewParams{
			WorkflowRunID:     step.WorkflowRunID,
			WorkflowStepRunID: step.ID,
		}); err != nil {
			return db.WorkflowStepRun{}, fmt.Errorf("create workflow review: %w", err)
		}
		if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
			ID:     step.WorkflowRunID,
			Status: workflowRunStatusWaiting,
		}); err != nil {
			return db.WorkflowStepRun{}, fmt.Errorf("mark workflow run waiting: %w", err)
		}
		return updated, nil
	}
	updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
		ID:     step.ID,
		Status: workflowStepStatusCompleted,
	})
	if err != nil {
		return db.WorkflowStepRun{}, fmt.Errorf("complete workflow step run: %w", err)
	}
	return updated, s.advanceWorkflowAfterStepChange(ctx, q, step.WorkflowRunID)
}

func (s *TaskService) StartWorkflowStepRun(ctx context.Context, stepID pgtype.UUID) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		if step.Status != workflowStepStatusReady {
			return fmt.Errorf("workflow step is not ready")
		}
		updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusRunning,
		})
		if err != nil {
			return fmt.Errorf("start workflow step run: %w", err)
		}
		if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
			ID:     step.WorkflowRunID,
			Status: workflowRunStatusRunning,
		}); err != nil {
			return fmt.Errorf("start workflow run: %w", err)
		}
		out = updated
		return nil
	})
	return out, err
}

func (s *TaskService) CompleteManualWorkflowStepRun(ctx context.Context, stepID pgtype.UUID) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		if step.ExecutionKind != workflowStepExecutionManual {
			return fmt.Errorf("workflow step is not manual")
		}
		updated, err := s.completeWorkflowStepRunInTx(ctx, q, step)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	return out, err
}

func (s *TaskService) FailWorkflowStepRun(ctx context.Context, stepID pgtype.UUID, reason string) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusFailed,
			Error:  textParam(reason),
		})
		if err != nil {
			return fmt.Errorf("fail workflow step run: %w", err)
		}
		out = updated
		if step.Required {
			if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
				ID:     step.WorkflowRunID,
				Status: workflowRunStatusFailed,
			}); err != nil {
				return fmt.Errorf("fail workflow run: %w", err)
			}
			return nil
		}
		return s.advanceWorkflowAfterStepChange(ctx, q, step.WorkflowRunID)
	})
	return out, err
}

func (s *TaskService) PauseWorkflowStepRun(ctx context.Context, stepID pgtype.UUID, reason string) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusPaused,
			Error:  textParam(reason),
		})
		if err != nil {
			return fmt.Errorf("pause workflow step run: %w", err)
		}
		out = updated
		if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
			ID:     step.WorkflowRunID,
			Status: workflowRunStatusWaiting,
		}); err != nil {
			return fmt.Errorf("pause workflow run: %w", err)
		}
		return nil
	})
	return out, err
}

func (s *TaskService) CreateWorkflowInputRequest(ctx context.Context, stepID pgtype.UUID, questionText string, maxRounds int32, requesterAgentID pgtype.UUID, sessionID, workDir string) (db.WorkflowInputRequest, error) {
	questionText = strings.TrimSpace(questionText)
	if questionText == "" {
		return db.WorkflowInputRequest{}, fmt.Errorf("question_text is required")
	}

	var (
		out             db.WorkflowInputRequest
		questionComment db.Comment
		createdComment  bool
	)
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		if step.ExecutionKind != workflowStepExecutionAgent {
			return fmt.Errorf("workflow input requests are only allowed for agent steps")
		}
		policy := workflowInputRequestPolicy(step.Snapshot)
		if !policy.Allowed {
			return fmt.Errorf("workflow step does not allow input requests")
		}
		policyMax := policy.MaxRounds
		if policyMax <= 0 {
			policyMax = 1
		}
		if maxRounds <= 0 {
			maxRounds = policyMax
		}
		if maxRounds > policyMax {
			return fmt.Errorf("max_rounds exceeds workflow step policy")
		}
		rounds, err := q.CountWorkflowInputRequestRoundsByStepRun(ctx, step.ID)
		if err != nil {
			return fmt.Errorf("count workflow input request rounds: %w", err)
		}
		if rounds >= maxRounds {
			return fmt.Errorf("workflow input request round limit reached")
		}
		if _, err := q.GetOpenWorkflowInputRequestByStepRun(ctx, step.ID); err == nil {
			return fmt.Errorf("workflow step already has an open input request")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check open workflow input request: %w", err)
		}

		run, err := q.GetWorkflowRun(ctx, step.WorkflowRunID)
		if err != nil {
			return fmt.Errorf("load workflow run: %w", err)
		}
		task, err := q.GetAgentTask(ctx, run.AgentTaskQueueID)
		if err != nil {
			return fmt.Errorf("load workflow task: %w", err)
		}
		if !requesterAgentID.Valid {
			requesterAgentID = task.AgentID
		}

		var questionCommentID pgtype.UUID
		if run.IssueID.Valid {
			comment, err := createWorkflowRuntimeComment(ctx, q, run.IssueID, "agent", requesterAgentID, questionText, pgtype.UUID{})
			if err != nil {
				return fmt.Errorf("create workflow input question comment: %w", err)
			}
			questionComment = comment
			questionCommentID = comment.ID
			createdComment = true
		}

		request, err := q.CreateWorkflowInputRequest(ctx, db.CreateWorkflowInputRequestParams{
			WorkspaceID:       run.WorkspaceID,
			WorkflowRunID:     run.ID,
			WorkflowStepRunID: step.ID,
			IssueID:           run.IssueID,
			ChatSessionID:     run.ChatSessionID,
			QuestionCommentID: questionCommentID,
			RequesterAgentID:  requesterAgentID,
			QuestionText:      questionText,
			MaxRounds:         maxRounds,
		})
		if err != nil {
			return fmt.Errorf("create workflow input request: %w", err)
		}
		if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusWaitingInput,
		}); err != nil {
			return fmt.Errorf("mark workflow step waiting for input: %w", err)
		}
		if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
			ID:     run.ID,
			Status: workflowRunStatusWaiting,
		}); err != nil {
			return fmt.Errorf("mark workflow run waiting for input: %w", err)
		}
		if _, err := q.SuspendAgentTaskForWorkflowInput(ctx, db.SuspendAgentTaskForWorkflowInputParams{
			ID:        run.AgentTaskQueueID,
			SessionID: textParam(sessionID),
			WorkDir:   textParam(workDir),
		}); err != nil {
			return fmt.Errorf("suspend task for workflow input: %w", err)
		}
		out = request
		return nil
	})
	if err != nil {
		return db.WorkflowInputRequest{}, err
	}
	if createdComment {
		s.publishWorkflowRuntimeComment(ctx, questionComment, "agent", requesterAgentID)
	}
	if out.WorkflowRunID.Valid {
		if run, runErr := s.Queries.GetWorkflowRun(ctx, out.WorkflowRunID); runErr == nil {
			task, taskErr := s.Queries.GetAgentTask(ctx, run.AgentTaskQueueID)
			if taskErr == nil {
				s.ReconcileAgentStatus(ctx, task.AgentID)
				s.broadcastTaskEvent(ctx, protocol.EventTaskProgress, task)
			}
		}
	}
	return out, nil
}

func (s *TaskService) AnswerWorkflowInputRequest(ctx context.Context, requestID, responderID pgtype.UUID, answerText string, shouldContinue bool) (db.WorkflowInputRequest, error) {
	answerText = strings.TrimSpace(answerText)
	if answerText == "" {
		return db.WorkflowInputRequest{}, fmt.Errorf("answer_text is required")
	}

	var (
		out           db.WorkflowInputRequest
		answerComment db.Comment
		createdAnswer bool
		requeuedTask  db.AgentTaskQueue
		didRequeue    bool
	)
	err := s.runInTx(ctx, func(q *db.Queries) error {
		request, err := q.GetWorkflowInputRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("load workflow input request: %w", err)
		}
		if request.Status != workflowInputRequestStatusRequested {
			return fmt.Errorf("workflow input request is not open")
		}

		var answerCommentID pgtype.UUID
		if request.IssueID.Valid {
			comment, err := createWorkflowRuntimeComment(ctx, q, request.IssueID, "member", responderID, answerText, request.QuestionCommentID)
			if err != nil {
				return fmt.Errorf("create workflow input answer comment: %w", err)
			}
			answerComment = comment
			answerCommentID = comment.ID
			createdAnswer = true
		}

		updated, err := q.AnswerWorkflowInputRequest(ctx, db.AnswerWorkflowInputRequestParams{
			ID:              request.ID,
			AnswerText:      textParam(answerText),
			AnswerCommentID: answerCommentID,
			ResponderID:     responderID,
		})
		if err != nil {
			return fmt.Errorf("answer workflow input request: %w", err)
		}
		out = updated
		if !shouldContinue {
			return nil
		}

		step, err := q.GetWorkflowStepRun(ctx, request.WorkflowStepRunID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		run, err := q.GetWorkflowRun(ctx, request.WorkflowRunID)
		if err != nil {
			return fmt.Errorf("load workflow run: %w", err)
		}
		nextStatus := workflowStepStatusReady
		if step.ExecutionKind != workflowStepExecutionAgent {
			nextStatus = initialWorkflowStepStatus(workflowStepDependencies(step), step.ExecutionKind)
		}
		if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: nextStatus,
		}); err != nil {
			return fmt.Errorf("mark workflow step ready after input: %w", err)
		}
		if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
			ID:     run.ID,
			Status: workflowRunStatusQueued,
		}); err != nil {
			return fmt.Errorf("mark workflow run queued after input: %w", err)
		}
		task, err := q.RequeueWaitingAgentTask(ctx, run.AgentTaskQueueID)
		if err != nil {
			return fmt.Errorf("requeue workflow input task: %w", err)
		}
		requeuedTask = task
		didRequeue = true
		return nil
	})
	if err != nil {
		return db.WorkflowInputRequest{}, err
	}
	if createdAnswer {
		s.publishWorkflowRuntimeComment(ctx, answerComment, "member", responderID)
	}
	if didRequeue {
		s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, requeuedTask)
		s.NotifyTaskEnqueued(ctx, requeuedTask)
	}
	return out, nil
}

func (s *TaskService) CancelWorkflowInputRequest(ctx context.Context, requestID pgtype.UUID) (db.WorkflowInputRequest, error) {
	var out db.WorkflowInputRequest
	err := s.runInTx(ctx, func(q *db.Queries) error {
		request, err := q.GetWorkflowInputRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("load workflow input request: %w", err)
		}
		if request.Status != workflowInputRequestStatusRequested {
			return fmt.Errorf("workflow input request is not open")
		}
		cancelled, err := q.CancelWorkflowInputRequest(ctx, request.ID)
		if err != nil {
			return fmt.Errorf("cancel workflow input request: %w", err)
		}
		if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     request.WorkflowStepRunID,
			Status: workflowStepStatusPaused,
			Error:  textParam("workflow input request cancelled"),
		}); err != nil {
			return fmt.Errorf("pause workflow step after input cancellation: %w", err)
		}
		out = cancelled
		return nil
	})
	return out, err
}

func (s *TaskService) SkipWorkflowStepRun(ctx context.Context, stepID pgtype.UUID) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		updated, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: workflowStepStatusSkipped,
		})
		if err != nil {
			return fmt.Errorf("skip workflow step run: %w", err)
		}
		out = updated
		return s.advanceWorkflowAfterStepChange(ctx, q, step.WorkflowRunID)
	})
	return out, err
}

func (s *TaskService) RetryWorkflowStepRun(ctx context.Context, stepID pgtype.UUID) (db.WorkflowStepRun, error) {
	var out db.WorkflowStepRun
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		status := workflowStepStatusPending
		latest, err := latestWorkflowStepRunsForRun(ctx, q, step.WorkflowRunID)
		if err != nil {
			return err
		}
		if workflowDependenciesSatisfied(step, latest) {
			status = initialWorkflowStepStatus(workflowStepDependencies(step), step.ExecutionKind)
		}
		retry, err := q.CreateWorkflowStepRunRetry(ctx, db.CreateWorkflowStepRunRetryParams{
			ID:     step.ID,
			Status: status,
		})
		if err != nil {
			return fmt.Errorf("retry workflow step run: %w", err)
		}
		out = retry
		return s.refreshWorkflowRunStatus(ctx, q, step.WorkflowRunID)
	})
	return out, err
}

func (s *TaskService) SaveWorkflowArtifact(ctx context.Context, stepID pgtype.UUID, logicalName, contentKind, contentText string, contentJSON []byte, producerType string, producerID pgtype.UUID) (db.WorkflowArtifact, error) {
	var out db.WorkflowArtifact
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		logicalName = strings.TrimSpace(logicalName)
		if logicalName == "" {
			return fmt.Errorf("artifact logical name is required")
		}
		contentKind = normalizeWorkflowArtifactKind(contentKind)
		if contentKind == "" {
			return fmt.Errorf("unsupported artifact content kind")
		}
		if contentKind == workflowArtifactKindJSON {
			if len(contentJSON) == 0 {
				contentJSON = []byte(strings.TrimSpace(contentText))
			}
			if !json.Valid(contentJSON) {
				return fmt.Errorf("artifact JSON content is invalid")
			}
			contentText = ""
		}
		artifact, err := q.CreateWorkflowArtifactVersion(ctx, db.CreateWorkflowArtifactVersionParams{
			WorkflowRunID:     step.WorkflowRunID,
			WorkflowStepRunID: step.ID,
			LogicalName:       logicalName,
			ContentKind:       contentKind,
			ProducerType:      normalizeWorkflowProducerType(producerType),
			ContentText:       textParam(contentText),
			ContentJson:       nullableJSON(contentJSON),
			ProducerID:        producerID,
		})
		if err != nil {
			return fmt.Errorf("create workflow artifact: %w", err)
		}
		out = artifact
		return nil
	})
	return out, err
}

func (s *TaskService) GetWorkflowArtifactDiff(ctx context.Context, artifactID pgtype.UUID, baseVersion, targetVersion int32) (WorkflowArtifactDiff, error) {
	if baseVersion <= 0 || targetVersion <= 0 {
		return WorkflowArtifactDiff{}, fmt.Errorf("base_version and target_version must be positive")
	}
	if baseVersion == targetVersion {
		return WorkflowArtifactDiff{}, fmt.Errorf("base_version and target_version must differ")
	}
	base, err := s.Queries.GetWorkflowArtifactVersionByAnchor(ctx, db.GetWorkflowArtifactVersionByAnchorParams{
		ID:      artifactID,
		Version: baseVersion,
	})
	if err != nil {
		return WorkflowArtifactDiff{}, fmt.Errorf("load base artifact version: %w", err)
	}
	target, err := s.Queries.GetWorkflowArtifactVersionByAnchor(ctx, db.GetWorkflowArtifactVersionByAnchorParams{
		ID:      artifactID,
		Version: targetVersion,
	})
	if err != nil {
		return WorkflowArtifactDiff{}, fmt.Errorf("load target artifact version: %w", err)
	}
	if base.WorkflowStepRunID != target.WorkflowStepRunID || base.LogicalName != target.LogicalName {
		return WorkflowArtifactDiff{}, fmt.Errorf("artifact versions do not belong to the same logical artifact")
	}
	if base.ContentKind != target.ContentKind {
		return WorkflowArtifactDiff{}, fmt.Errorf("artifact versions use different content kinds")
	}
	baseText := workflowArtifactContentForDiff(base)
	targetText := workflowArtifactContentForDiff(target)
	return WorkflowArtifactDiff{
		LogicalName:   base.LogicalName,
		BaseVersion:   base.Version,
		TargetVersion: target.Version,
		ContentKind:   base.ContentKind,
		UnifiedDiff:   unifiedLineDiff(fmt.Sprintf("%s v%d", base.LogicalName, base.Version), fmt.Sprintf("%s v%d", target.LogicalName, target.Version), baseText, targetText),
	}, nil
}

func (s *TaskService) ReportWorkflowQualityGate(ctx context.Context, stepID, artifactID pgtype.UUID, status string, blocking bool, reportText string, reportJSON []byte, producerType string, producerID pgtype.UUID) (db.WorkflowQualityGateResult, error) {
	var out db.WorkflowQualityGateResult
	err := s.runInTx(ctx, func(q *db.Queries) error {
		step, err := q.GetWorkflowStepRun(ctx, stepID)
		if err != nil {
			return fmt.Errorf("load workflow step run: %w", err)
		}
		status = normalizeWorkflowQualityStatus(status)
		if status == "" {
			return fmt.Errorf("unsupported quality gate status")
		}
		if len(reportJSON) > 0 && !json.Valid(reportJSON) {
			return fmt.Errorf("quality gate report JSON is invalid")
		}
		result, err := q.CreateWorkflowQualityGateResult(ctx, db.CreateWorkflowQualityGateResultParams{
			WorkflowRunID:      step.WorkflowRunID,
			WorkflowStepRunID:  step.ID,
			WorkflowArtifactID: artifactID,
			Status:             status,
			Blocking:           blocking,
			ProducerType:       normalizeWorkflowProducerType(producerType),
			ProducerID:         producerID,
			ReportText:         textParam(reportText),
			ReportJson:         nullableJSON(reportJSON),
		})
		if err != nil {
			return fmt.Errorf("create workflow quality gate result: %w", err)
		}
		out = result
		if blocking && status == workflowQualityStatusFail {
			if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
				ID:     step.ID,
				Status: workflowStepStatusBlocked,
				Error:  textParam("blocking quality gate failed"),
			}); err != nil {
				return fmt.Errorf("block workflow step: %w", err)
			}
			if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
				ID:     step.WorkflowRunID,
				Status: workflowRunStatusBlocked,
			}); err != nil {
				return fmt.Errorf("block workflow run: %w", err)
			}
		}
		return nil
	})
	return out, err
}

func (s *TaskService) DecideWorkflowReview(ctx context.Context, reviewID, reviewerID pgtype.UUID, status, notes string) (db.WorkflowReview, error) {
	var out db.WorkflowReview
	err := s.runInTx(ctx, func(q *db.Queries) error {
		review, err := q.GetWorkflowReview(ctx, reviewID)
		if err != nil {
			return fmt.Errorf("load workflow review: %w", err)
		}
		status = normalizeWorkflowReviewDecision(status)
		if status == "" {
			return fmt.Errorf("unsupported review decision")
		}
		updated, err := q.UpdateWorkflowReviewDecision(ctx, db.UpdateWorkflowReviewDecisionParams{
			ID:            review.ID,
			Status:        status,
			ReviewerID:    reviewerID,
			DecisionNotes: textParam(notes),
		})
		if err != nil {
			return fmt.Errorf("update workflow review: %w", err)
		}
		out = updated
		if !review.WorkflowStepRunID.Valid {
			return nil
		}
		step, err := q.GetWorkflowStepRun(ctx, review.WorkflowStepRunID)
		if err != nil {
			return fmt.Errorf("load reviewed workflow step: %w", err)
		}
		switch status {
		case workflowReviewStatusApproved:
			if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
				ID:     step.ID,
				Status: workflowStepStatusCompleted,
			}); err != nil {
				return fmt.Errorf("complete reviewed workflow step: %w", err)
			}
			return s.advanceWorkflowAfterStepChange(ctx, q, step.WorkflowRunID)
		case workflowReviewStatusRejected:
			if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
				ID:     step.ID,
				Status: workflowStepStatusFailed,
				Error:  textParam("human review rejected"),
			}); err != nil {
				return fmt.Errorf("reject reviewed workflow step: %w", err)
			}
			if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
				ID:     step.WorkflowRunID,
				Status: workflowRunStatusFailed,
			}); err != nil {
				return fmt.Errorf("fail rejected workflow run: %w", err)
			}
		}
		return nil
	})
	return out, err
}

func (s *TaskService) advanceWorkflowAfterStepChange(ctx context.Context, q *db.Queries, runID pgtype.UUID) error {
	if err := s.advanceReadyWorkflowSteps(ctx, q, runID); err != nil {
		return err
	}
	return s.refreshWorkflowRunStatus(ctx, q, runID)
}

func (s *TaskService) advanceReadyWorkflowSteps(ctx context.Context, q *db.Queries, runID pgtype.UUID) error {
	latest, err := latestWorkflowStepRunsForRun(ctx, q, runID)
	if err != nil {
		return err
	}
	for _, step := range latest {
		if step.Status != workflowStepStatusPending {
			continue
		}
		if !workflowDependenciesSatisfied(step, latest) {
			continue
		}
		nextStatus := initialWorkflowStepStatus(nil, step.ExecutionKind)
		if _, err := q.UpdateWorkflowStepRunStatus(ctx, db.UpdateWorkflowStepRunStatusParams{
			ID:     step.ID,
			Status: nextStatus,
		}); err != nil {
			return fmt.Errorf("advance workflow step %q: %w", step.StepDefinitionID, err)
		}
		step.Status = nextStatus
		latest[step.StepDefinitionID] = step
	}
	return nil
}

func (s *TaskService) refreshWorkflowRunStatus(ctx context.Context, q *db.Queries, runID pgtype.UUID) error {
	latest, err := latestWorkflowStepRunsForRun(ctx, q, runID)
	if err != nil {
		return err
	}
	if len(latest) == 0 {
		return nil
	}

	allRequiredDone := true
	hasBlocked := false
	hasFailed := false
	hasWaiting := false
	hasActive := false

	for _, step := range latest {
		if step.Required && !workflowStepIsSuccessfulTerminal(step.Status) {
			allRequiredDone = false
		}
		switch step.Status {
		case workflowStepStatusBlocked:
			hasBlocked = true
		case workflowStepStatusFailed:
			if step.Required {
				hasFailed = true
			}
		case workflowStepStatusWaitingInput, workflowStepStatusWaitingManual, workflowStepStatusWaitingExternal, workflowStepStatusWaitingReview, workflowStepStatusWaitingQuality, workflowStepStatusPaused, workflowStepStatusPending:
			hasWaiting = true
		case workflowStepStatusReady, workflowStepStatusRunning:
			hasActive = true
		}
	}

	status := workflowRunStatusRunning
	switch {
	case hasFailed:
		status = workflowRunStatusFailed
	case hasBlocked:
		status = workflowRunStatusBlocked
	case allRequiredDone:
		status = workflowRunStatusCompleted
	case hasWaiting && !hasActive:
		status = workflowRunStatusWaiting
	}

	if _, err := q.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
		ID:     runID,
		Status: status,
	}); err != nil {
		return fmt.Errorf("refresh workflow run status: %w", err)
	}
	return nil
}

func workflowStepTitle(step workflowRuntimeStep) string {
	for _, value := range []string{step.Title, step.Name, step.ID} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func workflowStepOrder(order int32, index int) int32 {
	if order > 0 {
		return order
	}
	return int32(index + 1)
}

func workflowStepRequired(required *bool) bool {
	if required == nil {
		return true
	}
	return *required
}

func normalizeWorkflowStepExecutionKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case workflowStepExecutionManual:
		return workflowStepExecutionManual
	case workflowStepExecutionExternal:
		return workflowStepExecutionExternal
	default:
		return workflowStepExecutionAgent
	}
}

func initialWorkflowStepStatus(dependsOn []string, executionKind string) string {
	if len(dependsOn) > 0 {
		return workflowStepStatusPending
	}
	switch executionKind {
	case workflowStepExecutionManual:
		return workflowStepStatusWaitingManual
	case workflowStepExecutionExternal:
		return workflowStepStatusWaitingExternal
	default:
		return workflowStepStatusReady
	}
}

func normalizedStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func marshalJSONArray(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("[]"), nil
	}
	return raw, nil
}

func normalizedRawJSONArray(raw json.RawMessage) []byte {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	normalized, err := json.Marshal(arr)
	if err != nil {
		return nil
	}
	return normalized
}

func latestWorkflowStepRunsForRun(ctx context.Context, q *db.Queries, runID pgtype.UUID) (map[string]db.WorkflowStepRun, error) {
	steps, err := q.ListWorkflowStepRunsByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list workflow step runs: %w", err)
	}
	latest := make(map[string]db.WorkflowStepRun, len(steps))
	for _, step := range steps {
		current, ok := latest[step.StepDefinitionID]
		if !ok || step.Attempt > current.Attempt || (step.Attempt == current.Attempt && step.CreatedAt.Time.After(current.CreatedAt.Time)) {
			latest[step.StepDefinitionID] = step
		}
	}
	return latest, nil
}

func workflowStepDependencies(step db.WorkflowStepRun) []string {
	var deps []string
	if len(step.DependsOnStepIds) == 0 {
		return deps
	}
	if err := json.Unmarshal(step.DependsOnStepIds, &deps); err != nil {
		return nil
	}
	return normalizedStringList(deps)
}

func workflowDependenciesSatisfied(step db.WorkflowStepRun, latest map[string]db.WorkflowStepRun) bool {
	for _, depID := range workflowStepDependencies(step) {
		dep, ok := latest[depID]
		if !ok || !workflowStepIsSuccessfulTerminal(dep.Status) {
			return false
		}
	}
	return true
}

func workflowStepIsSuccessfulTerminal(status string) bool {
	switch status {
	case workflowStepStatusCompleted, workflowStepStatusSkipped:
		return true
	default:
		return false
	}
}

func workflowStepIsTerminal(status string) bool {
	switch status {
	case workflowStepStatusCompleted, workflowStepStatusSkipped, workflowStepStatusFailed:
		return true
	default:
		return false
	}
}

func workflowStepRequiresReview(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var step workflowRuntimeStep
	if err := json.Unmarshal(raw, &step); err != nil {
		return false
	}
	if step.ReviewRequired != nil {
		return *step.ReviewRequired
	}
	return step.Review.Required
}

func workflowInputRequestPolicy(raw []byte) workflowRuntimeInputRequests {
	if len(raw) == 0 {
		return workflowRuntimeInputRequests{}
	}
	var step workflowRuntimeStep
	if err := json.Unmarshal(raw, &step); err != nil {
		return workflowRuntimeInputRequests{}
	}
	return step.InputRequests
}

func hasBlockingQualityFailure(ctx context.Context, q *db.Queries, stepID pgtype.UUID) bool {
	results, err := q.ListWorkflowQualityGateResultsByStep(ctx, stepID)
	if err != nil {
		return false
	}
	for _, result := range results {
		if result.Blocking && result.Status == workflowQualityStatusFail {
			return true
		}
	}
	return false
}

func normalizeWorkflowArtifactKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "", workflowArtifactKindText:
		return workflowArtifactKindText
	case workflowArtifactKindMarkdown:
		return workflowArtifactKindMarkdown
	case workflowArtifactKindJSON:
		return workflowArtifactKindJSON
	default:
		return ""
	}
}

func normalizeWorkflowQualityStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case workflowQualityStatusPass:
		return workflowQualityStatusPass
	case workflowQualityStatusFail:
		return workflowQualityStatusFail
	case workflowQualityStatusWarning:
		return workflowQualityStatusWarning
	default:
		return ""
	}
}

func normalizeWorkflowReviewDecision(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case workflowReviewStatusApproved:
		return workflowReviewStatusApproved
	case workflowReviewStatusRejected:
		return workflowReviewStatusRejected
	default:
		return ""
	}
}

func normalizeWorkflowProducerType(producerType string) string {
	switch strings.TrimSpace(strings.ToLower(producerType)) {
	case "agent":
		return "agent"
	default:
		return "member"
	}
}

func textParam(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func nullableJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func createWorkflowRuntimeComment(ctx context.Context, q *db.Queries, issueID pgtype.UUID, authorType string, authorID pgtype.UUID, content string, parentID pgtype.UUID) (db.Comment, error) {
	issue, err := q.GetIssue(ctx, issueID)
	if err != nil {
		return db.Comment{}, err
	}
	if parentID.Valid {
		if parent, err := q.GetComment(ctx, parentID); err == nil && parent.ParentID.Valid {
			parentID = parent.ParentID
		}
	}
	content = mention.ExpandIssueIdentifiers(ctx, q, issue.WorkspaceID, content)
	return q.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issueID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  authorType,
		AuthorID:    authorID,
		Content:     content,
		Type:        "comment",
		ParentID:    parentID,
	})
}

func (s *TaskService) publishWorkflowRuntimeComment(ctx context.Context, comment db.Comment, actorType string, actorID pgtype.UUID) {
	if s.Bus == nil {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, comment.IssueID)
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   actorType,
		ActorID:     util.UUIDToString(actorID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  util.TimestampToString(comment.CreatedAt),
				"updated_at":  util.TimestampToString(comment.UpdatedAt),
			},
			"issue_title":         issue.Title,
			"issue_assignee_type": util.TextToPtr(issue.AssigneeType),
			"issue_assignee_id":   util.UUIDToPtr(issue.AssigneeID),
			"issue_status":        issue.Status,
		},
	})
}

func workflowArtifactContentForDiff(artifact db.WorkflowArtifact) string {
	if artifact.ContentKind == workflowArtifactKindJSON && len(artifact.ContentJson) > 0 {
		var indented bytes.Buffer
		if err := json.Indent(&indented, artifact.ContentJson, "", "  "); err == nil {
			return indented.String()
		}
		return string(artifact.ContentJson)
	}
	if artifact.ContentText.Valid {
		return artifact.ContentText.String
	}
	return ""
}

func unifiedLineDiff(baseName, targetName, baseText, targetText string) string {
	baseLines := splitDiffLines(baseText)
	targetLines := splitDiffLines(targetText)
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", baseName)
	fmt.Fprintf(&b, "+++ %s\n", targetName)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", len(baseLines), len(targetLines))

	lcs := make([][]int, len(baseLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(targetLines)+1)
	}
	for i := len(baseLines) - 1; i >= 0; i-- {
		for j := len(targetLines) - 1; j >= 0; j-- {
			if baseLines[i] == targetLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < len(baseLines) && j < len(targetLines) {
		switch {
		case baseLines[i] == targetLines[j]:
			fmt.Fprintf(&b, " %s\n", baseLines[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			fmt.Fprintf(&b, "-%s\n", baseLines[i])
			i++
		default:
			fmt.Fprintf(&b, "+%s\n", targetLines[j])
			j++
		}
	}
	for ; i < len(baseLines); i++ {
		fmt.Fprintf(&b, "-%s\n", baseLines[i])
	}
	for ; j < len(targetLines); j++ {
		fmt.Fprintf(&b, "+%s\n", targetLines[j])
	}
	return b.String()
}

func splitDiffLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
