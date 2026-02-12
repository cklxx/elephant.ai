---
name: lark-conversation-governor
description: Lark 对话治理技能 — 主动消息节奏控制、静默窗、停用信号检测。
triggers:
  intent_patterns:
    - "lark|飞书|主动提醒|proactive message|消息治理"
  context_signals:
    keywords: ["lark", "message", "reminder", "quiet"]
  confidence_threshold: 0.65
priority: 8
requires_tools: [channel]
max_tokens: 260
cooldown: 60
capabilities: [lark_chat, proactive_messaging]
governance_level: medium
activation_mode: auto
depends_on_skills: [meta-orchestrator, autonomous-scheduler]
produces_events:
  - workflow.skill.meta.link_executed
requires_approval: false
---

# lark-conversation-governor

管理 Lark 主动沟通策略：停用词优先、静默时段约束、中频推进节奏。

## 调用

```bash
python3 skills/lark-conversation-governor/run.py '{"action":"evaluate","text":"今天提醒我复盘"}'
python3 skills/lark-conversation-governor/run.py '{"action":"compose","objective":"周报","status":"已完成70%","next_step":"补齐风险项"}'
```
