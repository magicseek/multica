# Connector Contracts

Reusable contracts for optional external connectors that are not part of the
default public Multica surface.

## Scenario: Profile-gated task-scoped connectors

### 1. Scope / Trigger

- Trigger: adding an external integration that carries private deployment
  assumptions, user-owned credentials, project-scoped resources, or task-run
  side effects.
- Scope: provider registry, deployment env, workspace settings, encrypted
  credentials, project resources, daemon task identity, CLI commands, handler
  authorization, audit events, and frontend metadata.
- Rule: company-specific providers are optional connector providers. They must
  not be modeled as always-visible product settings, public routes, prompt
  guidance, or provider cards.

### 2. Signatures

- Env:
  - `MULTICA_CONNECTOR_PROFILES`: comma-separated profile list.
  - `CONNECTOR_CREDENTIAL_ENCRYPTION_KEY`: 32-byte base64/hex/raw key required
    by credential-bearing profiles.
  - Provider endpoint env keys are profile-owned deployment config, not user
    settings.
- Backend:
  - `connectors.NewRegistry(cfg)` registers providers only for enabled
    profiles.
  - `connectors.NewClientSet(cfg, httpClient)` creates provider adapters only
    for enabled profiles.
  - `GET /api/connectors/providers`
  - `GET /api/connectors/workspace`
  - `PUT /api/connectors/providers/{providerID}`
  - `GET /api/connectors/providers/{providerID}/credential`
  - `PUT /api/connectors/providers/{providerID}/credential`
  - `DELETE /api/connectors/providers/{providerID}/credential`
  - `POST /api/connectors/actions`
- Task command:
  - CLI command group: `multica connector ...`
  - Required daemon headers on action calls: `X-Workspace-ID`, `X-Agent-ID`,
    `X-Task-ID`.
- Database:
  - Workspace connector state: provider enablement plus JSON settings.
  - Connector credentials: `workspace_id`, `provider_id`, `owner_user_id`,
    encrypted secret fields, validation status, upstream identity metadata.
  - Agent task queue: `connector_delegated_user_id`.
  - Audit: connector capability, task, agent, delegated user, project resource,
    status, reason, metadata.

### 3. Contracts

- Provider response fields:
  - `id`, `display_name`, `profile`, `capabilities`, `resource_types`,
    `requires_user_credential`, `endpoints`, optional `remote_write_policies`.
  - Providers absent from the registry must be absent from frontend settings and
    resource forms.
- Credential request:
  - `secret`: raw user token accepted only on save/validation.
  - Raw token is never returned, logged, written to task context, written to
    `.multica/project/resources.json`, or injected into daemon environment.
- Credential response:
  - Status metadata only: `configured`, `status`, `upstream_identity`,
    `last_validated_at`, `updated_at`.
- Project resources:
  - Resource validators accept provider-specific JSON only when the provider is
    currently registered.
  - Resource payloads are scope pointers, not credentials.
- Agent runtime:
  - Runtime guidance may mention connector commands only when matching project
    resources exist.
  - Runtime resource rendering may include provider resource identifiers but
    never encrypted credential fields or raw secrets.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
|---|---|
| Profile disabled | Provider registry empty for that profile; settings section hidden; resource validators reject profile resource types; action handler returns not found/bad request |
| Profile enabled without credential key | Server startup fails closed |
| Provider endpoint env missing/invalid | Server startup fails closed for that profile |
| Credential save upstream validation fails | Return upstream/auth error; do not store token |
| Credential save succeeds | Store encrypted secret, validation status, upstream identity, timestamp |
| Action missing `X-Agent-ID` or `X-Task-ID` | Reject before upstream call |
| Agent/task mismatch or inactive task | Reject before upstream call |
| Missing `connector_delegated_user_id` | Reject before upstream call |
| Project resource missing or not on task project | Reject before upstream call |
| Provider/resource type mismatch | Reject before upstream call |
| Delegated user has no valid credential | Reject before upstream call |
| Upstream auth fails during action | Mark credential invalid and return recoverable auth error |
| External write capability disabled by workspace policy | Reject before upstream call |

### 5. Good/Base/Bad Cases

- Good: a daemon task with a captured delegated user calls
  `multica connector gitlab create-mr <resource-id>`; the server verifies the
  active task, resource, credential, and write policy, then calls the provider
  adapter server-side.
- Base: a member saves their own connector token from Settings; validation
  succeeds and the API response returns only status plus upstream identity.
- Bad: an agent tries to use a provider PAT from environment variables or a
  local git credential helper. This bypasses server authorization and must not
  be implemented.
- Bad: a provider-specific settings card renders when
  `GET /api/connectors/providers` does not list that provider.

### 6. Tests Required

- Config/profile tests:
  - disabled profile hides providers.
  - enabled profile requires encryption key and endpoint env values.
- Credential tests:
  - save validates against fake upstream.
  - auth failure rejects without storing token.
  - API responses never include raw token.
- Delegated-user tests:
  - issue assignment, human comments, agent-comment inheritance, chat tasks,
    autopilot, retry, and rerun preserve the intended delegated user.
- Resource tests:
  - profile-disabled resource types reject.
  - profile-enabled resource refs normalize and render without secrets.
- Action tests:
  - missing identity, task mismatch, inactive task, missing resource, invalid
    credential, and disabled write policy reject before upstream call.
  - fake provider servers cover validation, read/search, auth failures, and
    write guardrails.
  - Tests that seed active `agent_task_queue` rows directly for task-scoped
    connector actions must clean up the task and owning issue or use an
    isolated agent/runtime. Leaked `running` rows consume agent capacity and
    can make later daemon claim tests return `task:null`.
- Frontend tests/typecheck:
  - provider metadata gates settings and resource UI.
  - shared types stay in `packages/core`; business rendering stays in
    `packages/views`.

### 7. Wrong vs Correct

#### Wrong

```go
// Always registers a private provider, so public deployments expose it even
// when the operator never opted in.
registry.Register(ringCentralGitLabProvider)
```

```go
// Leaks user-owned PATs into the runtime and makes authorization depend on
// agent behavior.
env["RINGCENTRAL_GITLAB_TOKEN"] = credential.RawToken
```

#### Correct

```go
registry := connectors.NewRegistry(cfg)
clients := connectors.NewClientSet(cfg, http.DefaultClient)
```

```go
// The CLI carries only task identity. The server decrypts and uses the
// delegated user's credential after authorization succeeds.
req.Header.Set("X-Agent-ID", agentID)
req.Header.Set("X-Task-ID", taskID)
```
