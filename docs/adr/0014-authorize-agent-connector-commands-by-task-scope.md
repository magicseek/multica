# Authorize agent connector commands by task scope

Agent-initiated connector operations should use Task-scoped Connector Commands. Each command must carry the daemon-provided agent and task identity, and the server must verify that the task belongs to the agent, the workspace matches, the task is still active, a Connector Delegated User is present, the task's project has a matching Project Resource, and the delegated user owns a valid connector credential for the provider.

After those checks, the server decrypts the delegated user's credential, invokes the provider adapter, and records a Connector Capability Audit Event containing task, agent, delegated user, provider, capability, resource, and result metadata. Human UI and CLI flows may configure credentials, validate credentials, search external resources, and attach Project Resources, but they do not bypass task-scoped authorization for agent capability execution.

The rejected alternative was to let agents use user-global CLI commands, daemon credentials, or raw provider tokens. That would make connector authority depend on local runtime state, make retries and reassignment hard to audit, and allow external operations outside the project resources selected for the task.
