# Plan: Split SSE Handler (2026-01-25)

## Goal
- Reduce `internal/server/http/sse_handler.go` size by splitting into focused files with no behavior changes.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Extract cache/LRU helpers into `sse_handler_cache.go`.
2. Move stream processing helpers into `sse_handler_stream.go`.
3. Keep constructor/options/types in `sse_handler.go`.
4. Keep event rendering helpers in `sse_handler_render.go`.
5. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Split SSE handler into cache/stream/render files; ran fmt/vet/test.
