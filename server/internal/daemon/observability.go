package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

type taskUsageMetadata struct {
	PromptBytes               int            `json:"prompt_bytes"`
	SystemPromptBytes         int            `json:"system_prompt_bytes"`
	RuntimeBriefBytes         int            `json:"runtime_brief_bytes"`
	RuntimeBriefStableBytes   int            `json:"runtime_brief_stable_bytes"`
	RuntimeBriefDynamicBytes  int            `json:"runtime_brief_dynamic_bytes"`
	RuntimeBriefStableSHA256  string         `json:"runtime_brief_stable_sha256"`
	RuntimeBriefDynamicSHA256 string         `json:"runtime_brief_dynamic_sha256"`
	WorkDirReused             bool           `json:"work_dir_reused"`
	EnvRootReused             bool           `json:"env_root_reused"`
	CodexRunnerEnabled        bool           `json:"codex_runner_enabled"`
	CodexRunnerReused         bool           `json:"codex_runner_reused"`
	RuntimeEnvFile            bool           `json:"runtime_env_file"`
	ResumeAttempted           bool           `json:"resume_attempted"`
	ResumeHit                 bool           `json:"resume_hit"`
	ResumeFallback            bool           `json:"resume_fallback"`
	PriorSessionID            string         `json:"prior_session_id,omitempty"`
	SessionID                 string         `json:"session_id,omitempty"`
	AgentDiagnostics          map[string]any `json:"agent_diagnostics,omitempty"`
}

func newTaskUsageMetadata(prompt, systemPrompt string, brief execenv.RuntimeBrief) taskUsageMetadata {
	return taskUsageMetadata{
		PromptBytes:               len([]byte(prompt)),
		SystemPromptBytes:         len([]byte(systemPrompt)),
		RuntimeBriefBytes:         len([]byte(brief.Full)),
		RuntimeBriefStableBytes:   len([]byte(brief.Stable)),
		RuntimeBriefDynamicBytes:  len([]byte(brief.Dynamic)),
		RuntimeBriefStableSHA256:  sha256Hex(brief.Stable),
		RuntimeBriefDynamicSHA256: sha256Hex(brief.Dynamic),
	}
}

func (m taskUsageMetadata) raw() json.RawMessage {
	data, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
