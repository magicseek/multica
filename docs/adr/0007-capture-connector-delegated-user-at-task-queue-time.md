# Capture connector delegated user at task queue time

Agent tasks that use user-owned connector credentials should capture a Connector Delegated User when the task is queued. Manual issue assignment uses the human who started the run, human comment triggers use the comment author, chat tasks use the chat creator or triggering human message author, autopilot tasks require an explicit delegated user, and retries or reruns inherit the original delegated user unless explicitly re-authorized.

The rejected alternative was to infer connector authority later from issue creator, assignee, runtime owner, commenter, or workspace owner. Runtime inference would produce surprising access changes after comments, reassignment, retries, or ownership edits, while queue-time capture gives each task an auditable authorization boundary.
