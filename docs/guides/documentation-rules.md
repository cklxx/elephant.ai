# Documentation Rules

Updated: 2026-01-31

## Purpose
Define how docs are structured, indexed, and maintained so nothing is orphaned.

## Directory ownership
- `docs/reference/` canonical references (config/schema/protocols/flows).
- `docs/guides/` how-to guides and contributor workflows.
- `docs/plans/` execution plans and progress logs.
- `docs/operations/` deployment and runtime operations.
- `docs/research/` research notes and investigation writeups.
- `docs/memory/` long-term memory only.
- `docs/error-experience/` and `docs/good-experience/` logs and summaries.

## Indexing (no floating docs)
- Every directory with markdown files must have an index file named `README.md`.
- The index lists all markdown files in that directory (excluding itself).
- The root `docs/README.md` links to each directory index and top-level docs.

## Naming
- Plans/logs: `YYYY-MM-DD-short-slug.md`.
- Reference/guides: `kebab-case.md`.

## Content rules
- Config examples: YAML only.
- Diagrams: Mermaid only.
- Tables: prefer 4 columns (Step/Behavior/Notes/Location) or (Field/Meaning/Default/Location).
- Keep a single source of truth; other docs link to the canonical doc.

## Updates
- If code changes affect behavior or config, update the related reference/guide.
- Add new docs to the directory index and the root index.
- Plans must record progress in their `Progress` section.

## Error/Good experience formats
- Error entry (`docs/error-experience/entries/`):
  - Filename: `YYYY-MM-DD-short-slug.md`
  - Content:
    - `Error: ...`
    - `Remediation: ...`
- Error summary (`docs/error-experience/summary/entries/`):
  - Filename: `YYYY-MM-DD-short-slug.md`
  - Content:
    - `Summary: ...`
    - `Remediation: ...`
- Good entry (`docs/good-experience/entries/`):
  - Filename: `YYYY-MM-DD-short-slug.md`
  - Content:
    - `Practice: ...`
    - `Impact: ...`
- Good summary (`docs/good-experience/summary/entries/`):
  - Filename: `YYYY-MM-DD-short-slug.md`
  - Content:
    - `Summary: ...`
    - `Impact: ...`
