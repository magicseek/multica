package daemon

import "encoding/json"

const (
	TaskChatSummaryManifestRelativePath    = ".multica/chat-summary.json"
	TaskPlanSummaryManifestRelativePath    = ".multica/plan-summary.json"
	TaskIssueProposalsManifestRelativePath = ".multica/issue-proposals.json"
	TaskOutputManifestRelativePath         = ".multica/outputs.json"
	TaskChatSummaryManifestFileName        = "chat-summary.json"
	TaskPlanSummaryManifestFileName        = "plan-summary.json"
	TaskIssueProposalsManifestFileName     = "issue-proposals.json"
	TaskOutputManifestFileName             = "outputs.json"
)

// AgentEntry describes a single available agent CLI.
type AgentEntry struct {
	Path  string // path to CLI binary
	Model string // model override (optional)
}

// Runtime represents a registered daemon runtime.
type Runtime struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

// RepoData holds repository information from the workspace.
type RepoData struct {
	URL string `json:"url"`
}

// RepositoryOperation mirrors the daemon-facing repository operation lifecycle
// payload returned by /api/daemon/repository-operations/*.
type RepositoryOperation struct {
	ID              string                          `json:"id"`
	RepositoryID    string                          `json:"repository_id"`
	WorkspaceID     string                          `json:"workspace_id"`
	OperationType   string                          `json:"operation_type"`
	Status          string                          `json:"status"`
	RequestedByType string                          `json:"requested_by_type"`
	RequestedByID   *string                         `json:"requested_by_id,omitempty"`
	TargetDaemonID  *string                         `json:"target_daemon_id,omitempty"`
	TargetRuntimeID *string                         `json:"target_runtime_id,omitempty"`
	BindingID       *string                         `json:"binding_id,omitempty"`
	Request         json.RawMessage                 `json:"request"`
	Result          json.RawMessage                 `json:"result"`
	Error           *string                         `json:"error,omitempty"`
	CreatedAt       string                          `json:"created_at"`
	UpdatedAt       string                          `json:"updated_at"`
	CompletedAt     *string                         `json:"completed_at,omitempty"`
	Binding         *RepositoryOperationBindingData `json:"binding,omitempty"`
}

type RepositoryOperationBindingData struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	State     string  `json:"state"`
	DaemonID  string  `json:"daemon_id"`
	RuntimeID *string `json:"runtime_id,omitempty"`
	LocalPath string  `json:"local_path"`
}

// TaskRepositoryBindingData mirrors the sanitized binding summary returned by
// the daemon claim endpoint. local_path is populated only when the binding is
// current for the claiming daemon/runtime.
type TaskRepositoryBindingData struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	MachineLabel   string `json:"machine_label,omitempty"`
	DaemonID       string `json:"daemon_id,omitempty"`
	RuntimeID      string `json:"runtime_id,omitempty"`
	LocalPath      string `json:"local_path,omitempty"`
	Available      bool   `json:"available"`
	CurrentDaemon  bool   `json:"current_daemon,omitempty"`
	CurrentRuntime bool   `json:"current_runtime,omitempty"`
}

// TaskRepositoryData carries first-class repository semantics for a claimed
// task. Repos remains the legacy remote URL list used by repo checkout.
type TaskRepositoryData struct {
	ID                  string                     `json:"id"`
	Name                string                     `json:"name"`
	SourceState         string                     `json:"source_state"`
	RemoteURL           *string                    `json:"remote_url,omitempty"`
	DefaultBranch       *string                    `json:"default_branch,omitempty"`
	Role                string                     `json:"role,omitempty"`
	Position            int32                      `json:"position"`
	Compatibility       bool                       `json:"compatibility,omitempty"`
	CompatibilitySource string                     `json:"compatibility_source,omitempty"`
	BindingAvailable    bool                       `json:"binding_available"`
	Binding             *TaskRepositoryBindingData `json:"binding,omitempty"`
}

// ProjectResourceData mirrors handler.ProjectResourceData — a single project
// resource as delivered to the daemon. resource_ref is type-specific JSON.
type ProjectResourceData struct {
	ID           string          `json:"id"`
	ResourceType string          `json:"resource_type"`
	ResourceRef  json.RawMessage `json:"resource_ref"`
	Label        string          `json:"label,omitempty"`
}

