# Multica Domain Context

Multica coordinates human and AI work in a shared workspace. This context records product language that should stay stable across planning, UI, and architecture documents.

## Language

**Workspace**:
A team boundary that owns issues, projects, agents, skills, chats, and repositories.

**Repository**:
A code working target that an agent can read or write, whether it is backed by a remote Git URL, a user-selected local directory, or an agent-created local work directory.
_Avoid_: Codebase

**Repository Binding**:
A machine- or runtime-scoped authorization that maps a repository to an actual local path or daemon-managed work directory.
_Avoid_: Global path, shared local path

**Project**:
A planning container for related issues, similar to a Linear project or Jira epic.
_Avoid_: Repository, Codebase

**Chat Session**:
A private, creator-owned multi-turn conversation between one user and one agent.
_Avoid_: Team chat, shared project conversation

**Project-Associated Chat Session**:
A private **Chat Session** grouped under a **Project** for organization without changing who can see it.
_Avoid_: Project chat

**Chat-Originated Issue**:
An **Issue** created as an output of a **Chat Session**.
_Avoid_: Project issue by implication

**Chat Issue Proposal**:
A set of candidate issues proposed inside a **Chat Session** and awaiting user approval.
_Avoid_: Auto-created issues

**Chat Issue Proposal Item**:
One candidate issue inside a **Chat Issue Proposal**.
_Avoid_: Draft issue

**Proposal Artifact**:
A structured output from a chat agent that records a **Chat Issue Proposal** separately from prose.
_Avoid_: Markdown issue list

**Lead Agent**:
The agent selected to bootstrap, coordinate, or own work when a repository starts without an existing Git remote or user-selected local directory.

**Output Metadata**:
Privacy-minimized facts about local or agent-managed task outputs, such as file names, relative paths, sizes, MIME types, and creation times, excluding file contents, diffs, logs, stack traces, screenshots, and absolute local paths.

## Relationships

