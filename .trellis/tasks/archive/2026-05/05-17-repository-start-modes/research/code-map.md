# Code Map for Repository Start Modes

## Existing Repository Storage

- `server/migrations/014_workspace_repos.up.sql` adds `workspace.repos` JSONB.
- `server/migrations/065_project_resources.up.sql` adds `project_resource`; current repo resources are `resource_type = "github_repo"`.
- `server/migrations/034_projects.up.sql` defines Project as a planning/work management container.

## Current Task Claim Flow

- `server/internal/handler/daemon.go` assembles task payloads for daemon claim.
- Project `github_repo` resources override workspace repositories in task context.
- Without project resources, task claim falls back to workspace repositories.
- Chat tasks currently do not have a durable repository identity.

## Current Daemon Checkout Flow

- `server/internal/daemon/repocache/cache.go` manages bare clone cache and per-task worktree checkouts.
- `server/internal/daemon/execenv/execenv.go` creates per-task working environments under the daemon workspaces root.
- `server/internal/daemon/health.go` exposes local daemon repo checkout behavior.
- `server/cmd/multica/cmd_repo.go` implements `multica repo checkout <url>`.

## Current Frontend Surfaces

- `packages/core/types/workspace.ts` defines workspace repositories as URL objects.
- `packages/core/types/project.ts` defines `ProjectResourceType = "github_repo"`.
- `packages/views/settings/components/repositories-tab.tsx` manages workspace-level repositories.
- `packages/views/projects/components/project-resources-section.tsx` manages project resources.
- `packages/views/modals/create-project.tsx` supports repository attachment during project creation.
- `packages/core/chat/store.ts` and `packages/views/chat/components/chat-window.tsx` carry chat session state without repository identity.

## Implementation Dependency Order

1. Repository schema and compatibility reads.
2. Project/chat repository references.
3. Task claim payload shape.
4. Daemon binding/operation execution.
5. Workspace Settings and work-surface UI.
6. Output metadata ingestion/rendering.
