package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

const (
	diagFirstEventMs               = "first_event_ms"
	diagFirstTextMs                = "first_text_ms"
	diagFirstToolUseMs             = "first_tool_use_ms"
	diagFirstToolResultMs          = "first_tool_result_ms"
	diagTaskMessageTextCount       = "task_message_text_count"
	diagTaskMessageThinkingCount   = "task_message_thinking_count"
	diagTaskMessageToolUseCount    = "task_message_tool_use_count"
	diagTaskMessageToolResultCount = "task_message_tool_result_count"
	diagTaskMessageErrorCount      = "task_message_error_count"
	diagAssistantTextBytes         = "assistant_text_bytes"
	diagThinkingBytes              = "thinking_bytes"
	diagToolInputBytes             = "tool_input_bytes"
	diagToolResultBytes            = "tool_result_bytes"
)

type taskUsageMetadata struct {
	PromptBytes                int            `json:"prompt_bytes"`
	SystemPromptBytes          int            `json:"system_prompt_bytes"`
	RuntimeBriefBytes          int            `json:"runtime_brief_bytes"`
	RuntimeBriefStableBytes    int            `json:"runtime_brief_stable_bytes"`
	RuntimeBriefDynamicBytes   int            `json:"runtime_brief_dynamic_bytes"`
	RuntimeBriefStableSHA256   string         `json:"runtime_brief_stable_sha256"`
	RuntimeBriefDynamicSHA256  string         `json:"runtime_brief_dynamic_sha256"`
	ChatMessageBytes           int            `json:"chat_message_bytes,omitempty"`
	ChatAttachmentCount        int            `json:"chat_attachment_count,omitempty"`
	AgentInstructionsBytes     int            `json:"agent_instructions_bytes,omitempty"`
	AgentSkillCount            int            `json:"agent_skill_count,omitempty"`
	AgentSkillBytes            int            `json:"agent_skill_bytes,omitempty"`
	RepoCount                  int            `json:"repo_count,omitempty"`
	RepositoryCount            int            `json:"repository_count,omitempty"`
	ProjectResourceCount       int            `json:"project_resource_count,omitempty"`
	WorkflowSnapshotBytes      int            `json:"workflow_snapshot_bytes,omitempty"`
	WorkflowStepSnapshotBytes  int            `json:"workflow_step_snapshot_bytes,omitempty"`
	AutopilotDescriptionBytes  int            `json:"autopilot_description_bytes,omitempty"`
	AutopilotPayloadBytes      int            `json:"autopilot_payload_bytes,omitempty"`
	QuickCreatePromptBytes     int            `json:"quick_create_prompt_bytes,omitempty"`
	ExecEnvMs                  int64          `json:"exec_env_ms,omitempty"`
	RuntimeConfigMs            int64          `json:"runtime_config_ms,omitempty"`
	BackendCreateMs            int64          `json:"backend_create_ms,omitempty"`
	AgentRunMs                 int64          `json:"agent_run_ms,omitempty"`
	DaemonRunMs                int64          `json:"daemon_run_ms,omitempty"`
	FirstEventMs               int64          `json:"first_event_ms,omitempty"`
	FirstTextMs                int64          `json:"first_text_ms,omitempty"`
	FirstToolUseMs             int64          `json:"first_tool_use_ms,omitempty"`
	FirstToolResultMs          int64          `json:"first_tool_result_ms,omitempty"`
	TaskMessageTextCount       int64          `json:"task_message_text_count,omitempty"`
	TaskMessageThinkingCount   int64          `json:"task_message_thinking_count,omitempty"`
	TaskMessageToolUseCount    int64          `json:"task_message_tool_use_count,omitempty"`
	TaskMessageToolResultCount int64          `json:"task_message_tool_result_count,omitempty"`
	TaskMessageErrorCount      int64          `json:"task_message_error_count,omitempty"`
	AssistantTextBytes         int64          `json:"assistant_text_bytes,omitempty"`
	ThinkingBytes              int64          `json:"thinking_bytes,omitempty"`
	ToolInputBytes             int64          `json:"tool_input_bytes,omitempty"`
	ToolResultBytes            int64          `json:"tool_result_bytes,omitempty"`
	AgentResultOutputBytes     int            `json:"agent_result_output_bytes,omitempty"`
	WorkDirReused              bool           `json:"work_dir_reused"`
	EnvRootReused              bool           `json:"env_root_reused"`
	CodexRunnerEnabled         bool           `json:"codex_runner_enabled"`
	CodexRunnerReused          bool           `json:"codex_runner_reused"`
	RuntimeEnvFile             bool           `json:"runtime_env_file"`
	ResumeAttempted            bool           `json:"resume_attempted"`
	ResumeHit                  bool           `json:"resume_hit"`
	ResumeFallback             bool           `json:"resume_fallback"`
	PriorSessionID             string         `json:"prior_session_id,omitempty"`
	SessionID                  string         `json:"session_id,omitempty"`
	AgentDiagnostics           map[string]any `json:"agent_diagnostics,omitempty"`
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

func (m *taskUsageMetadata) recordContextProfile(task Task, ctx execenv.TaskContextForEnv) {
	m.ChatMessageBytes = len([]byte(task.ChatMessage))
	m.ChatAttachmentCount = len(task.ChatMessageAttachments)
	m.AgentInstructionsBytes = len([]byte(ctx.AgentInstructions))
	m.AgentSkillCount = len(ctx.AgentSkills)
	for _, skill := range ctx.AgentSkills {
		m.AgentSkillBytes += len([]byte(skill.Content))
		for _, file := range skill.Files {
			m.AgentSkillBytes += len([]byte(file.Content))
		}
	}
	m.RepoCount = len(ctx.Repos)
	m.RepositoryCount = len(ctx.Repositories)
	m.ProjectResourceCount = len(ctx.ProjectResources)
	m.WorkflowSnapshotBytes = len([]byte(ctx.WorkflowRenderedMarkdown))
	if ctx.WorkflowCurrentStep != nil {
		m.WorkflowStepSnapshotBytes = len([]byte(ctx.WorkflowCurrentStep.Snapshot))
	}
	m.AutopilotDescriptionBytes = len([]byte(ctx.AutopilotDescription))
	m.AutopilotPayloadBytes = len([]byte(ctx.AutopilotTriggerPayload))
	m.QuickCreatePromptBytes = len([]byte(ctx.QuickCreatePrompt))
}

func (m *taskUsageMetadata) recordAgentDiagnostics(diag map[string]any) {
	m.FirstEventMs = int64FromDiagnostic(diag, diagFirstEventMs)
	m.FirstTextMs = int64FromDiagnostic(diag, diagFirstTextMs)
	m.FirstToolUseMs = int64FromDiagnostic(diag, diagFirstToolUseMs)
	m.FirstToolResultMs = int64FromDiagnostic(diag, diagFirstToolResultMs)
	m.TaskMessageTextCount = int64FromDiagnostic(diag, diagTaskMessageTextCount)
	m.TaskMessageThinkingCount = int64FromDiagnostic(diag, diagTaskMessageThinkingCount)
	m.TaskMessageToolUseCount = int64FromDiagnostic(diag, diagTaskMessageToolUseCount)
	m.TaskMessageToolResultCount = int64FromDiagnostic(diag, diagTaskMessageToolResultCount)
	m.TaskMessageErrorCount = int64FromDiagnostic(diag, diagTaskMessageErrorCount)
	m.AssistantTextBytes = int64FromDiagnostic(diag, diagAssistantTextBytes)
	m.ThinkingBytes = int64FromDiagnostic(diag, diagThinkingBytes)
	m.ToolInputBytes = int64FromDiagnostic(diag, diagToolInputBytes)
	m.ToolResultBytes = int64FromDiagnostic(diag, diagToolResultBytes)
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

func int64FromDiagnostic(diag map[string]any, key string) int64 {
	if len(diag) == 0 {
		return 0
	}
	switch v := diag[key].(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}
