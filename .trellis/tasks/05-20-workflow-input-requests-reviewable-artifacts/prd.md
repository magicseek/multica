# Implement workflow input requests and reviewable planning artifacts

## Summary

Implement the workflow execution extensions described by ADR 0004:

- Workflow-scoped human input requests that pause a workflow execution batch and resume the same workflow run after an explicit answer.
- Cloud-reviewable planning artifacts that preserve the local-output privacy boundary by requiring explicit artifact save.
- Complete artifact versions with reviewer-facing diffs between versions.

## Goals

- Let agents ask bounded clarification questions during eligible workflow steps without creating a new issue status, failing the task, or triggering the ordinary comment workflow.
- Release daemon capacity while waiting for human input, then resume the same workflow run and task context when the user answers.
- Let planning, brainstorming, and requirements documents be saved as reviewable workflow artifacts for cloud approval.
- Preserve complete artifact versions as the source of truth while exposing diffs as review aids.
- Keep issue UX unified: normal comments remain normal comments; answer-bound comments resume workflow input requests.

## Non-goals

- Do not add a `waiting_for_clarification` issue status or board swimlane.
- Do not make every local output cloud-visible.
- Do not implement AI-generated workflow creation.
- Do not let artifact approval directly create issues.
- Do not store artifact revisions as patch-only content.
- Do not implement unbounded brainstorming loops inside implementation issues.

## User Experience Requirements

- Issues with open workflow input requests remain in their existing issue status but show a `Needs input` attention state in cards, detail header, and workflow run viewer.
- Workflow step rows show when a step is allowed to ask a workflow input request, so users can see the intended question path even before the agent opens a request.
- The issue detail activity stream shows agent questions as workflow input request activities with `Answer & continue`, `Comment only`, and cancel affordances.
- Submitting through `Answer & continue` records an issue-visible answer and resumes the workflow; it must not enqueue a Comment Response workflow even if the answer mentions an agent.
- Submitting through the ordinary comment composer remains an ordinary comment and only follows existing mention/comment trigger rules.
- Planning artifacts appear in the workflow run viewer with review controls. Reviewers can view full versions and diffs between versions.
- Artifact lists must be content-reviewable, not just metadata lists. A reviewer must be able to read the saved Markdown/text/JSON artifact in Multica without opening a daemon-local path.
- Approving a reviewable artifact unblocks the workflow; a later explicit step generates Chat Issue Proposals or issues.

## Functional Requirements

### Workflow Input Requests

- Add persistent workflow input requests linked to workspace, workflow run, workflow step run, and optional issue/chat context.
- Add task lifecycle support for a waiting/suspended state that is active but not daemon-claimable.
- Validate step policy before allowing input requests:
  - step execution kind must be `agent`;
  - step snapshot must allow input requests;
  - round limit must not be exceeded;
  - only one open request may exist per step run.
- Creating an input request:
  - creates or links an issue-visible question comment when issue context exists;
  - sets workflow run to `waiting`;
  - sets the step to a waiting state;
  - suspends the backing task without completing unfinished steps.
- Answering an input request:
  - creates or links an issue-visible answer comment;
  - marks the request answered;
  - makes the step resumable;
  - requeues the same backing task;
  - preserves provider session/workdir when available.

### Comment Routing

- Answer-bound comments must bypass ordinary comment-trigger task enqueue.
- Ordinary comments must not answer input requests unless routed through the explicit answer action.
- Ordinary mention/comment behavior remains unchanged outside answer-bound submissions.

### Reviewable Workflow Artifacts

- Continue using explicit `workflow artifact save` as the cloud publication boundary.
- Ensure reviewable planning artifacts are visible in workflow run responses and UI.
- Review approval unblocks dependent steps only.
- Review rejection/request-changes leads to retry/revision flow without overwriting prior artifact versions.

### Artifact Versions And Diffs

- Save every artifact version as complete content.
- Add deterministic diff support between two versions of the same logical artifact.
- Support text, markdown, and JSON content in first version.
- UI defaults to latest complete version and can show `vN -> vN+1` diff.
- Agents may use local patch or section-level editing to reduce output tokens, but final saved artifact version must be complete.

## Implementation Slices

1. Data model and sqlc:
   - migrations for `agent_task_queue.waiting` and `workflow_input_request`;
   - queries for creating/listing/answering/cancelling input requests;
   - query changes so waiting tasks are active but not claimable.

2. Workflow service and API:
   - service methods for request/answer/cancel input request;
   - run response includes input requests;
   - artifact diff service/handler;
   - reviewable artifact behavior remains explicit save plus review gate.

3. Comment routing:
   - explicit answer route creates or links answer comments;
   - answer-bound comments bypass comment-trigger workflow;
   - ordinary comments remain unchanged.

4. Daemon and CLI:
   - CLI commands for input request and artifact diff;
   - daemon suspend path when workflow waits for input;
   - resume same task/run with prior session/workdir.

5. Frontend/core/views:
   - workflow input request and diff types;
   - issue attention state and answer-targeted composer;
   - workflow run viewer input request controls;
   - artifact version/diff review UI.

## Acceptance Criteria

- A workflow step that allows input requests can ask a question, suspend the task, show `Needs input`, and release daemon capacity.
- The user can answer through `Answer & continue`; the same workflow run resumes in a new execution batch without creating a comment response task.
- Ordinary issue comments still behave exactly as before.
- Round limits prevent unbounded clarification loops.
- A planning workflow can save a markdown plan as a reviewable workflow artifact, request review, receive approval/rejection, and preserve immutable versions.
- Artifact diff UI/API shows changes between complete artifact versions.
- The workflow run viewer renders the latest artifact content inline and has a regression test preventing a names-only artifact list.
- No new issue status is introduced.
- Local outputs remain private unless explicitly saved as workflow artifacts.

## Verification Plan

- Backend migration and sqlc tests for new task status and input request table.
- Service tests for request, answer, cancel, round limit, duplicate open request, same-run resume, and task waiting behavior.
- Handler tests for input request routes, answer-bound comment routing, and artifact diff.
- Daemon tests for suspend/resume without complete/fail and session/workdir preservation.
- Frontend tests for issue attention state, answer composer routing, workflow run viewer controls, artifact version display, and diff rendering.
- Run `make test`, focused Go tests, `pnpm typecheck`, and `pnpm test` for touched packages.

## References

- `CONTEXT.md`
- `docs/adr/0004-workflow-input-requests-and-reviewable-planning-artifacts.md`
- `docs/adr/0003-migrate-ai-desk-flows-through-multica-workflow-runs.md`
- `.trellis/spec/workflows/execution-workflow-definitions.md`
