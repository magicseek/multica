package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildQuickCreatePromptRules locks in the rules that govern how the
// quick-create agent is allowed to translate raw user input into the issue
// description body. Each substring corresponds to a concrete failure mode
// observed in production output:
//   - meta-instructions ("create an issue", "cc @X") leaking into the body
//   - the Context section being misused as an apology log when no external
//     references were actually fetched
//   - hard-line rules being silently dropped on prompt rewrites
func TestBuildQuickCreatePromptRules(t *testing.T) {
	out := buildQuickCreatePrompt(Task{QuickCreatePrompt: "fix the login button color"})

	mustContain := []string{
		// high-fidelity invariant
		"Faithfully restate what the user wants",
		"Preserve specific names, identifiers, file paths",
		// strip non-spec material: verbal routing wrappers + conversational fillers
		"verbal routing wrappers about creating the issue",
		"pure conversational fillers",
		// cc routing must survive: mention link stays in description so the
		// auto-subscribe path fires (multica issue create has no --subscriber flag)
		"CC exception",
		"auto-subscribes members",
		// context section is conditional and must not be an apology log
		"include ONLY when the input cited external resources",
		"never use it as an apology log",
		// output/reporting must be workspace-prefix agnostic. Workspaces can
		// use custom issue prefixes, so a successful issue creation should
		// not look failed merely because the identifier does not match one
		// fixed prefix.
		"multica issue create --output json",
		"JSON response",
		"identifier",
		"Do not scrape human output",
		"do not assume any workspace issue prefix",
		"Created <identifier-or-id>: <title>",
		// hard rules
		"never invent requirements",
		"never reduce multi-sentence input",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildQuickCreatePrompt output missing required rule: %q", s)
		}
	}
}

// TestBuildQuickCreatePromptAssigneeIncludesSquads locks in the MUL-2165
// fix: the assignee-resolution rules must tell the agent to consult the
// squad list alongside members and agents. Before this, a quick-create
// input like "assign to <SquadName>" silently fell through to
// "Unrecognized assignee" because squads were never queried.
func TestBuildQuickCreatePromptAssigneeIncludesSquads(t *testing.T) {
	out := buildQuickCreatePrompt(Task{QuickCreatePrompt: "fix the login button color"})
	mustContain := []string{
		"multica squad list",
		"Squads are first-class assignees",
		"Treat bare @-routing as an assignee directive",
		"让 @独立团 review 这个 PR",
		"pass the squad's `id` as `--assignee-id`",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildQuickCreatePrompt assignee block missing %q\n--- output ---\n%s", s, out)
		}
	}
}

// TestBuildQuickCreatePromptSquadDefaultsToSquad locks in the MUL-2203
// fix: when the picker was a squad, the task runs on the squad's leader
// agent, but the default assignee for issues created by this run must
// point at the SQUAD's UUID — not the leader agent's UUID. The previous
// "default to YOURSELF" instruction made squad-created issues land under
// the leader, hiding them from the squad's delegation flow.
func TestBuildQuickCreatePromptSquadDefaultsToSquad(t *testing.T) {
	const (
		squadID   = "aaaa1111-2222-3333-4444-555555555555"
		squadName = "独立团"
		leaderID  = "bbbb1111-2222-3333-4444-666666666666"
	)
	out := buildQuickCreatePrompt(Task{
		QuickCreatePrompt: "fix the login button color",
		Agent:             &AgentData{ID: leaderID, Name: "leader-agent"},
		SquadID:           squadID,
		SquadName:         squadName,
	})

	// The default-assignee instruction must point at the squad UUID.
	if !strings.Contains(out, "--assignee-id \""+squadID+"\"") {
		t.Errorf("buildQuickCreatePrompt with SquadID must default to the squad's UUID, got:\n%s", out)
	}
	// And it must NOT tell the agent to default to itself (the leader).
	if strings.Contains(out, "--assignee-id \""+leaderID+"\"") {
		t.Errorf("buildQuickCreatePrompt with SquadID must NOT default to the leader agent's UUID, got:\n%s", out)
	}
	// The squad name should appear in the instruction so the agent has
	// human-readable context for the routing decision.
	if !strings.Contains(out, squadName) {
		t.Errorf("buildQuickCreatePrompt with SquadID should mention the squad name %q, got:\n%s", squadName, out)
	}
	// And the prompt must explicitly call out the squad-vs-leader rule
	// so the agent does not silently regress to "default to YOURSELF".
	mustContain := []string{
		"picker SQUAD",
		"running on the squad's behalf",
		"do not assign it to your own agent UUID",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildQuickCreatePrompt with SquadID missing %q\n--- output ---\n%s", s, out)
		}
	}
}

