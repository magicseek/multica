export type WorkflowOrigin = "system_seeded" | "user";

export type WorkflowRevisionStatus = "draft" | "published" | "deprecated";

export type WorkflowApplicability =
  | "assignment"
  | "comment"
  | "chat"
  | "autopilot";

export interface WorkflowVariable {
  key: string;
  description?: string;
  required?: boolean;
}

export interface WorkflowStep {
  id: string;
  name?: string;
  title: string;
  order?: number;
  required?: boolean;
  depends_on?: string[];
  execution?: {
    kind?: "agent" | "manual" | "external" | string;
    prompt?: string;
    rules?: string;
  };
  artifact?: {
    name?: string;
    content_kind?: "markdown" | "json" | "text" | string;
    format?: "markdown" | "json" | "text" | string;
    template?:
      | {
          format?: "markdown" | "json" | "text" | string;
          content?: string;
          files?: Array<{
            path?: string;
            content?: string;
          }>;
        }
      | string;
    inputs?: Array<{
      step_id?: string;
      artifact_name?: string;
      name?: string;
      required?: boolean;
    }>;
    description?: string;
  };
  input_artifacts?: Array<{
    step_id?: string;
    artifact_name?: string;
    name?: string;
    required?: boolean;
  }>;
  input_requests?: {
    allowed?: boolean;
    max_rounds?: number;
    question_policy?: "one_at_a_time" | string;
  };
  review?: {
    required?: boolean;
    reviewer_role?: string;
    instructions?: string;
  };
  quality_gate?: {
    enabled?: boolean;
    blocking?: boolean;
    prompt?: string;
    report_mode?: "summary" | "full" | "json" | string;
  };
  body_template?: string;
  description?: string;
  checklist?: string[];
  output?: {
    description?: string;
  };
}

export interface WorkflowGate {
  id: string;
  title: string;
  description?: string;
}

export interface WorkflowValidationIssue {
  code: string;
  message: string;
  severity: "blocking" | "warning" | string;
}

export interface WorkflowValidation {
  publishable: boolean;
  issues: WorkflowValidationIssue[];
}

export interface WorkflowSchema {
  schema_version?: number;
  version?: number;
  name?: string;
  description?: string;
  applicability?: WorkflowApplicability[];
  source?: {
    format?: "markdown" | string;
    mode?: "markdown" | string;
    body_template?: string;
  };
  variables?: WorkflowVariable[];
  steps?: WorkflowStep[];
  gates?: WorkflowGate[];
  [key: string]: unknown;
}

