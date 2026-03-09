---
name: feishu-calendar
description: |
  飞书日历管理工具。支持日历事件的创建、查询、更新、删除，参会人管理和忙闲查询。

  **当以下情况时使用此 Skill**：
  (1) 需要创建、查询、修改、删除日历事件
  (2) 需要查看忙闲状态或安排会议
  (3) 需要管理参会人
  (4) 用户提到"日历"、"日程"、"会议"、"calendar"
triggers:
  intent_patterns:
    - "日历|日程|calendar|会议|meeting"
    - "创建日程|安排会议|查看日历|空闲时间|忙闲"
  context_signals:
    keywords: ["calendar", "日历", "日程", "会议", "event", "attendee", "freebusy"]
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# Feishu Calendar Skill

## 快速索引

| 用户意图 | module | tool_action | 必填参数 |
|---------|--------|-------------|---------|
| 创建日程 | calendar | create | title, start, duration |
| 查询日程 | calendar | query | start, end |
| 更新日程 | calendar | update | event_id + 要更新的字段 |
| 删除日程 | calendar | delete | event_id |
| 列出日历 | calendar | list_calendars | - |
| 查忙闲 | calendar | freebusy | user_ids, start, end |

## 调用示例

### 创建日程

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "calendar",
  "tool_action": "create",
  "title": "周会",
  "start": "2026-03-10 10:00",
  "duration": "60m",
  "attendees": ["ou_xxx", "ou_yyy"],
  "description": "每周同步进度"
}'
```

### 查询日程

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "calendar",
  "tool_action": "query",
  "start": "2026-03-10",
  "end": "2026-03-14"
}'
```

### 查忙闲

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "calendar",
  "tool_action": "freebusy",
  "user_ids": ["ou_xxx", "ou_yyy"],
  "start": "2026-03-10 09:00",
  "end": "2026-03-10 18:00"
}'
```

## 核心约束

- **时间格式**：支持 `"2026-03-10 10:00"` 或 ISO 8601 `"2026-03-10T10:00:00+08:00"`
- **duration**: 支持 `"30m"`, `"1h"`, `"1h30m"` 等格式
- **attendees**: 使用 `open_id`（`ou_` 开头），数组格式
- **时区**: 默认使用服务器时区，可通过 `timezone` 参数指定（如 `"Asia/Shanghai"`）
- **日历 ID**: 大部分操作默认使用主日历，可通过 `calendar_id` 指定其他日历
- **重复日程**: 使用 `recurrence` 参数（RRULE 格式），如 `"FREQ=WEEKLY;BYDAY=MO"`

## 常见错误

| 错误 | 原因 | 解决 |
|------|------|------|
| 无权限访问日历 | 未授权 calendar 相关 scope | 执行 OAuth 授权 |
| 参会人无法添加 | user_id 格式错误 | 确认使用 open_id（ou_开头） |
| 时间冲突 | 目标时间段已有日程 | 先查询忙闲确认空闲 |
