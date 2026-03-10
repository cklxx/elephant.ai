# Weekly Pulse Git Enrichment Plan

Date: 2026-03-10

Scope:
- enhance `internal/app/pulse/weekly.go`
- reuse `internal/domain/signal/ports/git_signal.go`
- add git-enriched pulse tests

Plan:
1. Inspect the current weekly pulse generation flow and existing git-signal event model.
2. Add an optional git-signal dependency to the weekly pulse service.
3. Derive 7-day git metrics from normalized signal events and commit activity.
4. Render git metrics in the weekly digest without weakening existing task metrics.
5. Add tests covering git metric aggregation and digest formatting.
6. Run focused tests and lint, then mandatory review, commit, merge, and remove the worktree.
