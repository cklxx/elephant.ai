# Blocker Radar Git Signal Integration Plan

Date: 2026-03-10

Scope:
- enhance `internal/app/blocker/radar.go`
- reuse `internal/domain/signal/ports/git_signal.go`
- add mixed-source blocker tests

Plan:
1. Inspect the existing blocker radar flow and current git signal port/provider capabilities.
2. Add a git-signal dependency to blocker radar using the existing port boundary.
3. Detect review bottlenecks for PRs waiting more than 24 hours and convert them into blocker alerts.
4. Merge task-based and git-based blockers in `NotifyBlockedTasks` without weakening existing task behavior.
5. Add tests covering task-only, git-only, and mixed-source detection.
6. Run focused tests and lint, then mandatory review, commit, merge, and remove the worktree.
