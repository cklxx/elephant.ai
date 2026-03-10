# Memory Indexing & Search

Updated: 2026-03-10

Implementation details for the Markdown memory index and search pipeline.

See [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) for storage layout.

## Graph Artifacts

- `docs/memory/index.yaml` — node registry (IDs, paths, type, date, tags).
- `docs/memory/edges.yaml` — normalized link edges.
- `docs/memory/tags.yaml` — controlled tag vocabulary.

Node types: `error_entry`, `error_summary`, `good_entry`, `good_summary`, `long_term`, `plan`.

## Link Normalization

| Link type | Direction |
|-----------|-----------|
| `related` | Bidirectional |
| `supersedes` | Directed |
| `derived_from` | Directed |
| `see_also` | Directed (unless explicitly reciprocal) |

New entries should include a `## Metadata` YAML block with `links` for edge creation.

## ID Derivation (Legacy)

Entries without metadata: ID derived from filename as `<type>-YYYY-MM-DD-<slug>`, type inferred from folder, tags best-effort from title/Summary/Remediation lines.

## Search Pipeline

- Chunking: ~400 tokens, ~80 overlap.
- Embeddings: `nomic-embed-text` (Ollama).
- Ranking: `final = 0.7 * vector + 0.3 * bm25`, `minScore = 0.35`.
- DB: `~/.alex/memory/index.sqlite`.

## Runtime Graph

- `memory_search` returns hit-level `related` counts.
- `memory_related` traverses 1-hop bidirectional `related` edges only.
- Fallback: lightweight Markdown link parsing when index unavailable.
