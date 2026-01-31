# Plan: Docs organization + indexing rules (2026-01-31)

## Goal
- Add a documentation rules guide and ensure every doc is referenced from an index (no floating docs).

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Add `docs/guides/documentation-rules.md`.
2. Add per-directory index files (README.md) for docs subdirectories with markdown files.
3. Convert error/good experience index files to index-only and move format rules into the new guide.
4. Update `docs/README.md` to point to indexes.

## Progress
- 2026-01-31: Plan created; engineering practices reviewed.
- 2026-01-31: Added `docs/guides/documentation-rules.md` and directory indexes for all docs subtrees.
- 2026-01-31: Converted error/good experience indexes to index-only and linked new indexes from `docs/README.md`.
- 2026-01-31: Ran `./dev.sh lint` and `./dev.sh test` (LC_DYSYMTAB linker warnings observed).
