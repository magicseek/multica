package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowdefs"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WorkflowSnapshot struct {
	SchemaVersion      int                         `json:"schema_version"`
	TriggerType        string                      `json:"trigger_type"`
	DefinitionID       string                      `json:"definition_id"`
	RevisionID         string                      `json:"revision_id"`
	RevisionNumber     int32                       `json:"revision_number"`
	WorkflowName       string                      `json:"workflow_name"`
	Origin             string                      `json:"origin"`
	SystemKey          string                      `json:"system_key,omitempty"`
	Schema             json.RawMessage             `json:"schema"`
	RenderedMarkdown   string                      `json:"rendered_markdown"`
	CapabilityWarnings []WorkflowCapabilityWarning `json:"capability_warnings,omitempty"`
	ResolvedAt         string                      `json:"resolved_at,omitempty"`
}

type WorkflowCapabilityWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ResolvedWorkflowSnapshot struct {
	DefinitionID pgtype.UUID
	RevisionID   pgtype.UUID
	SnapshotJSON []byte
}

func (s *TaskService) EnsureSystemWorkflowDefinitions(ctx context.Context, workspaceID pgtype.UUID) (map[string]db.WorkflowDefinition, error) {
	byKey := make(map[string]db.WorkflowDefinition)
	for _, seed := range workflowdefs.SystemSeeds() {
		def, err := s.Queries.UpsertSystemWorkflowDefinition(ctx, db.UpsertSystemWorkflowDefinitionParams{
			WorkspaceID: workspaceID,
			Name:        seed.Name,
			Description: seed.Description,
			SystemKey:   pgtype.Text{String: seed.Key, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("upsert system workflow %s: %w", seed.Key, err)
		}
		rev, err := s.Queries.UpsertSystemWorkflowRevision(ctx, db.UpsertSystemWorkflowRevisionParams{
			WorkflowDefinitionID: def.ID,
			Schema:               seed.Schema,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert system workflow revision %s: %w", seed.Key, err)
		}
		if def.CurrentPublishedRevisionID != rev.ID {
			def, err = s.Queries.SetWorkflowCurrentPublishedRevision(ctx, db.SetWorkflowCurrentPublishedRevisionParams{
				ID:                         def.ID,
				CurrentPublishedRevisionID: rev.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("set current system workflow revision %s: %w", seed.Key, err)
			}
		}
		byKey[seed.Key] = def
	}

	standard := byKey[workflowdefs.SystemStandardAssignment]
	comment := byKey[workflowdefs.SystemCommentResponse]
	if standard.ID.Valid && comment.ID.Valid {
		if _, err := s.Queries.SetWorkspaceWorkflowDefaultsIfNull(ctx, db.SetWorkspaceWorkflowDefaultsIfNullParams{
			ID:                                    workspaceID,
			DefaultAssignmentWorkflowDefinitionID: standard.ID,
			DefaultCommentWorkflowDefinitionID:    comment.ID,
		}); err != nil {
			return nil, fmt.Errorf("set workspace workflow defaults: %w", err)
		}
	}

	return byKey, nil
}

func (s *TaskService) ResolveWorkflowSnapshotForIssueTask(ctx context.Context, issue db.Issue, triggerCommentID pgtype.UUID) (*ResolvedWorkflowSnapshot, error) {
	systemDefs, err := s.EnsureSystemWorkflowDefinitions(ctx, issue.WorkspaceID)
	if err != nil {
		return nil, err
	}

	workspace, err := s.Queries.GetWorkspace(ctx, issue.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load workspace workflow defaults: %w", err)
	}

	var warnings []WorkflowCapabilityWarning
	defID := pgtype.UUID{}
	fallbackKey := workflowdefs.SystemStandardAssignment
	triggerType := "assignment"
	if triggerCommentID.Valid {
		triggerType = "comment"
		fallbackKey = workflowdefs.SystemCommentResponse
		defID = workspace.DefaultCommentWorkflowDefinitionID
	} else if issue.WorkflowOverrideDefinitionID.Valid {
		defID = issue.WorkflowOverrideDefinitionID
	} else if issue.ProjectID.Valid {
		project, err := s.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
			ID:          issue.ProjectID,
			WorkspaceID: issue.WorkspaceID,
		})
		if err == nil && project.WorkflowDefinitionID.Valid {
			defID = project.WorkflowDefinitionID
		}
	} else {
		defID = workspace.DefaultAssignmentWorkflowDefinitionID
	}

	if !defID.Valid {
		if fallback, ok := systemDefs[fallbackKey]; ok {
			defID = fallback.ID
		}
	}

	def, rev, err := s.loadPublishedWorkflow(ctx, issue.WorkspaceID, defID)
	if err != nil {
		warnings = append(warnings, WorkflowCapabilityWarning{
			Code:    "workflow_fallback",
			Message: "selected workflow was unavailable; used system fallback",
		})
		fallback, ok := systemDefs[fallbackKey]
		if !ok {
			return nil, err
		}
		def, rev, err = s.loadPublishedWorkflow(ctx, issue.WorkspaceID, fallback.ID)
		if err != nil {
			return nil, err
		}
	}

	rendered := workflowdefs.Render(rev.Schema, workflowdefs.RenderContext{
		IssueID:          util.UUIDToString(issue.ID),
		TriggerCommentID: util.UUIDToString(triggerCommentID),
	})
	for _, warning := range rendered.Warnings {
		warnings = append(warnings, WorkflowCapabilityWarning{
			Code:    "render_warning",
			Message: warning,
		})
	}

	systemKey := ""
	if def.SystemKey.Valid {
		systemKey = def.SystemKey.String
	}
	snapshot := WorkflowSnapshot{
		SchemaVersion:      1,
		TriggerType:        triggerType,
		DefinitionID:       util.UUIDToString(def.ID),
		RevisionID:         util.UUIDToString(rev.ID),
		RevisionNumber:     rev.RevisionNumber,
		WorkflowName:       def.Name,
		Origin:             def.Origin,
		SystemKey:          systemKey,
		Schema:             json.RawMessage(rev.Schema),
		RenderedMarkdown:   rendered.Markdown,
		CapabilityWarnings: warnings,
		ResolvedAt:         time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow snapshot: %w", err)
	}

	return &ResolvedWorkflowSnapshot{
		DefinitionID: def.ID,
		RevisionID:   rev.ID,
		SnapshotJSON: raw,
	}, nil
}

func (s *TaskService) loadPublishedWorkflow(ctx context.Context, workspaceID, definitionID pgtype.UUID) (db.WorkflowDefinition, db.WorkflowRevision, error) {
	if !definitionID.Valid {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, pgx.ErrNoRows
	}
	def, err := s.Queries.GetWorkflowDefinitionInWorkspace(ctx, db.GetWorkflowDefinitionInWorkspaceParams{
		ID:          definitionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, err
	}
	if def.ArchivedAt.Valid {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, pgx.ErrNoRows
	}
	rev, err := s.Queries.GetCurrentWorkflowRevision(ctx, db.GetCurrentWorkflowRevisionParams{
		ID:          def.ID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, err
	}
	if rev.Status != "published" {
		return db.WorkflowDefinition{}, db.WorkflowRevision{}, pgx.ErrNoRows
	}
	return def, rev, nil
}

func (s *TaskService) logWorkflowResolutionError(issue db.Issue, err error) {
	slog.Warn("workflow snapshot resolution failed",
		"issue_id", util.UUIDToString(issue.ID),
		"workspace_id", util.UUIDToString(issue.WorkspaceID),
		"error", err,
	)
}
