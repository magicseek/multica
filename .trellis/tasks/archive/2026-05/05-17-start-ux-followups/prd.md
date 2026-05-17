# Improve Project And Issue Start UX

## Goal

Make the new repository start-mode workflow usable for real first-run product
creation: users should be able to choose local folders without typing paths,
bind repositories and workflows during project creation, choose workflows for
agent-created issues, and get visible issue/project records even when the
first agent attempt fails.

## What I Already Know

* User tested an `Agent Force` workspace and found several onboarding/start UX
  gaps after the repository-start-modes and workflow features landed.
* Workspace Settings can create `local_dir` repositories today, but the UI asks
  users to type a local path manually.
* New Project creation currently exposes Git repository selection but does not
  expose local folder selection in the modal.
* Project-level workflow binding exists, but New Project creation does not yet
  let users choose the workflow beside the repository.
* New Issue manual mode can choose a workflow, but agent mode cannot.
* Creating an issue without a project can be blocked when the agent has no
  usable repository/project context.
* A first issue in the Vokly project failed with "agent finished without
  creating an issue", and the failed attempt was not visible in the project.
* The existing New Project modal already auto-creates an `agent_managed`
  repository when the lead is an agent and the user selected no repository.
  This is useful precedent, but it is explicit project creation rather than a
  hidden side effect during issue creation.
* Manual issue creation already has `workflow_override_definition_id` end to
  end; agent quick-create does not carry that field today.
* Agent quick-create currently creates only an async task first. The issue is
  created later by the agent/CLI with `origin_type=quick_create`. If the agent
  completes without creating that issue, the server writes an inbox failure but
  has no project issue to show.

## Assumptions (Temporary)

* The MVP should improve the existing repository/workflow model rather than
  introduce a separate "workspace wizard".
* Desktop should use native folder picking when available; web should degrade
  gracefully because browsers cannot reveal arbitrary local paths without user
  mediated directory picker APIs.
* Project workflow selection should use workflow definition IDs as API values
  and display workflow names in UI labels.
* Issue/project visibility matters more than perfectly predicting whether an
  agent will succeed; failure should leave a visible object that can be retried
  or inspected.

## Open Questions

* None blocking for MVP. Remaining implementation details should be derived from
  existing code patterns and tests.

## Requirements (Evolving)

* Workspace Settings local directory binding should offer a folder picker
  instead of requiring manual path typing.
* Web should also expose a folder picker control for local directory binding;
  the UX should not require users to type paths as the primary path.
* Web folder picking must be daemon-assisted: the browser UI can start the
  "Choose folder" action, but the selected local runtime/daemon must resolve or
  confirm the daemon-readable absolute path before a `local_dir` binding is
  created.
* New Project modal should allow selecting or creating a local directory
  repository during creation.
* New Project local-dir repository creation should mirror the existing ad-hoc
  remote Git URL behavior: collect the selected folder/runtime in modal state,
  create the reusable workspace Repository on final submit, then attach it to
  the new project.
* New Project modal should allow selecting an assignment workflow near the
  repository selector.
* New Issue agent mode should expose workflow selection with behavior matching
  manual mode where applicable.
* Issue workflow selection should preserve the existing model: `null`
  `workflow_override_definition_id` means "use the project/default workflow";
  choosing a workflow writes an explicit issue override.
* When an issue has a selected project with a project workflow, the issue
  workflow pill should show the concrete inherited workflow name, e.g.
  "Project workflow: <name>", instead of only "Project default".
* No-project agent issue creation needs a deliberate fallback design so users do
  not hit a confusing blocked state.
* First-agent-attempt failures should leave visible project context so users can
  understand and recover from the failure.
* Agent issue creation should use the normal issue-assignment execution path:
  the server creates a visible issue first, records the selected project and
  workflow override, then dispatches the selected agent/squad against that
  issue. The agent should not be responsible for creating the first durable
  issue record.
* The initial seed issue title should default to the prompt's first line. When
  the assigned agent starts execution, its first step should summarize the
  request and update the issue title to a better task title.
* On agent execution failure, the visible seed issue should be marked `blocked`
  with the failure detail/comment/activity attached, instead of only writing an
  inbox item.
* Avoid silently creating an unnamed project. A project is durable workspace
  organization, so hidden unnamed projects are likely to create clutter,
  confusion, and poor recovery paths.
* Agent mode with no selected project must not silently dispatch. It should
  require an explicit project decision, with a one-click "Create project from
  title" affordance that creates a named project and, when no repository is
  selected, an `agent_managed` repository.
* If Agent mode's one-click project creation is triggered while the selected
  actor is a squad, the new project should use the squad leader agent as
  `lead_type=agent` / `lead_id=<leader_id>` and for the agent-managed repository
  lead. The newly created issue should still be assigned to the squad.

## Acceptance Criteria (Evolving)

* [ ] Desktop Workspace Settings local directory row can populate its path from
  a native folder picker.
* [ ] Web Workspace Settings local directory row can start a folder-pick flow
  through the selected local runtime/daemon and receives a daemon-readable
  absolute path before saving.
* [ ] New Project creation can bind a selected local directory repository as
  the primary repository.
* [ ] New Project local-dir creation does not leave orphan repositories when the
  modal is cancelled before submit.
* [ ] New Project creation can set `workflow_definition_id` at creation time.
* [ ] New Issue agent mode can set or override the assignment workflow.
* [ ] New Issue manual and agent modes both make it clear whether the issue is
  using the project workflow or an explicit override.
* [ ] Workflow pills show the concrete inherited project workflow name when a
  project workflow exists.
* [ ] Creating an agent issue without project/repository context has an
  intentional, tested behavior instead of silently blocking.
