# Plan: Coding Agent P0 Implementation (2026-02-11)

## Goal
- Implement coding-agent P0 on top of existing external bridge stack.
- Keep `bg_dispatch` as the entry surface and enable high-efficiency coding defaults.

## Scope
- Extend `bg_dispatch` for coding-first parameters and defaults.
- Add coding execution policy wrapper (full-access defaults, verify, retry loop).
- Wire wrapper into DI so background external tasks go through coding policy.
- Add strict merge gate behavior for coding tasks on isolated workspaces.
- Add tests for critical behavior and regressions.

## Checklist
- [completed] Add coding-focused dispatch parameters (`task_kind`, verify args, retry, merge flags).
- [completed] Implement managed coding executor wrapper over external executor.
- [completed] Integrate wrapper in `internal/app/di/container_builder.go`.
- [completed] Add auto-merge-on-success path for coding tasks with workspace isolation.
- [completed] Add/adjust tests in orchestration + coding + background runtime.
- [completed] Run lint + test; run mandatory code review workflow before commit.
- [in_progress] Commit in incremental chunks; merge branch back to `main`; clean temporary worktree.

## Progress Log
- 2026-02-11 15:02: Plan created; implementation started with dispatch surface changes first.
- 2026-02-11 16:12: Added managed coding executor (full-access defaults + verify/retry), wired through DI, and implemented coding auto-merge gate.
- 2026-02-11 16:25: Fixed web lint regressions (`Building.tsx`, `Buildings.tsx`) and stabilized supervisor async backoff assertion for CI.
- 2026-02-11 16:48: Full lint/test passed (`./dev.sh lint`, `./dev.sh test`); code review issue fixed (`verify=false` now respected by managed executor) with regression test.
