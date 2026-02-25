# Plan: Speed up session title updates (2026-01-25)

## Goal
- Identify why session titles update slowly and fix both frontend and backend paths to update promptly.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Trace title update flow (frontend state update + server persistence path).
2. Implement targeted fixes to remove latency (debounce, sync, or async gaps).
3. Add/adjust tests for session title updates.
4. Run `make fmt`, `make vet`, `make test`, plus `npm run lint` and `npm test` in `web/`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Implemented immediate title updates from plan metadata and added early backend persistence with tests.
- 2026-01-25: Ran `make fmt`, `make vet`, `make test`, `npm run lint`, and `npm test`.
