# Configure RingCentral endpoints at deployment profile level

RingCentral connector endpoint base URLs should be deployment/profile configuration, not per-user settings. The RingCentral Connector Profile reads configured GitLab API base URL, GitLab web base URL, Jira base URL, and Wiki base URL at startup. Workspace connector status may display those endpoints read-only, but ordinary workspace admins do not edit them in the MVP.

Test and private deployments may override endpoints through environment/configuration values. Public Multica logic should not hardcode RingCentral URLs into default providers or user settings.

The rejected alternative was to let each user enter base URLs alongside PATs. User-level endpoints would create inconsistent provider behavior inside one workspace, make audit records harder to interpret, and weaken the profile boundary that controls whether RingCentral services exist in a deployment.
