# 2026-02-09 — Foundation Hard Problem Expansion & Systematic Optimization R4

## Goal
- Search and introduce more genuinely hard agent evaluation problems (benchmark-inspired + project-core-goal aligned).
- Expand datasets with concrete, named examination dimensions instead of generic "hard" naming.
- Run full-suite evaluation, summarize failures systematically, and optimize routing heuristics end-to-end.

## Scope
- Add new foundation eval datasets under `evaluation/agent_eval/datasets/`.
- Add them to `foundation_eval_suite.yaml` with clear `id/name/dimension` semantics.
- Tune offline routing heuristics in `evaluation/agent_eval/foundation_eval.go` plus regression tests.
- Publish analysis report with x/x scoreboard, top failure pairs, good/bad sampling, and optimization actions.

## Checklist
- [ ] Research difficult benchmark patterns (BrowseComp / LongBench v2 / TAU-bench / WebArena-Verified / SWE-bench Verified context).
- [ ] Design and add new hard collections with explicit examination objectives.
- [ ] Run full suite baseline after expansion.
- [ ] Decompose top failures (`expected => top1`) and true fails.
- [ ] Implement targeted heuristic optimizations + tests.
- [ ] Re-run full suite and compare before/after.
- [ ] Update README/report/docs memory + good-experience records.
- [ ] Run full lint + tests.
- [ ] Commit in incremental chunks and merge back to main.
- [x] Research difficult benchmark patterns (BrowseComp / LongBench v2 / TAU-bench / WebArena-Verified / SWE-bench Verified context).
- [x] Design and add new hard collections with explicit examination objectives.
- [x] Run full suite baseline after expansion.
- [x] Decompose top failures (`expected => top1`) and true fails.
- [x] Implement targeted heuristic optimizations + tests.
- [x] Re-run full suite and compare before/after.
- [x] Update README/report/docs memory + good-experience records.
- [x] Run full lint + tests.
- [ ] Commit in incremental chunks and merge back to main.

## Progress Log
- 2026-02-09 15:20: Plan initialized, environment and memory loaded.
- 2026-02-09 15:26: Pre-expand baseline captured (`22/22`, `pass@1=526/608`, `pass@5=608/608`).
- 2026-02-09 15:30: Added three benchmark-inspired hard collections (+48 cases, suite 25 collections / 656 cases).
- 2026-02-09 15:31: Post-expand baseline dropped to `pass@1=556/656`, `pass@5=648/656`, exposing 8 true failures.
- 2026-02-09 15:34: Applied targeted routing convergence on scheduler/path-content/artifact-trace conflicts with regression assertions.
- 2026-02-09 15:34: Final run reached `25/25`, `pass@1=562/656`, `pass@5=656/656` (0 failed cases).
- 2026-02-09 15:36: Full checks passed: `golangci-lint` + `make test`.
