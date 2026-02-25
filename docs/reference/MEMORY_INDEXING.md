# Memory Indexing & Search (Engineering Reference)

Updated: 2026-02-23

This document captures **implementation details** for the Markdown memory index and search pipeline. It is intended for engineers and should not be injected into the LLM prompt.

## Repo Documentation Index (Networked Memory)
Repo memory docs (`docs/error-experience/*`, `docs/good-experience/*`, `docs/memory/long-term.md`) are indexed into a graph so entries can cross-reference each other.

Repo memory graph also includes:
- Memory-related plans under `docs/plans/` (type `plan`)
- `docs/memory/long-term.md` sections as anchor nodes (type `long_term`)
Reference: `docs/memory/networked/README.md`.

### Graph Artifacts
- `docs/memory/index.yaml` — node registry (IDs, paths, type, date, tags).
- `docs/memory/edges.yaml` — normalized link edges (bidirectional `related` plus directed `supersedes`/`derived_from`).
- `docs/memory/tags.yaml` — controlled vocabulary for tag normalization.

### ID Derivation (Legacy)
If an entry lacks a metadata block, derive:
- `id` from filename: `<type>-YYYY-MM-DD-<slug>`
- `type` from folder (error_entry/good_entry/error_summary/good_summary)
- `date` from filename
- `tags` from title and `Summary:`/`Remediation:` lines (best-effort)
- `links` empty

### Link Normalization
- For `related` links, the indexer emits **bidirectional** edges.
- For `supersedes`/`derived_from`, edges remain **directed**.
- `see_also` edges are **directed** unless explicitly marked reciprocal.
- New entries should include a `## Metadata` YAML block with `links` to drive edge creation. See `docs/memory/networked/README.md`.

### Backward Compatibility
Legacy entries remain valid without metadata and are still searchable. The indexer avoids inventing links; it only materializes edges when explicitly declared.

## Runtime Graph Retrieval
- Memory chunk indexing now extracts link edges from:
  - `[[memory:<path>#<anchor>]]`
  - Markdown links to `.md` targets (e.g. `[x](memory/2026-02-02.md#anchor)`).
- Edge storage: SQLite `memory_edges` table keyed by `(src_path, src_start_line, src_end_line, dst_path, dst_anchor, edge_type)`.
- `memory_search` returns hit-level `related` counts from indexed edges.
- `memory_related` traverses 1-hop related entries from a source path/range.
- `memory_related` traverses only `related` edges (bidirectional); `see_also`/`supersedes`/`derived_from` remain directed and are not auto-expanded.
- If index lookup is unavailable, engine falls back to lightweight Markdown link parsing.

## Index storage
- Default DB: `~/.alex/memory/index.sqlite`
- Shared DB: `~/.alex/memory/index.sqlite` (no per-user subdirectory)

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
