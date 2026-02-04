# Fix lark worktree ensure for non-worktree directories

**Date**: 2026-02-04
**Status**: In Progress
**Author**: cklxx

## Context
The FAST gate runs from `.worktrees/test`. If that path exists but is not a git worktree (e.g., only `logs/` and `tmp/`), `worktree.sh ensure` fails to create the test worktree and the gate runs in an empty directory, yielding `stat .../cmd/alex: directory not found` and `go test` matching no packages.

## Goal
Make `scripts/lark/worktree.sh ensure` robust when `.worktrees/test` exists but is not a valid git worktree, while preserving existing logs/tmp.

## Plan
1. Detect stale or non-worktree test roots and cleanly recover by relocating the directory before creating the worktree.
2. Restore `logs/` and `tmp/` into the new worktree when possible.
3. Run `./dev.sh lint` and `./dev.sh test`.

## Progress Log
- 2026-02-04: Plan created.
