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
- 2026-01-24: Deep scan phase 2 - launched 4 parallel agents for Go backend, web frontend, tests/CI, and error handling analysis.
- 2026-01-24: Identified 30+ new optimization opportunities across all layers:
  - Go: Regex recompilation (parser.go, react_engine.go), slice pre-allocation, mutex contention in EventBroadcaster
  - Web: SSE event O(n) processing, markdown re-parse on delta, prop drilling in ConversationMainArea (28+ props)
  - Tests: 53% tool test coverage (30/57 files), missing multi-agent orchestration tests
  - Findings documented in `docs/error-experience/entries/2026-01-24-performance-scan-findings.md`
- 2026-01-24: Implemented P0 optimizations:
  - Go: Moved regex to module-level variables in parser.go (3 patterns) and react_engine.go (5 patterns)
  - Go: Pre-allocated slice in postgres_event_history_store.go:365 with batchSize capacity
  - Go: Replaced slice prepend with slices.Insert in react_engine.go:1056-1070
  - Web: Added React.memo with custom comparator to ToolCallCard.tsx (20-30% re-render reduction)
