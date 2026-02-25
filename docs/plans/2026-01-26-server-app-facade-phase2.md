# Plan: Server/app facade for HTTP decoupling (Phase 2) (2026-01-26)

## Goal
- Introduce a server/app facade that removes `internal/server/http` dependencies on builtin subtask events and presentation formatting while preserving SSE payload behavior.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Current state notes
- `internal/server/http/sse_handler_render.go` checks for `*builtin.SubtaskEvent` to flatten subtask events into the SSE envelope.
- `internal/server/http/sse_handler.go` still imports `internal/presentation/formatter` (the formatter field is currently unused).
- Subagent events are forwarded directly to the broadcaster via `builtin.WithParentListener`, so subtask wrappers bypass the workflow envelope translator.

## Plan
1. Add an app-level facade (e.g., `EventPresenter` or `ToolArgsPresenter`) that exposes:
   - Subtask unwrapping via `agent.SubtaskWrapper`.
   - Optional tool-argument presentation using the presentation formatter.
2. Update `internal/server/http` SSE handling to use the facade instead of direct builtin/formatter imports.
3. Replace subtask tests with a local `agent.SubtaskWrapper` stub to keep HTTP tests independent of builtin.
4. (Optional) Enrich tool-start payloads with `arguments_preview` using the facade, keeping raw `arguments` intact.
5. Refresh refactor ledger/status docs if needed.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed; analysis captured.
