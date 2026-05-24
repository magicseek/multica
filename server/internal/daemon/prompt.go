package daemon

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig. The provider string is used by
// comment-triggered tasks: Codex's per-turn reply template needs the
// platform-aware "stdin or file" variant, every other provider gets a
// lightweight inline template (or Windows file for any provider on
// Windows).
func BuildPrompt(task Task, provider string) string {
	if task.ChatSessionID != "" {
		return buildChatPrompt(task)
	}
	if task.TriggerCommentID != "" {
		return buildCommentPrompt(task, provider)
	}
	if task.AutopilotRunID != "" {
		return buildAutopilotPrompt(task)
	}
	if task.QuickCreatePrompt != "" {
		return buildQuickCreatePrompt(task)
	}
	if task.TaskBundle != nil {
		return buildTaskBundlePrompt(task)
	}
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then complete it.\n", task.IssueID)
	fmt.Fprintf(&b, "If you need comment history, `multica issue comment list %s --output json` returns all comments for the issue (server caps at 2000). Pass `--since <RFC3339>` to fetch only comments newer than a known cursor.\n", task.IssueID)
	return b.String()
}

func buildTaskBundlePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a request-efficient Multica task bundle executor.\n\n")
	fmt.Fprintf(&b, "Task bundle ID: %s\n", task.TaskBundle.ID)
	fmt.Fprintf(&b, "Runtime budget: %d seconds\n", task.TaskBundle.RuntimeBudgetSeconds)
	fmt.Fprintf(&b, "Changeset mode: %s\n\n", task.TaskBundle.ChangesetMode)
	b.WriteString("Process the bundle items sequentially in the listed order. Do not spawn separate provider sessions for each item. Start the next issue only after checkpointing the current item.\n\n")
	b.WriteString("For each item:\n")
	b.WriteString("1. Run `multica issue get <issue-id> --output json` and inspect comments/resources.\n")
	b.WriteString("2. Write outputs under the item's output namespace so issue artifacts do not overwrite each other.\n")
	b.WriteString("3. When the item reaches a terminal outcome, run `multica task-bundle checkpoint <item-id> --status completed|failed|blocked|input_needed|cancelled` before continuing.\n\n")
	if task.Agent != nil && task.Agent.RequestEfficientEnabled && strings.Contains(task.Agent.Instructions, "## Squad Operating Protocol") {
		b.WriteString("Because you are a request-efficient squad lead for this bundle, execute directly by default. Delegate only when the user explicitly requested delegation, another squad member has a unique required capability, or you checkpoint the current item as blocked and need follow-up.\n\n")
	}
	b.WriteString("Bundle items:\n")
	for _, item := range task.TaskBundle.Items {
		fmt.Fprintf(&b, "- %d. item `%s`, issue `%s`, status `%s`, output namespace `%s`\n", item.Position, item.ID, item.IssueID, item.Status, item.OutputNamespace)
	}
	return b.String()
}

