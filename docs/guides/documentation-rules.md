# Documentation Rules

Updated: 2026-02-10

## Purpose
Keep documentation discoverable, accurate, and synchronized with code.

## 1) Document classes

- **Non-record (living) docs**
  - `docs/reference/` (architecture/config/protocol/tooling truth)
  - `docs/guides/` (how-to workflows)
  - `docs/operations/` (deployment/runtime operations)
  - Top-level navigation docs (`README.md`, `docs/README.md`, indexes)
- **Record docs**
  - `docs/plans/` (execution plans/progress logs)
  - `docs/research/`, `docs/analysis/`, `docs/reviews/`
  - `docs/error-experience/` and `docs/good-experience/` entries/summaries

Rule: behavior/config/path/tool-surface changes must update related **non-record docs** in the same delivery cycle.

## 2) Directory ownership

- `docs/reference/`: canonical references (config/schema/protocols/flows).
- `docs/guides/`: contributor and usage workflows.
- `docs/operations/`: deployment and runtime operations.
- `docs/memory/`: long-term memory rules and migration notes.
- `docs/plans/`: plan artifacts with status/progress.
- `docs/error-experience/` + `docs/good-experience/`: incident/win records.

## 3) Indexing rules

- Every docs directory with markdown files must have `README.md`.
- Indexes must list all markdown files in the directory (excluding itself).
- `docs/README.md` links to all major directory indexes.

## 4) Naming

- Plan/log style: `YYYY-MM-DD-short-slug.md`.
- Living references/guides/ops: `kebab-case.md`.

## 5) Content rules

- Config examples use YAML only.
- Keep one canonical doc per topic; others should link rather than fork content.
- Mark legacy snapshots clearly if they intentionally keep historical context.

## 6) Update protocol

When code changes affect runtime behavior:
1. Update the canonical non-record doc.
2. Update directory index entries if file set changed.
3. Update timestamp (`Updated:` or `Last updated:`) in touched docs.
4. Keep record docs untouched unless a new record entry is intentionally added.

## 7) Plans and records

- Non-trivial tasks require a plan under `docs/plans/`.
- Plan files must include status and progress updates.
- Error/good indexes remain index-only; entries/summaries live in their `entries/` subdirectories.
