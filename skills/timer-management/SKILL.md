---
name: timer-management
description: 定时提醒、倒计时、周期性任务的设置与管理。
triggers:
  intent_patterns:
    - "提醒|remind|timer|定时|倒计时|countdown|闹钟|alarm"
  context_signals:
    keywords: ["提醒", "remind", "timer", "定时", "倒计时"]
  confidence_threshold: 0.5
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# timer-management

定时提醒的设置、查询和取消。

## 调用

```bash
# 设置定时器
python3 scripts/cli/timer/timer_cli.py set '{"delay":"30m", "task":"喝水提醒"}'

# 查看所有定时器
python3 scripts/cli/timer/timer_cli.py list

# 取消定时器
python3 scripts/cli/timer/timer_cli.py cancel '{"id":"timer-12345"}'
```

## 参数

### set
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| delay | string | 是 | 延迟时间（30s / 5m / 2h） |
| task | string | 是 | 提醒内容 |

### cancel
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 定时器 ID |
