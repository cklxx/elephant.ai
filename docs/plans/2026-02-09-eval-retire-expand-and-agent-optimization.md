# 2026-02-09 Eval Retire/Expand and Agent Optimization

## Context
- User asks to continue retiring weak/easy evaluation cases, add more challenging cases, and optimize agent routing.
- Current baseline already includes motivation-aware collection and conflict-heavy suites.

## Goals
1. Retire/replace easy low-signal cases that repeatedly pass with limited diagnostic value.
2. Add harder, conflict-rich, multi-constraint cases that better stress top1 routing precision.
3. Improve routing heuristics for dominant remaining failure clusters.
4. Re-run full suite and provide updated scoring report.

## Plan
- [x] Initialize fresh worktree from `main` and copy `.env`.
- [x] Load engineering practices + long-term memory + latest summaries.
- [x] Identify candidate easy cases to retire and hard cases to add.
- [x] Update datasets (retire + expand) and keep YAML-only schema.
- [x] Update routing heuristics + tests for top conflict pairs.
- [ ] Run `make fmt`, `go test ./...`, and full suite eval.
- [x] Publish updated report and records.
- [ ] Commit incrementally, merge back to `main`, remove worktree.

## Progress Notes
- 2026-02-09 14:31 CST: `make fmt` passed.
- 2026-02-09 14:31 CST: full `go test ./...` hit existing supervisor timing test failure (`internal/devops/supervisor TestTickRestartBackoffIsAsync`), targeted rerun still failed in current environment.
