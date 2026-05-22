# Server Backend Guidelines

> Backend implementation contracts for `server/`, including database schema,
> API payloads, daemon claim data, runtime prompt wiring, and repository
> operation lifecycles.

## Pre-Development Checklist

- Read [Repository Contracts](./repository-contracts.md) before changing repository, binding, project-repository, chat repository, repository-operation, or task-output metadata behavior.
- Read [Chat Contracts](./chat-contracts.md) before changing chat session creation, messages, title provenance, chat-session websocket events, or project-associated chat behavior.
- Read [Agent Execution Protocol](./agent-execution-protocol.md) before changing opt-in task-agent execution protocol settings, API payloads, daemon claim data, or prompt injection gates.
- Read [Daemon Runtime Contracts](./daemon-runtime-contracts.md) before changing daemon registration metadata, local health/bridge endpoints, runtime metadata gates, or native daemon helpers.
- Read workflow specs under `../workflows/` before changing workflow definitions, revisions, bindings, overrides, or queue-time snapshots.

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Repository Contracts](./repository-contracts.md) | Contracts for first-class repositories, bindings, project references, chat defaults, daemon repository operations, and task output metadata | Filled |
| [Chat Contracts](./chat-contracts.md) | Contracts for chat session creation, first-message titles, title provenance, and realtime cache visibility | Filled |
| [Agent Execution Protocol](./agent-execution-protocol.md) | Contracts for opt-in task-agent execution protocol settings, API payloads, daemon claim data, and prompt injection gates | Filled |
| [Daemon Runtime Contracts](./daemon-runtime-contracts.md) | Contracts for daemon registration metadata, local bridge endpoints, folder selection, and CLI version gates | Filled |

## Quality Check

- Confirm API request/response fields match the documented contracts.
- Confirm database migrations include reversible down migrations.
- Confirm sqlc output is regenerated after query or schema changes.
- Confirm local/private fields are filtered before returning workspace-wide responses or realtime events.
- Confirm compatibility reads do not become long-term dual writes.
- `make sqlc`
- `cd server && go test ./internal/daemon/execenv ./internal/daemon ./internal/handler`
- Frontend typecheck/tests for any `packages/core` or `packages/views` contract changes

**Language**: All documentation should be written in **English**.
