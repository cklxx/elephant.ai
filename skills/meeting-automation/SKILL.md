---
name: meeting-automation
description: 飞书视频会议的查询和会议室管理。
triggers:
  intent_patterns:
    - "会议|meeting|视频会议|vc|会议室|zoom"
  context_signals:
    keywords: ["会议", "meeting", "视频会议", "vc", "会议室", "zoom", "录制"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-vc
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# meeting-automation

飞书视频会议管理：查询会议列表、获取会议详情、查询会议室。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_meetings` | 列出指定时间范围的会议 |
| `get_meeting` | 获取会议详情 |
| `list_rooms` | 列出会议室 |

## 参数

### list_meetings
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_time | string | 是 | 开始时间（Unix 时间戳） |
| end_time | string | 是 | 结束时间（Unix 时间戳） |
| page_size | integer | 否 | 每页数量（默认 20） |
| page_token | string | 否 | 分页 token |

### get_meeting
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| meeting_id | string | 是 | 会议 ID |

### list_rooms
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| room_level_id | string | 否 | 会议室层级 ID |
| page_size | integer | 否 | 每页数量（默认 20） |
| page_token | string | 否 | 分页 token |

## 示例

```
查看今天的会议
-> channel(action="list_meetings", start_time="1700000000", end_time="1700086400")

查看会议详情
-> channel(action="get_meeting", meeting_id="m_xxx")

列出会议室
-> channel(action="list_rooms")
```

## 安全等级

- 所有操作: L1 只读，无需审批
