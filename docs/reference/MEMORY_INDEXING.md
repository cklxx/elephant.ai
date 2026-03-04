# Memory Indexing & Search (Engineering Reference)

Updated: 2026-03-04

Implementation details for the Markdown memory index and search pipeline. Not intended for LLM prompt injection.

See [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) for the storage layout and repo documentation sources.

## Graph Artifacts

- `docs/memory/index.yaml` -- node registry (IDs, paths, type, date, tags).
- `docs/memory/edges.yaml` -- normalized link edges.
- `docs/memory/tags.yaml` -- controlled vocabulary for tag normalization.

Node types: `error_entry`, `error_summary`, `good_entry`, `good_summary`, `long_term`, `plan`.

Reference: `docs/memory/networked/README.md`.

## Link Normalization

| Link type | Direction |
|-----------|-----------|
| `related` | Bidirectional (indexer emits both directions) |
| `supersedes` | Directed |
| `derived_from` | Directed |
| `see_also` | Directed (unless explicitly marked reciprocal) |

New entries should include a `## Metadata` YAML block with `links` to drive edge creation.

## ID Derivation (Legacy Entries)

Entries without a metadata block are handled by convention:
- `id`: derived from filename as `<type>-YYYY-MM-DD-<slug>`
- `type`: inferred from folder (`error_entry`/`good_entry`/`error_summary`/`good_summary`)
- `date`: from filename
- `tags`: best-effort from title and `Summary:`/`Remediation:` lines
- `links`: empty

Legacy entries remain valid and searchable. The indexer only materializes edges when explicitly declared.

## Runtime Graph Retrieval

Chunk indexing extracts link edges from:
- `[[memory:<path>#<anchor>]]`
- Markdown links to `.md` targets (e.g. `[x](memory/2026-02-02.md#anchor)`)

Edge storage: SQLite `memory_edges` table keyed by `(src_path, src_start_line, src_end_line, dst_path, dst_anchor, edge_type)`.

- `memory_search` returns hit-level `related` counts from indexed edges.
- `memory_related` traverses 1-hop `related` edges (bidirectional only); `see_also`/`supersedes`/`derived_from` are not auto-expanded.
- Fallback: lightweight Markdown link parsing when index is unavailable.

## Index Storage

Default DB: `~/.alex/memory/index.sqlite` (shared, no per-user subdirectory).

## Chunking

- Target: ~400 tokens per chunk
- Overlap: ~80 tokens

## Embeddings

Default local model: `nomic-embed-text` (Ollama).

## Ranking

Hybrid fusion: `final = 0.7 * vectorScore + 0.3 * bm25Score`, `minScore = 0.35`.

## Fallback

If the index is unavailable, memory search falls back to scanning Markdown files directly.
