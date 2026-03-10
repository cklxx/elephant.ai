# Markdown Memory System

Updated: 2026-03-10

Memory is Markdown-only. Markdown files are the single source of truth.

## Storage

```text
~/.alex/memory/
├── MEMORY.md          # durable facts (preferences, decisions, contacts)
└── memory/
    └── YYYY-MM-DD.md  # daily append-only notes
```

Repo documentation: `docs/error-experience/`, `docs/good-experience/`, `docs/memory/long-term.md`, `docs/plans/`.

See [MEMORY_INDEXING.md](MEMORY_INDEXING.md) for graph artifacts and search internals.

## Session Boot

1. Read `SOUL.md` (identity) and `USER.md` (user profile).
2. Read today's and yesterday's `memory/YYYY-MM-DD.md`.
3. Main session: also read `MEMORY.md`.

See [memory-management.md](../guides/memory-management.md) for the always-load set and on-demand policy.

## Writing

- User says "remember this" → write it.
- Decisions/preferences/constraints that matter later → write them.
- Prefer daily log first; promote to `MEMORY.md` only when durable.
- Auto-capture 1-3 short bullets into daily log on successful tasks.

Daily log: short, timestamped, append-only. Include exact identifiers when relevant.
`MEMORY.md`: compact, grouped by topic, remove outdated facts.

## Retrieval

1. `memory_search` → find relevant memories.
2. `memory_get` → read exact lines from results.
3. `memory_related` → expand 1-hop linked memories.
4. Nothing found → say so, don't guess.

## Bootstrap Injection

On first turn (`prompt.mode=full`), bootstrap files injected under `# Workspace Files`:
- Priority: global-first (`~/.alex/memory/<file>` then `<workspace>/<file>`).
- Per-file cap: `proactive.prompt.bootstrap_max_chars` (default 20000).

## Compaction Safety

Before context compression, store high-signal facts in the daily log. Compression summaries are lossy.
