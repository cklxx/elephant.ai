# Plan: TUI chat usability + approval flow (2026-01-27)

## Goal
- Make the interactive TUI chat fully usable by handling tool approvals inside the UI, keeping streaming output responsive, and preventing deadlocks on dangerous tool calls.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Audit the current gocui TUI flow and identify why approvals block the chat UI.
2. Design a TUI-native approval prompt + response flow (session-wide allow supported) and integrate it into the gocui event loop.
3. Add parsing + approval logic tests; keep CLI approval behavior consistent.
4. Run `./dev.sh lint` and `./dev.sh test`.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added TUI-native approval flow (prompt, response parsing, session-wide allow) and wired gocui approver into interactive chat.
- 2026-01-27: Added approval decision parser tests and shared CLI/TUI parsing logic.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (Go linker emitted LC_DYSYMTAB warnings on macOS, tests passed).
