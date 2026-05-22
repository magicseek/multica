# Isolate chat structured output handoffs

## Goal

Prevent different Project/Chat sessions that share one local repository workdir from reading, deleting, or re-uploading each other's structured handoff files. Source workdirs may remain shared for local development ergonomics, but chat proposal, summary, and output metadata manifests must be isolated by chat session/task ownership.

## What I Already Know

* Project-associated chat tasks can run against a local repository binding where `env.WorkDir` is the user's actual project directory.
* Current structured handoff files live under `env.WorkDir/.multica/`: `chat-summary.json`, `issue-proposals.json`, and `outputs.json`.
* A tactical cleanup commit removes stale `.multica/*` handoff manifests before every daemon task, which prevents stale replay but can still delete another chat's in-flight manifest when chats share the same external workdir.
* DB/API proposal listing is already scoped by `chat_session_id`; the bug is in local filesystem handoff ownership.
* Codex chat env-root reuse already keys by `workspace_id + provider + chat_session_id + workdir`, so daemon-owned env roots can be session-specific even when the source workdir is shared.

## Requirements

* Do not make the source/project workdir chat-specific. Agents should keep running from the local project path when a local binding is selected.
* Do make structured chat handoff manifests chat-scoped so two chat sessions using the same local project directory do not collide.
* The daemon must read chat structured outputs from the chat-scoped location, not the legacy shared `.multica/issue-proposals.json` path.
* Cleanup must only target the current chat/task handoff location, never the shared project `.multica` root.
* Existing durable project context such as `.multica/project/*` must be preserved.
* New proposal batches in the same chat may replace/archive earlier unacted proposals within that same `chat_session_id`.
* Proposal replacement must not cross chat sessions and must not erase accepted/partially accepted user actions.
* Runtime prompt/config instructions must tell agents exactly where to write chat structured manifests.

## Acceptance Criteria

* [x] Two chat sessions sharing one external local workdir can each write proposal manifests without deleting or ingesting the other session's manifest.
* [x] A chat task that writes no fresh manifest produces no new proposals, even if the shared workdir contains a legacy `.multica/issue-proposals.json`.
* [x] Same-chat new proposal manifests supersede old pending proposals for that chat only.
* [x] Accepted, partially accepted, or otherwise user-acted proposal items are preserved and are not replaced by a later manifest.
* [x] `go test ./internal/daemon` passes for the structured output path tests.
* [x] Focused handler tests cover proposal supersede behavior by chat session.

## Out of Scope

* Moving the agent's cwd away from the selected local project path.
* Uploading file contents from output manifests.
* Redesigning the Chat Analytics UI.
* Hard-resetting or cleaning the user's project `.multica` directory.

## Technical Notes

* Likely daemon files:
  * `server/internal/daemon/daemon.go`
  * `server/internal/daemon/task_output_manifest.go`
  * `server/internal/daemon/types.go`
  * `server/internal/daemon/prompt.go`
  * `server/internal/daemon/execenv/runtime_config.go`
* Likely server files:
  * `server/internal/handler/chat_structured_outputs.go`
  * `server/pkg/db/queries/chat.sql`
  * generated sqlc output after query changes
* Spec updates should refine the current "Reused Workdir Handoff Files" guidance to distinguish shared source workdir from chat-scoped handoff ownership.