- A **Workspace** contains zero or more **Repositories**
- A **Workspace** contains zero or more **Projects**
- A **Workspace** contains zero or more **Chat Sessions**
- A **Project** may reference one or more **Repositories**
- A **Project** may group zero or more **Project-Associated Chat Sessions**
- A **Project-Associated Chat Session** remains visible only to its creator
- A **Project-Associated Chat Session** is discovered through **Projects** navigation and the owning **Project** detail page
- The **Projects** sidebar tree may show a small recent subset of **Project-Associated Chat Sessions** under each **Project**
- The **Projects** sidebar tree lists all active **Projects**, even when a **Project** has no recent **Project-Associated Chat Sessions**
- An expanded **Project** sidebar row shows at most three recent **Project-Associated Chat Sessions**
- A **Project-Associated Chat Session** may be started from the Project navigation row action or from the owning **Project** detail page
- A **Project-Associated Chat Session** remains as private history if its associated **Project** is archived or deleted
- A **Project-Associated Chat Session** stores a creation-time **Project** snapshot so deleted Projects can still be represented in chat history
- A loose **Chat Session** is discovered through the workspace **Chats** navigation entry
- A loose **Chat Session** may be started from the top-level New Chat action
- A **Chat Session** is opened as a page-level workspace route rather than a global floating window
- A **Chat Session** title may start from the first user message as a fallback
- A **Chat Session** title may be updated after the first agent execution using an agent-provided summary title
- A user-edited **Chat Session** title is not overwritten by an agent-provided summary title
- Archiving or deleting a **Chat Session** does not delete its **Chat-Originated Issues** or **Output Metadata**
- The workspace sidebar shows only active **Chat Sessions** updated in the last five days as a quick-access tree
- The workspace sidebar may label loose **Chat Session** quick access as **Recents** while the full product concept remains **Chats**
- The **Recents** sidebar section lists loose **Chat Sessions** only, not **Project-Associated Chat Sessions**
- **Recents** initially shows active loose **Chat Sessions** updated in the last five days, then loads older active loose **Chat Sessions** in pages
- **Recents** groups loose **Chat Sessions** by recency labels such as today, yesterday, the last five days, and older
- **Recents** loads older loose **Chat Sessions** inline in the sidebar rather than navigating to the full **Chats** archive
- Primary workspace navigation groups **Inbox**, **My Issues**, **Issues**, **Projects**, **Agents**, **Squads**, **Autopilot**, **Usage**, and **Recents** without a separate **Workspace** heading
- **Runtimes**, **Workflows**, and **Skills** are **Configure** surfaces inside **Settings**, not primary workspace navigation entries
- **Settings** groups its middle navigation as **Account**, **Configure**, and **Workspace**
- The **Configure** settings group contains **Runtimes**, **Workflows**, and **Skills**
- The workspace sidebar exposes **Settings** as one fixed bottom entry; settings subsections are discovered inside the **Settings** page
- The workspace sidebar keeps **Settings** fixed at the bottom while the main navigation and expandable **Projects** and **Recents** lists scroll independently above it
- A **Project-Associated Chat Session** provides the default **Project** for new **Chat-Originated Issues**
- A **Chat-Originated Issue** keeps an explicit link to its source **Chat Session**
- A **Chat-Originated Issue** is created after explicit user approval from a **Chat Issue Proposal** or issue creation flow
- **Chat-Originated Issues** created from a **Chat Issue Proposal** default to backlog status
- A **Chat Issue Proposal** is persisted before approval
- A **Chat Issue Proposal** contains one or more **Chat Issue Proposal Items**
- A **Chat Session** may contain multiple **Chat Issue Proposals**
- A **Proposal Artifact** is the source of truth for a **Chat Issue Proposal**
- A **Proposal Artifact** uses a minimal versioned schema with `version` and `proposals`
- A **Chat Issue Proposal Item** may include issue draft fields such as `title`, `description`, optional `priority`, optional `labels`, and optional `assignee_id`
- A **Chat Issue Proposal** appears inline in the **Chat Session** conversation after the proposing agent message
- The **Chat Session** Issues view shows the same **Chat Issue Proposal** objects for review and follow-up management
- The **Chat Session** Issues view may present pending **Chat Issue Proposal Items** in a **Proposed** review lane before the normal **Backlog** issue lane
- The **Chat Session** Issues view orders its Kanban lanes as **Proposed**, **Backlog**, and then the normal issue workflow lanes
- A **Chat Issue Proposal Item** in the **Proposed** lane uses proposal review controls rather than normal issue drag-and-drop
- Approving a **Chat Issue Proposal Item** removes it from **Proposed** and creates a real **Issue** in **Backlog**
- Batch approval from the **Chat Session** Issues view defaults to all selected pending **Chat Issue Proposal Items** in that **Chat Session**, while preserving proposal grouping for context
- A **Chat Session** has `Chat`, `Issues`, and `Outputs` page tabs
- The **Chat Session** `Issues` tab count reflects created **Chat-Originated Issues**, not pending **Chat Issue Proposal Items**
- The **Chat Session** `Outputs` tab count reflects available **Output Metadata** records
- A **Chat Issue Proposal Item** may become zero or one **Chat-Originated Issue**
- A member may edit **Chat Issue Proposal Items** before approving issue creation
- A member may not edit the target **Project** of a **Chat Issue Proposal Item** before approval
- A **Chat Issue Proposal Item** keeps an approval-time snapshot of the content used to create its **Chat-Originated Issue**
- A partially approved **Chat Issue Proposal** records unapproved selected-out items as skipped
- A skipped **Chat Issue Proposal Item** can be restored to pending before later approval
- The approving member is the creator of a **Chat-Originated Issue**
- The proposing agent remains provenance for a **Chat Issue Proposal**, not the issue creator
- A **Chat Issue Proposal** records detailed provenance such as source chat message, source task, and proposing agent
- A **Chat-Originated Issue** keeps a lightweight origin link to the source **Chat Session**
- Approving multiple **Chat Issue Proposal Items** creates issues in one transaction; validation failure creates no partial batch
- A created **Chat Session** does not move between **Projects**
- **Chat-Originated Issues** follow normal workspace and project visibility
- **Output Metadata** from a **Chat Session** inherits the **Chat Session** privacy boundary until explicitly attached to a shared issue, project, or published artifact
- A **Chat Session** output view contains **Output Metadata**, not **Chat Issue Proposals**
- A **Chat Session** output view aggregates **Output Metadata** from the session's own chat tasks and from agent tasks on issues created from that session
- A chat may select or reference a **Repository**, but does not own it
- A **Repository** is a first-class workspace entity, not only JSON embedded in workspace settings
- A **Repository** may have zero or more **Repository Bindings**
- A **Lead Agent** may initialize a **Repository** when no user-selected local directory or remote Git URL exists
- A **Repository Binding** may store a local path, but the full path is visible only to the binding owner and the corresponding runtime or daemon by default
- When no Git remote or local directory is selected, the **Lead Agent** initializes a **Repository Binding** inside its own runtime's Multica-managed work area
- A **Repository** may move one way from local or agent-managed work to local Git, then to remote Git
- A from-scratch build request should create or select a **Repository** early instead of remaining a long-lived no-repository chat draft
- Local or agent-managed task outputs may expose **Output Metadata** to the cloud while keeping output contents local until explicitly published

