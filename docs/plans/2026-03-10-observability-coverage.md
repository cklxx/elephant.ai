# Observability Coverage Plan

Date: 2026-03-10

Scope:
- add focused unit tests in `internal/infra/observability/observability_coverage_test.go`
- improve coverage for bootstrap fallback paths, instrumentation, metrics hooks, and redaction logic

Plan:
1. Inspect `observability.go`, `instrumentation.go`, `metrics.go`, and existing tests.
2. Add table-driven tests for `New()` disabled and invalid metrics/tracing cases.
3. Add `InstrumentedLLMClient` success/error tests using a stub client and metrics hooks.
4. Add HTTP/SSE/task metrics tests for labels, zero/negative size handling, and hook invocation.
5. Add recursive redaction tests for nested tool args and API-key heuristics.
6. Run focused `go test`, run code review, commit, rebase/merge to `main`, and remove the worktree.
