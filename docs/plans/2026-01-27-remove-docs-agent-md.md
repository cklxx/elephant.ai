# Plan: Remove legacy agent doc references (2026-01-27)

## Goal
- Remove the legacy agent doc and clean up any references to it.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Locate references to the legacy agent doc in docs and update or remove them.
2. Delete the legacy agent doc.
3. Run full lint + tests (`make fmt`, `make vet`, `make test`, `cd web && npm run lint && npm run test`).

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Updated legacy agent doc references, added replacement anchors, and removed the doc.
- 2026-01-27: Ran `make fmt`, `make vet`, `make test`, `cd web && npm run lint`, and `cd web && npm run test`.
