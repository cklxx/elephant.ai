# Plan: Goroutine panic recovery + LogID chain (2026-01-26)

## Goal
- Ensure all non-test goroutines are protected by panic recovery.
- Ensure a single LogID is propagated across the main execution chain for readable logs.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Replace direct `go` spawns in non-test code with `async.Go` or add explicit recovery.
2. Guarantee LogID presence on the task execution context and pass logid-aware logger into key execution paths.
3. Update tests as needed and run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Wrapped non-test goroutine spawns with `async.Go` and added panic logger adapters for evaluation flows.
- 2026-01-26: Ensured LogID propagation in coordinator/server/CLI flows and embedded logid into LLM request IDs.
