# Markdown Memory System

Updated: 2026-02-02

## Overview
- Memory is Markdown-only (no database). Files live under `~/.alex/memory/`.
- Long-term memory is curated in `MEMORY.md`.
- Daily notes are append-only in `memory/YYYY-MM-DD.md`.
- Search is performed with `memory_search`, and reads are done with `memory_get`.

## Storage Layout

```
~/.alex/memory/
├── MEMORY.md
└── memory/
    ├── 2026-02-02.md
    ├── 2026-02-01.md
    └── ...
```

## Writing Memory

### Daily Log (`memory/YYYY-MM-DD.md`)
Use the daily log for short, time-stamped notes that were learned during the session.

Format:
```
# 2026-02-02

## 10:30 AM - Topic
One concise paragraph capturing the decision or fact.
```

Guidelines:
- Append only; do not rewrite history.
- Prefer short, high-signal notes (decisions, constraints, preferences).
- Include exact identifiers (IDs, URLs, config keys) when they matter.

### Long-Term Memory (`MEMORY.md`)
Use long-term memory for durable facts that should persist across sessions.

Format:
```
# Long-Term Memory

## User Preferences
- Prefers TypeScript over JavaScript.
- Likes concise explanations.
```

Guidelines:
- Keep entries compact and stable (avoid transient states).
- Group by topic (preferences, decisions, contacts, project facts).
- Remove outdated facts rather than piling on contradictory notes.

## Searching Memory
- Use `memory_search` with a natural-language query.
- Use `memory_get` with the `path` and line range returned by search.

Example:
```json
{ "query": "API decision REST vs GraphQL", "maxResults": 6, "minScore": 0.35 }
```

## System Prompt Injection
- When enabled, the system prompt loads:
  - `MEMORY.md`
  - today’s daily log
  - yesterday’s daily log
- This keeps context small while retaining durable knowledge.
