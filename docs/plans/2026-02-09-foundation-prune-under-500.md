# 2026-02-09 — Foundation Suite Prune to <500 Cases

## Goal
- Reduce foundation suite total case count to below 500 as requested.
- Prefer retiring low-signal / repeatedly easy / semantically redundant cases.
- Keep hard conflict coverage and full pass@5 stability.

## Checklist
- [ ] Capture current baseline size and score.
- [ ] Produce per-collection case counts and identify prune targets.
- [ ] Apply systematic pruning (net negative, explicit rationale).
- [ ] Re-run full suite and verify cases <500.
- [ ] Update README/report/plan records.
- [ ] Run full lint + tests.
- [ ] Commit incrementally and merge to main.
- [x] Capture current baseline size and score.
- [x] Produce per-collection case counts and identify prune targets.
- [x] Apply systematic pruning (net negative, explicit rationale).
- [x] Re-run full suite and verify cases <500.
- [x] Update README/report/plan records.
- [x] Run full lint + tests.
- [ ] Commit incrementally and merge to main.

## Progress
- 2026-02-09 15:45: Worktree created, memory/practices loaded.
- 2026-02-09 17:51: Baseline captured at `656/656`, `pass@1=562/656`, `pass@5=656/656`.
- 2026-02-09 17:51: Applied per-collection caps and retired 157 cases.
- 2026-02-09 17:51: Final rerun reached `499/499`, `pass@1=426/499`, `pass@5=499/499`.
- 2026-02-09 17:54: `golangci-lint` passed; first `make test` hit flaky supervisor timing case, rerun passed.
