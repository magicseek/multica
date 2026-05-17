export type RepositorySourceState =
  | "remote_git"
  | "local_dir"
  | "agent_managed"
  | "local_git";

export type RepositoryStatus = "initializing" | "ready" | "error" | "archived";

export type RepositoryBindingKind =
  | "local_dir"
  | "daemon_workdir"
  | "git_cache_worktree";

export type RepositoryBindingState =
  | "ready"
  | "missing"
  | "inaccessible"
  | "initializing"
  | "error";

export type RepositoryOperationType =
  | "create_binding"
  | "init_git"
  | "publish_remote"
  | "refresh_binding";

export type RepositoryOperationStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed";

export type ProjectRepositoryRole = "primary" | "secondary";

export type TaskOutputMetadataKind =
  | "source"
  | "doc"
  | "artifact"
  | "log"
  | "report"
  | "unknown";

export interface Repository {
  id: string;
  workspace_id: string;
  name: string;
  source_state: RepositorySourceState;
  remote_url: string | null;
  remote_key: string | null;
  default_branch: string | null;
  lead_agent_id: string | null;
  created_by: string | null;
  created_by_agent_id: string | null;
  status: RepositoryStatus;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  compatibility?: boolean;
  compatibility_source?: string;
}

export interface RepositoryBinding {
  id: string;
  repository_id: string;
  workspace_id: string;
  owner_user_id: string | null;
  daemon_id: string;
  runtime_id: string | null;
  machine_label: string;
  binding_kind: RepositoryBindingKind;
  local_path?: string | null;
  state: RepositoryBindingState;
  last_seen_at: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  local_path_visible: boolean;
}

export interface ProjectRepository {
  project_id: string;
  repository_id: string;
  role: ProjectRepositoryRole;
  position: number;
  created_at: string;
  repository: Repository;
}

export interface RepositoryOperation {
  id: string;
  repository_id: string;
  workspace_id: string;
  operation_type: RepositoryOperationType;
  status: RepositoryOperationStatus;
  requested_by_type: string;
  requested_by_id: string | null;
  target_daemon_id: string | null;
  target_runtime_id: string | null;
  binding_id: string | null;
  request: Record<string, unknown>;
  result: Record<string, unknown>;
  error: string | null;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}

export interface TaskOutputMetadata {
  id: string;
  workspace_id: string;
  repository_id: string | null;
  task_id: string;
  relative_path: string;
  filename: string;
  kind: TaskOutputMetadataKind;
  size_bytes: number | null;
  mime_type: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface ListRepositoriesResponse {
  repositories: Repository[];
  total: number;
}

export interface ListRepositoryBindingsResponse {
  bindings: RepositoryBinding[];
  total: number;
}

export interface ListProjectRepositoriesResponse {
  repositories: ProjectRepository[];
  total: number;
}

export interface ListRepositoryOperationsResponse {
  operations: RepositoryOperation[];
  total: number;
}

export interface ListTaskOutputMetadataResponse {
  outputs: TaskOutputMetadata[];
  total: number;
}

export interface CreateRepositoryRequest {
  name?: string;
  source_state?: RepositorySourceState;
  remote_url?: string | null;
  default_branch?: string | null;
  lead_agent_id?: string | null;
  status?: RepositoryStatus;
  metadata?: Record<string, unknown>;
  binding?: CreateRepositoryBindingRequest | null;
}

export interface UpdateRepositoryRequest {
  name?: string;
  remote_url?: string | null;
  default_branch?: string | null;
  lead_agent_id?: string | null;
  status?: RepositoryStatus;
  metadata?: Record<string, unknown>;
}

export interface CreateRepositoryBindingRequest {
  daemon_id: string;
  runtime_id?: string | null;
  machine_label?: string;
  binding_kind?: RepositoryBindingKind;
  local_path: string;
  state?: RepositoryBindingState;
  metadata?: Record<string, unknown>;
}

export interface SetProjectRepositoryItem {
  repository_id: string;
  role?: ProjectRepositoryRole;
  position?: number;
}

export interface SetProjectRepositoriesRequest {
  repositories: SetProjectRepositoryItem[];
}

export interface CreateRepositoryOperationRequest {
  operation_type?: RepositoryOperationType;
  target_daemon_id?: string | null;
  target_runtime_id?: string | null;
  binding_id?: string | null;
  request?: Record<string, unknown>;
}
