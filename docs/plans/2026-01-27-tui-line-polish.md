# Plan: Line-mode TUI polish (2026-01-27)

## Goal
- Bring the native line-mode chat UX up to CLI industry standards (readline editing, history, command help, and predictable prompt behavior).

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add a readline-style prompt (history, completion) and fall back to buffered input when not interactive.
2. Introduce /help and update command parsing + tests.
3. Persist history under the CLI storage root and add prompt-abort handling.
4. Run `./dev.sh lint` and `./dev.sh test`.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added readline prompt wrapper with history + completion and non-interactive fallback; added /help command and line-mode help output.
- 2026-01-27: Added line-mode tests for prompt aborts, history recording, and command parsing.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (linker emits LC_DYSYMTAB warnings on macOS, tests passed).