## Example Dialogue

> **Dev:** "Should we create a new codebase object for from-scratch work?"
> **Domain expert:** "No. Reuse **Repository**. A repository may start as a local draft before it has any remote Git URL."

## Flagged Ambiguities

- "Project" was used to mean both a planning container and a codebase. Resolved: **Project** remains the issue-planning container; **Repository** is the code working target.
- "Project chat" can imply a shared team conversation. Resolved: use **Project-Associated Chat Session** for private chats grouped under a project.
- The primary **Chats** navigation entry can imply every chat in the workspace. Resolved: **Chats** is the total entry for loose **Chat Sessions**; **Project-Associated Chat Sessions** are found through **Projects** navigation and Project detail.
- **Recents** is a sidebar quick-access label for recent loose **Chat Sessions**, not a replacement term for **Chat Session** or the full **Chats** archive.
- The five-day **Recents** window is the initial quick-access window only; pagination can continue beyond five days through older active loose **Chat Sessions**.
- **Recents** date grouping is a presentation aid and does not create a new **Chat Session** category.
- The old **Workspace** sidebar section heading is removed; primary workspace navigation remains flat until expandable **Projects** and **Recents** sections.
- Existing direct URLs for **Runtimes**, **Workflows**, and **Skills** remain addressable, but sidebar discovery moves under **Settings**.
- Older **Project-Associated Chat Sessions** are recovered through the owning **Project** detail `Chats` view, not through **Recents**.
- The owning **Project** detail `Chats` view remains the complete history surface for that **Project**'s **Project-Associated Chat Sessions**.
- When a **Project** has more associated chat history than the sidebar subset, the sidebar links to the owning **Project** detail `Chats` view for the full list.
- Project visibility in the **Projects** sidebar tree is based on active **Project** status, not on recent chat activity.
- Starting a **Project-Associated Chat Session** from different surfaces should not create different flows. Resolved: Project navigation row actions and Project detail `Chats` use the same new-chat route with the selected Project context, and the actual session is created when the first message is sent.
- Starting a loose **Chat Session** uses the top-level New Chat action and a new-chat route without Project context. The first send requires an explicit agent selection.
- The old global Chat floating action button and floating window are replaced by page-level Chat routes as the primary workspace chat experience.
- **Chat Session** titles should not repeat Project names because navigation already provides that hierarchy. The initial title may fall back to the first user message, then the first agent execution may provide a more accurate summary title.
- Title generation needs an explicit source or user-edited marker so agent-generated summary titles do not overwrite a title the user already changed.
- Agent-provided **Chat Session** summary titles travel through the same task completion metadata handoff as other structured chat outputs, and the backend applies them only when the user has not edited the title.
- A chat task may produce multiple structured outputs, such as summary title metadata, **Chat Issue Proposals**, and **Output Metadata**
- The daemon may read multiple fixed local manifests for structured chat outputs, but uploads them together in one task completion payload for backend processing
- The sidebar's five-day chat window is a quick-access filter only. Complete loose chat history remains available from **Chats**, and complete project-associated chat history remains available from the owning **Project** detail `Chats` view.
- If a **Project** is archived or deleted, its associated **Chat Sessions** are not shown under the active **Projects** sidebar tree. The **Chat Session** page shows an archived or deleted Project context chip, and already created issues keep their normal issue state and provenance.
- If the underlying **Project** row is deleted, a **Project-Associated Chat Session** does not become a loose **Chat Session**. It retains its creation-time Project snapshot for historical display.
- Data model should not infer loose chat status from `project_id IS NULL` alone. A deleted Project may also leave `project_id` null, so the session needs an explicit project association marker plus a creation-time Project snapshot.
- If a **Chat Session** is archived or deleted, its created issues and output metadata are retained. The session's issue provenance remains readable even if the conversation is no longer active.
- A **Project-Associated Chat Session** records the project context chosen at session creation, not a mutable label. **Chat-Originated Issues** keep their own source link.
- Issues merely sharing the same **Project** as a **Chat Session** are not **Chat-Originated Issues** unless the explicit source link is present.
- A **Chat Issue Proposal** is not an **Issue** until a user approves creation.
- A **Chat Issue Proposal Item** is not an **Issue** and should not appear on issue boards before approval.
- **Proposed** is a review lane for pending **Chat Issue Proposal Items** inside a **Chat Session** Issues view, not an **Issue** status.
- A Markdown checklist in a chat reply is not a **Chat Issue Proposal** unless it is backed by a **Proposal Artifact**.
- A **Proposal Artifact** does not let the proposing agent choose issue status. Created issues use backlog status from backend rules.
- **Chat Issue Proposal Item** Project assignment is derived from the source **Chat Session** context. Project reassignment happens later through normal issue editing, not inside proposal approval.
- Inline proposal controls in the chat transcript and proposal controls in the **Chat Session** Issues view operate on the same persisted **Chat Issue Proposal**, not duplicated draft state.
- Pending **Chat Issue Proposals** are presented inside the **Chat Session** `Issues` tab without increasing the created issue count.
- **Chat Issue Proposals** belong with issue creation workflow, not the **Output Metadata** view.
- The **Chat Session** output view is scoped to outputs produced by the chat and by issues that were explicitly created from that chat; unrelated issues in the same Project are excluded.
- The member approval action, not the agent proposal, is the authorization boundary for creating **Chat-Originated Issues**.
- Detailed proposal provenance belongs on **Chat Issue Proposal** records. The created **Issue** keeps only the lightweight source **Chat Session** origin link and can reach proposal/item details through the proposal tables.
- Batch approval for **Chat Issue Proposal Items** is atomic. If any selected item fails validation, no issues are created and the user edits the proposal before retrying.
- **Chat Issue Proposal Items** keep approval-time snapshots so later issue edits do not erase what was approved from the proposal.
- Partial approval sets the **Chat Issue Proposal** to a partially accepted state, marks unapproved items as skipped, and allows skipped items to be restored later from the **Chat Session** `Issues` tab.
- The **Chat Session** `Issues` tab groups by **Chat Issue Proposal**, shows pending proposals first, and then orders proposal groups by creation time.
- **Repository** ownership is workspace-level. **Project** and chat flows reference repositories but do not own them.
- **Repository** is a first-class entity. Workspace settings may expose repositories, but they are not only an embedded JSON list.
- Repository management belongs under **Workspace** settings rather than the primary workspace navigation.
- Work surfaces such as chat, project detail, and quick-create show lightweight repository chips or selectors even though full repository management lives in workspace settings.
- Project-level repository settings store references to **Repositories**, not copied remote URLs.
- Local paths are binding-private. Other workspace members may see binding availability and machine labels, but not absolute local paths by default.
- No-source repository startup is runtime-owned: the server records intent, and the selected lead agent's runtime creates the actual local working directory.
- Repository source transitions are one-way: `local_dir` or `agent_managed` may become `local_git`, and `local_git` may become `remote_git`.
- Build-from-scratch flows should establish a **Repository** before sustained agent work so chats, issues, and projects can share the same repository identity.
- Code-oriented **Projects** should automatically reference a primary **Repository**; ordinary planning **Projects** may have no repository.
- Cloud-visible **Output Metadata** must not include contents, diffs, detailed logs, screenshots, stack traces, or absolute local paths by default.

