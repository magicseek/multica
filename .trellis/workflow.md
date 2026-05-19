# Trellis Workflow

This file is the phase guide consumed by `.trellis/scripts/get_context.py --mode phase`.

## Phase Index

### Phase 1: Plan

#### 1.1 Clarify Requirements

Use this phase when no task PRD exists or the user request is still ambiguous.

- Create or update `.trellis/tasks/<task>/prd.md`.
- Capture goals, non-goals, acceptance criteria, and constraints.
- Do not start implementation until the PRD is concrete enough to verify.

#### 1.2 Configure Task Context

Curate task context before implementation.

- Put durable requirements in `prd.md`.
- Put spec, design, and research references in `implement.jsonl` and `check.jsonl`.
- Keep manifests focused on useful context; avoid dumping unrelated code paths.

### Phase 2: Execute

#### 2.1 Load PRD And Context

Use this phase when the active task already has `prd.md`.

Required actions:

- Read the active task's `prd.md`.
- Validate `implement.jsonl` and `check.jsonl`.
- Load relevant spec indexes from `.trellis/spec/`.
- Read detailed spec files listed by those indexes before editing code.
- For cross-layer work, read `.trellis/spec/guides/cross-layer-thinking-guide.md`.
- For repeated patterns or new helpers, read `.trellis/spec/guides/code-reuse-thinking-guide.md`.
- Confirm the worktree branch and task metadata match the active task.

For Codex inline mode:

- Keep implementation in the current agent unless a bounded subagent improves throughput.
- Use `trellis-before-dev` before code edits.
- Use `trellis-check` before claiming completion.

#### 2.2 Implement

Make scoped, reversible changes that satisfy `prd.md`.

- Follow package boundaries and loaded specs.
- Prefer existing patterns and utilities.
- Keep generated artifacts in sync.
- Run focused verification as changes land.

### Phase 3: Finish

#### 3.1 Verify And Record

Before completion:

- Run the quality checks required by touched layers.
- Record untested gaps honestly.
- Update task status only after the acceptance criteria are satisfied.
- Use `trellis-finish-work` for final verification and session recording.

## Customizing Trellis (for forks)

Keep this section header because the phase extractor stops before it.
