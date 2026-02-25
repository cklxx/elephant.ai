# Tool Retry Middleware X3 (Retry + Circuit Breaker + Registry Wiring)

Created: 2026-02-01
Owner: cklxx
Status: done

## Goals
- Add global tool retry middleware with exponential backoff + jitter.
- Add lightweight per-tool circuit breaker (open/half-open/close).
- Wire tool policy config into tool registry wrapper chain.
- Add unit tests for retry + breaker behavior.

## Plan
- [x] Implement retry + circuit breaker middleware in `internal/toolregistry`.
- [x] Wire tool policy into tool registry creation and wrapper chain.
- [x] Add retry/breaker tests.
- [x] Run full lint/tests; restart dev (`./dev.sh down && ./dev.sh`).

## Progress Log
- 2026-02-01: plan created.
- 2026-02-01: retry + breaker middleware wired with tests; toolregistry tests passing.
