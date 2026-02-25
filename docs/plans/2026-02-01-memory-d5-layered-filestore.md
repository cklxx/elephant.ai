# Plan: Memory D5 Layered FileStore + Daily Summary + Long-Term Extraction

**Date:** 2026-02-01
**Status:** done
**Owner:** cklxx

## Scope
- Refactor memory FileStore into layered structure (`entries/`, `daily/`, `MEMORY.md`).
- Add daily summary pipeline (rule-based, file-backed).
- Add long-term extraction from daily summaries (rule-based, file-backed).
- Update tests + migration notes.

## Plan
1. Review current memory store behavior + D5 requirements; define minimal layered format and summary/extraction rules.
2. Implement LayeredFileStore (entries/daily/MEMORY) with migration from legacy flat files; update search ordering + dedupe.
3. Implement daily summarizer + long-term extractor (rule-based) with file IO helpers.
4. Update/extend tests for layered store, daily summary, long-term extraction, and migration.
5. Add migration notes doc + update memory index + long-term memory timestamp.
6. Run gofmt, full lint + tests, restart stack.

## Progress Log
- 2026-02-01: Plan created; implementation pending.
- 2026-02-02: Implemented layered FileStore, daily summarizer, long-term extractor, and added tests + migration notes.
- 2026-02-02: gofmt + tests run; `./dev.sh lint` and `./dev.sh test` failed due to pre-existing toolregistry/typecheck issues. `./dev.sh down && ./dev.sh` failed in `scripts/lib/common/sandbox.sh` (env_flags unbound variable).
- 2026-02-02: Full `./dev.sh lint` + `./dev.sh test` passed; `./dev.sh down && ./dev.sh` completed successfully.
