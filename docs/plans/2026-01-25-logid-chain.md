# Plan: LogID Correlation & Log Readability (2026-01-25)

## Goal
- Introduce a logid that can correlate request → task → LLM/tool execution logs with minimal behavior change.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Add logid generation + context helpers in `internal/utils/id`.
2. Extend logging adapters to include logid in log lines when present.
3. Seed logid at HTTP entrypoints (middleware) and propagate via context.
4. Reuse logid as LLM request_id where available for correlation.
5. Add tests for logid propagation and logging format.
6. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Added logid context helpers + generator and logging adapters.
- 2026-01-25: Added HTTP middleware logid propagation + LLM request id reuse.
- 2026-01-25: Propagated logid-aware logging in server coordinator + SSE; added middleware tests.
