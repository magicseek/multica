package execenv

import (
	"fmt"
	"strings"
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
	var b strings.Builder

	b.WriteString("## Task Execution Protocol\n\n")
	b.WriteString("This agent has the execution protocol setting enabled. Follow this protocol for assignment-triggered issue work. Use your Skills and Agent Identity inside each phase.\n\n")

	b.WriteString("1. **Context First**\n")
	fmt.Fprintf(&b, "   - Run `multica issue get %s --output json` to load the issue title, description, status, priority, and assignee.\n", ctx.IssueID)
	fmt.Fprintf(&b, "   - Run `multica issue comment list %s --output json` and read the full conversation history. This is mandatory; comments often carry the latest repo, handoff, blocker, or acceptance detail.\n", ctx.IssueID)
	b.WriteString("   - If project resources or repositories are listed in this file, inspect only the resources relevant to the task. For code work, check out the relevant repository with `multica repo checkout <url>`.\n\n")

	b.WriteString("2. **Work Contract**\n")
	b.WriteString("   - Synthesize the issue body, latest comments, Agent Identity, and Skills into a small work contract: requested outcome, acceptance criteria, constraints, and expected verification.\n")
	b.WriteString("   - Treat newer user comments as more current than the issue body when they conflict.\n")
	b.WriteString("   - If the work contract is still ambiguous enough that action would be guesswork, set the issue to `blocked` and post the exact missing input.\n\n")

	b.WriteString("3. **Plan Gate**\n")
	b.WriteString("   - For simple single-step work, proceed directly after forming the work contract.\n")
	b.WriteString("   - For non-trivial work, create a concise plan before editing: multi-file changes, database/schema changes, cross-layer behavior, security/data/concurrency risk, unclear tests, or unfamiliar code.\n")
	b.WriteString("   - If the issue itself asks for a plan, the plan is the deliverable; post it as the final issue comment and move the issue to `in_review`.\n\n")

	b.WriteString("4. **Execute**\n")
	fmt.Fprintf(&b, "   - After Context First and before implementation, run `multica issue status %s in_progress`.\n", ctx.IssueID)
	b.WriteString("   - Make the smallest coherent change that satisfies the work contract. Keep unrelated cleanup out of scope.\n")
	b.WriteString("   - Use the available Multica CLI for platform state, comments, attachments, repositories, and issue updates. Do not bypass it with direct HTTP calls.\n\n")

	b.WriteString("5. **Verify**\n")
	b.WriteString("   - Run the verification that proves the claim: targeted tests for changed behavior, typecheck/build for shared contracts, and manual checks when automated coverage is not available.\n")
	b.WriteString("   - If verification cannot run, record the exact reason and the residual risk in the final comment.\n\n")

	b.WriteString("6. **Report**\n")
	fmt.Fprintf(&b, "   - Post final results as a comment with `multica issue comment add %s ...`. This step is mandatory; terminal output and run logs are not delivered to the user.\n", ctx.IssueID)
	b.WriteString("   - The comment should state outcome, changed artifacts or links, verification run, and any remaining risk. Keep it concise and natural.\n")
	fmt.Fprintf(&b, "   - When done, run `multica issue status %s in_review`.\n\n", ctx.IssueID)

	b.WriteString("7. **Blocked Path**\n")
	fmt.Fprintf(&b, "   - If you cannot make meaningful progress, run `multica issue status %s blocked` and post a comment explaining the blocker, what you tried, and the exact decision or input needed.\n\n", ctx.IssueID)

	return b.String()
}
