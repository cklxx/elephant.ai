---
name: ppt-deck
description: When the user needs a presentation → PPT playbook (goal/audience → storyline → layout → accessibility → delivery).
triggers:
  intent_patterns:
    - "ppt|slides|deck|presentation|演示稿|路演|汇报"
  context_signals:
    keywords: ["ppt", "slides", "deck", "presentation", "演示", "汇报"]
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
