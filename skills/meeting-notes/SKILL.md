---
name: meeting-notes
description: 将会议/访谈记录快速转成可分发纪要，突出决策、行动项、风险与待澄清问题。
triggers:
  intent_patterns:
    - "meeting notes|会议纪要|会议记录|会议总结|sync"
  context_signals:
    keywords: ["会议", "纪要", "notes", "summary"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# meeting-notes

Transform raw meeting/interview notes into structured, distributable minutes with decisions, action items (owner + deadline), risks, and open questions. All extraction, formatting, and validation logic are handled by run.py.

## 调用

```bash
python3 skills/meeting-notes/run.py '{"action":"summarize","raw_notes":"..."}'
```
