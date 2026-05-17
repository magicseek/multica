# Workflow definitions, bindings, and overrides

Agent execution workflows are editable workspace-level definitions, not hardcoded templates and not primarily agent-owned settings. Projects bind to a default workflow so issues in the same project keep the same working logic even when reassigned to different agents; individual issues can override that binding with another workflow such as a lightweight Direct Task or Research Note workflow. Each queued or claimed agent task stores a workflow snapshot so later edits to the definition do not silently change in-flight work.

## Considered Options

- Agent-level workflow selection: rejected because reassignment would change the work process for the same project issue.
- Project-only selection with no issue override: rejected because small bugs and research tasks sometimes need a lighter process than the project default.
- Boolean workflow skip: rejected because it bypasses governance and makes minimum execution requirements implicit.

## Consequences

- Workflow selection resolves in this order: issue override, project binding, workspace default, built-in fallback.
- "Direct Task" and "Research Note" should be regular editable workflow definitions, not code-level bypasses.
- Existing agent-level execution protocol fields should be treated as compatibility or exceptional override surfaces rather than the primary model.
- Workflow editing should support the full authoring surface: metadata, Markdown/template source fields, structured step graph/schema, and resolved preview.
- The preview UI should be built with Multica's existing UI library and design system rather than copying ai-desk styling.
- The workflow schema is the canonical stored representation. Markdown/template source and step graph editing surfaces must read and write that schema, and preview must render from it.
- Built-in workflows such as Standard Assignment, Trellis Task, Direct Task, and Research Note should be seeded into each workspace as read-only system workflow definitions.
- Users customize built-ins by copying or forking them into editable workflow definitions.
- Bindings and overrides should reference workflow definition IDs rather than hardcoded slugs, preserving upgrade paths for system-seeded definitions.
- Workflow definitions use a draft, published, deprecated lifecycle.
- Project bindings and issue overrides can only select workflow definitions that have a published revision for the target trigger.
- Editing a published workflow creates or updates a draft revision; publishing makes that revision the definition's current published revision and affects only newly queued tasks.
- Already queued, running, or historical tasks keep their workflow snapshot and are not silently changed by later publishes.
- Issue assignment tasks resolve workflow selection and store the workflow snapshot when `agent_task_queue` is created, not when a daemon later claims the task.
- Comment-triggered `@agent` tasks default to a lightweight Comment Response workflow rather than inheriting the project's full workflow.
- Running the full issue workflow from a comment should be an explicit action or instruction, not the default behavior.
- Workflow schema declares trigger applicability, such as assignment, comment response, chat, or autopilot run.
- UI selectors and API validation should filter workflow choices by trigger applicability.
- Agents do not own default workflow selection. They may declare workflow capabilities or constraints, while issue, project, and workspace resolution remains authoritative.
- In the first version, capability mismatch is warning-only: UI surfaces it, queue-time snapshot metadata records it, and execution is not blocked.
