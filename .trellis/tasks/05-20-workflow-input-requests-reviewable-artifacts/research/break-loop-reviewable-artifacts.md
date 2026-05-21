# Break-loop analysis: reviewable planning artifacts were not reviewable enough

## 1. Root Cause Category

- **Category**: B - Cross-Layer Contract
- **Specific Cause**: Workflow runtime added quality evidence, input request APIs, and artifact records, but the product contract across daemon prompt, workflow schema, API response, and control UI did not require cloud-reviewable plan/design documents to render as content. Local Output Metadata and final issue comments could still look like "artifacts" even though they only contained daemon-local paths.
- **Secondary Category**: A - Missing Spec
- **Secondary Cause**: The spec said planning artifacts are reviewable, but did not explicitly forbid names-only/path-only artifact UX or require a regression that fails when artifact content is not previewable.

## 2. Why Fixes Failed

1. **Quality gate first**: Quality gate data was visible, so runs looked governed, but quality evidence is not the same as a human decision flow.
2. **Metadata confused with artifact content**: `.multica/outputs.json` preserved the local-output privacy boundary, but the UI could still present those local paths as "artifacts", making them appear reviewable when they were not.
3. **Capability hidden until used**: `input_requests.allowed` existed in step snapshots, but the run UI only highlighted open requests. Users could not see that a step had a first-class question path before the agent opened one.
4. **Diff without preview**: The viewer could request a diff for versioned artifacts, but did not render the complete latest artifact content, which made approval ergonomically incomplete.

## 3. Prevention Mechanisms

| Priority | Mechanism | Specific Action | Status |
|----------|-----------|-----------------|--------|
| P0 | UI regression | Render saved artifact content inline in the workflow run viewer and test for Markdown content visibility. | DONE |
| P0 | UX contract | Show which steps can ask workflow input requests before an open request exists. | DONE |
| P0 | Agent prompt | Tell daemon agents that `.multica/outputs.json` or a local path comment is insufficient for review/approval; use `workflow artifact save`. | DONE |
| P0 | Spec | Update workflow spec to define content preview as required and names-only artifact lists as invalid review UI. | DONE |
| P1 | Cross-layer guide | Add a reusable reviewable artifact boundary checklist. | DONE |
| P1 | E2E | Add or keep an E2E that creates v1/v2 reviewable Markdown artifacts and verifies preview plus diff in Multica. | TODO |

## 4. Systematic Expansion

- **Similar Issues**: Any local-output surface that displays filenames can be mistaken for cloud-reviewable content. Chat outputs, issue activity comments, transcript output chips, and workflow run evidence need distinct language and affordances.
- **Design Improvement**: Treat "reviewable artifact" as its own product class: server-stored content, versioned, previewable, diffable, and optionally review-gated. Treat Output Metadata as discoverability only.
- **Process Improvement**: E2E issue generation must validate the user decision path, not just the happy-path quality gate. A complete planning E2E should include: agent asks a bounded question, user answers through `Answer & continue`, agent saves plan v1, user requests changes, agent saves v2, UI shows preview and diff, user approves.
- **Knowledge Gap**: "Artifact" had two meanings: local task output metadata and workflow artifact content. Specs and UI copy must keep these separate.

## 5. Knowledge Capture

- [x] Updated `.trellis/spec/workflows/execution-workflow-definitions.md`.
- [x] Updated `.trellis/spec/guides/cross-layer-thinking-guide.md`.
- [x] Updated the current Trellis PRD acceptance criteria.
- [x] Added focused component/daemon tests for the immediate regression.
- [ ] Add a full desktop/web E2E for reviewable artifact preview, v1-to-v2 diff, and approval.
