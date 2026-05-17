# Repository Start Modes Design

> Status: Draft  
> Date: 2026-05-17  
> Scope: Workspace repository model, local bindings, agent-managed starts, UI entry points, daemon execution, migration

## TL;DR

Multica should keep using **Repositories** as the product concept for code working targets. Repositories remain managed from **Workspace Settings**, but become first-class server entities instead of only `workspace.repos` JSON and `project_resource(github_repo)` payloads.

The new model supports three ways to start work:

1. **Remote Git**: the current flow. A repository has a cloneable remote URL.
2. **Local Directory**: the user binds a local folder on a specific machine/runtime.
3. **Agent-managed Directory**: the user selects no repo or folder; the selected lead agent runtime creates a managed work directory under its Multica workspace root.

Local and agent-managed repositories can later move one way through:

```text
local_dir | agent_managed -> local_git -> remote_git
```

Projects and chats reference repositories by `repository_id`. They do not copy repository URLs or paths.

## Current State

Multica already has a repository concept, but it is not yet strong enough for local or from-scratch starts.

Current storage:

- Workspace-level repositories live in `workspace.repos` as JSONB.
- Project-level repositories live as `project_resource` rows with `resource_type = "github_repo"`.
- Daemon task claim responses lift project `github_repo` resources into the task repo list. If a project has no repo resources, the claim path falls back to workspace repos.
- Daemon checkout assumes a remote Git URL. It keeps bare clone caches under `.repos` and creates per-task worktrees on demand through `multica repo checkout`.
- Chat sessions persist agent conversation state, but do not have repository identity.

This works for existing GitHub/GitLab style work, but creates a high onboarding bar: a new user must already have a Git repository or know how to bind one before an agent can build anything meaningful.

## Goals

- Let a new user start coding work without an existing remote Git repository.
- Let a user select a local directory as a repository binding.
- Let a lead agent initialize an agent-managed workspace directory when no directory is selected.
- Keep repositories workspace-owned and managed from Workspace Settings.
- Let projects and chats reference repositories without owning or duplicating them.
- Preserve privacy for local paths and local-derived content.
- Preserve the remote Git path as the default scalable collaboration model.
- Provide a clear path to initialize local Git and publish to remote Git.
- Migrate existing `workspace.repos` and `project_resource(github_repo)` without breaking current behavior.

## Non-Goals

- Do not add a top-level Repositories page to the primary workspace navigation.
- Do not rename the product concept to Codebase.
- Do not automatically upload local file contents, diffs, logs, or screenshots to the cloud.
- Do not force every planning Project to have a repository.
- Do not require Git initialization before an agent can start in an agent-managed directory.
- Do not keep long-term dual writes across old and new repository storage.

## Domain Decisions

### Repository, Not Codebase

Reuse **Repository** as the canonical product term.

A Repository is a code working target that an agent can read or write. It may be backed by:

- a remote Git URL,
- a user-selected local directory,
- a daemon-created agent-managed directory.

The existing **Project** concept remains a planning container for issues, similar to a Linear project or Jira epic. Project is not renamed and should not become the code working target.

### Workspace Ownership

Repositories are workspace-level resources.

Projects, chats, and tasks reference repositories, but do not own them. This keeps access, bindings, publish state, activity, and migration in one place.

### Settings Placement

Repository management stays under Workspace Settings. Work surfaces such as project detail, chat, and quick-create show lightweight repository chips/selectors, but advanced management lives in settings.

## Data Model

### `repository`

First-class repository identity.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `workspace_id` | Owner workspace |
| `name` | Human-readable label |
| `source_state` | `remote_git`, `local_dir`, `agent_managed`, `local_git` |
| `remote_url` | Nullable. Required only for `remote_git` |
| `remote_key` | Nullable normalized identity for dedupe when remote exists |
| `default_branch` | Nullable until Git is initialized or remote is known |
| `lead_agent_id` | Nullable. Used for agent-managed starts and default owner intent |
| `created_by` | Member who created the repository |
| `created_by_agent_id` | Nullable if created as part of an agent flow |
| `status` | `initializing`, `ready`, `error`, `archived` |
| `metadata` | Small shared metadata only. No local absolute paths or file contents |
| `created_at`, `updated_at` | Timestamps |

