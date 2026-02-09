# 2026-02-09 — Product Capability R7 Semantic Convergence

## Context
R7 focused on real product capability uplift under implicit prompts and tool-conflict scenarios, while preserving full-suite reliability.

## What Worked
- Used product-layer semantic convergence first (system prompt + builtin tool descriptions), not eval-only scoring tweaks.
- Kept local/sandbox tool semantics aligned so routing behavior does not diverge by execution backend.
- Applied regression-aware optimization loop: run full suite, detect lexical pollution regressions, tighten wording, rerun.
- Added routing-boundary tests to make prompt/tool semantics durable.

## Outcome
- Foundation suite improved from baseline `pass@1=387/420` to `389/420`.
- `pass@5` remained `420/420`; failed cases remained `0`; top1 misses reduced `33 -> 31`.
- Full validation passed: `golangci-lint` + `go test ./...`.

## Reusable Rule
When optimizing implicit-intent routing, avoid descriptive text that contains high-conflict tool keywords; use boundary wording that is specific but lexically minimal, and validate with full-suite delta checks.

