---
name: soul-self-evolution
description: When personality/collaboration strategy needs updating → evolve SOUL config under immutable-section protection with rollback support.
triggers:
  intent_patterns:
    - "soul|人格更新|self evolve|自我优化|更新协作风格"
    - "调整.*性格|adjust.*personality|改.*语气|change.*tone"
    - "你.*太.*了|you're.*too|不要.*这样|stop.*being"
    - "更.*主动|more.*proactive|更.*简洁|more.*concise|更.*温柔|more.*gentle"
    - "风格.*偏好|style.*preference|沟通.*方式|communication.*style"
    - "人格.*优化|persona.*optimize|行为.*模式|behavior.*pattern"
  context_signals:
    keywords: ["SOUL", "persona", "habit", "collaboration", "性格", "语气", "风格", "tone", "personality", "行为模式"]
  confidence_threshold: 0.7
priority: 9
requires_tools: [read_file, write_file]
max_tokens: 320
cooldown: 300
capabilities: [self_evolve_soul, policy_self_adjust]
governance_level: critical
activation_mode: semi_auto
depends_on_skills: [meta-orchestrator]
produces_events:
  - workflow.skill.meta.soul_updated
  - workflow.skill.meta.rollback_applied
requires_approval: false
---

# soul-self-evolution

对 `SOUL.md` 或 `docs/reference/SOUL.md` 进行受控更新：
- 只允许改动可演进段
- 记录 checkpoint
- 支持一键回滚

## 调用

```bash
python3 skills/soul-self-evolution/run.py apply --path docs/reference/SOUL.md --changes '[{"section":"## Collaboration Preferences","content":"- Keep updates concise."}]'
python3 skills/soul-self-evolution/run.py list_checkpoints
```