`source_state` is the current sharing/execution state. Binding rows preserve whether the actual path is a user local directory or a daemon-managed directory.

### `repository_binding`

Machine/runtime scoped path authorization.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `repository_id` | FK |
| `workspace_id` | Redundant for query scoping |
| `owner_user_id` | User who authorized this binding |
| `daemon_id` | Stable daemon machine identity |
| `runtime_id` | Nullable. Use when binding is runtime-specific |
| `machine_label` | User-facing label, e.g. "Troy MacBook Pro" |
| `binding_kind` | `local_dir`, `daemon_workdir`, `git_cache_worktree` |
| `local_path` | Full path. Filtered by API visibility rules |
| `state` | `ready`, `missing`, `inaccessible`, `initializing`, `error` |
| `last_seen_at` | Last daemon confirmation |
| `metadata` | Small private/binding metadata |
| `created_at`, `updated_at` | Timestamps |

Visibility rule:

- Full `local_path` is visible only to the binding owner and the corresponding daemon/runtime by default.
- Other workspace members may see machine label, binding kind, state, and availability.
- Local paths must not be emitted in workspace-wide realtime events or generic project/chat payloads.

### `project_repository`

Project-to-repository references.

Suggested fields:

| Field | Notes |
|---|---|
| `project_id` | FK |
| `repository_id` | FK |
| `role` | `primary`, `secondary` |
| `position` | Sort order |
| `created_at` | Timestamp |

Project-level repository settings store `repository_id` references only. They do not copy `remote_url` or paths.

### `chat_session.default_repository_id`

Chat sessions may have a default repository. This makes a long-running chat behave like a Codex-style project conversation: the chat has a durable working context, but the repository remains a workspace resource.

### `repository_operation`

Async operation ledger for repository actions.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `repository_id` | FK |
| `workspace_id` | FK |
| `operation_type` | `create_binding`, `init_git`, `publish_remote`, `refresh_binding`, `archive` |
| `status` | `queued`, `running`, `succeeded`, `failed`, `cancelled` |
| `requested_by_type` | `member`, `agent`, `system` |
| `requested_by_id` | Actor id |
| `target_daemon_id` | Required for local operations |
| `target_runtime_id` | Optional |
| `binding_id` | Optional operation target |
| `request` | Sanitized operation parameters |
| `result` | Sanitized result metadata |
| `error` | Sanitized error |
| `created_at`, `updated_at`, `completed_at` | Timestamps |

This avoids hiding long-running filesystem, Git, and provider operations inside synchronous API calls.

### `task_output_metadata`

Cloud-visible metadata for local outputs.

Suggested fields:

| Field | Notes |
|---|---|
| `id` | UUID |
| `workspace_id` | FK |
| `repository_id` | Nullable FK |
| `task_id` | FK to agent task |
| `relative_path` | Repo/workdir relative path only |
| `filename` | Display name |
| `kind` | `source`, `doc`, `artifact`, `log`, `report`, `unknown` |
| `size_bytes` | Optional |
| `mime_type` | Optional |
| `created_at` | Timestamp |
| `metadata` | Sanitized metadata only |

Output metadata comes from an explicit manifest produced by the agent/daemon. The daemon should not scan the entire workdir by default.

## Source State Transitions

### Create From Remote Git

Current behavior, moved to the new model.

1. User adds a remote URL in Workspace Settings or project creation.
2. Server creates `repository(source_state=remote_git, remote_url=...)`.
3. Daemon sees the repo through task payload or workspace repo sync.
4. Daemon uses existing bare cache and per-task worktree checkout.

### Create From Local Directory

1. User selects a local folder through desktop folder picker or local bridge.
2. Server creates `repository(source_state=local_dir)`.
3. Server creates `repository_binding(binding_kind=local_dir, local_path=..., daemon_id=..., owner_user_id=...)`.
4. The repository is available only to runtimes on that daemon until it becomes remote Git or another binding is added.

If the selected directory already has `.git`, the create flow may set `source_state=local_git` after daemon verification.

