# Plan: CLI/TUI framework migration for chat UX (2026-01-26)

## Goal
- Replace the current broken CLI/TUI chat implementation with a stable framework and refactor the UI layer to be maintainable and testable.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Inspect current CLI/TUI entrypoints (`cmd/alex/tui*.go`) and identify failure points.
2. Select target framework (default to Bubble Tea stack unless migration to tview/gocui is justified by concrete issues).
3. Define a UI abstraction boundary (session input, streaming output, subagent progress) to isolate framework specifics.
4. Implement a minimal vertical slice (session start, input, streaming output) and tests.
5. Migrate remaining UI features and remove dead code paths.

## Non-goals
- Feature expansion beyond restoring parity and stability.

## Risks
- Terminal input edge cases (IME, unicode grapheme handling, resizing).
- Streaming output alignment and subagent progress rendering.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Reviewed `cmd/alex/tui*.go` failure points (inline printing outside View, no alt-screen despite “fullscreen” mode, per-rune ANSI streaming, limited IME handling, silent fallback).
- 2026-01-26: Drafted migration options: stabilize Bubble Tea with viewport + unified rendering and chunked streaming, or migrate to tview with TextView/InputField and explicit event boundary.
