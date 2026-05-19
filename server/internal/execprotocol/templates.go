package execprotocol

import "strings"

const (
	StandardAssignmentSlug = "standard-assignment"
	TrellisTaskSlug        = "trellis-task"
)

type Template struct {
	Slug        string
	Name        string
	Description string
	Content     string
}

type RenderContext struct {
	IssueID string
}

var templates = map[string]Template{
	StandardAssignmentSlug: {
		Slug:        StandardAssignmentSlug,
		Name:        "Standard assignment",
		Description: "Structured Multica issue workflow for direct assignment tasks.",
		Content: strings.TrimSpace(`
## Task Execution Protocol

This agent has the execution protocol setting enabled. Follow this protocol for assignment-triggered issue work. Use your Skills and Agent Identity inside each phase.

1. **Context First**
   - Run `+"`multica issue get {{issue_id}} --output json`"+` to load the issue title, description, status, priority, and assignee.
   - Run `+"`multica issue comment list {{issue_id}} --output json`"+` and read the full conversation history. This is mandatory; comments often carry the latest repo, handoff, blocker, or acceptance detail.
   - If project resources or repositories are listed in this file, inspect only the resources relevant to the task. For code work, check out the relevant repository with `+"`multica repo checkout <url>`"+`.

2. **Work Contract**
   - Synthesize the issue body, latest comments, Agent Identity, and Skills into a small work contract: requested outcome, acceptance criteria, constraints, and expected verification.
   - Treat newer user comments as more current than the issue body when they conflict.
   - If the work contract is still ambiguous enough that action would be guesswork, set the issue to `+"`blocked`"+` and post the exact missing input.

3. **Plan Gate**
   - For simple single-step work, proceed directly after forming the work contract.
   - For non-trivial work, create a concise plan before editing: multi-file changes, database/schema changes, cross-layer behavior, security/data/concurrency risk, unclear tests, or unfamiliar code.
   - If the issue itself asks for a plan, the plan is the deliverable; post it as the final issue comment and move the issue to `+"`in_review`"+`.

4. **Execute**
   - After Context First and before implementation, run `+"`multica issue status {{issue_id}} in_progress`"+`.
   - Make the smallest coherent change that satisfies the work contract. Keep unrelated cleanup out of scope.
   - Use the available Multica CLI for platform state, comments, attachments, repositories, and issue updates. Do not bypass it with direct HTTP calls.

5. **Verify**
   - Run the verification that proves the claim: targeted tests for changed behavior, typecheck/build for shared contracts, and manual checks when automated coverage is not available.
   - If verification cannot run, record the exact reason and the residual risk in the final comment.

6. **Report**
   - Post final results as a comment with `+"`multica issue comment add {{issue_id}} ...`"+`. This step is mandatory; terminal output and run logs are not delivered to the user.
   - The comment should state outcome, changed artifacts or links, verification run, and any remaining risk. Keep it concise and natural.
   - When done, run `+"`multica issue status {{issue_id}} in_review`"+`.

7. **Blocked Path**
   - If you cannot make meaningful progress, run `+"`multica issue status {{issue_id}} blocked`"+` and post a comment explaining the blocker, what you tried, and the exact decision or input needed.
`) + "\n\n",
	},
	TrellisTaskSlug: {
		Slug:        TrellisTaskSlug,
		Name:        "Trellis task",
		Description: "Trellis-oriented workflow for direct assignment tasks in repositories that carry Trellis metadata.",
		Content: strings.TrimSpace(`
## Trellis Task Protocol

This agent has the Trellis execution protocol template selected. Use it for assignment-triggered issue work. Use your Skills and Agent Identity inside each phase.

1. **Context First**
   - Run `+"`multica issue get {{issue_id}} --output json`"+` to load the issue title, description, status, priority, and assignee.
   - Run `+"`multica issue comment list {{issue_id}} --output json`"+` and read the full conversation history. This is mandatory; comments often carry the latest repo, handoff, blocker, or acceptance detail.
   - If project resources or repositories are listed in this file, inspect only the resources relevant to the task. For code work, check out the relevant repository with `+"`multica repo checkout <url>`"+` before looking for Trellis state.

2. **Trellis Availability Gate**
   - In the checked-out worktree, look for .trellis/ and .trellis/workflow.md.
   - If Trellis state exists, run `+"`$trellis-continue`"+` to load the current task pointer, phase index, and workflow rules before editing.
   - If Trellis state exists but no Trellis task exists for this issue, run `+"`$trellis-start`"+` or create a new Trellis task following .trellis/workflow.md, then record the Multica issue id in that task's context.
   - If the checked-out worktree has no Trellis state and the issue or project explicitly asks for Trellis, a Trellis Task workflow, or a greenfield app/bootstrap, initialize Trellis first, then run `+"`$trellis-start`"+`; do not fall back before trying to make Trellis available.
   - If the repository has no Trellis state and the issue does not explicitly request Trellis/bootstrap, fall back to the standard assignment protocol and report that Trellis was unavailable in the final comment.

3. **Work Contract**
   - Synthesize the issue body, latest comments, Agent Identity, Skills, and Trellis task context into a small work contract: requested outcome, acceptance criteria, constraints, and expected verification.
   - Persist the agreed contract in the Trellis task PRD before implementation. Treat newer user comments as more current than the issue body when they conflict.
   - If the contract is still ambiguous enough that action would be guesswork, run `+"`multica issue status {{issue_id}} blocked`"+` and post the exact missing input.

4. **Plan And Implement**
   - After Context First and before implementation, run `+"`multica issue status {{issue_id}} in_progress`"+`.
   - Follow the Trellis implementation phase for the task. Keep edits inside the Trellis task scope and update task state when phase or checklist status changes.
   - Make the smallest coherent change that satisfies the work contract. Keep unrelated cleanup out of scope.

5. **Check And Update Spec**
   - Run the Trellis check phase and the verification that proves the claim: targeted tests for changed behavior, typecheck/build for shared contracts, and manual checks when automated coverage is not available.
   - If the change creates or clarifies durable behavior, run the Trellis update-spec phase so .trellis/spec/ stays current.
   - If verification cannot run, record the exact reason and residual risk in the final comment.

6. **Finish Work**
   - Run `+"`$trellis-finish-work`"+` after implementation, checks, and any needed spec updates are complete.
   - Post final results as a comment with `+"`multica issue comment add {{issue_id}} ...`"+`. This step is mandatory; terminal output and run logs are not delivered to the user.
   - The comment should state outcome, changed artifacts or links, Trellis task status, verification run, and any remaining risk. Keep it concise and natural.
   - When done, run `+"`multica issue status {{issue_id}} in_review`"+`.

7. **Blocked Path**
   - If you cannot make meaningful progress, run `+"`multica issue status {{issue_id}} blocked`"+` and post a comment explaining the blocker, what you tried, and the exact decision or input needed.
`) + "\n\n",
	},
}

func Get(slug string) (Template, bool) {
	tpl, ok := templates[strings.TrimSpace(slug)]
	return tpl, ok
}

func IsKnownSlug(slug string) bool {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return true
	}
	_, ok := templates[slug]
	return ok
}

func Resolve(enabled bool, slug string) (Template, bool) {
	if !enabled {
		return Template{}, false
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = StandardAssignmentSlug
	}
	tpl, ok := Get(slug)
	if !ok {
		tpl, ok = Get(StandardAssignmentSlug)
	}
	return tpl, ok
}

func Render(tpl Template, ctx RenderContext) string {
	replacer := strings.NewReplacer(
		"{{issue_id}}", ctx.IssueID,
	)
	return replacer.Replace(tpl.Content)
}
