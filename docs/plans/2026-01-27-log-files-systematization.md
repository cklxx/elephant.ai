# Plan: Log files systematization doc (2026-01-27)

## Goal
- Document all log files, their locations, writers, and correlation keys in a single reference doc.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Inventory log file outputs (service/LLM/latency/request payload/dev helper).
2. Write a reference doc with formats, environment overrides, and correlation notes.
3. Run full lint + tests.
4. Commit changes.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added log files reference doc with paths, writers, and correlation notes.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (pass; LC_DYSYMTAB linker warnings emitted).