// TestBuildQuickCreatePromptProjectPinning verifies that when the user
// pins a project in the quick-create modal, the prompt instructs the agent
// to pass `--project <uuid>` exactly. Without this, the agent would re-read
// the workspace default and silently drop the user's selection — the same
// "I have to retype 'in project X' every time" failure mode the modal
// addition was meant to fix.
func TestBuildQuickCreatePromptProjectPinning(t *testing.T) {
	const projectID = "11111111-2222-3333-4444-555555555555"
	out := buildQuickCreatePrompt(Task{
		QuickCreatePrompt: "fix the login button color",
		ProjectID:         projectID,
		ProjectTitle:      "Web App",
	})
	mustContain := []string{
		"--project \"" + projectID + "\"",
		"Web App",
		"modal selection is authoritative",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildQuickCreatePrompt with project missing %q\n--- output ---\n%s", s, out)
		}
	}

	// Without a project, the prompt must keep the legacy "omit" instruction
	// so the agent doesn't accidentally start passing --project on plain
	// quick-create runs.
	plain := buildQuickCreatePrompt(Task{QuickCreatePrompt: "fix the login button color"})
	if !strings.Contains(plain, "**project**: omit") {
		t.Errorf("buildQuickCreatePrompt without project must keep the omit instruction, got:\n%s", plain)
	}
	if strings.Contains(plain, "--project") {
		t.Errorf("buildQuickCreatePrompt without project must NOT mention --project, got:\n%s", plain)
	}
}

// TestBuildChatPromptRoutesIssueCreationToProposals locks in the chat contract:
// when a user asks inside chat to create or split tasks, the daemon should
// produce reviewable issue proposals instead of directly creating issues.
func TestBuildChatPromptRoutesIssueCreationToProposals(t *testing.T) {
	out := buildChatPrompt(Task{
		ChatSessionID: "chat-1",
		ChatMessage:   "创建合适数量的 task",
	})

	mustContain := []string{
		"MULTICA_STRUCTURED_OUTPUT_DIR",
		".multica/chats/chat-1/issue-proposals.json",
		"proposal cards",
		"Do not run `multica issue create`",
		"explicitly asks to create immediately",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildChatPrompt must route chat task creation through proposals, missing %q\n--- output ---\n%s", s, out)
		}
	}
}

func TestBuildPlanChatPromptIncludesEngineSummaryAndSquadRules(t *testing.T) {
	out := buildChatPrompt(Task{
		ChatSessionID: "chat-plan-1",
		ChatMessage:   "Plan a launch",
		Plan: &ChatPlanTaskData{
			RunID:       "plan-1",
			ActorType:   "squad",
			ActorID:     "squad-1",
			LeadAgentID: "lead-1",
			Status:      "brainstorming",
			TaskKind:    "plan_lead",
			PlanEngine: ChatPlanEngineTaskData{
				ID:          "office_hours",
				Label:       "Office hours",
				Description: "Challenge assumptions",
				Version:     "abcd1234abcd1234",
				Protocol:    "Ask one sharp question before planning.",
			},
			Summary: json.RawMessage(`{"confirmed_requirements":["proposal first"]}`),
			Transcript: []ChatPlanTranscriptMessage{
				{Role: "user", AuthorType: "member", Content: "Plan a launch"},
			},
			Squad: &ChatPlanSquadTaskData{
				Name:        "Planning Squad",
				LeadMention: "[@Lead](mention://agent/lead-1)",
				Helpers: []ChatPlanSquadHelperData{
					{Name: "Researcher", Role: "research", Mention: "[@Researcher](mention://agent/helper-1)"},
				},
			},
			ProposalPath:    ".multica/chats/chat-plan-1/issue-proposals.json",
			PlanSummaryPath: ".multica/chats/chat-plan-1/plan-summary.json",
		},
	})

	mustContain := []string{
		"Plan mode assistant",
		"Engine: Office hours (`office_hours`)",
		"Engine version: abcd1234abcd1234",
		"Ask one sharp question before planning.",
		`"confirmed_requirements"`,
		".multica/chats/chat-plan-1/plan-summary.json",
		".multica/chats/chat-plan-1/issue-proposals.json",
		"Do not run `multica issue create`",
		"Plan Transcript",
		"Plan a launch",
		"Eligible helper agents",
		"[@Researcher](mention://agent/helper-1)",
		"Mentions of agents outside this roster are ignored",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("buildPlanChatPrompt missing %q\n--- output ---\n%s", s, out)
		}
	}
}

