# Networked Memory Docs

Updated: 2026-02-23

## Purpose
Create a graph-style memory layer across error/good entries, summaries, and long-term memory so that related decisions, incidents, and remediations can be traversed bidirectionally via IDs, tags, and link edges.

## Node Types
- `error_entry`: `docs/error-experience/entries/*.md`
- `error_summary`: `docs/error-experience/summary/entries/*.md`
- `good_entry`: `docs/good-experience/entries/*.md`
- `good_summary`: `docs/good-experience/summary/entries/*.md`
- `long_term`: `docs/memory/long-term.md` sections (anchor IDs)
- `plan`: memory-related plan files under `docs/plans/`

## Required Metadata Block (New Entries)
Add a YAML metadata block under `## Metadata` in each new entry. Existing entries remain valid without this block.

```yaml
id: err-2026-02-12-lark-test-config-provider-key-mismatch-blocks-start
type: error_entry
date: 2026-02-12
tags:
  - lark
  - config
  - provider
links:
  related:
    - good-2026-02-12-llm-profile-client-provider-decoupling
  supersedes: []
  see_also: []
  derived_from: []
```

## Link Semantics
- `related`: peer concepts or incidents with shared root cause.
- `supersedes`: newer entry replaces guidance in older entry.
- `see_also`: supplementary context (design docs, plans, or summaries).
- `derived_from`: an entry distilled from another (e.g., summary from full entry).

Bidirectional links are enforced via the indexer: if `A -> related -> B`, the indexer emits `B -> related -> A` in `docs/memory/edges.yaml`.

## ID Conventions
ID format is defined in `docs/memory/networked/id-conventions.yaml` and defaults to `<prefix>-YYYY-MM-DD-<slug>`.

## Index Artifacts
These YAML artifacts are the authoritative graph index (manual or generated):
- `docs/memory/index.yaml` — node registry (IDs, paths, tags, type, date).
- `docs/memory/edges.yaml` — normalized link edges for bidirectional traversal.
- `docs/memory/tags.yaml` — controlled tag vocabulary and descriptions.

## Templates
- `docs/memory/networked/entry-template.md`
- `docs/memory/networked/example-links.yaml`
- `docs/memory/networked/id-conventions.yaml`

## Full Backfill
- Regenerate all nodes/edges/tags from current docs with:
  - `go run ./scripts/memory/backfill_networked.go`
- This backfills all error/good entries and summaries into `index.yaml` / `edges.yaml` / `tags.yaml` without rewriting legacy entry bodies.

## Lark Integration
- In Lark contexts, keep retrieval order:
  1. `memory_search`
  2. `memory_get` for exact lines
  3. `memory_related` for graph-linked expansion
  4. `lark_chat_history` for recent thread recall

## Backward Compatibility
Entries without metadata remain valid. The indexer treats them as legacy nodes and infers:
- `id` from filename,
- `type` from folder,
- `date` from filename,
- `tags` from keywords (best-effort),
- `links` as empty.

## Authoring Checklist
1. Add metadata block with stable `id` and tags.
2. Add at least one `related` or `see_also` link.
3. Update `docs/memory/index.yaml` and `docs/memory/edges.yaml` if not auto-generated.
4. If a new tag is introduced, add it to `docs/memory/tags.yaml`.
