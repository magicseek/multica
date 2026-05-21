package execenv

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/execprotocol"
)

func shouldUseTaskExecutionProtocol(ctx TaskContextForEnv) bool {
	return (ctx.WorkflowRenderedMarkdown != "" || ctx.ExecutionProtocolEnabled) &&
		ctx.IssueID != "" &&
		ctx.TriggerCommentID == "" &&
		ctx.ChatSessionID == "" &&
		ctx.AutopilotRunID == "" &&
		ctx.QuickCreatePrompt == "" &&
		!ctx.IsSquadLeader
}

func renderTaskExecutionProtocol(ctx TaskContextForEnv) string {
	titleInstruction := fmt.Sprintf("Before following the workflow, run `multica issue get %s --output json`. If the title is just the user's first prompt line or you can make it materially clearer, immediately run `multica issue update %s --title \"...\"` with a concise task title.\n\n", ctx.IssueID, ctx.IssueID)
	if ctx.WorkflowRenderedMarkdown != "" {
		if ctx.WorkflowCurrentStep != nil && ctx.WorkflowCurrentStep.ID != "" {
			return titleInstruction + "## Current Workflow Step\n\nFollow the current workflow step context in the Workflow Step Tracking section above. The queued workflow snapshot remains immutable and server-owned; use `multica workflow run get \"$MULTICA_WORKFLOW_RUN_ID\" --output json` only when you need later step state after completing the current step.\n\n"
		}
		return titleInstruction + ctx.WorkflowRenderedMarkdown
	}
	tpl, ok := execprotocol.Resolve(ctx.ExecutionProtocolEnabled, ctx.ExecutionProtocolSlug)
	if !ok {
		return ""
	}
	return titleInstruction + execprotocol.Render(tpl, execprotocol.RenderContext{IssueID: ctx.IssueID})
}
