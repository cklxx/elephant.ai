# Plan: Unify docs and CLI references to AGENTS.md (2026-01-27)

## Goal
Ensure all references to the legacy agent doc point to `AGENTS.md`, remove the legacy doc file, and align CLI/help text accordingly.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Inventory every legacy agent doc reference in docs/CLI and decide updates.
2. Update doc references (README, docs portal, architecture flow notes, plans) to `AGENTS.md`.
3. Remove the legacy agent doc file to avoid drift.
4. Update CLI usage text to mention `AGENTS.md`.
5. Run `./dev.sh lint` and `./dev.sh test`.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Inventory complete for legacy agent doc references.
- 2026-01-27: Updated doc references and plan notes to use AGENTS.md.
- 2026-01-27: Confirmed legacy agent doc file already removed; updated CLI usage text.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (macOS linker warnings only).
