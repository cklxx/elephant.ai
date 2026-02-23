# Markdown Memory System

Updated: 2026-02-23

## Overview
- Memory is Markdown-only; Markdown files are the single source of truth under `~/.alex/memory/`.
- Long-term memory is curated in `MEMORY.md`.
- Daily notes are append-only in `memory/YYYY-MM-DD.md`.
- Search with `memory_search`, then read exact lines with `memory_get`, and expand linked memory with `memory_related`.
- There is no dedicated `memory_write` tool; write memories via normal file tools.

## Context vs Memory
- **Context**: system prompt + project context + conversation history + tool outputs + current message. It is transient and bounded by the model window.
- **Memory**: durable Markdown files on disk (`MEMORY.md` + `memory/*.md`). It is persistent, cheap to store, and searchable.

## Storage Layout

```text
~/.alex/memory/
├── MEMORY.md
└── memory/
    ├── 2026-02-02.md
    ├── 2026-02-01.md
    └── ...
```

## Repo Documentation Memory (Networked)
Project memory docs live under `docs/` and are intentionally networked via IDs, tags, and link edges.

### Sources
- `docs/error-experience/entries/`
- `docs/error-experience/summary/entries/`
- `docs/good-experience/entries/`
- `docs/good-experience/summary/entries/`
- `docs/memory/long-term.md`
- Memory-related plans under `docs/plans/`

### Networked Index Artifacts
- `docs/memory/index.yaml` — node registry (IDs, paths, type, date, tags).
- `docs/memory/edges.yaml` — normalized edges.
- `docs/memory/tags.yaml` — controlled vocabulary for tags.

Cross-reference semantics: `related` is bidirectional; `see_also`/`supersedes`/`derived_from` are directed. `memory_related` expands only `related` edges. See `docs/memory/networked/README.md`.

### Entry Metadata (New Entries)
Add a YAML metadata block under `## Metadata` in new error/good entries. Legacy entries without metadata remain valid; the indexer infers IDs/tags from filenames and content when possible.

Reference: `docs/memory/networked/README.md`.

## Session Boot Sequence
**Identity-critical (must not be skipped):**
1. **[IDENTITY]** Read `SOUL.md` — who you are.
2. **[IDENTITY]** Read `USER.md` — who you are helping.

**Memory context (before doing anything else):**
3. **[RECENT]** Read `memory/YYYY-MM-DD.md` for **today** and **yesterday**.
4. **[MAIN]** If this is the **main session** (direct with the human), also read `MEMORY.md`.

Rule: do this automatically; do not ask for permission.

## 每次会话（身份信息必须优先）
在干别的事之前：
1. 读 `SOUL.md` —— 这是「你是谁」
2. 读 `USER.md` —— 这是「你在帮谁」
3. 读 `memory/YYYY-MM-DD.md`（今天和昨天的）以此获取最近的上下文
4. 如果是在主会话（MAIN SESSION）（直接跟人类聊天），还要读 `MEMORY.md`

别问许可，直接干就完了。

## Writing Memory
### When to write
- If the user says “记下来 / remember this”, write it.
- If a decision, preference, constraint, or contact will matter later, write it.
- Prefer the daily log first; promote to `MEMORY.md` only when the fact is durable.
- On successful tasks, auto-capture 1-3 short memory bullets into the daily log (LLM summary with rule-based fallback).

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
- If unsure about durability, keep it here and do not promote.

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
- Promote durable daily entries here; delete outdated facts.

## Retrieval Workflow
1. Run `memory_search` before answering questions about prior work, decisions, dates, people, or preferences.
2. Use `memory_get` to read the exact lines from the top results.
3. Use `memory_related` to expand 1-hop linked memories when continuity matters.
4. If nothing relevant is found, say so instead of guessing.

Example:
```json
{ "query": "API decision REST vs GraphQL" }
```

## Compaction Safety
- Before context compression, store high-signal facts in the daily log.
- Compression summaries are lossy; durable facts must live in Markdown.

## System Prompt Injection
- When enabled, the system prompt loads:
  - `MEMORY.md`
  - today’s daily log
  - yesterday’s daily log
- This keeps context small while retaining durable knowledge.

## Bootstrap File Injection (First Turn)
- On the first turn of a session in `prompt.mode=full`, bootstrap files are injected into the prompt under `# Workspace Files`.
- Source priority is **Global-first**:
  1) `~/.alex/memory/<file>`
  2) `<workspace>/<file>`
- Per-file size is capped by `proactive.prompt.bootstrap_max_chars` (default `20000`).
- Truncated files are marked with `...[TRUNCATED]`.
- Missing files are represented by a short missing-file marker line.
