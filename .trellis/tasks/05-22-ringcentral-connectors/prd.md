# Add RingCentral GitLab Jira Wiki Connectors

**Created**: 2026-05-22
**Assignee**: troy.huang
**Priority**: P1
**Status**: Planning
**Parent**: `05-19-ai-desk-flows-to-multica-workflows`
**Branch**: `feat/ringcentral-connectors`
**Base Branch**: `trellis/ai-desk-flows-to-multica-workflows`
**Worktree**: `/Users/troy.huang/workspace/AI/multica-ringcentral-integrations`

## Goal

Add optional RingCentral GitLab, Jira, and Wiki connector support to Multica so agents and issue tasks can use project-scoped RingCentral context and GitLab merge request preparation capabilities, while keeping company-specific services out of the default public Multica product surface.

## Source Documents

- `CONTEXT.md`
- `docs/ringcentral-connectors-implementation-plan.md`
- `docs/adr/0005-enable-ringcentral-connectors-through-an-explicit-profile.md`
- `docs/adr/0006-start-ringcentral-connectors-with-user-owned-credentials.md`
- `docs/adr/0007-capture-connector-delegated-user-at-task-queue-time.md`
- `docs/adr/0008-split-ringcentral-section-admin-controls-from-user-credentials.md`
- `docs/adr/0009-limit-ringcentral-mvp-to-project-scoped-capabilities.md`
- `docs/adr/0010-expose-ringcentral-capabilities-through-cli-first.md`
- `docs/adr/0011-store-ringcentral-credentials-in-an-encrypted-vault.md`
- `docs/adr/0012-register-ringcentral-as-optional-connector-providers.md`
- `docs/adr/0013-use-container-and-pinned-project-resources.md`
- `docs/adr/0014-authorize-agent-connector-commands-by-task-scope.md`
- `docs/adr/0015-do-not-migrate-github-in-the-ringcentral-mvp.md`
- `docs/adr/0016-gate-ringcentral-through-startup-profile-registration.md`
- `docs/adr/0017-constrain-gitlab-writes-to-semantic-task-branches.md`
- `docs/adr/0018-use-server-mediated-gitlab-api-before-git-auth-broker.md`
- `docs/adr/0019-configure-ringcentral-endpoints-at-deployment-profile-level.md`
- `docs/adr/0020-validate-ringcentral-credentials-before-save.md`
- `docs/adr/0021-keep-ringcentral-settings-separate-from-project-resource-management.md`
- `docs/adr/0022-implement-ringcentral-connectors-security-first.md`
- `docs/adr/0023-verify-ringcentral-connectors-with-an-acceptance-gate.md`

## What Is Known

- AI Desk stores Jira, Wiki, and GitLab tokens on user settings and uses direct provider clients for validation, reads, GitLab branches, commits, and merge requests.
- Multica already has a GitHub App integration, workspace integration settings, project resources, daemon task identity headers, and CLI commands that can call the server from agent runtime.
- The RingCentral work must introduce a generic connector boundary without migrating the existing GitHub App integration in the MVP.
- RingCentral-specific providers are visible only when the server starts with an explicit RingCentral Connector Profile.
- Credential-bearing profiles must fail closed unless encryption key or KMS configuration is available.
- The first implementation is security-first: profile gating, credential vault, delegated user capture, and project resource scoping come before agent-facing connector commands.

## Requirements

- Add a generic connector framework with provider registry, workspace connector records, user-owned connector credentials, encrypted credential storage, provider capabilities, and connector capability audit events.
- Register RingCentral GitLab, Jira, and Wiki adapters only through an explicit startup profile. Public/default deployments must not expose RingCentral providers, routes, validators, runtime guidance, or settings UI.
- Store RingCentral credentials as encrypted server-side secrets owned by individual users. Raw tokens must never be returned, logged, included in task context, included in `.multica/project/resources.json`, or exposed through agent environment variables.
- Validate credentials before save. Persist validation status, upstream identity metadata, and validation timestamps. Mark credentials invalid on use-time upstream auth failures.
- Capture `connector_delegated_user_id` at task queue time for issue assignments, human comment triggers, chat tasks, and explicit Autopilot configuration. Retries, reruns, and agent-authored comment triggers inherit the original delegated user unless explicitly reauthorized.
- Add a RingCentral Section under Workspace Settings -> Integrations only when the profile is enabled. Admins can manage provider enablement and GitLab remote write policy. Each member can manage only their own credentials.
- Add project resource support for `ringcentral_gitlab_repo`, `ringcentral_jira_project`, `ringcentral_jira_issue`, `ringcentral_wiki_space`, and `ringcentral_wiki_page`.
- Keep resource search, attach, review, and remove flows on Project surfaces, not in settings.
- Expose agent capabilities through `multica connector ...` CLI/API commands first. MCP wrappers are deferred and must use the same backend authorization model if added later.
- Authorize every agent connector command with daemon-provided `X-Agent-ID` and `X-Task-ID`, active task state, workspace match, connector delegated user, matching project resource, valid credential, provider policy, and capability checks.
- Jira and Wiki MVP capabilities are read-only: validate, list/search, get issue/page/space context, and attach read-only context.
- GitLab MVP capabilities include validate, project access validation, tree/file reads, list branches/MRs, create semantic task branch, commit file actions through GitLab API, push via API, and create MR.
- GitLab writes must be restricted to attached repositories, semantic task branches using prefixes like `feat/`, `fix/`, `refactor/`, `docs/`, `test/`, `chore/`, or `perf/`, and non-default/non-protected branches.
- GitLab local checkout authentication through connector credentials is out of scope. Existing Multica Repository and Repository Binding flows remain responsible for local checkout and test workflows.

