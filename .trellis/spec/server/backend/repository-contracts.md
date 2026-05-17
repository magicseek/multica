# Repository Backend Contracts

## Scenario: First-Class Repository Start Modes

### 1. Scope / Trigger

- Trigger: repository start modes change backend API, database schema, generated sqlc models, chat responses, project references, and realtime events.
- Applies when modifying:
  - `repository`
  - `repository_binding`
  - `project_repository`
  - `repository_operation`
  - `task_output_metadata`
  - `chat_session.default_repository_id`
  - compatibility reads from `workspace.repos` and `project_resource(github_repo)`

### 2. Signatures

Database tables:

- `repository(id, workspace_id, name, source_state, remote_url, remote_key, default_branch, lead_agent_id, created_by, created_by_agent_id, status, metadata, created_at, updated_at)`
- `repository_binding(id, repository_id, workspace_id, owner_user_id, daemon_id, runtime_id, machine_label, binding_kind, local_path, state, last_seen_at, metadata, created_at, updated_at)`
- `project_repository(project_id, repository_id, workspace_id, role, position, created_at)`
- `repository_operation(id, repository_id, workspace_id, operation_type, status, requested_by_type, requested_by_id, target_daemon_id, target_runtime_id, binding_id, request, result, error, created_at, updated_at, completed_at)`
- `task_output_metadata(id, workspace_id, repository_id, task_id, relative_path, filename, kind, size_bytes, mime_type, metadata, created_at)`

API routes:

- `GET /api/repositories?workspace_id=...`
- `POST /api/repositories`
- `GET /api/repositories/{id}`
- `PATCH /api/repositories/{id}`
- `DELETE /api/repositories/{id}`
- `GET /api/repositories/{id}/bindings`
- `POST /api/repositories/{id}/bindings`
- `DELETE /api/repositories/{id}/bindings/{bindingId}`
- `GET /api/repositories/{id}/operations`
- `POST /api/repositories/{id}/operations`
- `POST /api/repositories/{id}/operations/{operationType}`
- `GET /api/projects/{id}/repositories`
- `PUT /api/projects/{id}/repositories`
- `GET /api/tasks/{taskId}/outputs`
- `POST /api/chat/sessions` may accept `default_repository_id`
- `PATCH /api/chat/sessions/{sessionId}` may set or clear `default_repository_id`

Daemon repository operation routes:

- `GET /api/daemon/repository-operations/claim?runtime_id=...`
- `POST /api/daemon/repository-operations/{operationId}/start?runtime_id=...`
- `POST /api/daemon/repository-operations/{operationId}/complete?runtime_id=...`
- `POST /api/daemon/repository-operations/{operationId}/fail?runtime_id=...`
- `POST /api/daemon/tasks/{taskId}/outputs`

Daemon repository operation claim/start payload may include:

- `operation.binding.id`
- `operation.binding.kind`
- `operation.binding.state`
- `operation.binding.daemon_id`
- `operation.binding.runtime_id?`
- `operation.binding.local_path`

Daemon task claim payload:

- `task.repositories[]`
  - `id`
  - `name`
  - `source_state`
  - `remote_url?`
  - `default_branch?`
  - `role`
  - `position`
  - `compatibility?`
  - `compatibility_source?`
  - `binding_available`
  - `binding?`
- `task.repositories[].binding`
  - `id`
  - `kind`
  - `state`
  - `machine_label?`
  - `daemon_id?`
  - `runtime_id?`
  - `available`
  - `current_daemon?`
  - `current_runtime?`
- `task.repos[]` remains the legacy remote-only URL list derived from `repositories[].remote_url`.

Realtime events:

- `repository:created`
- `repository:updated`
- `repository:binding_updated`
- `repository:published`
- `task:outputs_updated`

### 3. Contracts

