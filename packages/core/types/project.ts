export type ProjectStatus = "planned" | "in_progress" | "paused" | "completed" | "cancelled";

export type ProjectPriority = "urgent" | "high" | "medium" | "low" | "none";

export interface Project {
  id: string;
  workspace_id: string;
  title: string;
  description: string | null;
  icon: string | null;
  status: ProjectStatus;
  priority: ProjectPriority;
  lead_type: "member" | "agent" | null;
  lead_id: string | null;
  workflow_definition_id?: string | null;
  created_at: string;
  updated_at: string;
  issue_count: number;
  done_count: number;
  resource_count: number;
}

export interface CreateProjectRequest {
  title: string;
  description?: string;
  icon?: string;
  status?: ProjectStatus;
  priority?: ProjectPriority;
  lead_type?: "member" | "agent";
  lead_id?: string;
  workflow_definition_id?: string | null;
  // Resources to attach in the same transaction as the project. Server returns
  // 4xx (and rolls back) if any one is invalid or duplicate.
  resources?: CreateProjectResourceRequest[];
}

export interface UpdateProjectRequest {
  title?: string;
  description?: string | null;
  icon?: string | null;
  status?: ProjectStatus;
  priority?: ProjectPriority;
  lead_type?: "member" | "agent" | null;
  lead_id?: string | null;
  workflow_definition_id?: string | null;
}

export interface ListProjectsResponse {
  projects: Project[];
  total: number;
}

// ProjectResource is a typed pointer from a project to an external resource.
// The resource_ref shape depends on resource_type (e.g. github_repo carries
// { url, default_branch_hint? }). New types add a case in
// validateAndNormalizeResourceRef on the server and a renderer in the UI;
// no schema or type changes required.
export type ProjectResourceType =
  | "github_repo"
  | "ringcentral_gitlab_repo"
  | "ringcentral_jira_project"
  | "ringcentral_jira_issue"
  | "ringcentral_wiki_space"
  | "ringcentral_wiki_page";

export interface GithubRepoResourceRef {
  url: string;
  default_branch_hint?: string;
}

export interface RingCentralGitLabRepoResourceRef {
  project_id: string;
  web_url?: string;
  default_branch?: string;
}

export interface RingCentralJiraProjectResourceRef {
  project_key: string;
  name?: string;
}

export interface RingCentralJiraIssueResourceRef {
  issue_key: string;
  summary?: string;
}

export interface RingCentralWikiSpaceResourceRef {
  space_key: string;
  name?: string;
}

export interface RingCentralWikiPageResourceRef {
  page_id: string;
  space_key?: string;
  title?: string;
}

export type RingCentralResourceRef =
  | RingCentralGitLabRepoResourceRef
  | RingCentralJiraProjectResourceRef
  | RingCentralJiraIssueResourceRef
  | RingCentralWikiSpaceResourceRef
  | RingCentralWikiPageResourceRef;

export interface ProjectResource {
  id: string;
  project_id: string;
  workspace_id: string;
  resource_type: ProjectResourceType;
  resource_ref: GithubRepoResourceRef | RingCentralResourceRef | Record<string, unknown>;
  label: string | null;
  position: number;
  created_at: string;
  created_by: string | null;
}

export interface CreateProjectResourceRequest {
  resource_type: ProjectResourceType;
  resource_ref: GithubRepoResourceRef | RingCentralResourceRef | Record<string, unknown>;
  label?: string;
  position?: number;
}

export interface ListProjectResourcesResponse {
  resources: ProjectResource[];
  total: number;
}
