# Private Project-Associated Chat Sessions and User-Approved Issue Proposals

Accepted: 2026-05-18

Chat Sessions remain private, creator-owned conversations even when they are associated with a Project. Project association is an organizational and defaulting context: the session appears under the Project navigation, uses the selected Project as the default target for chat-originated issues, and is also visible from the Project detail Chats tab for its creator. It does not make the conversation visible to other Project members.

Chat-originated issues are not created directly by agent reasoning. The agent may produce a structured Chat Issue Proposal, the backend persists and validates that payload, and the approving user may edit proposal items before creating issues. Approved items create normal Issues in backlog with explicit provenance back to the Chat Session and proposal; the approving user is the issue creator, while the agent remains proposal provenance.

## Considered Options

- Make Project-associated chats shared with Project members. Rejected because the current chat model is private, and Project association should not silently change the privacy boundary.
- Let chat agents create issues directly. Rejected because issue creation is a workspace side effect that needs user approval and an editable review point.
- Store proposals only as chat markdown. Rejected because the UI needs item-level approval, editing, skipped/restored states, and reliable provenance.
- Treat issue proposals as Outputs. Rejected because Outputs are artifact/output metadata, while proposals are part of the issue creation workflow.
- Allow moving Chat Sessions between Projects. Rejected because the session's selected Project and fixed agent form execution context; moving would imply unclear agent/context changes with little product value.

## Consequences

- A Chat Session has at most one Project association chosen at creation time, and that association is not moved later.
- A Chat Session must have a selected agent at creation or first send; the selected agent stays fixed for the session.
- Project-associated chats are listed under Projects and in the Project detail Chats tab for the creator. Loose chats are listed under Chats.
- The primary Chats navigation entry is the total entry for loose chats; Project-associated chats are discovered through Projects navigation and Project detail.
- Project-associated chats can be started from the Project navigation row action or the Project detail Chats tab; both use the same new-chat route with Project context and create the session on first send.
- Loose chats can be started from the top-level New Chat action, use the same new-chat route without Project context, and require explicit agent selection before first send.
- Chat Session titles should not include the Project name because navigation already provides hierarchy. The title may fall back to the first user message initially, then be updated after the first agent execution using an agent-provided summary title.
- Agent-provided summary titles must not overwrite user-edited titles. The data model should track title source or an equivalent user-edited marker.
- Agent-provided summary titles use the task completion metadata handoff, and the backend applies them only when the user has not edited the title.
- The daemon may read multiple fixed manifests for structured chat outputs, but uploads them together in one task completion payload. The backend then processes summary title metadata, issue proposals, and output metadata through separate modules from the same completion event.
- The global Chat floating action button and floating window are replaced by page-level Chat routes as the primary workspace chat experience.
- The workspace sidebar shows only active Chat Sessions updated in the last five days as a quick-access tree. Complete loose chat history remains available from Chats, and complete project-associated chat history remains available from the Project detail Chats tab.
- If a Project is archived or deleted, associated Chat Sessions remain as private history but are no longer shown under the active Projects sidebar tree. The Chat Session page displays an archived or deleted Project context chip, and already created issues keep their normal issue state and provenance. Chat Sessions store a creation-time Project snapshot so deletion does not turn them into loose chats.
- The chat session data model should use a nullable Project foreign key plus an explicit project-association marker and creation-time Project snapshot. Loose chat status must not be inferred from `project_id IS NULL` alone.
- Archiving or deleting a Chat Session does not cascade to chat-originated issues or output metadata. Created issues and provenance remain readable even when the conversation is no longer active.
- Chat Issue Proposals are shown inline in the Chat transcript after the proposing agent message, and the Chat Session Issues tab shows the same persisted proposals for review and follow-up management.
- Chat Session pages use fixed Chat, Issues, and Outputs tabs. The Issues badge counts created chat-originated issues, not pending proposal items; pending proposals are shown inside the Issues tab.
- Proposal artifacts use a minimal versioned schema with `version` and `proposals`. Proposal items carry issue draft fields such as title, description, optional priority, labels, and assignee, but agents cannot choose issue status.
- Users may edit proposal item content before approval, but not the target Project. Project assignment is derived from the Chat Session context; later reassignment uses normal issue editing.
- Proposal records store detailed provenance such as source chat message, source task, and proposing agent. Created issues keep the lightweight `chat_session` origin link; proposal/item tables provide the detailed source trail.
- Proposal items keep an approval-time snapshot of the content used for issue creation so later issue edits do not erase what was approved.
- Partial approval marks unapproved items as skipped and the proposal as partially accepted. Skipped items can be restored to pending from the Chat Session Issues tab for later approval.
- A Chat Session may contain multiple proposals. The Issues tab groups by proposal, shows pending proposals first, and then orders proposal groups by creation time.
- Chat Issue Proposals require first-class persistence, item-level status, item editing, and atomic transactional issue creation for selected proposal items.
- The backend does not infer issues from prose. It only accepts structured proposal payloads from the task/daemon handoff path, validates them, stores them, and later creates issues after explicit user approval.
- The Outputs tab remains scoped to output metadata and does not include Chat Issue Proposals. It aggregates outputs from the Chat Session's own chat tasks and from agent tasks on issues created from that Chat Session.