## Agent Workflows

This context defines how Multica describes reusable agent execution behavior across workspaces, projects, issues, and task runs.

## Language

**Workflow Definition**:
A workspace-level editable template that describes how an agent should execute a class of work.
_Avoid_: hardcoded execution protocol, agent prompt blob

**System-Seeded Workflow Definition**:
A read-only workflow definition provided by Multica as a built-in starting point inside a workspace.
_Avoid_: hardcoded built-in template, magic slug

**Workflow Fork**:
An editable user-owned copy of a workflow definition.
_Avoid_: editing the system seed directly

**Workflow Revision**:
A versioned schema for a workflow definition that can be drafted, published, deprecated, and snapshotted.
_Avoid_: silent in-place template mutation

**Project Workflow Binding**:
The default workflow selected for issues that belong to a project.
_Avoid_: agent workflow, project prompt

**Issue Workflow Override**:
An explicit per-issue selection of a different workflow, usually to route a small bug, research task, or one-off task through a lighter process.
_Avoid_: skip workflow, bypass workflow

**Workflow Snapshot**:
The immutable workflow content captured for a specific agent task when the task is queued.
_Avoid_: live workflow reference during execution

**Workflow Source**:
The editable template body and metadata fields inside a workflow definition's canonical schema.
_Avoid_: hidden prompt, generated-only workflow