// buildQuickCreatePrompt constructs a prompt for quick-create tasks. The
// user typed a single natural-language sentence in the create-issue modal;
// the agent's job is to translate it into one `multica issue create` CLI
// invocation, using its judgment to decide whether fetching referenced URLs
// would produce a better issue. No issue exists yet, so the agent must NOT
// call `multica issue get` or attempt to comment — there's nothing to read
// or reply to.
func buildQuickCreatePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a quick-create assistant for a Multica workspace.\n\n")
	b.WriteString("A user captured the following input via the quick-create modal. There is NO existing issue. Your job is to create a well-formed issue from this input with a single `multica issue create` command.\n\n")
	fmt.Fprintf(&b, "User input:\n> %s\n\n", task.QuickCreatePrompt)

	b.WriteString("Field rules:\n\n")

	// title
	b.WriteString("- **title**: required. A concise but semantically rich summary. If the input references external resources (PRs, issues, URLs), use your judgment on whether fetching the resource would produce a meaningfully better title — e.g. \"review PR #123\" → \"Review PR #123: Refactor auth module to OAuth2\". Strip filler words but preserve key semantic information.\n\n")

	// description — the core optimization
	b.WriteString("- **description**: The description is the executing agent's primary context. Aim for high fidelity — they should grasp the user's intent as if they had read the raw input themselves. Use a two-section structure:\n\n")
	b.WriteString("  1. **User request** — Faithfully restate what the user wants in their own words. Preserve specific names, identifiers, file paths, code snippets, and technical terms verbatim. Strip non-spec material before writing it (this is removal, not paraphrasing): verbal routing wrappers about creating the issue or routing it (e.g. \"create an issue\", \"分配给 X\", \"让 @X 处理\") and pure conversational fillers (e.g. \"对吧？\"). When in doubt, keep it.\n\n")
	b.WriteString("     CC exception: `multica issue create` has no `--subscriber` flag, and the platform auto-subscribes members whose `[@Name](mention://member/<uuid>)` link appears in the description. When the user wrote \"cc @Y\", strip the verbal \"cc\" wrapper from the User request body and append a final `CC: <mention link(s)>` line to the description so the cc routing still fires.\n\n")
	b.WriteString("  2. **Context** — include ONLY when the input cited external resources AND you successfully fetched them AND they produced verifiable facts worth recording. Summarize facts only (e.g. \"PR #45 changes auth to JWT\"), not interpretation or unsolicited reference implementations. If you have nothing factual to add, omit the section entirely — never use it as an apology log for resources you could not fetch.\n\n")
	b.WriteString("  Hard rules: never invent requirements, implementation details, or acceptance criteria the user did not express; never reduce multi-sentence input to a single vague sentence; never echo the title.\n\n")

	// priority
	b.WriteString("- **priority**: one of `urgent`, `high`, `medium`, `low`, or omit. Map P0/P1 → urgent/high; \"asap\" → urgent. If unspecified, omit.\n\n")

	// assignee
	b.WriteString("- **assignee**:\n")
	b.WriteString("    - When the user names someone (\"assign to X\" / \"@X\"), call `multica workspace members --output json`, `multica agent list --output json`, and `multica squad list --output json` and find the matching entity by display name. Squads are first-class assignees too — a squad name (e.g. \"Super Human\") routes work to the squad leader, who then delegates. On a clean unambiguous match, prefer `--assignee-id <uuid>` using the `user_id` (member) or `id` (agent or squad) from that JSON — UUID matching is exact and robust to name collisions in workspaces with overlapping names. `--assignee <name>` (fuzzy) is acceptable as a fallback when names are unambiguous. On no match or ambiguous match, do NOT pass either flag — instead append a final line to the description: `Unrecognized assignee: X`.\n")
	b.WriteString("    - Treat bare @-routing as an assignee directive even when the user did not write the English word \"assign\". This includes Chinese imperatives like `让 @独立团 review 这个 PR`, `给 @X 处理`, or `交给 @X`; strip the leading `@`/`＠` before matching display names. Do not keep that routing wrapper or `@Name` in the description unless it is a true CC-style notification rather than ownership. If the matched entity is a squad, pass the squad's `id` as `--assignee-id`, not the leader agent's id.\n")
	agentID := ""
	agentName := ""
	if task.Agent != nil {
		agentID = task.Agent.ID
		agentName = task.Agent.Name
	}
	switch {
	case task.SquadID != "":
		// The user opened quick-create with a SQUAD selected. The task
		// runs on the squad's leader agent, but the squad is the expected
		// owner — assigning to the leader would mask the squad's
		// delegation flow. Always point the default at the squad UUID.
		if task.SquadName != "" {
			fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to the picker SQUAD %q: pass `--assignee-id %q` (the squad's UUID). The user opened quick-create with the squad selected; you (the leader agent) are running on the squad's behalf, so the squad — not you — is the expected owner. Never leave the issue unassigned, and do not assign it to your own agent UUID.\n\n", task.SquadName, task.SquadID)
		} else {
			fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to the picker SQUAD: pass `--assignee-id %q` (the squad's UUID). The user opened quick-create with the squad selected; you (the leader agent) are running on the squad's behalf, so the squad — not you — is the expected owner. Never leave the issue unassigned, and do not assign it to your own agent UUID.\n\n", task.SquadID)
		}
	case agentID != "":
		fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to YOURSELF: pass `--assignee-id %q` (your agent UUID). The picker agent is the expected owner because the user opened quick-create with you selected — never leave the issue unassigned. Use the UUID flag, not `--assignee <name>`, so the assignment is unambiguous even when other agents share part of your name.\n\n", agentID)
	case agentName != "":
		fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to YOURSELF: pass `--assignee %q`. The picker agent is the expected owner because the user opened quick-create with you selected — never leave the issue unassigned.\n\n", agentName)
	default:
		b.WriteString("    - When the user did NOT name an assignee, default to YOURSELF (the picker agent): pass `--assignee-id <your agent UUID>` (preferred) or `--assignee <your agent name>`. Never leave the issue unassigned.\n\n")
	}

	// project — pinned by the modal when the user picked one, otherwise
	// omitted so the platform routes to the workspace default. Always pass
	// the UUID (never a name) so the issue lands in the right project even
	// when several share a title.
	if task.ProjectID != "" {
		if task.ProjectTitle != "" {
			fmt.Fprintf(&b, "- **project**: required for this run. Pass `--project %q` so the new issue lands in project %q (the user picked it in the quick-create modal). Do not infer a different project from the prompt text — the modal selection is authoritative.\n", task.ProjectID, task.ProjectTitle)
		} else {
			fmt.Fprintf(&b, "- **project**: required for this run. Pass `--project %q` so the new issue lands in the project the user picked in the quick-create modal. Do not infer a different project from the prompt text — the modal selection is authoritative.\n", task.ProjectID)
		}
	} else {
		b.WriteString("- **project**: omit. The platform will route the issue to the workspace default.\n")
	}
	b.WriteString("- **status**: omit (defaults to `todo`).\n")
	b.WriteString("- **attachments**: do NOT pass `--attachment`. The flag only accepts LOCAL file paths. Any image URL in the user input is already markdown — keep it inline in `--description` instead.\n\n")

	// output format
	b.WriteString("Output format:\n")
	b.WriteString("- Run exactly one `multica issue create --output json` invocation. Do not retry for any reason — even on non-zero exit. The issue may already exist; another attempt would create a duplicate.\n")
	b.WriteString("- Parse the JSON response to read the created issue's `identifier` (preferred) or `id` (fallback). Do not scrape human output and do not assume any workspace issue prefix such as `MUL-`; workspaces can use custom prefixes.\n")
	b.WriteString("- After success, print exactly one line: `Created <identifier-or-id>: <title>` and exit. No commentary, no follow-up tool calls.\n")
	b.WriteString("- Do NOT call `multica issue get` or `multica issue comment add` — there is no issue to query or comment on.\n")
	b.WriteString("- On CLI error or JSON parse error, exit with the error as the only output. The platform writes a failure notification automatically.\n")
	return b.String()
}

// buildCommentPrompt constructs a prompt for comment-triggered tasks.
// The triggering comment content is embedded directly so the agent cannot
// miss it, even when stale output files exist in a reused workdir.
// The reply instructions (including the current TriggerCommentID as --parent)
// are re-emitted on every turn so resumed sessions cannot carry forward a
// previous turn's --parent UUID.
func buildCommentPrompt(task Task, provider string) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	if task.TriggerCommentContent != "" {
		authorLabel := "A user"
		if task.TriggerAuthorType == "agent" {
			name := task.TriggerAuthorName
			if name == "" {
				name = "another agent"
			}
			authorLabel = fmt.Sprintf("Another agent (%s)", name)
		}
		fmt.Fprintf(&b, "[NEW COMMENT] %s just left a new comment. Focus on THIS comment — do not confuse it with previous ones:\n\n", authorLabel)
		fmt.Fprintf(&b, "> %s\n\n", task.TriggerCommentContent)
		if task.TriggerAuthorType == "agent" {
			b.WriteString("⚠️ The triggering comment was posted by another agent. Decide whether a reply is warranted. If you produced actual work this turn (investigated, fixed something, answered a real question), post the result as a normal reply — that is NOT a noise comment, and the standard rule that final results must be delivered via comment still applies. If the triggering comment was a pure acknowledgment, thanks, or sign-off AND you produced no work this turn, do NOT reply — and do NOT post a comment saying 'No reply needed' or similar. Simply exit with no output. Silence is the preferred way to end agent-to-agent threads. If you do reply, do not @mention the other agent as a sign-off (that re-triggers them and starts a loop).\n\n")
		}
		if task.Agent != nil && strings.Contains(task.Agent.Instructions, "## Squad Operating Protocol") {
			fmt.Fprintf(&b, "⚠️ **Squad leader no_action rule:** If you decide no action is needed, call `multica squad activity %s no_action --reason \"...\"` and EXIT. DO NOT post any comment — not even one that says \"no action needed\" or \"exiting silently\". The squad activity call records your decision; a comment is redundant noise.\n\n", task.IssueID)
		}
	}
	if task.TriggerAuthorType == "agent" {
		fmt.Fprintf(&b, "If this is a concrete handoff from another agent and you actually do the work, manage the issue status: run `multica issue status %s in_progress` before starting, then run `multica issue status %s in_review` after posting your result comment. If the comment only needs a small answer, acknowledgment, or no action, leave the issue status unchanged.\n\n", task.IssueID, task.IssueID)
	}
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then decide how to proceed.\n\n", task.IssueID)
	fmt.Fprintf(&b, "If you need comment history, `multica issue comment list %s --output json` returns all comments for the issue (server caps at 2000). Pass `--since <RFC3339>` to fetch only comments newer than a known cursor.\n\n", task.IssueID)
	b.WriteString(execenv.BuildCommentReplyInstructions(provider, task.IssueID, task.TriggerCommentID))
	return b.String()
}

