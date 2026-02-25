# Plan: Remove gocui, restore line-mode TUI (2026-01-27)

## Goal
- Remove gocui TUI implementation and ship a stable native line-mode interactive chat with small usability upgrades.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Remove gocui TUI code + dependencies and clean any now-unused helpers.
2. Restore line-mode interactive loop with session reuse and improved /clear behavior.
3. Add line-mode tests around the prompt loop and command handling.
4. Run `./dev.sh lint` and `./dev.sh test`.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Removed gocui UI code and dependency; restored line-mode interactive loop with session reuse and improved /clear behavior.
- 2026-01-27: Added line-mode loop tests and EOF handling coverage.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (tests failed in `internal/tools/builtin/orchestration` due to unrelated prompt changes; logged error-experience entry).