- `repository.source_state` is one of `remote_git`, `local_dir`, `agent_managed`, `local_git`.
- `remote_git` repositories require `remote_url`.
- `repository.status = archived` means hidden from list reads and inaccessible by direct repository lookups.
- `repository.remote_key` is workspace-unique for non-archived repositories when present.
- Project settings store `repository_id` references only; do not copy remote URLs into project settings.
- Chat sessions expose `default_repository_id` as a nullable field. Clearing it is done by sending JSON `null`.
- Compatibility reads may synthesize read-only repository responses from old `workspace.repos` and `project_resource(github_repo)` storage. New writes must target first-class repository storage.
- `repository_binding.local_path` is private. Return it only to the binding owner or the corresponding daemon/runtime context.
- `repository_binding.metadata` is private whenever `local_path` is private. Treat metadata as potentially containing path fragments, tool output, or machine-local details.
- Workspace-wide realtime events must not include `local_path` or private binding metadata.
- Agents publish task output metadata by writing `.multica/outputs.json` under their workdir. The daemon uploads metadata only from that explicit manifest; it must not scan worktrees or upload file contents.
- `task_output_metadata.relative_path` must be repository/workdir relative. Reject absolute paths, parent traversal, backslashes, Windows drive-letter paths, and `~/...`.
- `task_output_metadata.metadata` must be a JSON object and must not include absolute local paths, workdir fields, secrets, logs, stack traces, screenshots, or file contents.
- Repository operations are a daemon-claim lifecycle with statuses `queued -> running -> succeeded|failed`.
- Operation types currently exposed through the lifecycle are `create_binding`, `init_git`, `publish_remote`, and `refresh_binding`; do not expose operation types that have no daemon/server completion semantics.
- `create_binding`, `init_git`, `publish_remote`, and `refresh_binding` require a target daemon. `init_git`, `publish_remote`, and `refresh_binding` require an existing binding.
- `init_git` is only valid from `local_dir` or `agent_managed`; successful completion transitions the repository to `local_git`.
- `publish_remote` is only valid from `local_git`; successful completion requires a valid `remote_url`, transitions the repository to `remote_git`, and emits `repository:published`.
- Daemon operation claim may be daemon-wide or runtime-scoped. If a daemon passes `runtime_id`, start/complete/fail must reject operations targeted at a sibling runtime, even when the daemon id matches.
- Operation request/result/error responses must redact or reject local paths. Binding completion may receive `binding.local_path` for server storage, but public repository operation responses, terminal daemon callback responses, and workspace realtime events must not echo it.
- Daemon repository operation claim/start responses may include `operation.binding.local_path` only when the operation targets a binding owned by the authenticated daemon and, when runtime-scoped, the authenticated runtime. Do not include binding metadata in this daemon-only operation payload.
- Daemon-side `create_binding` for agent-managed repositories creates only daemon-owned directories under `{workspaces_root}/{workspace_id}/repositories/{repository_id}/workdir`.
- Daemon-managed repository directory creation must reject symlink components and validate the resolved workdir remains under the resolved workspaces root.
- Daemon-side repository mutation execution must serialize operations by `binding_id` when present, otherwise by `repository_id`.
- Daemon terminal callbacks (`complete`/`fail`) must retry transient server errors after an operation has started; otherwise operations can be left indefinitely `running`.
- Daemon-side `init_git`, `refresh_binding`, and `publish_remote` must fail explicitly when the daemon claim/start payload lacks a matching ready binding path; never guess paths from repository ids, workspace ids, or workspaces-root layout.
- Daemon-side `init_git` runs only `git init`/HEAD setup and sanitized metadata collection. It must not run `git add .` or upload file names, contents, diffs, logs, screenshots, absolute paths, or stack traces.
- Daemon-side `publish_remote` must require an existing commit before pushing. If there is no commit, fail with a recoverable `no_commits_to_push` result and leave the repository source state unchanged.
- Task claim repository precedence is task-kind specific: issue and quick-create tasks use project first-class `project_repository`, then legacy project `project_resource(github_repo)`, then workspace first-class repositories, then legacy `workspace.repos`; chat tasks use `chat_session.default_repository_id`, then workspace first-class repositories, then legacy `workspace.repos`; autopilot run-only tasks use workspace first-class repositories, then legacy `workspace.repos`.
- Task claim compatibility payloads are read-only synthetic repositories. They must populate `compatibility=true` and `compatibility_source` with `project_resource.github_repo` or `workspace.repos`.
- Task claim `binding` is a sanitized current-runtime/current-daemon summary only. A `ready` binding on a different daemon/runtime is not claim-eligible and must not appear as `binding_available=true`.
- Task claim `binding` must never include `local_path` or binding `metadata`, even for the daemon/runtime that owns the binding.
- Daemon execenv may render repository identity, source state, checkout command, and sanitized binding summary. It must keep remote checkout behavior for repositories with `remote_url`.
- Until local binding cwd switching is implemented, daemon execenv must not switch cwd to local/agent-managed bindings and must tell the agent not to run `multica repo checkout` for repositories without `remote_url`.

### 4. Validation & Error Matrix

- Invalid repository UUID -> `400`.
- Repository outside workspace -> `404`.
- Archived repository direct lookup -> `404`.
- Duplicate non-archived `remote_key` in a workspace -> `409`.
- `source_state=remote_git` without `remote_url` -> `400`.
- Invalid Git remote URL -> `400`.
- Project repository reference to a missing or archived repository -> `404`.
- More than one `primary` repository for a project -> `400`.
- Binding create without `daemon_id` or `local_path` -> `400`.
- Binding delete by non-owner non-admin -> `403`.
- Non-object `metadata`, `request`, or `result` JSON -> `400`.
- Repository operation with invalid operation type -> `400`.
- Repository operation requiring daemon without `target_daemon_id` or daemon-bearing `target_runtime_id`/`binding_id` -> `400`.
- Repository operation with sibling runtime trying to start/complete/fail -> `404`.
- Repository operation complete/fail when status is not `running` -> `409`.
- `init_git` from non-local source state -> `400`.
- `publish_remote` from non-`local_git` source state -> `400`.
- `publish_remote` completion without valid `remote_url` -> `400`.
- Daemon-managed `create_binding` path has unsafe components or symlink traversal -> daemon reports operation `failed`.
- Daemon-side `init_git`, `refresh_binding`, or `publish_remote` without a safe binding path channel -> daemon reports operation `failed` with an unsupported reason.
- `publish_remote` without `request.remote_url` -> `400`.
- `publish_remote` with no local commits -> daemon reports operation `failed` with `reason=no_commits_to_push` and `recoverable=true`.
- Task claim with first-class repositories that lack `remote_url` -> `task.repositories` populated and legacy `task.repos` empty or filtered to remote-backed repositories only.
- Task claim with only foreign ready bindings -> `binding_available=false` and no `binding` object in the claim payload.

