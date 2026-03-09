---
name: feishu-im-read
description: |
  飞书消息读取工具。支持读取群聊/私聊历史消息、话题回复和文件下载。

  **当以下情况时使用此 Skill**：
  (1) 需要读取聊天历史消息
  (2) 需要搜索消息
  (3) 需要下载消息中的图片/文件
  (4) 用户提到"消息记录"、"聊天历史"、"message history"
triggers:
  intent_patterns:
    - "消息记录|聊天历史|message history|聊天记录"
    - "发送消息|send message|消息搜索"
  context_signals:
    keywords: ["message", "消息", "chat", "聊天", "history", "send_message"]
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# Feishu IM Read Skill

## 快速索引

| 用户意图 | module | tool_action | 必填参数 |
|---------|--------|-------------|---------|
| 发送消息 | message | send_message | chat_id/user_id, content |
| 读取消息历史 | message | history | chat_id |
| 上传文件 | message | upload_file | file_path, file_type |

## 调用示例

### 发送消息

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "message",
  "tool_action": "send_message",
  "chat_id": "oc_xxx",
  "content": "Hello! 这是一条测试消息"
}'
```

### 读取消息历史

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "message",
  "tool_action": "history",
  "chat_id": "oc_xxx",
  "start_time": "2026-03-01",
  "end_time": "2026-03-10",
  "page_size": 20
}'
```

## 核心约束

- **chat_id**: 群聊 ID（`oc_` 开头）
- **user_id**: 用户 open_id（`ou_` 开头），用于私聊
- **消息类型**: 支持 text、post（富文本）、image、file 等
- **分页**: 使用 `page_token` 和 `page_size` 进行分页
- **时间过滤**: `start_time` 和 `end_time` 支持日期字符串
