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
