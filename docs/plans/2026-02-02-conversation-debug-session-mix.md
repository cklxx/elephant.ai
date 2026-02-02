# Plan: Fix conversation debug session mix (2026-02-02)

## Goals
- Ensure session debug page never shows stale session/turn snapshots.
- Prevent SSE event lists from mixing across sessions or replay modes by default.

## Steps
- [x] Review current conversation-debug session loading and SSE replay behavior.
- [x] Add a request guard helper with tests to prevent stale async updates.
- [x] Update conversation debug page to use request guards, clear events on new connect, and default replay to session scope.
- [x] Run lint + tests and capture results.

## Results
- `make fmt vet`
- `make test`
- `cd web && npm run lint`
- `cd web && npm run test`
