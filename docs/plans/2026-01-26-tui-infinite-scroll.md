# Plan: TUI chat infinite scroll (2026-01-26)

## Goal
- Update TUI chat mode to behave like terminal-native infinite scroll (Claude Code style), avoiding full-screen takeover.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Inspect current TUI modes (bubbletea vs line mode) and rendering flow.
2. Introduce a non-fullscreen, scrollable terminal-native chat view.
3. Update tests and run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Defaulted interactive chat UI to terminal-native scrolling mode; made fullscreen TUI opt-in via env.
- 2026-01-26: Routed TUI mode env lookup through runtime config helpers to satisfy getenv guard.
