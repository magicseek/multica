# Server Backend Guidelines

> Backend contracts for Multica's Go server, database, daemon, and runtime
> prompt wiring.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Agent Execution Protocol](./agent-execution-protocol.md) | Contracts for opt-in task-agent execution protocol settings, API payloads, daemon claim data, and prompt injection gates | Filled |

---

## Quality Check

For backend changes that cross database, API, daemon, and frontend layers:

1. Read the relevant scenario spec in this directory.
2. Verify the full data path, not only the edited package.
3. Regenerate generated code after SQL changes.
4. Add tests at each manual mapping boundary.

Minimum checks for agent execution protocol changes:

- `make sqlc`
- `cd server && go test ./internal/daemon/execenv ./internal/daemon ./internal/handler`
- Frontend typecheck/tests for any `packages/core` or `packages/views` contract changes

