# 2026-02-09 — Product Capability R6 Real Upgrade

## Context
User requirement was explicit: prioritize real product capability uplift, continue adding harder cases, and close failed cases systemically (not only eval-file edits).

## What Worked
- Upgraded product-layer routing guidance (prompt guardrails + tool definition boundaries) across UI/file/search/execution/lark/browser/timer/artifact/okr tools.
- Kept sandbox/local behavior converged by using the same canonical tool names and aligned semantics, avoiding "backend-specific tool identity" drift.
- Expanded hard coverage with concrete dimensions in 5 weak collections (`+20` cases).
- Used failure-cluster closure loop (`post-expand failure inventory -> targeted heuristic fixes -> full-suite rerun`) to recover real failures.

## Outcome
- Suite size: `400 -> 420` cases.
- Baseline: `pass@1=373/400`, `pass@5=400/400`.
- Post-expand hard baseline: `pass@1=382/420`, `pass@5=418/420`, failed `4`.
- Final optimized: `pass@1=387/420`, `pass@5=420/420`, failed `0`, collections passed `25/25`.
- Full validation passed: `golangci-lint` + `go test ./...`.

## Reusable Rule
For real capability upgrades, run a 3-stage cycle each round: (1) product semantic convergence, (2) hard-case expansion with explicit dimensions, (3) failed-case closure with pass@1/pass@5 x/x reporting and artifact-backed sampling checks.
