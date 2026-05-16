# Technical Handoff

## Design Summary

The implementation adds a narrow opt-in execution protocol gate at the agent setting level. The setting is stored on the `agents` table, exposed through agent API request/response types, copied into daemon task claim payloads, and passed through `TaskContextForEnv` into `execenv`.

`execenv` owns protocol text rendering in `execution_protocol.go`. This keeps the injected workflow centralized instead of duplicating prompt fragments across daemon call sites.

## Important Constraints

- Default behavior must remain legacy unless the agent setting is explicitly enabled.
- The setting should affect ordinary task-agent execution, not comment/chat/autopilot/quick-create/squad leader flows.
- Database, Go generated code, API types, frontend types, and UI must stay in sync.
- React Query remains the server-state owner. Do not add agent setting state to Zustand.

## Prior Impact Notes

GitNexus impact checks were run before the original implementation. Most changed symbols were low risk, but two areas had broader fanout:

- `AgentResponse` was flagged critical because API response shape is widely consumed.
- Frontend `Agent` and request types were flagged high because the type flows across views and core hooks.

The chosen mitigation was to make the field additive and defaulted, preserving existing behavior for omitted values.

## Remaining Review Focus

- Verify that every SQL query returning agents includes `execution_protocol_enabled`.
- Verify nullable/default handling for old rows and old clients.
- Verify no non-task prompt path accidentally receives the protocol text.
- Decide whether the root `CONTEXT.md` domain glossary belongs in tracked repo docs or should be converted into Trellis/spec documentation.