* [ ] Agent mode without a project prevents send and offers a visible one-click
  create-project path named from the issue title/prompt.
* [ ] Squad-selected Agent mode can one-click create a project using the squad
  leader as project/repository lead while keeping the issue assigned to the
  squad.
* [ ] A failed first project-generation attempt leaves a visible issue or
  project record with blocked/error status.
* [ ] Agent-mode issue creation produces a visible issue immediately, before
  agent execution begins.
* [ ] Agent-mode seed issue title defaults to the prompt first line.
* [ ] Agent execution protocol instructs the assigned agent to refine/update
  the issue title at the start of execution.
* [ ] Agent execution failures for newly created project-scoped issues are
  visible inside the project, not only in Inbox.

## Definition Of Done

* Tests added/updated for core request payloads and views behavior.
* Desktop path picking manually verified.
* `pnpm lint`, `pnpm typecheck`, relevant unit tests, Go tests, and E2E checks
  pass for touched layers.
* Spec updates added if repository/workflow creation contracts change.

## Out Of Scope (Explicit)

* Uploading local repository contents to cloud.
* Implementing remote Git hosting publish success against real GitHub during
  this follow-up.
* Replacing the existing project/repository/workflow data model.

## Technical Notes

* Active worktree: `/Users/troy.huang/workspace/AI/multica-repository-start-modes`
* Current branch: `trellis/repository-start-modes`
* Local services remain running for manual validation:
  backend `18889`, web `13809`, desktop renderer `15173`, Electron debug `9223`.
* `packages/views/settings/components/repositories-tab.tsx` creates local-dir
  repositories from a manual `localPath` draft and inline `repository_binding`
  payload. No folder picker integration exists yet.
* `apps/desktop/src/preload/index.ts` exposes `desktopAPI` via `ipcRenderer`
  but has no `selectDirectory`/folder picker method. `apps/desktop/src/main`
  has existing IPC patterns for `shell:openExternal`, `file:download-url`, and
  `window:setImmersive`; a folder picker should follow that main/preload
  contract with Electron `dialog.showOpenDialog({ properties: ["openDirectory"] })`.
* For web, browser APIs can present a directory picker but do not directly solve
  `local_dir` binding as currently modeled. `showDirectoryPicker()` returns a
  `FileSystemDirectoryHandle`, and `webkitdirectory` exposes selected files with
  paths relative to the selected directory. The repository binding API requires
  `local_path`, an absolute path the selected local daemon/runtime can access.
  Therefore web folder picking likely needs a local-runtime/daemon bridge, or it
  must fall back to manual path entry when no bridge is available.
* `packages/views/modals/create-project.tsx` currently supports existing
  repository selection plus ad-hoc remote Git URL creation. It passes no
  `workflow_definition_id` to `createProject`, even though the core/server
  create-project request supports it.
* Local-dir creation in New Project should follow this existing submit-time
  creation pattern to avoid orphan repositories on cancelled modals.
* `packages/views/modals/create-project.tsx` auto-creates an `agent_managed`
  repository only after the user explicitly creates an agent-led project with no
  selected repository.
* `packages/views/modals/create-issue.tsx` manual mode stores
  `workflowOverrideDefinitionId`, renders `WorkflowPicker`, and sends
  `workflow_override_definition_id` to `createIssue`.
* `packages/views/workflows/components/workflow-picker.tsx` already treats
  `workflowId=null` as `picker.project_default`; this is the correct data
  contract for issue overrides, but the label may be too generic if the user
  expects to see the concrete project workflow name.
* `packages/views/modals/quick-create-issue.tsx` agent mode only renders
  `ProjectPicker` in its property toolbar and sends only `agent_id`/`squad_id`,
  `prompt`, and `project_id` to `/api/issues/quick-create`.
* `server/internal/handler/issue.go` quick-create request validates optional
  `project_id` and enqueues a quick-create task; it has no workflow override.
* `server/internal/daemon/prompt.go` tells the quick-create agent there is no
  existing issue and to run exactly one `multica issue create --output json`.
  That model makes failure invisible in a project because the durable issue is
  produced after the agent succeeds.
* `server/internal/service/task.go` emits "agent finished without creating an
  issue" when quick-create completion cannot find an issue with
  `origin_type=quick_create` and `origin_id=<task_id>`.

## Current Recommendation

* Implement items 1-4 as direct UX fixes.
* For item 6, replace the project-scoped Agent issue flow with the normal issue
  creation + assignment path: server creates the visible issue immediately,
  persists project/workflow/assignee fields, then dispatches the task. Failures
  mark that issue `blocked`.
* Title strategy: create the visible seed issue using the prompt's first line as
  the initial title, then rely on the agent execution protocol to summarize and
  update the title when execution starts.
* Workflow strategy: add the same workflow override capability to Agent mode,
  but keep `null` as "inherit project workflow" so project-level workflow
  binding remains the default path.
* Confirmed: workflow picker UI should show the concrete project workflow name
  when available, while preserving `null` as inherit/default in the payload.
* For item 5, use the explicit-project decision path: no-project Agent mode
  cannot silently dispatch; it prompts the user to select a project or create a
  named project from the current title/prompt. The create path can reuse the
  New Project agent-managed repository fallback.
* Confirmed: if the selected actor is a squad during one-click project creation,
  use the squad leader agent for the project lead and agent-managed repository,
  but keep the issue assigned to the squad.
* For web folder picker, honor the user's UX preference but do not treat a pure
  browser directory handle as a usable repository binding unless the daemon can
  resolve or confirm a concrete local path.
* Confirmed: Web should still expose a folder picker control, implemented via a
  local runtime/daemon-assisted path resolution flow rather than raw browser
  directory handles.
