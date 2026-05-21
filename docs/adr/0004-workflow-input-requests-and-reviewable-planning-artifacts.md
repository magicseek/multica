# Workflow input requests and reviewable planning artifacts

Multica workflow execution needs a way for agents to ask bounded human clarification questions and to publish planning documents for cloud review without turning every local output into cloud content. We will model human clarification as workflow-scoped input requests that pause and resume the same workflow run, and model cloud-reviewable planning documents as explicit workflow artifacts with human review gates.

## Considered Options

- Add a `waiting_for_clarification` issue status or swimlane: rejected because issue status is a coarse work lifecycle, while clarification is a workflow attention state tied to one step run.
- Treat any user reply on an issue with an open clarification as the answer: rejected because ordinary issue discussion and comment-triggered agent work must remain distinct from answering a workflow input request.
- Let clarification replies trigger the ordinary comment response workflow: rejected because answering a waiting workflow step should resume the existing workflow run, not start a separate comment task.
- Model clarification as a manual workflow step: rejected because a manual step means the human completes the step, while a workflow input request supplies information so the agent can continue the same agent step.
- Create a new workflow run after human input: rejected because it fragments step history and makes the original waiting point look terminal.
- Allow unbounded brainstorming inside implementation issues: rejected because it hides unready requirements inside active work and makes `in_progress` misleading.
- Keep planning and brainstorming documents as local outputs with metadata only: rejected because cloud review and approval need stable server-stored content.
- Let artifact approval directly create issues: rejected because approving a plan and creating workspace issues are separate side effects with different review needs.
- Store artifact revisions as patches only: rejected because review and downstream execution should not depend on replaying previous versions.

## Consequences

- Waiting for clarification is shown as an attention state derived from workflow runtime data, not as a new issue status.
- A workflow input request belongs to one workflow step run and is answered through an explicit answer action, such as `Answer & continue`.
- An answer-bound comment resolves the workflow input request and must not also trigger the ordinary comment response workflow.
- The workflow run remains the same after human input; the next daemon pass is a new workflow execution batch on that run.
- Implementations may need a task-level waiting or suspended lifecycle so daemon slots are released while preserving the workflow run, work directory, and provider session.
- Workflow input requests are bounded by workflow policy: execution steps should ask at most one round by default, while planning or contract steps may allow a small configured number of rounds.
- Initial open-ended requirements exploration belongs in chat sessions or planning workflows before implementation issues are created.
- Existing implementation issues may use workflow input requests only for narrow input needed to continue the current workflow.
- Planning, brainstorming, and requirements documents that require cloud review are saved explicitly as reviewable workflow artifacts.
- Ordinary local task outputs remain governed by Output Metadata privacy boundaries unless explicitly saved as workflow artifacts.
- Approving a reviewable workflow artifact unblocks dependent workflow steps; a later explicit step creates Chat Issue Proposals or issues.
- Each workflow artifact version is stored as a complete artifact.
- Workflow artifact diffs are generated or cached as review aids, not treated as the artifact source of truth.
- Agents may use patch-style or section-level edits to reduce generation cost, but the persisted artifact version remains the complete revised document.
