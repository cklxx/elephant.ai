# Memory Management

Updated: 2026-03-04

Operational policy for loading, authoring, and maintaining project memory.

See [MEMORY_SYSTEM.md](../reference/MEMORY_SYSTEM.md) for storage layout and writing rules.
See [MEMORY_INDEXING.md](../reference/MEMORY_INDEXING.md) for graph artifacts, link semantics, and indexing internals.

---

## Always-Load Set

Load at every conversation start (~8 KB total):

1. `docs/memory/long-term.md` -- stable cross-session rules.
2. `docs/guides/engineering-workflow.md` -- engineering standards and development cycle.
3. Latest 3 error summaries from `docs/error-experience/summary/entries/` (by filename date DESC).
4. Latest 3 good summaries from `docs/good-experience/summary/entries/` (by filename date DESC).

Filenames are date-sorted; read the most recent.

---

## On-Demand Loading

| Source | Trigger |
|--------|---------|
| Full error/good entry | Summary lacks detail for the current task |
| `docs/memory/index.yaml` + `edges.yaml` | Need to search history by topic or find related entries |
| `docs/memory/tags.yaml` | Need tag-based filtering |
| `docs/postmortems/incidents/` | Task touches a component with a known incident |
| `docs/plans/` | Entering planning phase; need prior design references |
| 1-hop graph expansion | Already found a relevant entry; need its neighbors |

---

## Retrieval Rules

- Summaries first; expand to full entry only when summary is insufficient.
- Prefer most recent item when multiple entries cover the same topic.
- Lark context: `memory_search` then `memory_get` then `memory_related` then `lark_chat_history`.

---

## Authoring Rules

### Long-Term Memory (`docs/memory/long-term.md`)

- Durable, long-lived lessons only.
- Update `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- Keep concise -- this file is loaded every session.

### Experience Entries

New entries must include a `## Metadata` block:

```yaml
id: <type>-YYYY-MM-DD-<short-slug>
type: error_entry | good_entry | error_summary | good_summary
date: YYYY-MM-DD
tags:
  - <tag>
links:
  related: []
  supersedes: []
  see_also: []
  derived_from: []
```

After editing memory docs, run: `go run ./scripts/memory/backfill_networked.go`.

### Index-Only Files

These files are indexes -- never put content in them:
- `docs/error-experience.md`
- `docs/error-experience/summary.md`
- `docs/good-experience.md`
- `docs/good-experience/summary.md`

---

## Topic Memory Files

| File | Domain |
|------|--------|
| `docs/memory/long-term.md` | Cross-session stable rules |
| `docs/memory/kernel-ops.md` | Kernel operations patterns |
| `docs/memory/lark-devops.md` | Lark integration and DevOps |
| `docs/memory/eval-routing.md` | Eval and routing decisions |
