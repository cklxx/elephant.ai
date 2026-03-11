# Blocker Pulse Coverage Plan

Date: 2026-03-11

Scope:
- inspect coverage for `./internal/app/blocker` and `./internal/app/pulse`
- identify important uncovered execution paths
- add focused tests for the missing critical branches
- run relevant validation and commit the scoped changes

Plan:
1. Run `go test -cover` for `internal/app/blocker` and `internal/app/pulse`.
2. Inspect existing tests and source to find the highest-signal uncovered paths.
3. Add minimal unit tests for those critical branches.
4. Re-run the two package suites, then run scoped Go lint and code review.
5. Commit only the plan doc and the new test changes.