// buildChatPrompt constructs a prompt for interactive chat tasks.
func buildChatPrompt(task Task) string {
	if task.Plan != nil {
		return buildPlanChatPrompt(task)
	}
	var b strings.Builder
	b.WriteString("You are running as a chat assistant for a Multica workspace.\n")
	b.WriteString("A user is chatting with you directly. Respond to their message.\n\n")
	proposalPath := filepath.ToSlash(filepath.Join(execenv.ChatStructuredOutputRelativeDir(task.ChatSessionID), TaskIssueProposalsManifestFileName))
	fmt.Fprintf(&b, "Issue/task creation requests in chat are proposal-first. If the user asks you to create, split, plan, or generate issues/tasks, write reviewable proposal cards to `$%s/%s` (relative path `%s`) and mention that they can approve them in the chat UI. Do not run `multica issue create` from chat unless the user explicitly asks to create immediately without approval.\n\n", execenv.StructuredOutputDirEnv, TaskIssueProposalsManifestFileName, proposalPath)
	fmt.Fprintf(&b, "User message:\n%s\n", task.ChatMessage)
	// List attachments by id + filename so the agent can fetch them via
	// the CLI. We deliberately do NOT inline the URL: chat attachments
	// live behind a signed CDN with a short TTL, so by the time the agent
	// has finished thinking the URL embedded in the markdown body may
	// have expired. `multica attachment download <id>` re-signs at click
	// time and is the only reliable path.
	if len(task.ChatMessageAttachments) > 0 {
		b.WriteString("\nAttachments on this message:\n")
		for _, a := range task.ChatMessageAttachments {
			if a.ContentType != "" {
				fmt.Fprintf(&b, "- id=%s filename=%q content_type=%s\n", a.ID, a.Filename, a.ContentType)
			} else {
				fmt.Fprintf(&b, "- id=%s filename=%q\n", a.ID, a.Filename)
			}
		}
		b.WriteString("Use `multica attachment download <id>` to fetch each file locally before referring to it.\n")
	}
	return b.String()
}

