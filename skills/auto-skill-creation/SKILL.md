---
name: auto-skill-creation
description: When a repeated workflow should be codified as a reusable skill → auto-create skill files, optionally using Codex/Claude.
triggers:
  intent_patterns:
    - "create skill|创建技能|新技能|auto skill|沉淀技能|skill creation"
    - "这个.*流程.*自动化|automate.*this.*workflow|封装.*技能|wrap.*skill"
    - "做成.*可复用|make.*reusable|模板化|templatize"
    - "以后.*直接.*用|use.*directly.*next.*time|固化.*下来|codify"
    - "新增.*能力|add.*capability|扩展.*技能|extend.*skill"
  context_signals:
    keywords: ["skill", "技能", "create", "创建", "沉淀", "auto", "封装", "复用", "模板", "自动化", "能力"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# auto-skill-creation

Automatically create new skills from repeated task patterns by dispatching to external agents (Codex/Claude), collecting results, and generating compliant skill directory structures. All task dispatch, status monitoring, and file generation logic are handled by run.py.

## 调用

```bash
python3 skills/auto-skill-creation/run.py create --skill_name my-new-skill --description 'What the skill does' --agent_type codex
```
