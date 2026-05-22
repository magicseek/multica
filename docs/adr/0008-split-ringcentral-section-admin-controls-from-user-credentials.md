# Split RingCentral section admin controls from user credentials

The RingCentral Section should live under workspace integration settings when the RingCentral Connector Profile is enabled, but user-owned connector credentials remain editable only by their owning user. Workspace owners and admins may control connector availability and see readiness/status, while each member enters and validates their own Jira, GitLab, and Wiki credentials.

The rejected alternative was to make the RingCentral Section purely workspace-owned because it appears under workspace settings. That would make personal PATs look like shared workspace secrets, invite admins to manage credentials they should not control, and weaken the delegated-user boundary used by agent tasks.
