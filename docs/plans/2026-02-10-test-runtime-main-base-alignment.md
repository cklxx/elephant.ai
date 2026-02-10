# Plan: Lark Test Runtime Main-Base Alignment — 2026-02-10

## Status: In Progress

## Goal
- Ensure the `test` process always runs on code aligned to `main`.
- Remove hard dependency on `scripts/lark/worktree.sh` runtime orchestration.
- Enforce startup/restart ordering: `test` first, then `main`; if `test` is unhealthy, `main` must not be started/restarted.
- Keep `.worktrees/test` management and `.env` sync behavior intact.

## Tasks
- [x] Replace `scripts/lark/worktree.sh` usage with shared helper logic (`scripts/lib/common/lark_test_worktree.sh`) and delete `scripts/lark/worktree.sh`.
- [x] Update `scripts/lark/test.sh` to align test runtime HEAD to `main` before build/start.
- [x] Update `scripts/lark/loop.sh` to avoid `switch test` and merge via test worktree `HEAD` commit.
- [x] Enforce `test`-gates-`main` startup order in `scripts/lark/supervisor.sh` (`run_tick`, startup reconcile, and upgrade/autofix restart paths).
- [x] Add regressions:
  - [x] detached/branch-agnostic test HEAD ff-only merge (`tests/scripts/lark-loop-merge-main-head.sh`)
  - [x] `test` unhealthy blocks `main` startup (`tests/scripts/lark-supervisor-test-gates-main.sh`)
- [x] Run targeted script tests + full `./dev.sh lint` + `./dev.sh test`.
- [x] Perform mandatory code review and commit in incremental commits.
- [ ] Merge changes back to `main` and cleanup temporary worktree.
