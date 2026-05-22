# Allow RingCentral Endpoint Overrides

## Summary

RingCentral GitLab, Jira, and Wiki connectors should default to the official
RingCentral service URLs when the RingCentral profile is enabled. Workspace
admins can override those service addresses per connector from Integrations
settings without changing per-user credentials.

## Requirements

1. RingCentral profile startup defaults:
   - GitLab API: `https://git.ringcentral.com/api/v4`
   - GitLab web: `https://git.ringcentral.com`
   - Jira: `https://jira.ringcentral.com`
   - Wiki: `https://wiki.ringcentral.com`
2. Environment variables may still override the deployment defaults.
3. Workspace connector settings may store admin-owned endpoint overrides.
4. Endpoint overrides must be valid HTTP(S) base URLs and may only override
   endpoint keys declared by the provider.
5. Credential validation and task-scoped connector actions must use effective
   endpoints: workspace override first, then provider default.
6. The Integrations UI must show the effective service addresses and provide an
   admin-only edit flow in the RingCentral detail section.
7. The UI should allow admins to clear overrides and return to profile defaults.

## Out of Scope

- Per-user endpoint settings.
- Public/default Multica exposure of RingCentral connectors without the explicit
  RingCentral profile.
- Migrating the existing GitHub App integration into the connector framework.

## Verification

- Backend config tests cover official RingCentral defaults and env overrides.
- Backend handler tests cover endpoint override persistence and validation.
- Frontend settings tests cover the edit/save/reset flow.
- Typecheck, lint, and focused frontend/backend tests pass.
