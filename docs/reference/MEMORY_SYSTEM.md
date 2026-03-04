# Markdown Memory System

Updated: 2026-03-04

## Overview

Memory is Markdown-only. Markdown files are the single source of truth.

- Long-term memory: curated in `MEMORY.md`.
- Daily notes: append-only in `memory/YYYY-MM-DD.md`.
- Search with `memory_search`, read exact lines with `memory_get`, expand linked memory with `memory_related`.
- No dedicated `memory_write` tool; write memories via normal file tools.

## Context vs Memory

- **Context**: system prompt + project context + conversation history + tool outputs + current message. Transient, bounded by model window.
- **Memory**: durable Markdown files on disk (`MEMORY.md` + `memory/*.md`). Persistent, cheap, searchable.

## Storage Layout

### User Memory

```text
~/.alex/memory/
├── MEMORY.md
└── memory/
    ├── 2026-02-02.md
    ├── 2026-02-01.md
    └── ...
```

### Repo Documentation Memory (Networked)

Project memory docs live under `docs/` and are networked via IDs, tags, and link edges.

Sources:
- `docs/error-experience/entries/` and `docs/error-experience/summary/entries/`
- `docs/good-experience/entries/` and `docs/good-experience/summary/entries/`
- `docs/memory/long-term.md`
- Memory-related plans under `docs/plans/`

See [MEMORY_INDEXING.md](MEMORY_INDEXING.md) for graph artifacts, link normalization, and ID derivation details.

Reference: `docs/memory/networked/README.md`.

## Session Boot Sequence

**Identity-critical (must not be skipped):**
1. **[IDENTITY]** Read `SOUL.md` -- who you are.
2. **[IDENTITY]** Read `USER.md` -- who you are helping.

**Memory context (before doing anything else):**
3. **[RECENT]** Read `memory/YYYY-MM-DD.md` for today and yesterday.
4. **[MAIN]** If this is the main session (direct with the human), also read `MEMORY.md`.

Rule: do this automatically; do not ask for permission.

See [memory-management.md](../guides/memory-management.md) for the always-load set and on-demand loading policy.

## Writing Memory

### When to Write

- If the user says "remember this", write it.
- If a decision, preference, constraint, or contact will matter later, write it.
- Prefer the daily log first; promote to `MEMORY.md` only when the fact is durable.
- On successful tasks, auto-capture 1-3 short memory bullets into the daily log.

### Daily Log (`memory/YYYY-MM-DD.md`)

Short, time-stamped notes learned during the session.

```
# 2026-02-02

## 10:30 AM - Topic
One concise paragraph capturing the decision or fact.
```

- Append only; do not rewrite history.
- Prefer short, high-signal notes (decisions, constraints, preferences).
- Include exact identifiers (IDs, URLs, config keys) when they matter.

### Long-Term Memory (`MEMORY.md`)

Durable facts that persist across sessions.

```
# Long-Term Memory

## User Preferences
- Prefers TypeScript over JavaScript.
- Likes concise explanations.
```

- Keep entries compact and stable (avoid transient states).
- Group by topic (preferences, decisions, contacts, project facts).
- Remove outdated facts rather than piling on contradictory notes.

See [memory-management.md](../guides/memory-management.md) for authoring rules on repo documentation entries.

## Retrieval Workflow

1. Run `memory_search` before answering questions about prior work, decisions, dates, people, or preferences.
2. Use `memory_get` to read the exact lines from the top results.
3. Use `memory_related` to expand 1-hop linked memories when continuity matters.
4. If nothing relevant is found, say so instead of guessing.

## Compaction Safety

- Before context compression, store high-signal facts in the daily log.
- Compression summaries are lossy; durable facts must live in Markdown.

## System Prompt Injection

When enabled, the system prompt loads:
- `MEMORY.md`
- today's daily log
- yesterday's daily log

## Bootstrap File Injection (First Turn)

On the first turn of a session in `prompt.mode=full`, bootstrap files are injected into the prompt under `# Workspace Files`.

- Source priority: global-first (`~/.alex/memory/<file>` then `<workspace>/<file>`).
- Per-file size capped by `proactive.prompt.bootstrap_max_chars` (default `20000`).
- Truncated files marked with `...[TRUNCATED]`.
- Missing files represented by a short missing-file marker line.
