# Daemon Runtime Contracts

## Scenario: Local daemon bridge and CLI version gates

### 1. Scope / Trigger

- Trigger: changes to daemon registration metadata, runtime metadata used by UI gates, local daemon health/bridge endpoints, or native helper actions launched from the daemon.
- Applies when modifying:
  - `DaemonRegisterRequest.cli_version`
  - runtime `metadata.cli_version`
  - daemon local `/health`
  - daemon local `/folder/select`
  - frontend quick-create preflight checks that mirror server gates
  - server quick-create gates that decide whether a daemon can run agent-created issues

### 2. Signatures

Daemon registration fields:

- `DaemonRegisterRequest.CLIVersion string json:"cli_version"`
- Runtime metadata stores the same value under `metadata.cli_version`.

Daemon local health response:

- `GET http://127.0.0.1:{health_port}/health`
- Response fields include `status`, `daemon_id`, `device_name`, `server_url`, `cli_version`, `active_task_count`, `agents`, and `workspaces`.

Daemon local folder picker bridge:

- `OPTIONS http://127.0.0.1:{health_port}/folder/select`
- `GET http://127.0.0.1:{health_port}/folder/select?daemon_id=<expected-daemon-id>`
- Success response:

```json
{
  "success": true,
  "canceled": false,
  "path": "/Users/name/project",
  "daemon_id": "daemon-id"
}
```

Quick-create CLI version gate:

- Server minimum: `agent.MinQuickCreateCLIVersion`
- Frontend mirror: `MIN_QUICK_CREATE_CLI_VERSION`
- Server checker: `CheckMinCLIVersion(detected string) error`
- Frontend checker: `checkQuickCreateCliVersion(detected?: string | null)`

### 3. Contracts

- The server gate is authoritative. Frontend preflight exists only to show an actionable error before submit.
- Frontend and server minimum constants must match when the quick-create CLI requirement changes.
- `cli_version` is a daemon-reported Multica CLI version, not the provider runtime version.
- Missing or unparsable `cli_version` fails closed for agent-created issues.
- Parsable semver below the minimum fails with a "too old" state.
- Parsable semver at or above the minimum passes.
- Source-built daemon versions pass when `cli_version` is either:
  - literal `dev` from `go run`
  - `git describe --tags --always --dirty` shape such as `v0.2.15-235-gdaf0e935`
- Do not use environment flags or desktop-only knowledge to bypass the CLI gate; the version string is the shared frontend/server signal.
- `/folder/select` is a local bridge endpoint only. It must run on the daemon host and return a daemon-readable absolute path selected by an OS-native picker.
- `/folder/select` may accept an expected `daemon_id`; when it does not match the running daemon, return a non-success response and do not open the native picker.
- Browser-only directory handles, selected folder names, and browser-relative paths are not valid `local_dir` binding paths.
- A canceled folder picker is not an error and must not create a repository or binding.
- Local bridge CORS is limited to localhost origins, production Multica origins, and explicit environment allowlists; include `Access-Control-Allow-Private-Network` for browser private-network preflights.
- Reusable daemon workdirs may preserve source files, checkouts, and project context, but task-scoped structured output manifests are single-run handoff files.
- Before spawning each task agent, the daemon must remove stale `.multica/chat-summary.json`, `.multica/issue-proposals.json`, and `.multica/outputs.json` from the selected workdir.
- Structured output manifest cleanup must not remove durable project context such as `.multica/project/resources.json` or other `.multica/project/*` files.
- A task that does not write a fresh structured output manifest must complete with no structured outputs, rather than re-uploading a previous chat or task's proposals.

### 4. Validation & Error Matrix

- Missing `cli_version` for quick-create -> frontend `missing`; server `ErrCLIVersionMissing`.
- Unparsable `cli_version` for quick-create -> frontend `missing`; server `ErrCLIVersionMissing`.
- `cli_version=0.2.19` when minimum is `0.2.20` -> frontend `too_old`; server `ErrCLIVersionTooOld`.
- `cli_version=0.2.20` -> allowed.
- `cli_version=dev` -> allowed for source-built local development.
- `cli_version=v0.2.15-235-gdaf0e935-dirty` -> allowed for source-built local development.
- `/folder/select` from a disallowed origin -> `403`.
- `/folder/select` with method other than `GET` or `OPTIONS` -> `405`.
- `/folder/select?daemon_id=other` -> `409` with `error=daemon_id_mismatch`; native picker must not open.
- Native picker unavailable or fails -> `500` with `success=false`.
- User cancels native picker -> `200`, `success=true`, `canceled=true`, empty path.

### 5. Good/Base/Bad Cases

- Good: desktop uses native IPC to pick a local folder and receives an absolute path.
- Good: web uses the selected runtime's daemon health port to call `/folder/select`, verifies the daemon id, and only then submits a `local_dir` binding.
- Good: local development daemon reporting `dev` can create agent issues after passing both frontend and server gates.
- Base: release daemon reporting `v0.2.20` passes the quick-create gate.
- Bad: web calls `showDirectoryPicker()` and submits the folder handle name as `local_path`.
- Bad: frontend treats missing `cli_version` as OK while the server rejects it.
- Bad: server bypasses `CheckMinCLIVersion` based on request origin, desktop app presence, or local environment variables.
- Bad: `/folder/select` opens the native picker before checking `daemon_id`.
- Bad: a reused local workdir still contains `.multica/issue-proposals.json` from a prior chat and the daemon uploads it as the current chat's proposal set.

### 6. Tests Required

- Server version gate tests:
  - missing/unparsable version returns `ErrCLIVersionMissing`
  - below-minimum semver returns `ErrCLIVersionTooOld`
  - minimum and higher semver pass
  - `dev` and git-describe source builds pass
- Frontend version gate tests mirror the same cases and states.
- Daemon health tests verify snake_case response keys including `cli_version` and `active_task_count`.
- Daemon folder bridge tests:
  - allowed local origin sets CORS/private-network headers
  - disallowed origin returns `403`
  - `OPTIONS` returns `204`
  - daemon id mismatch returns `409` and does not invoke the picker
  - cancel returns success with `canceled=true`
- View tests verify a browser-only handle path is not submitted and a daemon/native path is submitted as an inline local binding.
- Daemon structured output tests verify stale task manifests are removed before reuse while `.multica/project/*` context is preserved.

### 7. Wrong vs Correct

#### Wrong

```typescript
const handle = await window.showDirectoryPicker();
await createRepository({ source_state: "local_dir", binding: { local_path: handle.name } });
```

This submits a browser-only name that the selected daemon cannot read.

#### Correct

```typescript
const picked = await pickLocalDirectory({ runtime, desktopAPI });
if (picked.kind === "selected") {
  await createRepository({
    source_state: "local_dir",
    binding: { local_path: picked.path, runtime_id: runtime.id, daemon_id: runtime.daemon_id },
  });
}
```

The submitted path came from a native/daemon bridge and is scoped to the runtime that will use it.

#### Wrong

```go
if detected == "" {
    return nil
}
```

This lets older daemons run quick-create flows that rely on newer CLI behavior.

#### Correct

```go
if err := agent.CheckMinCLIVersion(detected); err != nil {
    return err
}
```

Only semver-compliant releases at the minimum or source-built dev signals pass.
