# Cross-Layer Thinking Guide

> **Purpose**: Think through data flow across layers before implementing.

---

## The Problem

**Most bugs happen at layer boundaries**, not within layers.

Common cross-layer bugs:
- API returns format A, frontend expects format B
- Database stores X, service transforms to Y, but loses data
- Multiple layers implement the same logic differently

---

## Before Implementing Cross-Layer Features

### Step 1: Map the Data Flow

Draw out how data moves:

```
Source → Transform → Store → Retrieve → Transform → Display
```

For each arrow, ask:
- What format is the data in?
- What could go wrong?
- Who is responsible for validation?

### Step 2: Identify Boundaries

| Boundary | Common Issues |
|----------|---------------|
| API ↔ Service | Type mismatches, missing fields |
| Service ↔ Database | Format conversions, null handling |
| Backend ↔ Frontend | Serialization, date formats |
| Component ↔ Component | Props shape changes |

### Step 3: Define Contracts

For each boundary:
- What is the exact input format?
- What is the exact output format?
- What errors can occur?

---

## Common Cross-Layer Mistakes

### Mistake 1: Implicit Format Assumptions

**Bad**: Assuming date format without checking

**Good**: Explicit format conversion at boundaries

### Mistake 2: Scattered Validation

**Bad**: Validating the same thing in multiple layers

**Good**: Validate once at the entry point

### Mistake 3: Leaky Abstractions

**Bad**: Component knows about database schema

**Good**: Each layer only knows its neighbors

### Mistake 4: Client-Seeded State Treated As Authoritative

**Bad**: A client creates an entity with a temporary or guessed title, then the
server refuses to replace it when the authoritative event arrives.

**Good**: Store provenance with generated fields and let the server-owned event
win exactly once. For chat sessions, `title_source` distinguishes user-renamed
titles from first-message titles, agent summaries, and legacy seeds.

Checklist:

- [ ] Identify whether a field is a durable user choice or a provisional client seed.
- [ ] Store provenance for generated fields (`*_source`, owner id, or
  equivalent) before adding overwrite rules.
- [ ] Patch every active cache shape that can display the field, including
  filtered lists and detail queries.
- [ ] Preserve explicit user edits even when backend summaries or first-message
  derivation arrive later.

---

## Checklist for Cross-Layer Features

Before implementation:
- [ ] Mapped the complete data flow
- [ ] Identified all layer boundaries
- [ ] Defined format at each boundary
- [ ] Decided where validation happens

After implementation:
- [ ] Tested with edge cases (null, empty, invalid)
- [ ] Verified error handling at each boundary
- [ ] Checked data survives round-trip

---

## Browser-To-Daemon Local Path Boundary

Local directory binding crosses a browser security boundary and a machine
boundary. A browser folder picker is not a daemon-readable path picker.

### Contract

- Browser APIs such as `showDirectoryPicker()` and `<input webkitdirectory>`
  may let the page read selected file handles or relative paths, but they do
  not expose a trustworthy absolute filesystem path.
- A `local_dir` repository binding requires an absolute path that the selected
  daemon/runtime can read. The path must be produced or confirmed by a trusted
  component on that same machine.
- Valid sources for a binding path:
  - desktop native picker through Electron main/preload IPC;
  - local daemon/native helper on `127.0.0.1` that opens an OS picker and
    returns the selected path to an authenticated caller;
  - server-mediated daemon operation that asks the target daemon to pick or
    confirm the path;
  - explicit manual path entry, with the UI making clear that the path is from
    the selected runtime's machine.
- Invalid source: a pure browser `FileSystemDirectoryHandle`, selected folder
  name, or `webkitRelativePath` root.

### Checklist: Before Implementing Local Folder Selection

- [ ] Identify which machine owns the selected runtime/daemon.
- [ ] Confirm the folder picker runs on that machine, not merely in the user's
  current browser tab.
- [ ] Treat the local path as private binding data; never include it in
  workspace-wide realtime events or non-owner responses.
- [ ] Disable or redirect the picker when the selected runtime has no reachable
  path-picking capability.
- [ ] Add a regression proving a browser-only directory handle does not create
  a `local_dir` binding.

