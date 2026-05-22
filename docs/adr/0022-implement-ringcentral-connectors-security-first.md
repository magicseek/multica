# Implement RingCentral connectors security first

RingCentral connector implementation should proceed in a security-first sequence:

1. Add the generic connector schema, provider registry, profile gating, and encrypted credential vault.
2. Add `connector_delegated_user_id` to queued tasks and implement delegated-user capture for issue assignments, comment triggers, chat tasks, autopilot tasks, retries, and reruns.
3. Add RingCentral GitLab, Jira, and Wiki adapters plus credential validation.
4. Add the RingCentral Section in Settings -> Integrations.
5. Add RingCentral Project Resource types, search/attach UI, and runtime resource rendering.
6. Add task-scoped `multica connector ...` CLI/API commands.
7. Add connector capability audit logs, recoverable errors, tests, and documentation.

The rejected alternative was to start with UI or provider adapters and retrofit authorization later. That would make it easy for credentials or external operations to leak through interim surfaces before profile gating, delegated-user capture, resource scoping, and auditability are in place.
