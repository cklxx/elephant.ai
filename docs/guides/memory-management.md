# Memory Management

Updated: 2026-03-10

Load memory in layers. Start small. Expand only when the current task needs it.

See [MEMORY_SYSTEM.md](../reference/MEMORY_SYSTEM.md) for storage layout and [MEMORY_INDEXING.md](../reference/MEMORY_INDEXING.md) for indexing internals.

## Always-Load Set

Load these at the start of every conversation:

1. `docs/memory/long-term.md`
2. `docs/guides/engineering-workflow.md`
3. Latest 3 error summaries from `docs/error-experience/summary/entries/`
4. Latest 3 good summaries from `docs/good-experience/summary/entries/`

Use filename date order. Newest first.

## Load On Demand

Load more only when the task needs it:

| Source | When to load |
|--------|--------------|
| Full error or good entry | The summary is not enough |
| `docs/memory/index.yaml` and `docs/memory/edges.yaml` | You need topic search or related history |
| `docs/memory/tags.yaml` | You need tag filtering |
| `docs/postmortems/incidents/` | The task touches a component with a known incident |
| `docs/plans/` | The task is in planning and past design work may matter |
| One-hop related entries | You already found one relevant memory and need nearby context |

## Retrieval Rules

- Read summaries before full entries.
- Prefer the most recent relevant item.
- Stop loading once you have enough context to act.
- For Lark context, use this order: `memory_search` -> `memory_get` -> `memory_related` -> `lark_chat_history`.

## Writing Rules

### Long-Term Memory

Use `docs/memory/long-term.md` only for durable rules that will matter across sessions.

- Keep it short.
- Update `Updated:` to hour precision: `YYYY-MM-DD HH:00`.

### Experience Entries

Every new entry needs this `## Metadata` block:

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

After editing memory docs, run:

```bash
go run ./scripts/memory/backfill_networked.go
```

### Index-Only Files

Do not put narrative content in these files:

- `docs/error-experience.md`
- `docs/error-experience/summary.md`
- `docs/good-experience.md`
- `docs/good-experience/summary.md`

## Topic Files

| File | Use |
|------|-----|
| `docs/memory/long-term.md` | Stable cross-session rules |
| `docs/memory/kernel-ops.md` | Kernel operations patterns |
| `docs/memory/lark-devops.md` | Lark and DevOps decisions |
| `docs/memory/eval-routing.md` | Eval and routing decisions |
