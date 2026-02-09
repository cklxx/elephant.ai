# 2026-02-09 — Foundation Suite Further Prune Review (R2)

## Goal
- Further reduce suite size from 499 while keeping diagnostic diversity.
- Preserve all 25 dimensions and keep hard stress collections unchanged.
- Target around 450 cases with pass@5 full.

## Checklist
- [ ] Analyze current per-collection volume and prune candidates.
- [ ] Apply second-round caps with explicit net reduction.
- [ ] Run full suite and validate score/coverage.
- [ ] Update README and report with added/retired/net deltas.
- [ ] Run lint + tests.
- [ ] Commit and merge back to main.
- [x] Analyze current per-collection volume and prune candidates.
- [x] Apply second-round caps with explicit net reduction.
- [x] Run full suite and validate score/coverage.
- [x] Update README and report with added/retired/net deltas.
- [x] Run lint + tests.
- [ ] Commit and merge back to main.

## Progress
- 2026-02-09 18:00: Started R2 pruning review.
- 2026-02-09 18:20: Applied R2 caps and reduced total cases from 499 to 445 (net -54).
- 2026-02-09 18:20: Full suite rerun: `pass@1=375/445`, `pass@5=445/445`, failed=0.
- 2026-02-09 18:24: `golangci-lint` passed; first `make test` hit flaky supervisor timing case, rerun passed.
