package daemon

import (
	"encoding/json"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

func TestTaskUsageMetadataRecordsPromptAndRuntimeBriefFacts(t *testing.T) {
	t.Parallel()

	brief := execenv.RuntimeBrief{
		Full:    "stable-guidance\ndynamic-facts",
		Stable:  "stable-guidance",
		Dynamic: "dynamic-facts",
	}
	meta := newTaskUsageMetadata("hello", "system", brief)
	meta.WorkDirReused = true
	meta.EnvRootReused = true
	meta.CodexRunnerEnabled = true
	meta.CodexRunnerReused = true
	meta.RuntimeEnvFile = true
	meta.ResumeAttempted = true
	meta.ResumeHit = true
	meta.PriorSessionID = "thr-old"
	meta.SessionID = "thr-old"
	meta.AgentDiagnostics = map[string]any{"usage_source": "event"}
	meta.AgentRunMs = 1234
	meta.DaemonRunMs = 1300
	meta.AgentResultOutputBytes = len("done")
	meta.recordAgentDiagnostics(map[string]any{
		diagFirstEventMs:            int64(25),
		diagFirstTextMs:             int64(40),
		diagFirstToolUseMs:          int64(55),
		diagFirstToolResultMs:       int64(75),
		diagTaskMessageTextCount:    int64(2),
		diagTaskMessageToolUseCount: int64(1),
		diagToolResultBytes:         int64(128),
	})

	var got map[string]any
	if err := json.Unmarshal(meta.raw(), &got); err != nil {
		t.Fatalf("metadata json: %v", err)
	}

	if got["prompt_bytes"] != float64(len("hello")) {
		t.Fatalf("prompt_bytes = %v", got["prompt_bytes"])
	}
	if got["system_prompt_bytes"] != float64(len("system")) {
		t.Fatalf("system_prompt_bytes = %v", got["system_prompt_bytes"])
	}
	if got["runtime_brief_stable_sha256"] != sha256Hex("stable-guidance") {
		t.Fatalf("stable hash = %v", got["runtime_brief_stable_sha256"])
	}
	if got["runtime_brief_dynamic_sha256"] != sha256Hex("dynamic-facts") {
		t.Fatalf("dynamic hash = %v", got["runtime_brief_dynamic_sha256"])
	}
	for _, key := range []string{
		"work_dir_reused",
		"env_root_reused",
		"codex_runner_enabled",
		"codex_runner_reused",
		"runtime_env_file",
		"resume_attempted",
		"resume_hit",
	} {
		if got[key] != true {
			t.Fatalf("%s = %v, want true", key, got[key])
		}
	}
	if got["prior_session_id"] != "thr-old" || got["session_id"] != "thr-old" {
		t.Fatalf("session ids not recorded: %+v", got)
	}
	if got["agent_run_ms"] != float64(1234) || got["daemon_run_ms"] != float64(1300) {
		t.Fatalf("run timings not recorded: %+v", got)
	}
	if got["first_event_ms"] != float64(25) || got["first_tool_result_ms"] != float64(75) {
		t.Fatalf("first event timings not recorded: %+v", got)
	}
	if got["task_message_tool_use_count"] != float64(1) || got["tool_result_bytes"] != float64(128) {
		t.Fatalf("message counters not recorded: %+v", got)
	}
	if got["agent_result_output_bytes"] != float64(len("done")) {
		t.Fatalf("output bytes not recorded: %+v", got)
	}
}

func TestTaskUsageMetadataRecordsContextProfile(t *testing.T) {
	t.Parallel()

	meta := newTaskUsageMetadata("prompt", "", execenv.RuntimeBrief{})
	meta.recordContextProfile(Task{
		ChatMessage: "hello",
		ChatMessageAttachments: []ChatAttachmentMeta{
			{ID: "att-1", Filename: "one.txt"},
			{ID: "att-2", Filename: "two.txt"},
		},
	}, execenv.TaskContextForEnv{
		AgentInstructions: "be concise",
		AgentSkills: []execenv.SkillContextForEnv{
			{
				Name:    "skill-one",
				Content: "skill body",
				Files: []execenv.SkillFileContextForEnv{
					{Path: "extra.md", Content: "extra body"},
				},
			},
		},
		Repos:                    []execenv.RepoContextForEnv{{URL: "https://example.test/repo.git"}},
		Repositories:             []execenv.RepositoryContextForEnv{{ID: "repo-1"}},
		ProjectResources:         []execenv.ProjectResourceForEnv{{ID: "res-1"}},
		WorkflowRenderedMarkdown: "workflow snapshot",
		WorkflowCurrentStep:      &execenv.WorkflowStepContextForEnv{Snapshot: "step snapshot"},
		AutopilotDescription:     "autopilot description",
		AutopilotTriggerPayload:  `{"event":"push"}`,
		QuickCreatePrompt:        "quick issue",
	})

	var got map[string]any
	if err := json.Unmarshal(meta.raw(), &got); err != nil {
		t.Fatalf("metadata json: %v", err)
	}

	wantSkillBytes := len("skill body") + len("extra body")
	checks := map[string]float64{
		"chat_message_bytes":           float64(len("hello")),
		"chat_attachment_count":        2,
		"agent_instructions_bytes":     float64(len("be concise")),
		"agent_skill_count":            1,
		"agent_skill_bytes":            float64(wantSkillBytes),
		"repo_count":                   1,
		"repository_count":             1,
		"project_resource_count":       1,
		"workflow_snapshot_bytes":      float64(len("workflow snapshot")),
		"workflow_step_snapshot_bytes": float64(len("step snapshot")),
		"autopilot_description_bytes":  float64(len("autopilot description")),
		"autopilot_payload_bytes":      float64(len(`{"event":"push"}`)),
		"quick_create_prompt_bytes":    float64(len("quick issue")),
	}
	for key, want := range checks {
		if got[key] != want {
			t.Fatalf("%s = %v, want %v (metadata=%+v)", key, got[key], want, got)
		}
	}
}
