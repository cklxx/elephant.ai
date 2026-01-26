# Plan: IME-friendly input for terminal chat (2026-01-26)

## Goal
- Ensure Chinese IME input and deletion behave correctly by avoiding raw-mode capture when requested.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Inspect TUI input flow (Bubble Tea raw mode vs. line-mode).
2. Add IME input mode switches (env-driven) to force line-mode when needed.
3. Add tests for IME env flags and precedence.
4. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Added IME input mode switches and tests to force line-mode input.
