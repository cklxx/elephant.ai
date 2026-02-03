# Memory Indexing & Search (Engineering Reference)

Updated: 2026-02-03

This document captures **implementation details** for the Markdown memory index and search pipeline. It is intended for engineers and should not be injected into the LLM prompt.

## Index storage
- Default DB: `~/.alex/memory/index.sqlite`
- Per-user DB: `~/.alex/memory/<user-id>/index.sqlite`

## Chunking
- Target chunk size: ~400 tokens per chunk
- Overlap: ~80 tokens

## Embeddings
- Default local embedding model: `nomic-embed-text` (Ollama)

## Ranking
- Hybrid fusion:
  - `final = 0.7 * vectorScore + 0.3 * bm25Score`
  - `minScore = 0.35`

## Fallback behavior
- If the index is unavailable, memory search falls back to scanning Markdown files directly.
