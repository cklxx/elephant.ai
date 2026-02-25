---
name: background-tasks
description: 后台任务管理 — 创建/查询/收集后台长时间运行的任务。
triggers:
  intent_patterns:
    - "后台|background|异步|async|长时间|long.?running"
  context_signals:
    keywords: ["background", "后台", "async", "异步"]
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# background-tasks

管理后台任务：派发、查询状态、收集结果。

## 调用

```bash
python3 skills/background-tasks/run.py '{"action":"dispatch","command":"go test ./...","description":"run tests"}'
python3 skills/background-tasks/run.py '{"action":"list"}'
python3 skills/background-tasks/run.py '{"action":"collect","task_id":"abc"}'
```