// Task represents a claimed task from the server.
// Agent data (name, skills) is populated by the claim endpoint.
type Task struct {
	ID                      string                `json:"id"`
	AgentID                 string                `json:"agent_id"`
	RuntimeID               string                `json:"runtime_id"`
	IssueID                 string                `json:"issue_id"`
	WorkspaceID             string                `json:"workspace_id"`
	Agent                   *AgentData            `json:"agent,omitempty"`
	Repos                   []RepoData            `json:"repos,omitempty"`
	Repositories            []TaskRepositoryData  `json:"repositories,omitempty"`
	ProjectID               string                `json:"project_id,omitempty"`               // issue's project, when present
	ProjectTitle            string                `json:"project_title,omitempty"`            // human-readable project title for context injection
	ProjectResources        []ProjectResourceData `json:"project_resources,omitempty"`        // project-scoped resources to expose to the agent
	PriorSessionID          string                `json:"prior_session_id,omitempty"`         // Claude session ID from a previous task on this issue
	PriorWorkDir            string                `json:"prior_work_dir,omitempty"`           // work_dir from a previous task on this issue
	TriggerCommentID        string                `json:"trigger_comment_id,omitempty"`       // comment that triggered this task
	TriggerCommentContent   string                `json:"trigger_comment_content,omitempty"`  // content of the triggering comment
	TriggerAuthorType       string                `json:"trigger_author_type,omitempty"`      // "agent" or "member" — author kind for the triggering comment
	TriggerAuthorName       string                `json:"trigger_author_name,omitempty"`      // display name of the triggering comment author
	ChatSessionID           string                `json:"chat_session_id,omitempty"`          // non-empty for chat tasks
	ChatMessage             string                `json:"chat_message,omitempty"`             // user message content for chat tasks
	ChatMessageAttachments  []ChatAttachmentMeta  `json:"chat_message_attachments,omitempty"` // attachments linked to the chat message; agent uses these to `multica attachment download <id>`
	ChatPlanRunID           string                `json:"chat_plan_run_id,omitempty"`
	ChatPlanConsultationID  string                `json:"chat_plan_consultation_id,omitempty"`
	ChatTaskKind            string                `json:"chat_task_kind,omitempty"`
	Plan                    *ChatPlanTaskData     `json:"plan,omitempty"`
	AutopilotRunID          string                `json:"autopilot_run_id,omitempty"`          // non-empty for autopilot run_only tasks
	AutopilotID             string                `json:"autopilot_id,omitempty"`              // autopilot that spawned this run
	AutopilotTitle          string                `json:"autopilot_title,omitempty"`           // autopilot title used as task context
	AutopilotDescription    string                `json:"autopilot_description,omitempty"`     // autopilot description used as task prompt
	AutopilotSource         string                `json:"autopilot_source,omitempty"`          // manual, schedule, webhook, or api
	AutopilotTriggerPayload json.RawMessage       `json:"autopilot_trigger_payload,omitempty"` // optional trigger payload for webhook/api runs
	QuickCreatePrompt       string                `json:"quick_create_prompt,omitempty"`       // user's natural-language input for quick-create tasks
	SquadID                 string                `json:"squad_id,omitempty"`                  // when the picker was a squad, the squad's UUID; Agent is still the resolved leader
	SquadName               string                `json:"squad_name,omitempty"`                // display name for the picker squad, used in prompt text
	WorkflowDefinitionID    string                `json:"workflow_definition_id,omitempty"`    // workflow definition selected at queue time
	WorkflowRevisionID      string                `json:"workflow_revision_id,omitempty"`      // immutable workflow revision selected at queue time
	WorkflowSnapshot        WorkflowSnapshot      `json:"workflow_snapshot,omitempty"`         // rendered workflow snapshot captured at queue time
	WorkflowRun             *WorkflowRun          `json:"workflow_run,omitempty"`              // materialized workflow run and initial step state
}

type WorkflowSnapshot struct {
	SchemaVersion      int                         `json:"schema_version"`
	TriggerType        string                      `json:"trigger_type"`
	DefinitionID       string                      `json:"definition_id"`
	RevisionID         string                      `json:"revision_id"`
	RevisionNumber     int32                       `json:"revision_number"`
	WorkflowName       string                      `json:"workflow_name"`
	Origin             string                      `json:"origin"`
	SystemKey          string                      `json:"system_key,omitempty"`
	Schema             json.RawMessage             `json:"schema"`
	RenderedMarkdown   string                      `json:"rendered_markdown"`
	CapabilityWarnings []WorkflowCapabilityWarning `json:"capability_warnings,omitempty"`
	ResolvedAt         string                      `json:"resolved_at,omitempty"`
}

type WorkflowCapabilityWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorkflowRun struct {
	ID                   string                 `json:"id"`
	WorkspaceID          string                 `json:"workspace_id"`
	AgentTaskQueueID     string                 `json:"agent_task_queue_id"`
	IssueID              *string                `json:"issue_id,omitempty"`
	WorkflowDefinitionID *string                `json:"workflow_definition_id,omitempty"`
	WorkflowRevisionID   *string                `json:"workflow_revision_id,omitempty"`
	TriggerType          string                 `json:"trigger_type"`
	Status               string                 `json:"status"`
	Steps                []WorkflowStepRun      `json:"steps,omitempty"`
	InputRequests        []WorkflowInputRequest `json:"input_requests,omitempty"`
	CurrentStep          *WorkflowStepRun       `json:"current_step,omitempty"`
}

type WorkflowStepRun struct {
	ID               string          `json:"id"`
	WorkflowRunID    string          `json:"workflow_run_id"`
	StepDefinitionID string          `json:"step_definition_id"`
	Title            string          `json:"title"`
	OrderIndex       int32           `json:"order_index"`
	Required         bool            `json:"required"`
	Status           string          `json:"status"`
	ExecutionKind    string          `json:"execution_kind"`
	Attempt          int32           `json:"attempt"`
	DependsOnStepIDs []string        `json:"depends_on_step_ids"`
	ArtifactInputs   json.RawMessage `json:"artifact_inputs"`
	Snapshot         json.RawMessage `json:"snapshot"`
}

type WorkflowInputRequest struct {
	ID                string  `json:"id"`
	WorkflowRunID     string  `json:"workflow_run_id"`
	WorkflowStepRunID string  `json:"workflow_step_run_id"`
	Status            string  `json:"status"`
	QuestionText      string  `json:"question_text"`
	AnswerText        *string `json:"answer_text,omitempty"`
	QuestionCommentID *string `json:"question_comment_id,omitempty"`
	AnswerCommentID   *string `json:"answer_comment_id,omitempty"`
	RoundIndex        int32   `json:"round_index"`
	MaxRounds         int32   `json:"max_rounds"`
	RequestedAt       string  `json:"requested_at,omitempty"`
	AnsweredAt        *string `json:"answered_at,omitempty"`
}

// ChatAttachmentMeta is the structured attachment metadata the daemon
// hands to the agent for chat tasks. We pass id + filename + content_type
// so the chat prompt can list them explicitly and instruct the agent to
// run `multica attachment download <id>` instead of guessing from a
// signed CDN URL (which expires).
type ChatAttachmentMeta struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
}

type ChatPlanTaskData struct {
	RunID           string                         `json:"run_id"`
	ActorType       string                         `json:"actor_type"`
	ActorID         string                         `json:"actor_id"`
	LeadAgentID     string                         `json:"lead_agent_id"`
	Status          string                         `json:"status"`
	TaskKind        string                         `json:"task_kind"`
	PlanEngine      ChatPlanEngineTaskData         `json:"plan_engine"`
	Summary         json.RawMessage                `json:"summary"`
	Transcript      []ChatPlanTranscriptMessage    `json:"transcript"`
	Consultations   []ChatPlanConsultationResponse `json:"consultations,omitempty"`
	Squad           *ChatPlanSquadTaskData         `json:"squad,omitempty"`
	Consultation    *ChatPlanConsultationTaskData  `json:"consultation,omitempty"`
	ProposalPath    string                         `json:"proposal_path"`
	PlanSummaryPath string                         `json:"plan_summary_path"`
}

type ChatPlanEngineTaskData struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
}

type ChatPlanTranscriptMessage struct {
	ID             string  `json:"id"`
	Role           string  `json:"role"`
	Content        string  `json:"content"`
	AuthorType     string  `json:"author_type"`
	AuthorAgentID  *string `json:"author_agent_id,omitempty"`
	ConsultationID *string `json:"consultation_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type ChatPlanConsultationResponse struct {
	ID                string  `json:"id"`
	PlanRunID         string  `json:"plan_run_id"`
	RequesterAgentID  string  `json:"requester_agent_id"`
	TargetAgentID     string  `json:"target_agent_id"`
	RequestMessageID  *string `json:"request_message_id"`
	ResponseMessageID *string `json:"response_message_id"`
	TaskID            *string `json:"task_id"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type ChatPlanSquadTaskData struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	LeadMention string                    `json:"lead_mention"`
	Helpers     []ChatPlanSquadHelperData `json:"helpers"`
}

type ChatPlanSquadHelperData struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	Mention string `json:"mention"`
}

