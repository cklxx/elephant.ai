# 2026-02-09 — Product Capability Upgrade + Dataset Expansion (R6)

## Goal
- Improve real product tool-routing capability (runtime prompt + tool definitions), not only eval heuristics.
- Expand hard evaluation cases for low-score collections and re-score.
- Optimize low-score clusters with product-facing changes and verify with suite metrics.

## Checklist
- [x] Capture baseline metrics and low-score collections.
- [x] Upgrade product routing guidance (system prompt + core tool descriptions/tags).
- [x] Add more hard cases in low-score collections with explicit coverage labels.
- [x] Run suite and compare pass@1/pass@5 and low-score collection deltas.
- [x] Update docs/report with x/x metrics and artifact paths.
- [x] Run lint + tests and commit incremental slices.

## Progress
- 2026-02-09 19:23: Started R6 product capability upgrade cycle.
- 2026-02-09 19:28: Captured baseline suite: `pass@1=373/400`, `pass@5=400/400`, failed `0`.
- 2026-02-09 19:34: Completed first round product-side tool-boundary convergence and expanded 5 low-score datasets (`+20` cases).
- 2026-02-09 19:35: Post-expand run exposed hard pressure and true failures: `pass@1=382/420`, `pass@5=418/420`, failed `4`.
- 2026-02-09 19:36: Failure-closure round finished: `pass@1=387/420`, `pass@5=420/420`, failed `0`.
- 2026-02-09 19:39: Full lint + full test pass completed (`golangci-lint` + `go test ./...`).
