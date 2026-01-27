# Plan: Clean up error experience history + add good practices (2026-01-27)

## Goal
- Prune low-value error experience entries, consolidate useful summaries, and add a Good Practices section with parallel structure to the error experience logs.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Inventory `docs/error-experience/entries` and `docs/error-experience/summary/entries`, decide what to keep, consolidate, or drop; ensure entry count <= 6.
2. Update summary entries to keep only actionable items and remove duplicates/noise; delete obsolete entry files.
3. Add Good Practices documentation structure mirroring error experience (index, entries, summary); seed initial entries from confirmed good patterns if available.
4. Run full lint + tests (`make fmt`, `make vet`, `make test`, `cd web && npm run lint && npm run test`).

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Pruned error-experience entries to 6 current items; removed low-value summaries and created concise summary entries for retained historical items.
- 2026-01-27: Added Good Experience docs structure and seeded initial good-practice entries.
- 2026-01-27: Ran `make fmt`, `make vet`, `make test`, `cd web && npm run lint`, and `cd web && npm run test`.
