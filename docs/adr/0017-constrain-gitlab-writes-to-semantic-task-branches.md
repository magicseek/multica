# Constrain GitLab writes to semantic task branches

RingCentral GitLab write capabilities should be limited to attached `ringcentral_gitlab_repo` project resources and task-scoped Semantic Task Branches. Branches should use intent prefixes such as `feat/`, `fix/`, `refactor/`, `docs/`, `test/`, `chore/`, or `perf/`, followed by issue or task identifiers and a short slug for traceability. The MVP must not use a product-name prefix such as `multica/`.

GitLab writes may create a semantic task branch, commit or push changes to that branch, and create a merge request from it. They must not write directly to default or protected branches, merge merge requests, delete branches, modify GitLab project settings, or modify protected branch rules.

GitLab writes also obey the workspace connector's Remote Write Policy. When ordinary remote writes are disabled, the adapter rejects generic branch writes and may allow only explicitly whitelisted merge-request preparation operations.

The rejected alternative was to use a fixed product prefix for all branches. Product-prefixed branches make the branch namespace less expressive, while semantic prefixes better communicate the work intent and align with conventional commit-style development workflows.
