# Documentation Rules

Updated: 2026-03-10

## Document Classes

**Living docs** (must stay current with code):
- `docs/reference/` — architecture, config, protocols
- `docs/guides/` — how-to workflows
- `docs/operations/` — deployment, runtime ops
- Top-level navigation (`README.md`, indexes)

**Record docs** (append-only):
- `docs/plans/` — execution plans with status
- `docs/research/`, `docs/analysis/`, `docs/reviews/`
- `docs/error-experience/`, `docs/good-experience/`

## Rules

1. Code changes affecting behavior must update related living docs in the same delivery cycle.
2. Every docs directory with markdown files must have a `README.md` index listing all files.
3. Plan/log files: `YYYY-MM-DD-short-slug.md`. Living docs: `kebab-case.md`.
4. Config examples use YAML only.
5. One canonical doc per topic — link, don't fork content.

## Update Protocol

When code changes affect runtime behavior:
1. Update the canonical living doc.
2. Update directory index if file set changed.
3. Update timestamp (`Updated:` or `Last updated:`).
4. Leave record docs untouched unless adding a new entry.
