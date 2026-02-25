---
name: scheduled-tasks
description: 定时任务管理 — 创建/查询/删除 cron 调度任务。
triggers:
  intent_patterns:
    - "定时|cron|schedule|定期|周期|定时任务|scheduled"
  context_signals:
    keywords: ["cron", "schedule", "定时", "定期", "周期"]
  confidence_threshold: 0.6
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# scheduled-tasks

管理 cron 调度任务：创建、列表、删除。

## 调用

```bash
python3 skills/scheduled-tasks/run.py '{"action":"create","name":"daily-report","cron":"0 9 * * *","command":"echo hello"}'
python3 skills/scheduled-tasks/run.py '{"action":"list"}'
python3 skills/scheduled-tasks/run.py '{"action":"delete","name":"daily-report"}'
```