### Create From No Directory

This is the new from-scratch path.

1. User starts a build flow and selects `Start in agent-managed workspace`.
2. User selects a lead agent.
3. Server creates `repository(source_state=agent_managed, status=initializing, lead_agent_id=...)`.
4. Server creates a `create_binding` repository operation for the lead agent's runtime/daemon.
5. Daemon creates a directory under its Multica workspaces root, for example:

   ```text
   {WorkspacesRoot}/{workspace_id}/repositories/{repository_id}/workdir
   ```

6. Daemon writes minimal bootstrap files such as `AGENTS.md`, `.gitignore`, and optionally `README.md`.
7. Daemon reports success and server creates `repository_binding(binding_kind=daemon_workdir, state=ready)`.
8. Any initial chat/task runs with this repository as the primary repository.

The server records intent. The daemon owns the actual filesystem path.

### Initialize Git

Action: `init_git`.

Allowed from:

- `local_dir`
- `agent_managed`

Flow:

1. User clicks `Initialize Git`.
2. Server validates source state and target binding ownership.
3. Server queues `repository_operation(type=init_git)`.
4. Daemon runs in the binding path:

   ```bash
   git init -b main
   ```

   The daemon must not run `git add .` by default. For a user-selected local directory, initialization should stop after `git init` plus safe `.gitignore` verification and return sanitized status metadata. For an agent-managed empty directory, the daemon may create an initial commit containing only known bootstrap files that it created itself.

5. If `.git` already exists, daemon adopts the existing local Git repo by reporting branch and HEAD instead of reinitializing.
6. Server updates:

   - `repository.source_state = local_git`
   - `repository.default_branch`
   - sanitized `head_commit` metadata

No file content, diff, log, or absolute path is uploaded.

### Publish Remote

Action: `publish_remote`.

Allowed from:

- `local_git`

Flow:

1. User clicks `Publish Remote`.
2. User chooses provider/account/namespace/repo name/visibility.
3. UI shows sanitized publish readiness metadata from the binding daemon, including whether the repository has commits and whether there are uncommitted files. It must not upload file content or diffs for this check.
4. Server creates remote repo through provider connector.
5. Server queues `repository_operation(type=publish_remote)` for the binding daemon.
6. Daemon runs:

   ```bash
   git remote add origin <remote_url>
   git push -u origin <default_branch>
   ```

   If there is no commit yet, the daemon fails with a recoverable `no_commits_to_push` result. The user or lead agent must explicitly create a commit before retrying publish.

7. Daemon reports remote URL, branch, and HEAD.
8. Server updates:

   - `repository.source_state = remote_git`
   - `repository.remote_url`
   - `repository.remote_key`
   - `repository.default_branch`

9. Server broadcasts `repository:published` and records activity/inbox notifications.

After publish, other runtimes can use the remote Git checkout path.

## Project Behavior

Projects stay planning containers.

Project creation should support two paths:

- **Planning only**: no repository required.
- **Build with repository**: create or select a primary repository.

For code-oriented project starts, automatically associate a primary repository:

- remote Git: link selected/created remote repository,
- local dir: link selected local repository,
- agent-managed: create and link the agent-managed repository.

Do not attach every workspace repository to a new project. Do not copy remote URLs into project settings.

## Chat Behavior

Chat sessions should support repository context without owning repositories.

Recommended UI:

- Chat input shows a repository chip:
  - `No repository`
  - `foo · remote`
  - `foo · local`
  - `foo · agent-managed`
- Clicking the chip opens a lightweight selector.
- Advanced management links to Workspace Settings -> Repositories.
- If a user sends a build-from-scratch request and no repository is selected, prompt for:
  - Use existing repository
  - Select local folder
  - Start in agent-managed workspace

Once chosen, `chat_session.default_repository_id` stores the repository reference. Sustained coding work should not remain a long-lived no-repository draft.

## Workspace Settings UI

Repositories remain a Workspace Settings tab.

Recommended layout:

- List rows:
  - name,
  - source state,
  - remote URL if present,
  - binding availability,
  - used by projects/chats,
  - status.