**Reference pattern**: ai-desk uses a daemon-assisted picker. The web service
first calls its daemon `browseFolder()` endpoint; only when that local daemon
path fails does it fall back to browser/manual handling. The browser picker is
not treated as the source of the absolute repository path.

---

## Reviewable Artifact Boundary

Reviewable workflow artifacts cross a privacy and product-contract boundary.
Local task outputs are discoverability metadata; workflow artifacts are
server-stored content that can be previewed, reviewed, approved, and diffed.

### Contract

- `.multica/outputs.json` and final issue comments may list local files, but
  they do not upload content and cannot support cloud review.
- Any design, plan, brainstorming note, requirements doc, or decision record
  that needs human approval must be saved with `workflow artifact save`.
- The server response must include the complete latest artifact content for
  supported text kinds (`markdown`, `text`, `json`), not only metadata.
- The UI must render the full latest artifact content inline and expose a
  version diff when more than one version exists.
- A quality gate is not a substitute for human review. Quality evidence says
  whether an artifact passed checks; review controls let a human approve,
  reject, or request a revised version.
- If an agent can continue only after a human decision, it should open a
  workflow input request where the step policy allows it. It should not leave
  a normal comment and hope the next reply is interpreted as a workflow answer.

### Checklist: Before Shipping Reviewable Planning Workflows

- [ ] Identify whether the user needs to review content, not just know that a
  file exists.
- [ ] Confirm the agent-facing prompt names the exact `workflow artifact save`
  call and logical artifact name.
- [ ] Confirm saved artifact content appears in the workflow run response.
- [ ] Confirm the control surface renders the content preview and version diff.
- [ ] Add an E2E or component regression that fails when only a local path or
  artifact count is visible.
- [ ] Verify normal comments remain separate from `Answer & continue` input
  request answers.

---

## Reused Workdir Handoff Files

Reusable daemon workdirs cross a filesystem, daemon, server, and UI boundary.
Files that look local can become server state if the daemon uploads them on task
completion.

### Contract

- Separate durable workspace state from single-run handoff files.
- Durable project context can persist across runs, for example
  `.multica/project/*`.
- Structured task outputs are single-run handoff files. Non-chat tasks use
  legacy paths such as `.multica/chat-summary.json`,
  `.multica/issue-proposals.json`, and `.multica/outputs.json`.
- Chat tasks that share a source workdir must isolate handoff files under
  `.multica/chats/<chat_session_id>/` and expose that directory to the agent
  as `MULTICA_STRUCTURED_OUTPUT_DIR`.
- Pre-run cleanup must target only the current handoff location. It must not
  remove durable `.multica/project/*` files, legacy shared manifests when the
  current task is chat-scoped, or sibling chat-session handoff directories.
- If a task agent does not write a fresh handoff file, the server should receive
  no data for that handoff type. It must not infer freshness from file presence
  alone in a reused directory.
- Daemon filesystem isolation is necessary but not sufficient. The server must
  still validate uploaded manifests against authoritative state because a
  running desktop dev build may lag behind source changes and continue using
  legacy root handoff paths.
- For Project chat issue proposals, validate proposal item identity at the
  Project boundary. A current-chat manifest is stale if its normalized item
  titles already exist as non-cancelled Project issues or as proposal items in
  sibling Project chats.

### Checklist: Before Adding Agent-Written Files

- [ ] Decide whether the file is durable context or a single-run handoff.
- [ ] If it is a handoff, decide whether its ownership key is task, chat
  session, issue, project, or repository before choosing a filesystem path.
- [ ] If it is a handoff, add pre-run cleanup in the daemon before agent spawn,
  scoped to that ownership key.
- [ ] Add a regression that reuses a workdir containing stale handoff files and
  proves stale data is not uploaded.
- [ ] Verify cleanup preserves durable `.multica/project/*` context.
- [ ] Add a server-side regression for stale-but-valid manifests reaching the
  API despite daemon isolation.
- [ ] When validating a desktop dev build, verify the running daemon
  `cli_version` and bundled CLI behavior, not only the Go source diff.

---

## Cross-Platform Template Consistency

In Trellis, command templates (e.g., `record-session.md`) exist in **multiple platforms** with identical or near-identical content. This is a cross-layer boundary.

