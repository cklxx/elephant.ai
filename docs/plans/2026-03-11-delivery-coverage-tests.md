# Delivery Coverage Test Plan

Date: 2026-03-11
status: completed

Scope:
- audit coverage for `./internal/delivery/...`
- identify the three lowest-coverage packages
- add focused tests for each package's critical paths
- run relevant validation, review, and merge back to `main`

Plan:
1. Run `go test -cover ./internal/delivery/...` and record package coverage.
2. Inspect the three lowest-coverage packages and existing tests to find critical uncovered paths.
3. Add minimal, high-signal tests that exercise success and failure/branch behavior.
4. Run focused Go tests plus delivery package coverage, then run lint and code review.
5. Commit in the worktree, fast-forward merge into `main`, and remove the worktree.

Findings:
- Initial lowest-coverage delivery packages were `internal/delivery/eval/bootstrap` (25.0%), `internal/delivery/eval/http` (35.8%), and `internal/delivery/channels/telegram` (43.0%).
- Added bootstrap failure-path tests for config loading, RL storage initialization, and task-store initialization to raise `internal/delivery/eval/bootstrap` to 54.2%.
- Added direct handler tests for RL read endpoints, eval task update/delete paths, and eval agent/evaluation guard branches to raise `internal/delivery/eval/http` to 63.4%.
- Strengthened Telegram progress listener coverage around unified/envelope events, final flush/update behavior, and sender helper no-op behavior to raise `internal/delivery/channels/telegram` to 55.8%.

Validation:
- `go test ./internal/delivery/eval/bootstrap ./internal/delivery/eval/http ./internal/delivery/channels/telegram`
- `go test -cover ./internal/delivery/...`
- `python3 skills/code-review/run.py review`
