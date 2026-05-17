# Multica Agent Workflows

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
