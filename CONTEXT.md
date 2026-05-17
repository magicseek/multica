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

**Lead Agent**:
The agent selected to bootstrap, coordinate, or own work when a repository starts without an existing Git remote or user-selected local directory.

**Output Metadata**:
Privacy-minimized facts about local or agent-managed task outputs, such as file names, relative paths, sizes, MIME types, and creation times, excluding file contents, diffs, logs, stack traces, screenshots, and absolute local paths.

## Relationships

- A **Workspace** contains zero or more **Repositories**
- A **Workspace** contains zero or more **Projects**
- A **Project** may reference one or more **Repositories**
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