- Detail drawer:
  - Overview,
  - Bindings,
  - Linked projects,
  - Linked chats,
  - Recent activity,
  - Output metadata,
  - Actions.

Primary actions:

- Add remote Git repository
- Bind local directory
- Start agent-managed repository
- Initialize Git
- Publish Remote
- Add binding on this machine
- Archive repository

The Settings page is the management surface. Project and chat pages are context surfaces.

## Task Dispatch and Daemon Execution

### Task Payload

Task claim responses should move from raw repo URL lists toward repository context:

```json
{
  "repositories": [
    {
      "id": "repo-id",
      "name": "app",
      "role": "primary",
      "source_state": "agent_managed",
      "remote_url": null,
      "default_branch": null,
      "binding": {
        "id": "binding-id",
        "kind": "daemon_workdir",
        "state": "ready"
      }
    }
  ]
}
```

For daemon-authenticated payloads, the binding may include the actual path when it belongs to that daemon. Generic frontend payloads must not include another user's local path.

### Runtime Eligibility

Repository binding is part of runtime eligibility.

| Repository state | Eligible runtimes |
|---|---|
| `remote_git` | Any runtime that can clone/fetch the remote |
| `local_dir` | Runtime/daemon with a ready binding |
| `agent_managed` | Runtime/daemon with the daemon workdir binding |
| `local_git` without remote | Runtime/daemon with a ready binding |

If a task references a local-only repository and the selected agent's runtime has no binding, the UI should block or guide the user to add a binding.

### CWD Resolution

Primary repository determines the task cwd.

Recommended daemon behavior:

- `remote_git`: keep existing repo cache and per-task worktree checkout.
- `local_dir`: run in the binding path or a daemon-managed task subdir, depending on safety policy.
- `agent_managed`: run in the daemon-managed binding path.
- `local_git` without remote: run in the binding path by default; future work may add local `git worktree` support.

For non-remote direct bindings, serialize mutating tasks per repository binding to avoid concurrent writes to the same directory.

Secondary repositories can be checked out or linked under the primary cwd as subdirectories.

## Output Metadata and Privacy

Local and agent-managed tasks may expose output metadata to the cloud, but not output contents by default.

Allowed by default:

- relative path,
- filename,
- kind,
- size,
- MIME type,
- producing task id,
- repository id,
- creation time.

Not allowed by default:

- file content,
- diffs,
- full logs,
- stack traces,
- screenshots,
- absolute local paths,
- ignored or secret-like path details.

The daemon should upload output metadata from an explicit manifest, for example:

```json
{
  "outputs": [
    {
      "relative_path": "docs/design.md",
      "kind": "doc",
      "size": 18342,
      "mime_type": "text/markdown"
    }
  ]
}
```

Clicking a local-only output in the UI should offer:

- Open on runtime,
- Publish/attach this file,
- Copy relative path.

Publishing or attaching content requires explicit user approval from the binding owner or an authorized local collaborator.

## Collaboration and Discovery

When a repository is published to remote Git, discovery should not depend on duplicated project settings.

Publish success should:

- update the `repository` row,
- broadcast a workspace realtime event,
- record activity,
- optionally create inbox notifications for relevant project/chat participants,
- include the remote Git repository in future task claim payloads.

Suggested event:

```json
{
  "repository_id": "repo-id",
  "workspace_id": "workspace-id",
  "name": "app",
  "source_state": "remote_git",
  "remote_url": "https://github.com/org/app.git",
  "default_branch": "main",
  "published_by": { "type": "member", "id": "member-id" },
  "previous_source_state": "agent_managed"
}
```

Do not include `local_path` in this event.

## API Sketch

Repository management:

```text
GET    /api/repositories?workspace_id=...
POST   /api/repositories
GET    /api/repositories/{id}
PATCH  /api/repositories/{id}
DELETE /api/repositories/{id}
```

Bindings:

```text
GET    /api/repositories/{id}/bindings
POST   /api/repositories/{id}/bindings
DELETE /api/repositories/{id}/bindings/{bindingId}
```

Operations:

```text
POST   /api/repositories/{id}/operations/init-git
POST   /api/repositories/{id}/operations/publish-remote
GET    /api/repositories/{id}/operations
```