func TestBuildPlanChatPromptConsultationTaskFocusesHelper(t *testing.T) {
	out := buildChatPrompt(Task{
		ChatSessionID: "chat-plan-2",
		Plan: &ChatPlanTaskData{
			RunID:       "plan-2",
			ActorType:   "squad",
			ActorID:     "squad-2",
			LeadAgentID: "lead-2",
			Status:      "consulting",
			TaskKind:    "plan_consultation",
			PlanEngine: ChatPlanEngineTaskData{
				ID:       "grill_with_docs",
				Label:    "Grill with docs",
				Version:  "abcd1234abcd1234",
				Protocol: "Challenge vague requirements.",
			},
			Consultation: &ChatPlanConsultationTaskData{
				ID:                 "consult-1",
				RequesterAgentID:   "lead-2",
				TargetAgentID:      "helper-2",
				RequestMessageID:   "message-1",
				RequestContent:     "What launch risk am I missing?",
				LeadMention:        "[@Lead](mention://agent/lead-2)",
				TargetAgentMention: "[@Helper](mention://agent/helper-2)",
			},
		},
	})

	mustContain := []string{
		"You are a squad helper",
		"do not take over the final synthesis",
		"[@Helper](mention://agent/helper-2)",
		"include this exact lead mention",
		"[@Lead](mention://agent/lead-2)",
		"What launch risk am I missing?",
		"Return one concise helper opinion",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("consultation plan prompt missing %q\n--- output ---\n%s", s, out)
		}
	}
}

// TestBuildPromptSquadLeaderNoActionForMemberTrigger verifies that the
// squad leader no_action prohibition is injected in the per-turn prompt
// regardless of whether the triggering comment was posted by an agent or
// a member. This was the root cause of the "LGTM is a pure acknowledgment
// — no reply needed. Exiting silently." noise comment: the prohibition
// only fired for agent-triggered comments, so member-triggered ones
// (like "LGTM") bypassed it.
func TestBuildPromptSquadLeaderNoActionForMemberTrigger(t *testing.T) {
	task := Task{
		IssueID:               "issue-123",
		TriggerCommentID:      "comment-456",
		TriggerCommentContent: "LGTM",
		TriggerAuthorType:     "member",
		TriggerAuthorName:     "Bohan",
		Agent: &AgentData{
			Instructions: "Some instructions\n\n## Squad Operating Protocol\n\nYou are the LEADER...",
		},
	}
	out := BuildPrompt(task, "claude")
	if !strings.Contains(out, "Squad leader no_action rule") {
		t.Errorf("buildCommentPrompt must inject squad leader no_action rule for member-triggered comments, got:\n%s", out)
	}
	if !strings.Contains(out, "DO NOT post any comment") {
		t.Errorf("buildCommentPrompt must contain DO NOT post prohibition for member-triggered squad leader, got:\n%s", out)
	}
}

// TestBuildPromptSquadLeaderNoActionForAgentTrigger verifies the rule also
// fires for agent-triggered comments (the original path that already worked).
func TestBuildPromptSquadLeaderNoActionForAgentTrigger(t *testing.T) {
	task := Task{
		IssueID:               "issue-123",
		TriggerCommentID:      "comment-456",
		TriggerCommentContent: "Deploy complete.",
		TriggerAuthorType:     "agent",
		TriggerAuthorName:     "deploy-boy",
		Agent: &AgentData{
			Instructions: "Some instructions\n\n## Squad Operating Protocol\n\nYou are the LEADER...",
		},
	}
	out := BuildPrompt(task, "claude")
	if !strings.Contains(out, "Squad leader no_action rule") {
		t.Errorf("buildCommentPrompt must inject squad leader no_action rule for agent-triggered comments, got:\n%s", out)
	}
}

// TestBuildPromptNonSquadLeaderNoRule verifies that non-squad-leader agents
// do NOT get the squad leader no_action rule injected.
func TestBuildPromptNonSquadLeaderNoRule(t *testing.T) {
	task := Task{
		IssueID:               "issue-123",
		TriggerCommentID:      "comment-456",
		TriggerCommentContent: "LGTM",
		TriggerAuthorType:     "member",
		TriggerAuthorName:     "Bohan",
		Agent: &AgentData{
			Instructions: "Some instructions without the squad marker",
		},
	}
	out := BuildPrompt(task, "claude")
	if strings.Contains(out, "Squad leader no_action rule") {
		t.Errorf("buildCommentPrompt must NOT inject squad leader no_action rule for non-squad-leader agents, got:\n%s", out)
	}
}
