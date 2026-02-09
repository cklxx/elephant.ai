---
name: lark-messaging
description: Lark 消息发送 — 通过 Lark Open API 发送消息到群组或个人。
triggers:
  intent_patterns:
    - "发消息|send.?message|lark.*消息|飞书.*消息|通知.*群"
  context_signals:
    keywords: ["lark", "飞书", "消息", "message", "通知"]
  confidence_threshold: 0.7
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# lark-messaging

Lark 消息发送：通过 Open API 发送文本/富文本消息。

## 调用

```bash
python3 skills/lark-messaging/run.py '{"action":"send","chat_id":"oc_xxx","content":"Hello!"}'
```
