# Plan: Full Optimization (Events + Memory + RAG) — 2026-01-31

## Goals
- Improve correctness across RAG chunking/indexing, event history/SSE replay, and memory retention/recall.
- Deliver in three iterations with tests and lint passing.

## Iteration 1 — RAG correctness
- [x] Add per-chunk metadata isolation in `internal/rag/chunker.go` + tests.
- [x] Fix `code_search` repo hash collision + tests.
- [x] Ensure vector delete works by storing document id in metadata + tests.

## Iteration 2 — Event system correctness
- [x] Add event history session TTL/max sessions + tests.
- [x] SSE replay dedupe by event_id/seq (not timestamp) + tests.
- [x] Web subagent grouping strict on parent_run_id + tests.

## Iteration 3 — Memory retention/recall
- [ ] Add memory retention policy + TTL pruning across stores + tests.
- [ ] Allow memory recall with no extracted keywords + tests.
- [ ] Add `query` to memory_recall tool + tests.
- [ ] Hybrid store tolerant vector failures (configurable) + tests.

## Validation
- [ ] `./dev.sh lint`
- [ ] `./dev.sh test`

## Notes
- Follow TDD and keep agent/ports free of memory/RAG deps.
