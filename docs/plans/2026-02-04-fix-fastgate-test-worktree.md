# Fix fast gate when test worktree is stale/partial

**Date**: 2026-02-04
**Status**: Completed
**Author**: cklxx

## Context
FAST gate runs from `.worktrees/test`. Recent runs showed `cmd/alex` missing and `go test ./...` matching no packages, indicating the test worktree existed but was not a valid checkout (stale admin entry or partial worktree).

## Goal
Make `scripts/lark/worktree.sh ensure` reliably recreate `.worktrees/test` when the worktree is stale or missing repo content, so FAST/SLOW gates run against a full checkout.

## Plan
1. Tighten validation in `worktree.sh` to detect partial/stale worktrees (e.g., missing `go.mod`).
2. Ensure stale admin entries are cleared before re-adding the worktree.
3. Run `./dev.sh lint` and `./dev.sh test`.

## Progress Log
- 2026-02-04: Plan created.
- 2026-02-04: Added worktree validation + stale admin cleanup in `worktree.sh`.
- 2026-02-04: Ran `./dev.sh lint` and `./dev.sh test`.
