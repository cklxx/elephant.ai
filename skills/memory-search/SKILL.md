---
name: memory-search
description: 记忆检索 — 搜索和读取对话记忆（存储为 Markdown 文件）。
triggers:
  intent_patterns:
    - "记忆|memory|回忆|之前说过|历史|上次"
  context_signals:
    keywords: ["memory", "记忆", "回忆", "历史"]
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# memory-search

搜索和读取对话记忆。

## 调用

```bash
python3 skills/memory-search/run.py '{"action":"search","query":"项目进度"}'
python3 skills/memory-search/run.py '{"action":"get","file":"2026-02-09-meeting.md"}'
```