### Checklist: After Modifying Any Command Template

- [ ] Find all platforms with the same command: `find src/templates/*/commands/trellis/ -name "<command>.*"`
- [ ] Update all platform copies (Markdown `.md` and TOML `.toml`)
- [ ] For Gemini TOML: adapt line continuations (`\\` vs `\`) and triple-quoted strings
- [ ] Run `/trellis:check-cross-layer` to verify nothing was missed

**Real-world example**: Updated `record-session.md` in Claude to use `--mode record`, but forgot iFlow, Kilo, OpenCode, and Gemini — caught by cross-layer check.

---

## Generated Runtime Template Upgrade Consistency

Some generated files are both documentation and runtime input. In Trellis,
`.trellis/workflow.md` is parsed by `get_context.py`, `workflow_phase.py`,
SessionStart filters, and per-turn hooks. Template changes must be validated
against both fresh init and upgrade paths.

### Checklist: After Modifying A Runtime-Parsed Template

- [ ] Identify every runtime parser that reads the template, not just the file
  writer that installs it
- [ ] Check whether relevant syntax lives outside obvious managed regions
  such as tag blocks
- [ ] Verify fresh `init` output and a versioned `update` scenario that writes
  the older `.trellis/.version`
- [ ] Add an upgrade regression using an older pristine template fixture, then
  assert the installed file reaches the current packaged shape
- [ ] Update the backend spec that owns the runtime contract

**Real-world example**: Codex inline mode changed workflow platform markers from
`[Codex]` / `[Kilo, Antigravity, Windsurf]` to `[codex-sub-agent]` /
`[codex-inline, Kilo, Antigravity, Windsurf]`. Fresh init was correct, but
`trellis update` only merged `[workflow-state:*]` blocks and preserved stale
markers outside those blocks. Result: upgraded projects got new hook scripts
but old workflow routing, so `get_context.py --mode phase --platform codex`
could return empty Phase 2.1 detail.

---

## Mode-Detection Probe Checklist

When a CLI auto-detects a mode by probing a remote resource (e.g., checking if `index.json` exists to decide marketplace vs direct download):

### Before implementing:
- [ ] Probe runs in **ALL** code paths that use the result (interactive, `-y`, `--flag` combos)
- [ ] 404 vs transient error are distinguished — don't treat both as "not found"
- [ ] Transient errors **abort or retry**, never silently switch modes
- [ ] Shared state (caches, prefetched data) is **reset** when context changes (e.g., user switches source)
- [ ] **Shortcut paths** (e.g., `--template` skipping picker) must have the same error-handling quality as the probed path — check that downstream functions don't call catch-all wrappers

### After implementing:
- [ ] Trace every path from probe result to the mode-decision branch — no fallthrough
- [ ] External format contracts (giget URI, raw URLs) are tested or at least documented as comments
- [ ] Metadata reads consume a complete response or use a streaming parser — never parse a fixed-size prefix as full JSON
- [ ] When reconstructing a composite identifier from parsed parts, verify **all** fields are included and in the **correct position** (e.g., `provider:repo/path#ref` not `provider:repo#ref/path`)
- [ ] Verify that **action functions** called after a shortcut don't internally use the old catch-all fetch — they must use the probe-quality variant when error distinction matters

**Real-world example**: Custom registry flow had 8 bugs across 3 review rounds: (1) probe only ran in interactive mode, (2) transient errors fell through to wrong mode, (3) giget URI had `#ref` in wrong position, (4) prefetched templates leaked across source switches, (5) `--template` shortcut bypassed probe but `downloadTemplateById` internally used catch-all `fetchTemplateIndex`, turning timeouts into "Template not found".

**Real-world example**: Agent-session update hints fetched npm `latest` metadata with `response.read(4096)` and then parsed it as complete JSON. The `@mindfoldhq/trellis` package metadata exceeded 4 KB, so the JSON was truncated, parse failed silently, and the first session injection showed no update hint. Fix: read the complete response before parsing, and add a regression where `version` is followed by an 8 KB metadata tail.

---

## When to Create Flow Documentation

Create detailed flow docs when:
- Feature spans 3+ layers
- Multiple teams are involved
- Data format is complex
- Feature has caused bugs before
