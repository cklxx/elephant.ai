# App Coverage Test Plan

Date: 2026-03-11

Scope:
- audit coverage for `./internal/app/...`
- identify the three lowest-coverage packages
- add focused unit tests for critical paths in those packages
- run relevant validation and commit the scoped changes

Plan:
1. Run `go test -cover ./internal/app/...` and capture per-package coverage.
2. Inspect the three lowest-coverage packages and the nearby patterns they already use.
3. Add minimal high-signal unit tests that cover important success and failure branches.
4. Re-run targeted tests and full `internal/app` coverage, then run Go lint and code review.
5. Commit only the plan doc and the new test files.
