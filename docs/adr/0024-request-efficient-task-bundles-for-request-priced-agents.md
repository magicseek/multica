# Request-efficient task bundles for request-priced agents

Multica should support request-efficient execution for agents whose providers charge materially per request or interactive connection, such as GitHub Copilot and Kiro. This should be an agent-level setting available to any provider, with Copilot and Kiro recommending it by default, rather than a provider-specific queue shortcut.

Task bundle controls should be gated by configured agent capability. Issue task status pages should show the Task Bundle option only when at least one available agent has request-efficient mode enabled; otherwise the normal issue execution flow remains the only visible path.

The execution unit should be a first-class Task Bundle. A bundle is created only at explicit user-facing boundaries such as assignment, Start Work Review, or chat proposal approval, and it owns one Bundle Execution Task plus ordered Task Bundle Items. The daemon must launch the provider once for a request-efficient bundle, pre-materialize all bundle context, and let the agent process items sequentially without Multica sending a new provider prompt for each issue. This guarantees one Multica provider execution boundary, not an exact external billing count when a provider reports internal premium or sub-request usage.

Task Bundle Items remain issue-first. Each item has its own status, checkpoint, outputs, rerun eligibility, and issue comments; only the active item moves to `in_progress` during sequential execution. Bundle-level UI should expose execution order, live transcript segments, aggregate usage, provider request metrics when available, item outcomes, runtime budget, and rerun scope.

Request-efficient squad leads should execute assigned work directly by default instead of stopping to assign work to squad members. Delegation is reserved for explicit user delegation, another agent's unique required capability, or a follow-up after the lead records a blocked outcome.

The rejected alternatives were hidden daemon-side batching, one provider turn per issue inside a bundle, JSON-only bundle payloads inside `agent_task_queue.context`, and parallel execution of bundled issues. Hidden batching changes task priority and status after enqueue; per-issue provider turns lose the request-efficiency benefit; JSON-only payloads cannot support durable UI, retry, output, and reverse-lookup requirements; parallel issue execution risks context, worktree, and artifact interference.

The default bundle size guardrail should be five issue work items. Larger selections should be split into multiple bundles unless a bounded deployment configuration explicitly permits a higher maximum.
