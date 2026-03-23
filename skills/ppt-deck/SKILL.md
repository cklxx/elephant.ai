---
name: ppt-deck
description: When the user needs a presentation → PPT playbook (goal/audience → storyline → layout → accessibility → delivery).
triggers:
  intent_patterns:
    - "ppt|slides|deck|presentation|演示稿|路演|汇报"
    - "做个.*PPT|make.*presentation|准备.*演示|prepare.*slides"
    - "周报|weekly.*report|月报|monthly.*report|总结.*汇报"
    - "pitch.*deck|融资.*BP|商业.*计划|business.*plan"
    - "技术.*分享|tech.*talk|培训.*材料|training.*material"
    - "keynote|演讲.*稿|speech.*draft|发言.*材料"
  context_signals:
    keywords: ["ppt", "slides", "deck", "presentation", "演示", "汇报", "周报", "月报", "pitch", "keynote", "演讲", "分享", "培训"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# ppt-deck

Generate presentation decks from topic to deliverable PPTX/PDF, following storyline templates (SCQA, Pyramid, Before/After/Bridge), accessibility best practices, and 10/20/30 guidelines. All slide generation, layout, and export logic are handled by run.py.

## 调用

```bash
python3 skills/ppt-deck/run.py create --topic 'Q1 Review' --audience leadership --format pptx
```
