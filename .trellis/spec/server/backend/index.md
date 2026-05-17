# Server Backend Guidelines

> Backend implementation contracts for `server/`.

## Pre-Development Checklist

- Read [Repository Contracts](./repository-contracts.md) before changing repository, binding, project-repository, chat repository, repository-operation, or task-output metadata behavior.

## Quality Check

- Confirm API request/response fields match the documented contracts.
- Confirm database migrations include reversible down migrations.
- Confirm sqlc output is regenerated after query or schema changes.
- Confirm local/private fields are filtered before returning workspace-wide responses or realtime events.
- Confirm compatibility reads do not become long-term dual writes.

**Language**: All documentation should be written in **English**.
