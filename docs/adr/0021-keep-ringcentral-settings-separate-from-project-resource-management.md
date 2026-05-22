# Keep RingCentral settings separate from project resource management

The RingCentral Section should appear inside Settings -> Integrations only when the RingCentral Connector Profile is enabled. It should contain compact provider cards for RingCentral GitLab, Jira, and Wiki. Each card shows deployment endpoint status, workspace connector enablement/status, the current user's credential status and upstream identity, and token save/test/delete controls. Workspace admins may also control provider enablement and RingCentral GitLab Remote Write Policy from this section.

Project-scoped search, attach, review, and removal flows for GitLab repos, Jira projects/issues, and Wiki spaces/pages belong in Project Resource Management UI on Project surfaces, not in the RingCentral settings section.

The rejected alternative was to make Settings -> Integrations a combined provider setup and project-context editor. That would make user-owned credential management compete with task-context editing, and it would hide the project-scoped authorization boundary behind a global settings page.
