# RingCentral connector implementation plan

This plan captures the approved design for adding RingCentral GitLab, Jira, and Wiki support to Multica without making those company-specific services part of the public default product surface.

## Source analysis

AI Desk currently stores RingCentral tokens directly on user settings and uses provider-specific clients:

- Jira: `~/workspace/RingCentral/AI/ai-desk/backend/app/clients/jira_client.py`
- GitLab: `~/workspace/RingCentral/AI/ai-desk/backend/app/clients/gitlab_client.py`
- Wiki: `~/workspace/RingCentral/AI/ai-desk/backend/app/clients/confluence_client.py`
- Token settings: `~/workspace/RingCentral/AI/ai-desk/backend/app/api/v1/settings.py`
- Settings UI: `~/workspace/RingCentral/AI/ai-desk/frontend/src/pages/SettingsPage.tsx`
- MCP config generation: `~/workspace/RingCentral/AI/ai-desk/frontend/src/services/McpConfigService.ts`

Multica currently has a GitHub-specific integration and a polymorphic `project_resource` table. The new RingCentral work should introduce a generic connector framework in parallel and should not migrate the existing GitHub App flow in this MVP.

## Approved architecture

- RingCentral providers are enabled only by an explicit RingCentral Connector Profile at server startup.
- Public/default Multica deployments do not show RingCentral providers, routes, settings, validators, or runtime guidance.
- RingCentral GitLab, Jira, and Wiki are optional Connector Providers registered into a generic connector provider registry.
- Provider-specific code lives in isolated connector packages and registers through startup profile registration.
- Credentials are user-owned and stored only in an encrypted Connector Credential Vault.
- Raw tokens never enter task context, `.multica/project/resources.json`, agent environment variables, logs, or API responses.
- Agent connector operations are CLI-first through task-scoped `multica connector ...` commands. MCP wrappers may be added later over the same backend capability model.
- Agent connector commands require `X-Agent-ID`, `X-Task-ID`, active task state, Connector Delegated User, matching Project Resource, and a valid credential.
- Jira and Wiki are read-only in the MVP. GitLab may create semantic task branches, commit file actions, push through the GitLab API, and create merge requests.
- GitLab branch names use semantic prefixes such as `feat/`, `fix/`, `refactor/`, `docs/`, `test/`, `chore/`, or `perf/`, not a product-name prefix.
- GitLab local checkout authentication is deferred. MVP GitLab connector operations are server-mediated API operations.

## Implementation sequence

1. Connector framework foundation
   - Add connector schema for providers, workspace connectors, encrypted credentials, and audit events.
   - Add provider registry and profile gating.
   - Add credential encryption configuration and fail-closed startup checks for credential-bearing profiles.
   - Keep existing GitHub tables and routes unchanged.

2. Delegated user capture
   - Add `connector_delegated_user_id` to queued agent tasks.
   - Populate it for issue assignment, human comment triggers, chat tasks, and explicit autopilot configuration.
   - Inherit it for agent-authored comment triggers, retries, and reruns unless explicitly re-authorized.

3. RingCentral provider adapters
   - Add RingCentral GitLab, Jira, and Wiki adapters under isolated connector packages.
   - Read base URLs from deployment/profile endpoint configuration.
   - Validate tokens before save and store upstream identity metadata.
   - Mark credentials invalid on upstream auth failures.

4. Settings UI
   - Extend Settings -> Integrations with a RingCentral Section only when the profile is enabled.
   - Show GitLab, Jira, and Wiki provider cards with endpoint status, workspace connector status, current-user credential status, and save/test/delete controls.
   - Allow admins to manage provider enablement and GitLab Remote Write Policy.

5. Project Resource support
   - Add validators and renderers for:
     - `ringcentral_gitlab_repo`
     - `ringcentral_jira_project`
     - `ringcentral_jira_issue`
     - `ringcentral_wiki_space`
     - `ringcentral_wiki_page`
   - Put search/attach/remove flows on Project surfaces, not in Settings.
   - Update runtime resource rendering and `.multica/project/resources.json` handling without exposing secrets.

6. Task-scoped connector commands
   - Add `multica connector ...` CLI/API commands for provider capabilities.
   - Enforce task, delegated-user, credential, provider, and project-resource checks on every call.
   - Record audit events for provider, capability, task, agent, delegated user, resource, and result metadata.

7. Tests and docs
   - Add fake-provider tests for GitLab, Jira, and Wiki adapters.
   - Add handler tests for profile-disabled absence, credential secrecy, delegated user capture, and command authorization.
   - Add UI tests for Settings/Project Resource separation.
   - Add docs for enabling the RingCentral Connector Profile and configuring endpoint/encryption settings.

## Acceptance gate

The work is not complete until these checks pass:

- With RingCentral profile disabled, provider registry, routes, RingCentral Section, resource validators, and runtime guidance are absent or rejected.
- With RingCentral profile enabled but no credential encryption key/KMS configured, the profile fails closed at startup.
- Raw tokens do not appear in API responses, task context, `.multica/project/resources.json`, agent environment variables, logs, or audit events.
- Delegated user rules are tested for issue assignment, human comments, agent-authored comment inheritance, chat tasks, autopilot tasks, retries, and reruns.
- Task-scoped connector commands reject missing task identity, agent/task mismatch, inactive task, missing delegated user, missing Project Resource, and missing/invalid credential.
- Fake HTTP servers cover RingCentral GitLab/Jira/Wiki validation, search/read behavior, auth failures, and GitLab semantic branch/MR guardrails.
- Settings shows provider and credential readiness only; Project Resource attachment stays on Project surfaces.
- A smoke flow can configure credentials, attach resources, read Jira/Wiki/GitLab context, and create a GitLab semantic task branch plus merge request.

## Explicitly deferred

- Migrating the existing GitHub App integration into the connector framework.
- MCP-only connector access or MCP wrappers as the first agent-facing surface.
- Jira or Wiki writes, including comments, status changes, pages, or spaces.
- Local GitLab checkout/push through connector credentials.
- Build tags, plugin binaries, or a separate RingCentral-specific server binary.
