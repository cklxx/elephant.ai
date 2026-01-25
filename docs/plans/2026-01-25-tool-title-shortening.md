# Plan: Tool Name Shortening in UI (2026-01-25)

## Goal
- Make tool titles shorter and more readable in the web UI while preserving key hints.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Add truncation logic for tool titles (keep base tool name + trimmed hint).
2. Add unit tests for title truncation behavior.
3. Run `make fmt`, `make vet`, `make test`, plus `npm run lint` and `npm test` in `web/`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Added tool title truncation tests and implemented shorter title formatting.
- 2026-01-25: Ran `make fmt`, `make vet`, `make test`, `npm run lint`, and `npm test`.
