# Plan: Full project scan for optimization opportunities (2026-01-24)

## Goal
- Perform a repo-wide scan and report optimization opportunities suited for top-tier performance requirements, with concrete file references.

## Plan
1. Inventory critical subsystems (agent loop, storage/session, observability, web streaming, evaluation harness) and map high-traffic code paths.
2. Run targeted searches for perf signals (TODO/perf, unbounded loops, heavy JSON work, file I/O, cache usage, concurrency patterns).
3. Inspect representative files in each subsystem and collect concrete optimization findings with references.
4. Summarize findings ordered by severity and expected performance impact; call out measurement gaps and recommended profiling.

## Progress
- 2026-01-24: Read engineering practices; plan created.
- 2026-01-24: Completed repo scan via targeted searches and inspection of streaming, storage, RAG, and web attachment paths; prepared performance findings with file references.
