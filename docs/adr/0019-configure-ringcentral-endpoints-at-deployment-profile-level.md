# Configure RingCentral endpoints with profile defaults and workspace overrides

RingCentral connector endpoint base URLs are profile-owned defaults with
admin-owned workspace overrides. The RingCentral Connector Profile defaults to
the official RingCentral services:

- GitLab API: `https://git.ringcentral.com/api/v4`
- GitLab web: `https://git.ringcentral.com`
- Jira: `https://jira.ringcentral.com`
- Wiki: `https://wiki.ringcentral.com`

Test and private deployments may still override those defaults through
environment/configuration values. Workspace admins may also override service
addresses in Integrations settings; overrides are stored in workspace connector
settings and are applied to credential validation plus task-scoped connector
actions.

Endpoint addresses are still not per-user settings. User-level endpoints would
create inconsistent provider behavior inside one workspace, make audit records
harder to interpret, and weaken the profile boundary that controls whether
RingCentral services exist in a deployment.