**Workflow Step Graph**:
The structured step model for workflows that need visible phases, dependencies, gates, or artifacts.
_Avoid_: separate workflow system, visual-only diagram

**Workflow Schema**:
The canonical structured representation of a workflow definition, including metadata, source fields, variables, steps, gates, and artifacts.
_Avoid_: secondary Markdown truth, UI-only schema

**Workflow Applicability**:
The trigger types a workflow revision is valid for, such as assignment, comment response, chat, or autopilot run.
_Avoid_: universal workflow by accident

**Workflow Capability**:
An agent-declared ability or constraint describing which workflow features it can execute safely.
_Avoid_: agent default workflow

**Workflow Capability Warning**:
A non-blocking warning that the selected agent may not fully support the workflow features selected for a task.
_Avoid_: hard capability gate in the first version

**Workflow Preview**:
A UI rendering of the resolved workflow content before it is saved or assigned to execution.
_Avoid_: raw-only editor, blind prompt injection

**Direct Task Workflow**:
A lightweight workflow definition for focused execution that still preserves minimum context, execution, verification, and reporting requirements.
_Avoid_: no workflow, quick skip

**Research Note Workflow**:
A lightweight workflow definition for investigation-only work that produces findings instead of code changes.
_Avoid_: research mode, ad hoc investigation

**Comment Response Workflow**:
A lightweight workflow definition for tasks triggered by mentioning an agent in an issue comment.
_Avoid_: silently continuing the full project workflow

## Relationships

