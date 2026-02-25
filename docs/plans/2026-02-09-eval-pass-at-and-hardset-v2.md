# 2026-02-09 Eval Pass@1/Pass@5 + Hardset V2 Plan

**Status**: In Progress  
**Branch**: `feat/eval-pass-metrics-hardset-20260209`  
**Updated**: 2026-02-09

## Goals
- Standardize offline foundation metrics around `pass@1` and `pass@5`.
- Add explicit top1-failure analysis in suite outputs.
- Expand evaluation collections with harder scenarios that stress tool disambiguation, memory/persona continuity, long-horizon autonomy, and architecture-heavy coding intent routing.
- Keep reporting in `x/x` format per collection and suite-level totals.

## Execution Steps
- [x] Baseline run on current suite and extract weakest top1 collections/cases.
- [x] Implement metric model changes (`pass@1`, `pass@5`) and backward-compatible aliases.
- [x] Update CLI/log/markdown/json reporting to prioritize pass@1/pass@5 and include top1-failure leaderboard.
- [x] Add/expand hard scenario datasets and include them in suite.
- [x] Add tests for new metrics/report fields and case counting.
- [x] Run focused tests and rerun suite for updated scorecard.
- [x] Run full tests (`make test`) and lint where available (`make fmt`); web lint blocked locally due missing `eslint`.
- [ ] Update memory/docs records and commit in incremental commits.

## Baseline Snapshot (before changes)
- Suite: `17/17` collections passed, `408/408` cases passed (Top-K with K=3).
- Avg top1 hit rate: `0.746`; avg top-k hit rate: `1.000`.
- Weakest top1 collections: memory-capabilities, long-horizon-multi-round, autonomy-initiative, user-habit-soul-memory.
- Representative top1-miss patterns:
  - `memory_get` often ranked below `memory_search` for evidence-open intents.
  - `request_user` often ranked below `clarify` for user-gated action intents.
  - `ripgrep` often ranked below `search_file` for repository scan intents.

## Notes
- `availability_error` remains non-failure per project rule when tool is unavailable (N/A).
- Hardset expansion should intentionally include some unresolved challenge cases to avoid saturated scores.
