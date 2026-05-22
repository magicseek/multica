# Limit RingCentral MVP to project-scoped capabilities

RingCentral GitLab, Jira, and Wiki connectors should start with explicit Connector Capabilities gated by Project Resources and a Connector Delegated User. GitLab may validate credentials and project access, read repository trees and files, list branches and merge requests, create branches, push commits, and create merge requests. Jira may validate credentials, list projects, search and retrieve issues, and attach read-only Jira issue context. Wiki may validate credentials, search and retrieve pages or spaces, and attach read-only Wiki context.

The rejected alternative was to copy AI Desk's integration surface wholesale or let agents use a user's RingCentral token to browse broad company systems. That would make the first version harder to audit, blur project boundaries, and introduce Jira/Wiki write behavior before Multica has a clear review and approval model for external side effects.
