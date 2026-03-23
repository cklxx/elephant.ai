---
name: memory-search
description: When you need to recall past conversation context → search and read conversation memories (stored as Markdown).
triggers:
  intent_patterns:
    - "记忆|memory|回忆|之前说过|历史|上次"
    - "你还记得|do you remember|我们.*讨论过|we.*discussed"
    - "之前.*提到|previously.*mentioned|earlier.*said|前几天"
    - "找.*聊天记录|search.*history|翻.*记录|look.*back"
    - "上一次.*怎么做的|last.*time|那次.*结论|what.*decided"
    - "忘了.*叫什么|forgot.*name|想不起来|can't recall"
  context_signals:
    keywords: ["memory", "记忆", "回忆", "历史", "之前", "上次", "记得", "讨论", "提到", "记录"]
  confidence_threshold: 0.55
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# memory-search

搜索和读取对话记忆。

## 调用

```bash
python3 skills/memory-search/run.py search --query '项目进度'
python3 skills/memory-search/run.py get --file 2026-02-09-meeting.md
```
