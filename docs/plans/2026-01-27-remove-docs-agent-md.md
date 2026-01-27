# Plan: Remove docs/AGENT.md references (2026-01-27)

## Goal
- Remove `docs/AGENT.md` and clean up any references to it.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Locate references to `docs/AGENT.md` / `AGENT.md` in docs and update or remove them.
2. Delete `docs/AGENT.md`.
3. Run full lint + tests (`make fmt`, `make vet`, `make test`, `cd web && npm run lint && npm run test`).

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Updated references to `docs/AGENT.md`, added replacement anchors, and removed the doc.
- 2026-01-27: Ran `make fmt`, `make vet`, `make test`, `cd web && npm run lint`, and `cd web && npm run test`.