### 5. Good/Base/Bad Cases

- Good: list repositories returns first-class rows plus compatibility rows from legacy storage, deduplicated by normalized remote key.
- Good: non-owner binding response includes machine label, kind, state, and visibility flag, but omits local path and returns `{}` metadata.
- Good: issue task in a project with `project_repository` returns only project repositories in `task.repositories`; workspace repositories do not leak into that task.
- Good: chat task with `default_repository_id` returns that repository as primary and does not fall back to workspace repositories.
- Good: daemon task claim for a repository with a ready binding on another machine omits `binding` and keeps `binding_available=false`.
- Good: daemon claims the oldest queued operation for its daemon, marks it running, completes it once, and receives `409` on a second terminal completion attempt.
- Good: `create_binding` completion stores a private binding path but returns only sanitized operation and binding summaries.
- Good: runtime-scoped operation cannot be started by a sibling runtime on the same daemon.
- Good: daemon creates an agent-managed workdir under the Multica workspaces root and rejects a pre-existing symlinked `repositories` path.
- Good: daemon retries a transient failed `/complete` callback instead of abandoning an already-started operation.
- Base: chat create/update with a valid workspace repository stores `default_repository_id` and returns it in session responses.
- Bad: direct GET continues returning an archived repository after soft delete.
- Bad: binding event includes `metadata.last_verified_path` or any local absolute path.
- Bad: task claim exposes `/Users/name/project`, binding metadata, or a foreign daemon binding as available to the claiming daemon.
- Bad: task output metadata accepts `C:\Users\name\repo\file.ts`, `/tmp/file`, `../secret`, or `~/secret`.
- Bad: daemon scans the whole workdir and uploads discovered file lists without an explicit `.multica/outputs.json` manifest.

### 6. Tests Required

- Repository lifecycle: create, update, list, archive, direct GET after archive.
- Compatibility read model: old workspace repos and project `github_repo` resources appear as compatibility repository responses.
- Binding privacy: owner sees `local_path` and metadata; other workspace member does not.
- Project repository references: setting primary/secondary rows validates workspace membership and primary uniqueness.
- Chat default repository: create, read/list, update, and clear nullable repository reference.
- Repository operation lifecycle: create/list, daemon claim/start/complete/fail, terminal conflict behavior, cross-workspace rejection, and runtime-scoped sibling rejection.
- Repository operation transitions: `create_binding` initializes a binding, `init_git` moves to `local_git`, and `publish_remote` moves to `remote_git` with canonical `remote_url`.
- Repository operation privacy: request/result/error responses redact local paths; binding completion does not echo `local_path` in operation responses or realtime events.
- Daemon operation executor: `create_binding` creates a daemon-owned workdir, rejects symlink escape attempts, serializes same-repository mutations, retries transient terminal callback failures, executes `init_git` without staging files, pushes `publish_remote` only when commits exist, and explicitly fails unsupported/missing-path operations without leaking local paths.
- Task claim repository precedence: project first-class overrides workspace fallback, project legacy `github_repo` still overrides workspace fallback, chat default overrides workspace fallback, and workspace first-class overrides legacy `workspace.repos`.
- Task claim binding privacy/eligibility: foreign ready bindings do not set `binding_available`, and claim JSON never contains `local_path` or binding metadata.
- Daemon execenv rendering: repositories with `remote_url` render `multica repo checkout`, while local/agent-managed repositories without `remote_url` render no checkout command and no local path.
- Task output metadata: daemon reads only `.multica/outputs.json`, handler replaces prior task output metadata on upload, user API lists metadata after workspace membership checks, and unsafe manifests are rejected.
- Migration/path constraints: output metadata rejects absolute and traversal-style paths.
- Realtime payload tests or handler assertions must verify binding events omit local paths and private metadata.

### 7. Wrong vs Correct

#### Wrong

```json
{
  "machine_label": "Troy MacBook",
  "local_path": "/Users/troy/private/app",
  "metadata": { "last_verified_path": "/Users/troy/private/app" }
}
```

This leaks private machine-local state to workspace members and realtime subscribers.

#### Correct

```json
{
  "machine_label": "Troy MacBook",
  "binding_kind": "local_dir",
  "state": "ready",
  "metadata": {},
  "local_path_visible": false
}
```

Reveal private path and metadata only through an owner/daemon-authorized response.
