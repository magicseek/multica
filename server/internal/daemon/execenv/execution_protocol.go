package execenv

import (
	"github.com/multica-ai/multica/server/internal/execprotocol"
)

func shouldUseTaskExecutionProtocol(ctx TaskContextForEnv) bool {
	return ctx.ExecutionProtocolEnabled &&
		ctx.IssueID != "" &&
		ctx.TriggerCommentID == "" &&
		ctx.ChatSessionID == "" &&
		ctx.AutopilotRunID == "" &&
		ctx.QuickCreatePrompt == "" &&
		!ctx.IsSquadLeader
}

func renderTaskExecutionProtocol(ctx TaskContextForEnv) string {
	tpl, ok := execprotocol.Resolve(ctx.ExecutionProtocolEnabled, ctx.ExecutionProtocolSlug)
	if !ok {
		return ""
	}
	return execprotocol.Render(tpl, execprotocol.RenderContext{IssueID: ctx.IssueID})
}
