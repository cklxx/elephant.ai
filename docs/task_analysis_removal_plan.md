# Task Analysis Removal Checklist

This document enumerates the changes required to fully remove the **task analysis** feature across the backend, streaming layer, frontend, docs, and tests.

## Current Touchpoints
- **Backend event + emission**: `TaskAnalysisEvent` is defined and cloned from LLM analysis results, then emitted before task execution when present.【F:internal/agent/domain/events.go†L52-L90】【F:internal/agent/app/coordinator.go†L163-L170】
- **Analysis service**: The pre-analysis service has been removed, so no task analysis payload is generated before execution.
- **Renderers / stream payloads**: Renderer interfaces no longer include task analysis formatting or event types.
- **Frontend behavior**: Conversation and rendering flows no longer rely on `task_analysis` events or the removed `TaskAnalysisCard` component.
- **Documentation**: SSE guides and examples have been updated to omit `task_analysis`.
- **Tests**: Acceptance expectations should no longer require `task_analysis` in streamed event types.

## Removal Steps
1. **Disable generation**
   - Remove `TaskAnalysisService` invocation during task preparation and any storage of its results so no analysis is produced.【F:internal/agent/app/task_analysis_service.go†L13-L127】
   - Delete the `TaskAnalysisEvent` type and its creation path, updating the coordinator to stop emitting pre-execution analysis events.【F:internal/agent/domain/events.go†L52-L90】【F:internal/agent/app/coordinator.go†L163-L170】

2. **Simplify renderers and interfaces**
   - Drop `RenderTaskAnalysis` from the renderer interface and concrete implementations (CLI, SSE, LLM/TUI), and adjust any callers expecting that method.【F:internal/output/renderer.go†L19-L47】【F:internal/output/cli_renderer.go†L94-L169】【F:internal/output/sse_renderer.go†L37-L87】
   - Remove styling/helpers dedicated to task analysis output (e.g., purple gradient header, success criteria sections) once the interface no longer exposes the event.【F:internal/output/cli_renderer.go†L94-L169】

3. **Prune streaming + HTTP surface area**
   - Delete SSE event wiring that maps task analysis payloads into event streams and ensure HTTP handlers/tests no longer expect that event type.【F:internal/output/sse_renderer.go†L37-L87】【F:tests/acceptance/sse_test.sh†L221-L262】
   - Update public SSE documentation/examples to omit the `task_analysis` event and renumber examples as needed.【F:docs/guides/SSE_SERVER_GUIDE.md†L61-L79】

4. **Remove frontend usage**
   - Strip session auto-naming based on `task_analysis` and any state derived from that event in the conversation page store/hooks.【F:web/app/conversation/ConversationPageContent.tsx†L95-L119】
   - Delete `TaskAnalysisCard` rendering branches and related styles/mocks/tests so the event list handles only remaining event types.【F:web/components/agent/EventLine/index.tsx†L139-L145】

5. **Clean up supporting assets**
   - Remove mock events, schemas/type guards, and analytics references that include `task_analysis` once the event is gone from the stream.
   - Drop acceptance/unit tests focused on the analysis event or rewrite them to cover the remaining lifecycle.
   - Sweep docs/README snippets that list `task_analysis` to keep public surface accurate.【F:docs/guides/SSE_SERVER_GUIDE.md†L61-L79】

6. **Migration + rollout**
   - Validate SSE streams and UI flows still function without the event; adjust client fallbacks where they relied on it for session naming or ordering.
   - Run acceptance tests that cover SSE streaming to ensure removal doesn’t regress other event types, updating expectations accordingly.【F:tests/acceptance/sse_test.sh†L221-L262】

By following these steps, the codebase will no longer generate, transport, or render task analysis data, and external docs/tests will align with the trimmed event set.