## Non-Goals

- Do not migrate the existing GitHub App integration into the connector framework in this MVP.
- Do not expose RingCentral providers in public/default Multica deployments.
- Do not add Jira or Wiki writes, including comments, status changes, page edits, or space changes.
- Do not inject RingCentral PATs into local git, daemon environment, task prompts, or `custom_env`.
- Do not implement MCP-only access as the first agent-facing surface.
- Do not add build tags, plugin binaries, or a separate RingCentral-specific server binary in the MVP.
- Do not allow agents to scan company-wide Jira/Wiki/GitLab outside attached project resources.

## Implementation Phases

1. Connector framework foundation
   - Add DB schema, sqlc queries, provider registry, startup profile gating, encrypted credential vault, and audit model.
   - Keep existing GitHub integration tables, routes, webhooks, and settings behavior unchanged.

2. Delegated user capture
   - Add `connector_delegated_user_id` to queued tasks.
   - Populate and inherit it across issue, comment, chat, Autopilot, retry, and rerun entry points.

3. RingCentral provider adapters
   - Implement GitLab, Jira, and Wiki adapters in isolated connector packages.
   - Read RingCentral endpoint URLs from deployment/profile-level configuration.
   - Add validation, status updates, fake HTTP tests, and auth failure handling.

4. Settings UI
   - Add profile-gated RingCentral Section to Integrations settings.
   - Show provider endpoint status, workspace connector state, current-user credential status, save/test/delete controls, and admin-only remote write policy.

5. Project Resource support
   - Add validators, API responses, frontend types/hooks, resource rendering, project attach/search flows, runtime resource rendering, and `.multica/project/resources.json` handling for RingCentral resource types.

6. Task-scoped connector commands
   - Add `multica connector ...` commands and server handlers.
   - Enforce task, delegated-user, credential, provider, policy, and project-resource checks for every call.
   - Add audit events and recoverable user reconfiguration errors.

7. Verification and documentation
   - Run backend, sqlc, TypeScript, frontend, and focused smoke checks.
   - Document profile enablement, endpoint config, encryption config, credential setup, project resource setup, and supported command capabilities.

## Acceptance Criteria

- With the RingCentral profile disabled, RingCentral provider registry entries, routes, settings section, resource validators, runtime guidance, and connector commands are absent or rejected.
- With the RingCentral profile enabled but no credential encryption key/KMS configured, startup fails closed before exposing credential storage.
- Raw tokens do not appear in API responses, task context, `.multica/project/resources.json`, agent environment variables, logs, or audit events.
- Delegated user rules are covered for issue assignments, human comment triggers, agent-authored comment inheritance, chat tasks, Autopilot tasks, retries, and reruns.
- Task-scoped connector commands reject missing task identity, agent/task mismatch, inactive task, missing delegated user, missing project resource, and missing/invalid credential.
- Fake HTTP servers cover RingCentral GitLab/Jira/Wiki validation, search/read behavior, auth failures, and GitLab semantic branch/MR guardrails.
- Settings shows provider and credential readiness only; Project Resource attachment remains on Project surfaces.
- A smoke flow can configure credentials, attach resources, read Jira/Wiki/GitLab context, and create a GitLab semantic task branch plus merge request.

## Implementation Guardrails

- Reuse Multica's existing migration, sqlc, handler, React Query, package boundary, and shared UI patterns before adding new abstractions.
- RingCentral provider code must live behind optional registration and must not be imported into the default public surface through always-on settings or runtime guidance.
- CLI commands must call the Multica server and use server-side authorization. Agent runtimes must never receive provider tokens.
- Project resources identify scope and context only; they are not credential stores.
- Settings credentials are per user. Admin controls do not let admins read or edit other users' secrets.
- All external-write behavior must be audit logged and constrained by GitLab Remote Write Policy.

## Open Questions

- None blocking for MVP implementation. Future work remains documented in `docs/ringcentral-connectors-implementation-plan.md`.

## Definition of Done

- Implementation phases are complete and acceptance criteria are verified.
- `make test`, `pnpm typecheck`, and relevant frontend tests pass, or any unavoidable gaps are documented with concrete reasons.
- sqlc and generated API/types are regenerated where schema/API changes require it.
- Docs describe enabling the RingCentral Connector Profile and configuring endpoints/encryption.
- Specs are updated if implementation establishes reusable connector patterns.
