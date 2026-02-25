# Plan: Fix SSE replay loss via event history schema sync (2026-01-29)

- Reviewed `docs/guides/engineering-practices.md`.
- Memory load completed (latest error/good entries + summaries + long-term memory).

## Goals
- Restore SSE replay after refresh by ensuring event history writes succeed against existing DBs.
- Align persisted event history schema with current event fields without manual DB rebuilds.
- Add regression coverage for schema upgrade behavior.

## Plan
1. Audit event history persistence paths and identify likely schema mismatch points.
2. Update event history schema initialization to add missing columns on existing tables.
3. Add tests covering schema upgrades from older tables.
4. Run `gofmt` if needed, then full lint + tests.
5. Update plan progress and commit changes.

## Progress
- 2026-01-29: Plan created; engineering practices reviewed; starting investigation.
- 2026-01-29: Added schema upgrade ALTERs for event history; added regression test for legacy table upgrade; gofmt applied.
- 2026-01-29: Ran `./dev.sh lint` and `./dev.sh test` (pass; linker warnings about LC_DYSYMTAB during Go tests).
