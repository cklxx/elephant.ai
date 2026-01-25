# Plan: Architecture + Performance Dual Track (2026-01-25)

## Goal
- Address P0 performance hotspots (JSON, caching, concurrency) and start architectural decomposition of oversized files without changing behavior.
- Keep changes small, testable, and maintainable; ship in multiple commits with full lint/test.

## Scope (Phase 1: Performance)
1. Validate hotspots
   - Confirm JSON usage in session storage + LLM parsing.
   - Confirm lock usage and goroutine lifecycle risks.
2. JSON performance
   - Introduce a dedicated fast JSON package for hot paths.
   - Migrate session stores + state store + event payload parsing to the fast JSON package.
   - Add focused tests to ensure parity.
3. LLM client cache
   - Replace unbounded map cache with LRU + TTL + health check.
   - Add tests for eviction + TTL expiry.
4. Concurrency
   - Refine toolCallBatch locks (RWMutex where safe).
   - Ensure goroutine lifecycle has cancellation/cleanup hooks where missing.

## Scope (Phase 2: Architecture)
1. Split oversized files with zero behavior change
   - seedream.go → per-tool files (text/image/vision/video/shared).
   - api_handler.go → per-resource handlers (session/task/evaluation/tool/etc.).
   - react_engine.go + react_engine_helpers.go → split tool execution + workflow + helpers.
   - execution_preparation_service.go → prep/history/analysis files.
2. Ports package restructuring
   - Split ports into subpackages by domain (llm/storage/tools/etc.).
3. DI container
   - Introduce builder-style construction with smaller init functions.

## Tests & Validation
- Add/extend unit tests before logic changes (TDD).
- Run `make fmt`, `make vet`, `make test` after each major batch.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Added LLM client cache controls (size + TTL), wiring through runtime config and DI; added eviction/TTL tests.
- 2026-01-25: Introduced jsonx wrapper on goccy/go-json and migrated session/state stores to it.
- 2026-01-25: Switched toolCallBatch attachment locking to RWMutex for better read concurrency; updated tests.
- 2026-01-25: Migrated LLM client JSON encoding/decoding to jsonx for hot paths.
- 2026-01-25: Split Seedream tools into per-domain files (common/text/image/vision/video/helpers) with no behavior change.
- 2026-01-25: Split API handler into resource-focused files (tasks, sessions, context, evaluations, misc, response) without behavior changes.
- 2026-01-25: Split react_engine_helpers into focused helper files (factory/tool-args/attachments/messages/context/world/feedback).
- 2026-01-25: Split react_engine.go into focused modules (types/constants/workflow/tool-batch/events/solve/tooling/prompts/finalize/placeholders/observe).
- 2026-01-25: Split execution_preparation_service into focused modules (analysis/attachments/history/inherited/presets/session) without behavior changes.
