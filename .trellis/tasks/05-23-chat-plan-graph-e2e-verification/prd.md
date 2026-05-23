# Verify chat Plan mode graph rollout end to end

## Goal

Validate that directed graph routing, Plan mode squad orchestration, message identity UI, composer footer controls, and proposal issue approval work together without regressions.

## Requirements

* Run backend tests covering graph, routing, Plan Run, proposal approval, and legacy compatibility.
* Run frontend typecheck and targeted Vitest tests for chat UI and core parsing.
* Run relevant full verification commands where feasible.
* Manually verify the main Plan squad flow in a local web/desktop dev session.
* Capture any final spec updates or known residual risks before finish.

## Acceptance Criteria

* [ ] Migrations and sqlc output are current.
* [ ] Go tests covering chat/plan routing pass.
* [ ] `pnpm typecheck` passes.
* [ ] Targeted chat UI tests pass.
* [ ] Manual flow verifies lead-first squad Plan Run, helper-to-lead replies, no-mention human continuation to lead, 5-wave limit behavior, and proposal approval.
* [ ] Final notes document commands run and any remaining risks.

## Suggested Files

* `.trellis/tasks/05-23-chat-plan-mode-polish-squad-routing/*`
* `.trellis/spec/*` if implementation reveals new durable conventions.
* Test files touched by the implementation tasks.

## Test Plan

* `make test` or targeted Go tests around chat handlers/services.
* `pnpm typecheck`.
* Targeted `pnpm test` for chat components/core.
* Local browser/desktop smoke test for the main chat Plan mode flow.

## Dependencies

Depends on all implementation child tasks.