Project references:

```text
GET    /api/projects/{id}/repositories
PUT    /api/projects/{id}/repositories
```

Chat reference:

```text
PATCH  /api/chat/sessions/{id}
       { "default_repository_id": "..." }
```

Daemon operations:

```text
GET    /api/daemon/repository-operations/claim
POST   /api/daemon/repository-operations/{id}/start
POST   /api/daemon/repository-operations/{id}/complete
POST   /api/daemon/repository-operations/{id}/fail
```

The exact daemon API can reuse existing daemon websocket wakeups and auth patterns.

## Migration Plan

### Phase 1: Add New Tables and Read Models

- Add `repository`, `repository_binding`, `project_repository`, `repository_operation`.
- Keep old fields.
- Expose repository list from new table when present.
- Generate compatibility repository rows in read paths when only `workspace.repos` exists.

### Phase 2: Backfill Existing Data

- Convert each `workspace.repos[].url` into `repository(source_state=remote_git)`.
- Deduplicate by normalized remote key per workspace.
- Convert `project_resource(github_repo)` into `project_repository` links.
- Preserve `project_resource` rows for compatibility reads.

### Phase 3: New Writes Only

- Workspace Settings -> Repositories writes `repository`.
- Project repository selector writes `project_repository`.
- Create Project with repo writes repository references, not `project_resource(github_repo)`.
- Daemon claim builds repository context from new tables.

### Phase 4: Deprecate Old Writes

- Stop writing `workspace.repos`.
- Stop writing `project_resource(github_repo)` for repository links.
- Keep old API compatibility responses for one release window.

### Phase 5: Remove Compatibility

- Drop old write paths and eventually old storage, after client compatibility windows close.

## Implementation Notes by Layer

### Server

- Add repository queries and handlers.
- Add repository operation service.
- Add workspace membership checks around every repository and binding route.
- Filter binding path visibility by user and daemon auth context.
- Update task claim assembly to resolve repository context.
- Add realtime event types:
  - `repository:created`
  - `repository:updated`
  - `repository:binding_updated`
  - `repository:published`

### Daemon

- Add repository operation claim loop.
- Add local binding verification.
- Add agent-managed directory creation under `WorkspacesRoot`.
- Add `init_git` and `publish_remote` execution.
- Extend execenv to support primary repository cwd.
- Upload explicit output metadata manifests.
- Enforce per-binding mutation locks for direct local/agent-managed paths.

### Core Package

- Add repository types, schemas, queries, mutations.
- Keep server response parsing defensive for desktop compatibility.
- Add repository selection state only where it is client state. Repository server data remains React Query state.

### Views

- Rework Workspace Settings -> Repositories tab around first-class repositories.
- Add repository chips/selectors to chat and project surfaces.
- Add build-oriented Project creation path.
- Add local-only output metadata UI.

### Desktop

- Folder picking and local path display stay desktop/local bridge responsibilities.
- Full local paths should only appear when the current user owns the binding or the local daemon confirms ownership.
- Desktop can provide "Open on runtime" for local outputs and bindings.

## Open Implementation Choices

These do not block the architecture, but should be resolved during implementation planning:

- Whether `init_git` creates an initial commit automatically or only runs `git init`.
- Whether local Git without remote should use direct cwd or local `git worktree` branches.
- Whether `repository_binding` should be keyed strictly to `daemon_id` or also to `runtime_id`.
- Whether output metadata should include ignored path names when the agent explicitly emits them. The default should be no.
- How long the old `workspace.repos` and `project_resource(github_repo)` compatibility window should last.

## Acceptance Criteria for the Design

- A user can create a coding Project from scratch without selecting Git or a local folder.
- The selected lead agent runtime creates the managed repository directory.
- The resulting Project and Chat reference the same repository identity.
- The repository can later be initialized as local Git.
- The repository can later be published to remote Git.
- Other users and agents discover the remote URL through repository events, activity, and task claim payloads.
- Local paths remain binding-private.
- Local output metadata is visible without exposing contents.
- Existing remote Git workflows keep working during migration.
