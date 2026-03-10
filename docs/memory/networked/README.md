# Networked Memory Docs

Updated: 2026-03-10

This layer links memory entries, summaries, long-term rules, and plans into a searchable graph.

## Node Types

- `error_entry`
- `error_summary`
- `good_entry`
- `good_summary`
- `long_term`
- `plan`

## Required Metadata

New entries should add a `## Metadata` block:

```yaml
id: err-YYYY-MM-DD-short-slug
type: error_entry
date: YYYY-MM-DD
tags:
  - lark
links:
  related: []
  supersedes: []
  see_also: []
  derived_from: []
```

## Link Semantics

- `related`: same root cause or closely related decision
- `supersedes`: newer guidance replaces older guidance
- `see_also`: supporting context
- `derived_from`: summary or derivative record

## Index Artifacts

- `docs/memory/index.yaml` — node registry
- `docs/memory/edges.yaml` — normalized link graph
- `docs/memory/tags.yaml` — tag vocabulary

## Regeneration

```bash
go run ./scripts/memory/backfill_networked.go
```

## Retrieval Order For Lark

1. `memory_search`
2. `memory_get`
3. `memory_related`
4. `lark_chat_history`

## Legacy Entries

Entries without metadata still work. The indexer infers basic fields from filename and path.

## Authoring Checklist

1. Add stable `id`, `type`, `date`, and tags.
2. Add at least one useful link when possible.
3. Regenerate graph artifacts if they are not updated automatically.
