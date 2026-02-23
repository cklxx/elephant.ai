# Plan: Networked Memory Runtime + Docs Implementation (2026-02-23)

## Goal
Implement graph-style memory usage so memories can reference each other and be traversed as a network, while organizing all repository memory docs (including Lark-related docs) under a consistent node/edge/tag model.

## Scope
- Runtime memory graph retrieval:
  - `internal/infra/memory/*`
  - `internal/infra/tools/builtin/memory/*`
  - `internal/app/toolregistry/*`
  - prompt/tool routing hints for memory usage.
- Documentation and memory-graph organization:
  - `docs/memory/*`
  - `docs/reference/MEMORY_SYSTEM.md`
  - `docs/reference/MEMORY_INDEXING.md`
  - `docs/reference/lark-web-agent-event-flow.md`
  - `AGENTS.md`
  - `CLAUDE.md`
- Full graph backfill script:
  - `scripts/memory/backfill_networked.go`

## Decisions
1. Deliver `文档 + 运行时检索` together.
2. Do `全量回填` via graph artifact generation (`index.yaml/edges.yaml/tags.yaml`) instead of rewriting all historical entry bodies.
3. Lark integration depth is retrieval-chain integration (`memory_search -> memory_get -> memory_related -> lark_chat_history`).

## Implementation Steps
1. Extend memory engine interface for related-graph retrieval and result metadata.
2. Add memory edge extraction, index persistence, and related query path in index store/indexer.
3. Add `memory_related` built-in tool and register it.
4. Update prompts/routing hints and Lark progress phrase mapping.
5. Add tests for links parser, index related queries, markdown fallback related retrieval, and tool behavior.
6. Add/refresh networked memory docs and AGENTS/CLAUDE memory loading guidance.
7. Generate full graph artifacts with backfill script.
8. Run full lint + tests, code review, incremental commits, merge to `main`.

## Progress
- 2026-02-23 19:40: Plan created.
- 2026-02-23 20:05: Runtime graph retrieval + `memory_related` tool implemented with tests.
- 2026-02-23 20:18: Networked docs/AGENTS/CLAUDE updates in progress.
- 2026-02-23 20:22: Backfill script added and graph artifacts generated.
- 2026-02-23 20:30: Lark memory-flow reference updated; ready for full lint/test and review.
