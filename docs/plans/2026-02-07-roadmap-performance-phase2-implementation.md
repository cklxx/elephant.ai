# Plan: Roadmap Performance Phase-2 Implementation (2026-02-07)

## Goal
- Add regression coverage for SLA-aware degradation routing in tool registry execution flow.
- Validate fallback attribution metadata and call-name rewrite semantics for downstream SLA accounting.

## Scope
- `internal/app/toolregistry/degradation_test.go`
- `internal/app/toolregistry/registry_test.go` (only if required for coverage wiring)
- This phase plan doc and validation log.

## Checklist
- [x] Review current degradation/registry implementation and existing tests.
- [x] Confirm expected phase-2 behavior from worker-A plan artifact.
- [x] Add regression test: fallback order is SLA-ranked when router configured.
- [x] Add regression test: pre-route can select healthier fallback when primary is unhealthy.
- [x] Add regression test: fallback execution rewrites `call.Name` for SLA attribution.
- [x] Ensure tests remain deterministic (fixed SLA samples, no time-based flakiness).
- [x] Run `gofmt` on touched Go files.
- [x] Run `go test ./internal/app/toolregistry`.
- [x] Commit changes.

## Progress Notes
- 2026-02-07: Loaded engineering practices and active memory guidance.
- 2026-02-07: Synced with worker-A changes and verified SLA-aware degradation hooks are present (`SLARouter`, `PreRouteWhenPrimaryUnhealthy`, fallback call-name rewrite).
- 2026-02-07: Validated regression coverage in `degradation_test.go` for SLA-ranked fallback order, unhealthy-primary pre-route, and fallback `call.Name` rewrite.
- 2026-02-07: Tightened metadata assertions (`degraded_from` / `degraded_to`) in SLA-ranked and call-rewrite regression cases.

## Validation
- `gofmt -w internal/app/toolregistry/degradation_test.go`: passed.
- `go test ./internal/app/toolregistry`: passed (`ok alex/internal/app/toolregistry`), with pre-existing macOS cgo sqlite deprecation warnings.
- Notes / follow-ups: commit only worker-B owned artifacts (`degradation_test.go` + this phase plan doc); keep unrelated worker-A files untouched.
