# Plan: Go CLI/TUI chat framework re-research (2026-01-26)

## Goal
- Re-evaluate current Go CLI/TUI frameworks suitable for chat-style apps and provide a concise, up-to-date recommendation set.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Survey active Go TUI frameworks and supporting libraries.
2. Summarize strengths/weaknesses specifically for chat UX (scrollback, streaming, input editing, Unicode, IME, mouse).
3. Provide short-list recommendations and migration notes.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Reviewed current TUI stacks (Bubble Tea/Bubbles/Lip Gloss, tview, gocui, termui, tcell); recommended Bubble Tea for chat UX (viewport + text input), tview for quick stable UI, gocui/tcell for lower-level control; noted tui-go is archived and not recommended.
- 2026-01-26: Refreshed framework notes using official docs for Bubble Tea, Bubbles components, tview, gocui, and termui; captured citations for the CLI/TUI migration decision.
- 2026-01-26: Selected tview for the full cutover and began implementation of the new TUI shell.
- 2026-01-26: macOS input issues reported; pivoted to gocui for the interactive TUI implementation.
- 2026-01-26: Re-checked gocui update ordering notes and view scroll APIs; re-reviewed Bubble Tea MVU entrypoints for comparison when optimizing chat UX.
