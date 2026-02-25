---
name: autonomous-scheduler
description: 自主调度技能 — 管理周期任务、去重、触发窗口与执行状态。
triggers:
  intent_patterns:
    - "定时|schedule|cron|提醒计划|周期任务|自主调度"
  context_signals:
    keywords: ["schedule", "cron", "timer", "任务计划"]
  confidence_threshold: 0.65
priority: 8
requires_tools: [bash]
max_tokens: 260
cooldown: 45
capabilities: [self_schedule, scheduler_management]
governance_level: medium
activation_mode: auto
depends_on_skills: [meta-orchestrator]
produces_events:
  - schedule.created
  - schedule.updated
  - schedule.deleted
requires_approval: false
---

# autonomous-scheduler

维护自主任务计划：任务 upsert/list/delete、触发窗口查询、运行状态推进。

## 调用

```bash
python3 skills/autonomous-scheduler/run.py '{"action":"upsert","name":"weekly-retro","schedule":"0 18 * * 5","task":"发送复盘提醒"}'
python3 skills/autonomous-scheduler/run.py '{"action":"due","now":"2026-02-13T10:00:00Z"}'
```