export interface WorkflowRevision {
  id: string;
  workflow_definition_id: string;
  revision_number: number;
  status: WorkflowRevisionStatus;
  schema: WorkflowSchema;
  validation?: WorkflowValidation | null;
  created_by: string | null;
  published_at: string | null;
  deprecated_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowDefinition {
  id: string;
  workspace_id: string;
  name: string;
  description: string;
  origin: WorkflowOrigin;
  system_key: string | null;
  forked_from_definition_id: string | null;
  current_published_revision_id: string | null;
  published_revision?: WorkflowRevision | null;
  draft_revision?: WorkflowRevision | null;
  current_revision?: WorkflowRevision | null;
  has_unpublished_changes?: boolean;
  created_by: string | null;
  archived_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateWorkflowRequest {
  name: string;
  description?: string;
  schema?: WorkflowSchema;
}

export interface UpdateWorkflowRequest {
  name?: string;
  description?: string;
}

export interface WorkflowSchemaRequest {
  schema: WorkflowSchema;
}

export interface ForkWorkflowRequest {
  name?: string;
  description?: string;
}

export interface WorkflowPreviewRequest {
  schema: WorkflowSchema;
  issue_id?: string;
  trigger_comment_id?: string;
}

export interface WorkflowPreviewResponse {
  rendered_markdown: string;
  warnings?: string[] | null;
}

export interface ImportWorkflowRequest {
  name?: string;
  description?: string;
  format?: "yaml" | "json" | string;
  content: string;
}

export interface ImportWorkflowResponse {
  workflow: WorkflowDefinition;
  warnings?: string[] | null;
}

export interface ExportWorkflowResponse {
  format: "yaml" | "json" | string;
  content: string;
  warnings?: string[] | null;
}

export type WorkflowRunStatus =
  | "queued"
  | "running"
  | "waiting"
  | "blocked"
  | "failed"
  | "completed"
  | "cancelled"
  | string;

export type WorkflowStepRunStatus =
  | "pending"
  | "ready"
  | "running"
  | "waiting_input"
  | "waiting_manual"
  | "waiting_external"
  | "waiting_review"
  | "waiting_quality"
  | "paused"
  | "blocked"
  | "failed"
  | "completed"
  | "skipped"
  | string;

export interface WorkflowStepRun {
  id: string;
  workflow_run_id: string;
  step_definition_id: string;
  title: string;
  order_index: number;
  required: boolean;
  status: WorkflowStepRunStatus;
  execution_kind: "agent" | "manual" | "external" | string;
  attempt: number;
  depends_on_step_ids?: unknown;
  artifact_inputs?: unknown;
  snapshot?: unknown;
  started_at?: string | null;
  completed_at?: string | null;
  error?: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowArtifact {
  id: string;
  workflow_run_id: string;
  workflow_step_run_id: string;
  logical_name: string;
  version: number;
  content_kind: "markdown" | "json" | "text" | string;
  content_text?: string | null;
  content_json?: unknown;
  producer_type: "agent" | "member" | string;
  producer_id?: string | null;
  supersedes_artifact_id?: string | null;
  created_at: string;
}

export interface WorkflowReview {
  id: string;
  workflow_run_id: string;
  workflow_step_run_id?: string | null;
  workflow_artifact_id?: string | null;
  status: "requested" | "approved" | "rejected" | string;
  reviewer_id?: string | null;
  decision_notes?: string | null;
  reviewed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowQualityGateResult {
  id: string;
  workflow_run_id: string;
  workflow_step_run_id: string;
  workflow_artifact_id?: string | null;
  status: "pass" | "fail" | "warning" | string;
  blocking: boolean;
  producer_type: "agent" | "member" | string;
  producer_id?: string | null;
  report_text?: string | null;
  report_json?: unknown;
  created_at: string;
}

export interface WorkflowInputRequest {
  id: string;
  workspace_id: string;
  workflow_run_id: string;
  workflow_step_run_id: string;
  issue_id?: string | null;
  chat_session_id?: string | null;
  question_comment_id?: string | null;
  answer_comment_id?: string | null;
  requester_agent_id?: string | null;
  responder_id?: string | null;
  status: "requested" | "answered" | "cancelled" | "expired" | string;
  question_text: string;
  answer_text?: string | null;
  round_index: number;
  max_rounds: number;
  requested_at: string;
  answered_at?: string | null;
  cancelled_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowArtifactDiff {
  logical_name: string;
  base_version: number;
  target_version: number;
  content_kind: "markdown" | "json" | "text" | string;
  unified_diff: string;
  summary?: string | null;
}

export interface WorkflowRun {
  id: string;
  workspace_id: string;
  agent_task_queue_id: string;
  issue_id?: string | null;
  chat_session_id?: string | null;
  autopilot_run_id?: string | null;
  workflow_definition_id?: string | null;
  workflow_revision_id?: string | null;
  trigger_type: string;
  snapshot?: unknown;
  status: WorkflowRunStatus;
  started_at?: string | null;
  completed_at?: string | null;
  cancelled_at?: string | null;
  created_at: string;
  updated_at: string;
  steps?: WorkflowStepRun[];
  artifacts?: WorkflowArtifact[];
  reviews?: WorkflowReview[];
  quality_gate_results?: WorkflowQualityGateResult[];
  input_requests?: WorkflowInputRequest[];
  current_step?: WorkflowStepRun | null;
}
