# Implement Repository Start Modes

## Goal

Let Multica users start coding work without first binding a remote Git repository. The implementation should reuse the existing product concept **Repository** as a first-class workspace resource that can start from remote Git, a user-selected local directory, or a lead-agent-created managed directory, then optionally move to local Git and remote Git.

## Source Documents

- `CONTEXT.md` defines the shared product language: Workspace, Repository, Repository Binding, Project, Lead Agent, and Output Metadata.
- `docs/design-repository-start-modes.md` is the full design for data model, UI entry points, daemon execution, output metadata, discovery, API shape, and migration.
- `docs/adr/0001-repositories-as-workspace-code-working-targets.md` records the ADR to reuse Repository instead of adding Codebase.
- `/Users/troy.huang/workspace/RingCentral/AI/ai-desk/docs/design-local-git-binding.md` informed the source-state model and local-machine binding privacy constraints.

## What We Already Know

- Existing workspace repositories are stored as `workspace.repos` JSONB.
- Existing project-level repository links are `project_resource` rows with `resource_type = "github_repo"`.
- Task claim currently lifts project GitHub resources into task repo payloads, otherwise falling back to workspace repos.
- The daemon checkout path is remote Git oriented: bare clone cache plus per-task worktree checkout through `multica repo checkout`.
- Chat sessions persist conversation state but do not yet carry repository identity.
- Repository management should live under Workspace Settings, not as a new top-level workspace page.
- Project remains a planning container. Project and chat reference repositories by id; they do not own repository paths or copy URLs.
- Local paths are binding-private. Other workspace members can see machine label and availability, not absolute paths by default.
- Local and agent-managed outputs may expose cloud-visible output metadata from an explicit manifest, but not file contents/diffs/logs/screenshots by default.

## Requirements

### Repository Model

- Add first-class `repository` identity scoped to workspace.
- Support `source_state`: `remote_git`, `local_dir`, `agent_managed`, `local_git`.
- Add `repository_binding` for machine/runtime-specific local paths and daemon-managed directories.
- Add `repository_operation` for async local filesystem, Git init, publish, and binding refresh operations.
- Add project-to-repository references through `project_repository`.
- Add a default repository reference for chat sessions.
- Preserve compatibility with existing `workspace.repos` and `project_resource(github_repo)` during migration.

### Start Modes

- Remote Git remains the existing scalable collaboration path.
- Local Directory creates a repository plus a local binding authorized for a specific daemon/machine/user.
- Agent-managed start creates a repository intent on the server and a `create_binding` operation for the selected lead agent runtime/daemon.
- The daemon creates agent-managed directories under its Multica workspaces root.

### Source Transitions

- Allow one-way transitions:

  ```text
  local_dir | agent_managed -> local_git -> remote_git
  ```

- `init_git` must not run `git add .` by default.
- For user-selected local directories, `init_git` should stop after safe git initialization and sanitized status reporting.
- For empty agent-managed directories, an initial commit may include only bootstrap files the daemon created itself.
- `publish_remote` updates the canonical repository row. Project settings keep only repository references.
- Other users and agents learn about the remote through repository reads, realtime event, activity/inbox, and future task claim payloads.

### Task Dispatch and Daemon Behavior

- Task payloads should include repository semantics, not only raw URLs.
- Server resolves project/chat/workspace repository references before task claim.
- Daemon resolves binding eligibility and primary cwd.
- Primary repository determines task cwd.
- Remote Git uses existing checkout/cache behavior.
- Local/agent-managed direct bindings need mutation locks to avoid concurrent writes to the same directory.

### UI

- Workspace Settings -> Repositories becomes the full management surface.
- Chat, Project, and build/quick-create surfaces expose lightweight repository chips/selectors.
- Code-oriented Project creation automatically associates a primary repository.
- Planning-only Projects do not require repositories.
- Local-only output metadata appears in cloud UI without exposing contents.

## MVP Scope

1. Introduce first-class repository read/write model and compatibility reads.
2. Implement remote Git migration path from `workspace.repos` and `project_resource(github_repo)`.
3. Add Project and Chat repository references.
4. Add daemon-supported agent-managed directory creation.
5. Add local directory binding support for desktop/local daemon paths.
6. Add `init_git` and `publish_remote` operation flow.
7. Update Workspace Settings, Project creation/detail, and Chat surfaces.
8. Add explicit output metadata manifest ingestion and display.

## Out of Scope