type ChatPlanConsultationTaskData struct {
	ID                 string `json:"id"`
	RequesterAgentID   string `json:"requester_agent_id"`
	TargetAgentID      string `json:"target_agent_id"`
	RequestMessageID   string `json:"request_message_id"`
	RequestContent     string `json:"request_content"`
	LeadMention        string `json:"lead_mention"`
	TargetAgentMention string `json:"target_agent_mention"`
}

// AgentData holds agent details returned by the claim endpoint.
type AgentData struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Instructions             string            `json:"instructions"`
	Skills                   []SkillData       `json:"skills"`
	CustomEnv                map[string]string `json:"custom_env,omitempty"`
	CustomArgs               []string          `json:"custom_args,omitempty"`
	McpConfig                json.RawMessage   `json:"mcp_config,omitempty"`
	Model                    string            `json:"model,omitempty"`
	ExecutionProtocolEnabled bool              `json:"execution_protocol_enabled,omitempty"`
	ExecutionProtocolSlug    string            `json:"execution_protocol_slug,omitempty"`
}

// SkillData represents a structured skill for task execution.
type SkillData struct {
	Name    string          `json:"name"`
	Content string          `json:"content"`
	Files   []SkillFileData `json:"files,omitempty"`
}

// SkillFileData represents a supporting file within a skill.
type SkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// TaskUsageEntry represents token usage for a single model during a task execution.
type TaskUsageEntry struct {
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	InputTokens      int64           `json:"input_tokens"`
	OutputTokens     int64           `json:"output_tokens"`
	CacheReadTokens  int64           `json:"cache_read_tokens"`
	CacheWriteTokens int64           `json:"cache_write_tokens"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}

type TaskOutputManifest struct {
	Outputs []TaskOutputMetadata `json:"outputs"`
}

type StructuredTaskOutputs struct {
	ChatSummary    *ChatSummaryManifest    `json:"chat_summary,omitempty"`
	PlanSummary    *PlanSummaryManifest    `json:"plan_summary,omitempty"`
	IssueProposals *IssueProposalsManifest `json:"issue_proposals,omitempty"`
	Outputs        *TaskOutputManifest     `json:"outputs,omitempty"`
}

type ChatSummaryManifest struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
}

type PlanSummaryManifest struct {
	Version               int      `json:"version"`
	ConfirmedRequirements []string `json:"confirmed_requirements"`
	RejectedOptions       []string `json:"rejected_options"`
	ConsensusNotes        []string `json:"consensus_notes"`
	OpenQuestions         []string `json:"open_questions"`
}

type IssueProposalsManifest struct {
	Version   int                     `json:"version"`
	Proposals []IssueProposalManifest `json:"proposals"`
}

type IssueProposalManifest struct {
	Title   string                      `json:"title"`
	Summary *string                     `json:"summary,omitempty"`
	Items   []IssueProposalItemManifest `json:"items"`
	Status  json.RawMessage             `json:"status,omitempty"`
}

type IssueProposalItemManifest struct {
	Title        string          `json:"title"`
	Description  string          `json:"description,omitempty"`
	Priority     *string         `json:"priority,omitempty"`
	Labels       []string        `json:"labels,omitempty"`
	AssigneeType *string         `json:"assignee_type,omitempty"`
	AssigneeID   *string         `json:"assignee_id,omitempty"`
	Status       json.RawMessage `json:"status,omitempty"`
}

type TaskOutputMetadata struct {
	RepositoryID *string         `json:"repository_id,omitempty"`
	RelativePath string          `json:"relative_path"`
	Filename     *string         `json:"filename,omitempty"`
	Kind         string          `json:"kind,omitempty"`
	SizeBytes    *int64          `json:"size_bytes,omitempty"`
	Size         *int64          `json:"size,omitempty"`
	MimeType     *string         `json:"mime_type,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// TaskResult is the outcome of executing a task.
type TaskResult struct {
	Status              string           `json:"status"`
	Comment             string           `json:"comment"`
	BranchName          string           `json:"branch_name,omitempty"`
	EnvType             string           `json:"env_type,omitempty"`
	SessionID           string           `json:"session_id,omitempty"` // Claude session ID for future resumption
	WorkDir             string           `json:"work_dir,omitempty"`   // working directory used during execution
	EnvRoot             string           `json:"-"`                    // env root dir for writing GC metadata (not sent to server)
	StructuredOutputDir string           `json:"-"`                    // scoped manifest root; falls back to WorkDir when empty
	FailureReason       string           `json:"-"`                    // classifier forwarded to FailTask on the blocked path; empty falls back to 'agent_error'
	Usage               []TaskUsageEntry `json:"usage,omitempty"`      // per-model token usage
}
