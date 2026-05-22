# Verify RingCentral connectors with an acceptance gate

RingCentral connector delivery should require a Connector Acceptance Gate:

1. Profile disabled: provider registry, routes, RingCentral Section, resource validators, and agent runtime guidance are absent or rejected.
2. Credential vault: raw tokens never appear in API responses, task context, `.multica/project/resources.json`, agent environment variables, or logs; the RingCentral profile fails closed without a credential encryption key or KMS.
3. Delegated user: issue assignment, human comment trigger, agent-authored comment trigger inheritance, chat task, autopilot task, retry, and rerun rules are covered by tests.
4. Task-scoped command authorization: missing task identity, agent/task mismatch, inactive task, missing delegated user, missing project resource, and missing/invalid credential are rejected.
5. Provider adapters: fake HTTP servers cover GitLab, Jira, and Wiki validation, search/read behavior, auth failures, and GitLab semantic branch/MR guardrails.
6. UI: RingCentral settings show provider and credential readiness only, while Project Resource attachment stays on Project surfaces.
7. Smoke flow: configure credential, attach resources, run an agent connector command that reads Jira/Wiki/GitLab context, and create a GitLab semantic task branch plus merge request.

The rejected alternative was to rely on a happy-path manual demo. This integration crosses credentials, external side effects, task authorization, and optional deployment profile boundaries, so manual-only proof would miss the regressions most likely to leak access or expose company-specific functionality in public deployments.
