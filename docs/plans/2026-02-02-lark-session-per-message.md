# Lark session per message with in-flight reuse

**Date**: 2026-02-02
**Status**: In Progress
**Author**: cklxx

## Scope
- Generate a new Lark session ID for each message by default.
- Reuse the same session only when a task is in-flight or awaiting user input.
- Update gateway tests to reflect the new session selection rules.
- Run full lint + tests.

## Plan
1. Update/extend gateway tests for per-message session IDs, in-flight reuse, and await-user-input reuse.
2. Refactor Lark gateway session slot tracking to be chat-keyed and store pending session state.
3. Adjust /reset handling to target the active/pending session when available.
4. Run `./dev.sh lint` and `./dev.sh test`.

## Progress Log
- 2026-02-02: Plan created.
- 2026-02-02: Added tests for per-message sessions, await-user-input reuse, and in-flight reuse.
- 2026-02-02: Refactored Lark gateway session slot tracking and reset handling for per-message sessions.
- 2026-02-02: Ran `./dev.sh lint` (pass) and `./dev.sh test` (fail: TestNoUnapprovedGetenv in internal/config referencing skills_dir files).
