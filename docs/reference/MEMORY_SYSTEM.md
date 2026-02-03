# Markdown Memory System

Updated: 2026-02-03

## Overview
- Memory is Markdown-only; Markdown files are the single source of truth under `~/.alex/memory/`.
- Long-term memory is curated in `MEMORY.md`.
- Daily notes are append-only in `memory/YYYY-MM-DD.md`.
- Search with `memory_search`, then read exact lines with `memory_get`.
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

~/.alex/memory/<user-id>/
├── MEMORY.md
└── memory/
    ├── 2026-02-02.md
    └── ...
```

## Session Boot Sequence
**Identity-critical (must not be skipped):**
1. **[IDENTITY]** Read `SOUL.md` — who you are.
2. **[IDENTITY]** Read `USER.md` — who you are helping.

**Memory context (before doing anything else):**
3. **[RECENT]** Read `memory/YYYY-MM-DD.md` for **today** and **yesterday**.
4. **[MAIN]** If this is the **main session** (direct with the human), also read `MEMORY.md`.

Note: If `user_id` is missing, the agent reads/writes directly under `~/.alex/memory/` (no per-user subdir).
If `user_id` collides with reserved names (`memory`, `MEMORY.md`, `index.sqlite`, `users`), it is stored under `user-<user_id>` to avoid conflicts.

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
3. If nothing relevant is found, say so instead of guessing.

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
