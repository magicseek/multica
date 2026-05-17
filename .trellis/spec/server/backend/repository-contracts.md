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
- `GET /api/projects/{id}/repositories`
- `PUT /api/projects/{id}/repositories`
- `POST /api/chat/sessions` may accept `default_repository_id`
- `PATCH /api/chat/sessions/{sessionId}` may set or clear `default_repository_id`

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
- `task_output_metadata.relative_path` must be repository/workdir relative. Reject absolute paths, parent traversal, backslashes, Windows drive-letter paths, and `~/...`.
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
- Task claim with first-class repositories that lack `remote_url` -> `task.repositories` populated and legacy `task.repos` empty or filtered to remote-backed repositories only.
- Task claim with only foreign ready bindings -> `binding_available=false` and no `binding` object in the claim payload.

### 5. Good/Base/Bad Cases

- Good: list repositories returns first-class rows plus compatibility rows from legacy storage, deduplicated by normalized remote key.
- Good: non-owner binding response includes machine label, kind, state, and visibility flag, but omits local path and returns `{}` metadata.
- Good: issue task in a project with `project_repository` returns only project repositories in `task.repositories`; workspace repositories do not leak into that task.
- Good: chat task with `default_repository_id` returns that repository as primary and does not fall back to workspace repositories.
- Good: daemon task claim for a repository with a ready binding on another machine omits `binding` and keeps `binding_available=false`.
- Base: chat create/update with a valid workspace repository stores `default_repository_id` and returns it in session responses.
- Bad: direct GET continues returning an archived repository after soft delete.
- Bad: binding event includes `metadata.last_verified_path` or any local absolute path.
- Bad: task claim exposes `/Users/name/project`, binding metadata, or a foreign daemon binding as available to the claiming daemon.
- Bad: task output metadata accepts `C:\Users\name\repo\file.ts`, `/tmp/file`, `../secret`, or `~/secret`.

### 6. Tests Required

- Repository lifecycle: create, update, list, archive, direct GET after archive.
- Compatibility read model: old workspace repos and project `github_repo` resources appear as compatibility repository responses.
- Binding privacy: owner sees `local_path` and metadata; other workspace member does not.
- Project repository references: setting primary/secondary rows validates workspace membership and primary uniqueness.
- Chat default repository: create, read/list, update, and clear nullable repository reference.
- Task claim repository precedence: project first-class overrides workspace fallback, project legacy `github_repo` still overrides workspace fallback, chat default overrides workspace fallback, and workspace first-class overrides legacy `workspace.repos`.
- Task claim binding privacy/eligibility: foreign ready bindings do not set `binding_available`, and claim JSON never contains `local_path` or binding metadata.
- Daemon execenv rendering: repositories with `remote_url` render `multica repo checkout`, while local/agent-managed repositories without `remote_url` render no checkout command and no local path.
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
