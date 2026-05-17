export type WorkflowOrigin = "system_seeded" | "user";

export type WorkflowRevisionStatus = "draft" | "published" | "deprecated";

export type WorkflowApplicability = "assignment" | "comment";

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
  body_template?: string;
  description?: string;
  checklist?: string[];
}

export interface WorkflowGate {
  id: string;
  title: string;
  description?: string;
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
  current_revision?: WorkflowRevision | null;
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