- A top-level Repositories page in primary navigation.
- Renaming Repository to Codebase.
- Automatic upload of local file contents, diffs, logs, screenshots, stack traces, or absolute paths.
- Long-term dual writes to old and new repository storage.
- General-purpose multi-machine local directory sync.
- Full local Git worktree branch management for local-only repositories.

## Implementation Slices

### Slice 1: Data Model and API

- Add migrations for `repository`, `repository_binding`, `project_repository`, `repository_operation`, and output metadata storage.
- Add sqlc queries/types.
- Add repository handlers and membership checks.
- Add sanitized binding path filtering.
- Add compatibility read model for old workspace/project repo storage.

### Slice 2: Task Claim and Daemon Contract

- Update server task claim assembly to emit repository identities and source states.
- Update daemon payload parsing.
- Resolve primary repository cwd in execenv.
- Preserve remote Git checkout behavior.
- Add binding eligibility checks.

### Slice 3: Repository Operations

- Implement repository operation claim/start/complete/fail lifecycle.
- Add daemon create-binding for agent-managed directories.
- Add local directory verification.
- Add conservative `init_git`.
- Add `publish_remote` remote-add/push flow and repository published event.

### Slice 4: Frontend/Core State

- Add `@multica/core` repository types, queries, and mutations.
- Rework Workspace Settings -> Repositories around first-class repositories.
- Add project repository selector and build-oriented creation path.
- Add chat default repository selector/chip.

### Slice 5: Output Metadata

- Define explicit daemon/agent output manifest format.
- Ingest task output metadata on the server.
- Render local-only output metadata with actions such as Open on runtime, Publish/attach, and Copy relative path.

### Slice 6: Migration and Cleanup

- Backfill existing `workspace.repos` and `project_resource(github_repo)`.
- Stop new writes to old repository storage.
- Keep compatibility responses for one release window.
- Remove old write paths after compatibility window.

## Acceptance Criteria

- [ ] A user can create a code-oriented Project without selecting Git or a local folder.
- [ ] The selected lead agent runtime creates an agent-managed repository directory.
- [ ] The resulting Project and Chat reference the same repository identity.
- [ ] A user can bind a local directory as a repository without exposing the absolute path to other workspace members.
- [ ] A local or agent-managed repository can be initialized as local Git without automatically staging all files.
- [ ] A local Git repository can be published to remote Git and update the canonical repository row.
- [ ] Other users and agents discover the published remote through repository data, realtime/activity, and task claim payloads.
- [ ] Existing remote Git workflows continue to work during migration.
- [ ] Local output metadata is visible in cloud UI without exposing file contents or absolute paths.

## Verification Plan

- Server unit tests for repository CRUD authorization, binding path filtering, migration/backfill, project/chat references, and operation lifecycle.
- Daemon tests for agent-managed directory creation, local binding verification, init-git no-auto-add behavior, publish failure when no commit exists, and remote publish success path with a test remote.
- Integration tests for task claim repository payloads across workspace fallback, project repository, and chat default repository.
- Frontend tests for Workspace Settings repository management, Project creation repository association, Chat repository selector, and local-only output metadata rendering.
- Manual QA for remote Git regression, local directory privacy, agent-managed from-scratch start, publish discoverability, and output metadata actions.

## Technical Notes

- Existing backend touchpoints include:
  - `server/migrations/014_workspace_repos.up.sql`
  - `server/migrations/034_projects.up.sql`
  - `server/migrations/065_project_resources.up.sql`
  - `server/internal/handler/project.go`
  - `server/internal/handler/project_resource.go`
  - `server/internal/handler/daemon.go`
  - `server/internal/daemon/repocache/cache.go`
  - `server/internal/daemon/execenv/execenv.go`
  - `server/internal/daemon/health.go`
  - `server/cmd/multica/cmd_repo.go`
- Existing frontend touchpoints include:
  - `packages/core/types/workspace.ts`
  - `packages/core/types/project.ts`
  - `packages/views/settings/components/repositories-tab.tsx`
  - `packages/views/projects/components/project-resources-section.tsx`
  - `packages/views/modals/create-project.tsx`
  - `packages/core/chat/store.ts`
  - `packages/views/chat/components/chat-window.tsx`
- Computer Use could not inspect the local Codex app directly because the tool blocks `com.openai.codex`; Codex-style Project/Chat guidance is inferred from product behavior and the current Multica structure.

## Next Development Action

Start with Slice 1. It creates the canonical repository identity and compatibility read model that every later slice depends on. Do not implement daemon or UI flows until the first-class repository model and old-storage migration strategy are in place.
