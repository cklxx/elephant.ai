# Plan: Memory Vector Index (SQLite + sqlite-vec + Ollama)

Owner: cklxx
Date: 2026-02-03

## Goal
Align Markdown memory retrieval with Clawdbot: local SQLite + sqlite-vec index, Ollama embeddings, and hybrid (vector + BM25) scoring with configurable weights and thresholds.

## Scope
- Add memory index config and defaults.
- Implement Ollama embedding provider and SQLite index store (sqlite-vec + FTS5).
- Add indexer (scan + fsnotify + debounce), hybrid search, and fallback to file scan.
- Update memory docs and config reference.
- Add tests for chunking/index store/embedding and hybrid ranking.

## Non-Goals
- Remote embedding providers.
- Non-Markdown memory sources.
- Cross-user memory sharing.

## Plan of Work
1) Config + docs
   - Extend proactive memory config with index settings.
   - Add YAML examples and memory system documentation updates.
2) Memory index core
   - Add Ollama embedder.
   - Add SQLite index store (chunks + vec + fts + embedding cache).
   - Add indexer (scan, watch, debounce) and hybrid search.
   - Wire into Markdown engine + DI.
3) Tests + validation
   - Unit tests for chunking, embedding fallback, index store CRUD, hybrid scoring.
   - Run `./dev.sh lint` and `./dev.sh test`.

## Test Plan
- Unit: chunking overlap, embedding provider fallback, store insert/delete, hybrid ranking.
- Tool: memory_search returns hybrid results when index is enabled.
- Full: `./dev.sh lint`, `./dev.sh test`.

## Progress
- [x] Config + docs
- [x] Memory index core
- [x] Tests + validation
