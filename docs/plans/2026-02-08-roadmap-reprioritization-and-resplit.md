# Plan: Roadmap Reprioritization and Task Resplit (2026-02-08)

## Goal
- Reconcile all roadmap sources into one execution view.
- Re-evaluate each unfinished item by contribution to the core product goals (WTCR, TimeSaved, Accuracy).
- Re-split pending work into executable batches with clear dependencies and Definition of Done.

## Scope
- `docs/roadmap/roadmap.md`
- `docs/roadmap/roadmap-pending-2026-02-06.md`
- `docs/roadmap/draft/*.md` (reference only; identify stale/conflicting status)
- `docs/memory/long-term.md` (daily refresh requirement)

## Evaluation Method
- Use `docs/roadmap/roadmap.md` as status source-of-truth.
- Score unfinished work on:
  - North-star impact (Calendar + Tasks closed loop, WTCR/TimeSaved/Accuracy)
  - Dependency unblock value
  - Delivery risk if delayed
  - Estimated effort (for batch sizing only)
- Priority decision favors high impact + high unblock + high risk-of-delay with feasible effort.

## Checklist
- [x] Load engineering practices + active memory context.
- [x] Inventory roadmap files and detect conflicts.
- [x] Add consolidated priority evaluation to main roadmap.
- [x] Publish new pending backlog file with re-split executable batches.
- [x] Refresh long-term memory timestamp/date for first load of day.
- [x] Run lint + tests.
- [ ] Commit in incremental steps.
- [ ] Merge branch back to `main` (fast-forward) and remove temporary worktree.

## Notes
- Historical roadmap drafts remain reference material and are not status authority.
- Old pending snapshot (`2026-02-06`) is preserved for traceability.

## Validation
- `./dev.sh lint` failed on pre-existing `errcheck/unused` issues in `evaluation/*` and `internal/delivery/eval/http/api_handler_rl.go` (no changes from this task).
- `./dev.sh test` passed.
