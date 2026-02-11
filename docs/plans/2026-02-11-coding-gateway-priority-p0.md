# Plan: Elevate Coding Gateway Foundation to P0 (2026-02-11)

## Goal
- Promote Coding Gateway foundation priorities to P0:
  - Gateway abstraction (`Submit/Stream/Cancel/Status`)
  - Multi-adapter framework
  - Local CLI auto-detect

## Scope
- Update roadmap priority and pending execution queue docs.
- Keep terminology and milestone mapping internally consistent.

## Out of Scope
- Implementing Coding Gateway runtime/code changes.
- Rewriting historical record docs under `docs/research/`, `docs/analysis/`, or old draft snapshots.

## Checklist
- [completed] Update `docs/roadmap/roadmap-pending-2026-02-08.md` priority tiers and queue placement.
- [completed] Update `docs/roadmap/roadmap.md` priority table, batch plan, and section placement.
- [completed] Update `ROADMAP.md` active architecture priorities.
- [completed] Run lint/tests required by repo policy and record outcome.
- [completed] Perform mandatory code review before commit.
- [in_progress] Commit, merge back to `main` (prefer fast-forward), and clean up temporary worktree.

## Progress Log
- 2026-02-11 14:30: Plan created after user requested Coding Gateway foundation items be elevated to P0.
- 2026-02-11 15:40: Roadmap priority docs updated to move Coding Gateway foundation (abstraction/multi-adapter/CLI auto-detect) into Immediate P0.
- 2026-02-11 16:48: Full lint/test and mandatory review run completed as merge-readiness gate for reprioritization + implementation set.