- A **Workspace** owns zero or more **Workflow Definitions**.
- A **Workspace** may include **System-Seeded Workflow Definitions**.
- A **System-Seeded Workflow Definition** can be copied into a **Workflow Fork**.
- A **Workflow Definition** has exactly one canonical **Workflow Schema**.
- A **Workflow Definition** evolves through **Workflow Revisions**.
- A **Workflow Revision** may be draft, published, or deprecated.
- A **Workflow Schema** includes **Workflow Source** and may include a **Workflow Step Graph**.
- A **Workflow Schema** declares **Workflow Applicability**.
- A **Workflow Preview** renders from the **Workflow Schema** using Multica's current UI library and design system.
- A **Project** may have exactly one **Project Workflow Binding**.
- An **Issue** may have at most one **Issue Workflow Override**.
- **Project Workflow Bindings** and **Issue Workflow Overrides** can only select **Workflow Definitions** with a published **Workflow Revision** for the target trigger.
- Workflow selectors filter choices by **Workflow Applicability**.
- An **Issue Workflow Override** takes precedence over a **Project Workflow Binding**.
- A **Project Workflow Binding** takes precedence over the workspace default workflow.
- An issue assignment **Agent Task** resolves workflow selection and stores exactly one **Workflow Snapshot** when the task is queued.
- A comment-triggered **Agent Task** defaults to the **Comment Response Workflow** instead of inheriting the project's full workflow.
- A comment-triggered **Agent Task** uses the full issue workflow only when the triggering action explicitly requests it.
- An existing **Workflow Snapshot** is not changed by later edits or publishes.
- An **Agent** may declare **Workflow Capability**, but does not own a default workflow.
- A mismatch between **Workflow Capability** and selected workflow features produces a **Workflow Capability Warning**.
- A **Workflow Capability Warning** is recorded with the **Workflow Snapshot** but does not block first-version execution.
- Workflow selection remains owned by issue, project, and workspace resolution.
- **Project Workflow Bindings** and **Issue Workflow Overrides** reference workflow definition IDs, not hardcoded slugs; queue-time resolution snapshots the definition's current published revision.

## Example Dialogue

> **Dev:** "This project uses the full Trellis workflow, but this bug is a two-line fix. Should the assigned agent skip workflow?"
> **Domain expert:** "No. Select the Direct Task workflow as the issue override. It is lighter, but it still records context, verification, and final reporting."

## Flagged Ambiguities

- "workflow" was used to mean both a reusable template and a running task process. Resolved: use **Workflow Definition** for the editable template and **Workflow Snapshot** for the per-task execution copy.
- "skip workflow" suggests bypassing execution governance. Resolved: use **Issue Workflow Override** to choose a lighter **Workflow Definition** instead.
- Markdown source and step graph could drift if stored independently. Resolved: **Workflow Schema** is the only source of truth; Markdown is an editable field and rendered output, not a second persisted model.
- Built-in templates could remain hardcoded or become editable. Resolved: built-ins enter workspaces as read-only **System-Seeded Workflow Definitions**; user changes happen through **Workflow Forks**.
- Saving a workflow edit could silently affect task execution. Resolved: edits create draft **Workflow Revisions**; only publishing makes a revision available for new task resolution.
- Workflow resolution could happen when an agent claims work, but that would let delayed claims observe newer publishes than the user assigned. Resolved: issue assignment tasks snapshot workflow at queue time.
- Comment mentions could inherit the full project workflow, but that would make lightweight collaboration unexpectedly heavy. Resolved: comment-triggered tasks default to **Comment Response Workflow** and require explicit opt-in for full workflow execution.
- Workflows could accidentally be bound to incompatible triggers. Resolved: **Workflow Applicability** is declared in schema and used by UI/API selection rules.
- Agent-level workflow defaults would reintroduce assignment-dependent workflow changes. Resolved: agents declare **Workflow Capability** only; they do not select the workflow by default.
- Capability mismatch could block assignment, but early capability modeling will be incomplete. Resolved: first version emits **Workflow Capability Warning** and records it with the snapshot, without blocking execution.
