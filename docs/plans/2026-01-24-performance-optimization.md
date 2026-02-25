# Plan: Repo-wide performance optimizations (2026-01-24)

## Goal
- Apply the identified high/medium impact performance optimizations across server, tooling, RAG, and web attachment handling.
- Execution aligns to `docs/plans/2026-01-24-performance-optimization-plan.md` (full analysis + P0/P1).

## Scope
- SSE/DataCache: LRU cache, data URI memoization, and lower-overhead sanitization.
- Event history: sanitize inline payloads before in-memory history retention.
- RAG: delete stale chunks by file metadata when incrementally indexing.
- web_fetch: bounded LRU cache and cache sizing config.
- Web attachments: bounded Blob URL LRU + revoke on eviction.
- Tests + lint + full test suite.

## Plan
1. Implement DataCache LRU + data URI memoization and add tests.
2. Reduce SSE sanitization overhead for common types.
3. Sanitize in-memory event history payloads.
4. Add metadata-based deletes to RAG vector store and use in incremental indexing.
5. Add bounded LRU cache to web_fetch with config knobs and tests.
6. Add Blob URL LRU cache with eviction + revoke and tests.
7. Add P0 safeguards: event history DB indexes + DB timeouts + stream guard middleware.
8. Run full lint + tests; capture failures in error experience if needed.

## Progress
- 2026-01-24: Implemented DataCache LRU + data URI memoization with tests.
- 2026-01-24: Added SSE sanitization fast paths for common types.
- 2026-01-24: Sanitized in-memory event history payloads.
- 2026-01-24: Added metadata delete hook for RAG incremental indexing.
- 2026-01-24: Added bounded LRU cache to web_fetch with config defaults and tests.
- 2026-01-24: Added Blob URL LRU cache with eviction + revoke and tests.
- 2026-01-24: Added event history indexes, DB timeouts, and stream guard middleware.
