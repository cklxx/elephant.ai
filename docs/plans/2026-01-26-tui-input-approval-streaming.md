# Plan: TUI input + CLI approvals + streaming responsiveness (2026-01-26)

## Goal
- Fix CJK/IME deletion in chat TUI, add CLI approval flow with session-wide allow (local exec, no sandbox in CLI), and make assistant streaming emit smaller deltas (closer to character-by-character).

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add IME-aware input handling for chat TUI (CJK-friendly backspace, multi-rune insert) and preserve whitespace during streaming.
2. Add CLI approval prompts (approve once / approve all in session / reject) and wire into streaming + line chat + bubble TUI.
3. Improve streaming responsiveness by emitting smaller deltas from the LLM loop and flushing markdown buffers sooner.
4. Make local exec default for CLI (build tag rework), keep sandbox tools disabled in CLI preset.
5. Add tests for IME input handling, CLI approvals, and preset gating expectations.
6. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Enabled CLI approver (session-wide allow), IME-aware TUI input, and typewriter streaming; defaulted local exec builds and kept sandbox tools off in CLI preset.
- 2026-01-26: Tests updated/added; ran `make fmt`, `make vet`, `make test`.
