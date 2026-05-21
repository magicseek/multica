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
}
