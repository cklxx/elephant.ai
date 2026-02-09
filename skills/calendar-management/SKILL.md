---
name: calendar-management
description: 日历事件的创建、查询、修改、删除管理。
triggers:
  intent_patterns:
    - "日程|日历|calendar|会议|meeting|schedule|安排|约会"
  context_signals:
    keywords: ["日程", "日历", "calendar", "meeting", "schedule", "安排"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: calendar
requires_tools: [bash, channel]
max_tokens: 200
cooldown: 30
---

# calendar-management

日历事件管理：创建、查询、修改、删除。

## 调用

```bash
python3 skills/calendar-management/run.py '{"action":"create", "title":"周会", "start":"2026-02-10 14:00", "duration":"60m", "attendees":["user@example.com"]}'
python3 skills/calendar-management/run.py '{"action":"query", "start":"2026-02-10", "end":"2026-02-14"}'
python3 skills/calendar-management/run.py '{"action":"delete", "event_id":"evt_xxx"}'
```

## 参数

### create
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 是 | 事件标题 |
| start | string | 是 | 开始时间 (YYYY-MM-DD HH:MM) |
| duration | string | 否 | 时长（30m/1h/2h），默认 60m |
| attendees | string[] | 否 | 参会者邮箱列表 |
| description | string | 否 | 事件描述 |

### query
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start | string | 是 | 查询开始日期 |
| end | string | 否 | 查询结束日期 |

### delete / update
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| event_id | string | 是 | 事件 ID |
