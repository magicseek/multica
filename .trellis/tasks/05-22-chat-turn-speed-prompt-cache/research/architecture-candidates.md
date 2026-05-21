# Architecture candidates for chat turn speed and prompt cache reuse

This research artifact records the seven deepening candidates selected before implementation.

## 1. Chat Turn Context Module

Bind each queued Chat Session task to the exact triggering user message and attachments, or provide a dedicated claim query that avoids loading and scanning the full Chat Session transcript. This reduces DB payload, avoids mismatching rapid queued messages, and keeps the chat prompt small and stable.

## 2. Provider Session Runner Module

The current provider interface is per-task. Codex starts `codex app-server --listen stdio://` for every task even when `PriorSessionID` resumes the thread. Introduce a safe long-lived runner path for Codex first, with fallback to per-task execution.

## 3. Runtime Brief / Prompt Cache Module

`buildMetaSkillContent` currently mixes stable CLI/runtime guidance with per-turn dynamic facts. Split stable references, provider overlays, task-mode overlays, and dynamic task facts so prompt cache locality improves and inline providers do not receive unrelated bulk.

## 4. Codex Home Snapshot Module

Reuse CODEX_HOME and local Repository Binding env roots safely across Chat Session turns. Avoid repeated user skill copying and repeated setup while preserving auth/config/plugin cache freshness.

## 5. Chat Turn Client Module

Share optimistic Chat Session send logic across floating and page-level chat surfaces. Keep React Query as the server-state owner and reduce repeated refetches after each send.

## 6. Workflow Step Context Module

Give agents a current Workflow Step context instead of requiring each turn to fetch and infer from full Workflow Run state. Preserve server-owned Workflow Runs and immutable Workflow Snapshots.

## 7. Prompt Cache Observability Module

Record prompt sizes, stable/dynamic hashes, runner/env reuse flags, resume outcome, and cache usage so performance and cache-rate optimizations can be measured and attributed correctly.
