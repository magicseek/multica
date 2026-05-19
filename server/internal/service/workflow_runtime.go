package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
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
		case workflowStepStatusWaitingManual, workflowStepStatusWaitingExternal, workflowStepStatusWaitingReview, workflowStepStatusWaitingQuality, workflowStepStatusPaused, workflowStepStatusPending:
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
