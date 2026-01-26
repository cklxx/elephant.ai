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
- 2026-01-26: Decision — full cut to tview; remove Bubble Tea entrypoints and rebuild interactive TUI around `tview.Application` + `TextView` + `InputField`.
- 2026-01-26: Implemented tview-based TUI (`cmd/alex/tui_tview.go`), removed Bubble Tea files/tests, unified command parsing, and updated TUI mode selection.
- 2026-01-26: Updated dependencies (remove bubbletea/bubbles, add tview/tcell) and reran `go mod tidy`.
- 2026-01-26: Validation complete — `make fmt`, `make vet`, `make test` succeeded.
- 2026-01-26: macOS TUI could not accept input; switching from tview to gocui with plain rendering and preserving line-mode fallback.
- 2026-01-26: Implement gocui-based TUI (`cmd/alex/tui_gocui.go`), remove tview wiring, and update dependencies/tests.
- 2026-01-26: Validation complete — `make fmt`, `make vet`, `make test` succeeded after gocui cutover.
- 2026-01-26: Add ordered UI update queue, input history (Up/Down), and scrollback controls (PgUp/PgDn/Home/End); add unit tests for history behavior.
- 2026-01-26: Validation complete — `make fmt`, `make vet`, `make test` succeeded after gocui UX optimizations.

## Next Steps
1. Validate gocui UI on macOS (input focus, history, scrollback, ctrl+c/cancel).
2. Decide whether to reintroduce ANSI colors safely (currently plain rendering).
3. Re-run full lint/test after macOS verification (if any fixes land).