func buildPlanChatPrompt(task Task) string {
	plan := task.Plan
	var b strings.Builder
	b.WriteString("You are running as a Plan mode assistant for a Multica Chat Plan Run.\n")
	b.WriteString("A Chat Plan Run is a multi-turn planning exchange that turns a rough idea into reviewable issue proposals. Keep all plan state in the server-owned transcript and structured summary.\n\n")

	fmt.Fprintf(&b, "Plan run ID: %s\n", plan.RunID)
	fmt.Fprintf(&b, "Plan status: %s\n", plan.Status)
	fmt.Fprintf(&b, "Plan actor: %s %s\n", plan.ActorType, plan.ActorID)
	fmt.Fprintf(&b, "Lead agent ID: %s\n\n", plan.LeadAgentID)

	b.WriteString("## Plan Engine\n\n")
	fmt.Fprintf(&b, "Engine: %s (`%s`)\n", plan.PlanEngine.Label, plan.PlanEngine.ID)
	fmt.Fprintf(&b, "Engine version: %s\n", plan.PlanEngine.Version)
	if strings.TrimSpace(plan.PlanEngine.Description) != "" {
		fmt.Fprintf(&b, "Description: %s\n", plan.PlanEngine.Description)
	}
	b.WriteString("\nProtocol:\n")
	b.WriteString(plan.PlanEngine.Protocol)
	b.WriteString("\n\n")

	b.WriteString("## Current Plan Summary\n\n")
	if len(plan.Summary) > 0 {
		b.WriteString(prettyJSON(plan.Summary))
	} else {
		b.WriteString("{}")
	}
	b.WriteString("\n\n")

	b.WriteString("## Structured Outputs\n\n")
	planSummaryPath := plan.PlanSummaryPath
	if planSummaryPath == "" {
		planSummaryPath = filepath.ToSlash(filepath.Join(execenv.ChatStructuredOutputRelativeDir(task.ChatSessionID), TaskPlanSummaryManifestFileName))
	}
	proposalPath := plan.ProposalPath
	if proposalPath == "" {
		proposalPath = filepath.ToSlash(filepath.Join(execenv.ChatStructuredOutputRelativeDir(task.ChatSessionID), TaskIssueProposalsManifestFileName))
	}
	fmt.Fprintf(&b, "- Update the Plan Summary at `$%s/%s` (relative path `%s`) whenever requirements, rejected options, consensus notes, or open questions change.\n", execenv.StructuredOutputDirEnv, TaskPlanSummaryManifestFileName, planSummaryPath)
	b.WriteString("  Shape: `{\"version\":1,\"confirmed_requirements\":[],\"rejected_options\":[],\"consensus_notes\":[],\"open_questions\":[]}`\n")
	fmt.Fprintf(&b, "- When the plan is ready for human review, write issue proposals to `$%s/%s` (relative path `%s`).\n", execenv.StructuredOutputDirEnv, TaskIssueProposalsManifestFileName, proposalPath)
	b.WriteString("- Do not run `multica issue create` from Plan mode unless the user explicitly asks to bypass proposal approval.\n\n")

	if len(plan.Transcript) > 0 {
		b.WriteString("## Plan Transcript\n\n")
		for _, msg := range plan.Transcript {
			author := msg.AuthorType
			if msg.AuthorAgentID != nil {
				author = "agent:" + *msg.AuthorAgentID
			}
			fmt.Fprintf(&b, "- %s %s: %s\n", msg.Role, author, compactPromptLine(msg.Content))
		}
		b.WriteString("\n")
	}

	if len(plan.Consultations) > 0 {
		b.WriteString("## Consultation Status\n\n")
		for _, consultation := range plan.Consultations {
			fmt.Fprintf(&b, "- %s -> %s: %s\n", consultation.RequesterAgentID, consultation.TargetAgentID, consultation.Status)
		}
		b.WriteString("\n")
	}

	if plan.TaskKind == "plan_consultation" && plan.Consultation != nil {
		b.WriteString("## Consultation Task\n\n")
		b.WriteString("You are a squad helper. Answer the lead agent's specific planning request; do not take over the final synthesis.\n\n")
		if plan.Consultation.TargetAgentMention != "" {
			fmt.Fprintf(&b, "You are the helper addressed as: %s\n", plan.Consultation.TargetAgentMention)
		}
		if plan.Consultation.LeadMention != "" {
			fmt.Fprintf(&b, "When you respond, include this exact lead mention so the server can resume the lead: %s\n", plan.Consultation.LeadMention)
		}
		if strings.TrimSpace(plan.Consultation.RequestContent) != "" {
			fmt.Fprintf(&b, "\nLead request:\n%s\n\n", plan.Consultation.RequestContent)
		}
		b.WriteString("Return one concise helper opinion with risks, missing facts, and recommendation. Your final assistant output is saved into the same chat transcript.\n")
		return b.String()
	}

	if task.ChatMessage != "" {
		fmt.Fprintf(&b, "User message for this turn:\n%s\n\n", task.ChatMessage)
	} else {
		b.WriteString("This is a lead continuation after squad consultation updates. Synthesize available helper input and continue the plan.\n\n")
	}

	if plan.Squad != nil {
		b.WriteString("## Squad Consultation\n\n")
		fmt.Fprintf(&b, "Squad: %s\n", plan.Squad.Name)
		if plan.Squad.LeadMention != "" {
			fmt.Fprintf(&b, "Your lead mention: %s\n", plan.Squad.LeadMention)
		}
		if len(plan.Squad.Helpers) > 0 {
			b.WriteString("Eligible helper agents. Use only these exact mention links when a bounded helper opinion is needed:\n")
			for _, helper := range plan.Squad.Helpers {
				role := helper.Role
				if role == "" {
					role = "member"
				}
				fmt.Fprintf(&b, "- %s (%s): %s\n", helper.Name, role, helper.Mention)
			}
			b.WriteString("Mentions of agents outside this roster are ignored. Ask for a specific opinion; do not start an unbounded roundtable.\n\n")
		} else {
			b.WriteString("No eligible helper agents are available; synthesize the plan yourself and record the gap if it matters.\n\n")
		}
	}

	if len(task.ChatMessageAttachments) > 0 {
		b.WriteString("Attachments on this message:\n")
		for _, a := range task.ChatMessageAttachments {
			if a.ContentType != "" {
				fmt.Fprintf(&b, "- id=%s filename=%q content_type=%s\n", a.ID, a.Filename, a.ContentType)
			} else {
				fmt.Fprintf(&b, "- id=%s filename=%q\n", a.ID, a.Filename)
			}
		}
		b.WriteString("Use `multica attachment download <id>` to fetch each file locally before referring to it.\n\n")
	}

	b.WriteString("Final assistant output is captured as the chat reply. If more user input is needed, ask the next best question. If the plan is ready, say so and write proposal cards for review.\n")
	return b.String()
}

func compactPromptLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= 500 {
		return s
	}
	runes := []rune(s)
	return string(runes[:500]) + "..."
}

func prettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.TrimSpace(string(raw))
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(data)
}

// buildAutopilotPrompt constructs a prompt for run_only autopilot tasks.
func buildAutopilotPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	b.WriteString("This task was triggered by an Autopilot in run-only mode. There is no assigned Multica issue for this run.\n\n")
	fmt.Fprintf(&b, "Autopilot run ID: %s\n", task.AutopilotRunID)
	if task.AutopilotID != "" {
		fmt.Fprintf(&b, "Autopilot ID: %s\n", task.AutopilotID)
	}
	if task.AutopilotTitle != "" {
		fmt.Fprintf(&b, "Autopilot title: %s\n", task.AutopilotTitle)
	}
	if task.AutopilotSource != "" {
		fmt.Fprintf(&b, "Trigger source: %s\n", task.AutopilotSource)
	}
	if strings.TrimSpace(string(task.AutopilotTriggerPayload)) != "" {
		fmt.Fprintf(&b, "Trigger payload:\n%s\n", strings.TrimSpace(string(task.AutopilotTriggerPayload)))
	}
	b.WriteString("\nAutopilot instructions:\n")
	if strings.TrimSpace(task.AutopilotDescription) != "" {
		b.WriteString(task.AutopilotDescription)
		b.WriteString("\n\n")
	} else if task.AutopilotTitle != "" {
		fmt.Fprintf(&b, "%s\n\n", task.AutopilotTitle)
	} else {
		b.WriteString("No additional autopilot instructions were provided. Inspect the autopilot configuration before proceeding.\n\n")
	}
	if task.AutopilotID != "" {
		fmt.Fprintf(&b, "Start by running `multica autopilot get %s --output json` if you need the full autopilot configuration, then complete the instructions above.\n", task.AutopilotID)
	} else {
		b.WriteString("Complete the instructions above.\n")
	}
	b.WriteString("Do not run `multica issue get`; this run does not have an issue ID.\n")
	return b.String()
}
