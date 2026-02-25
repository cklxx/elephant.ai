---
name: meta-orchestrator
description: 元技能编排器 — 统一管理技能激活、联动、冲突仲裁、策略与审计摘要。
triggers:
  intent_patterns:
    - "meta.?skill|元技能|skill orchestration|技能联动|governance"
  context_signals:
    keywords: ["skills", "orchestrator", "governance", "联动"]
  confidence_threshold: 0.7
priority: 10
requires_tools: [bash]
max_tokens: 300
cooldown: 30
capabilities: [orchestrate_skills, skill_linkage, skill_governance]
governance_level: high
activation_mode: auto
depends_on_skills: []
produces_events:
  - workflow.skill.meta.route_selected
  - workflow.skill.meta.link_executed
  - workflow.skill.meta.rollback_applied
requires_approval: false
---

# meta-orchestrator

统一技能控制平面：根据策略执行技能筛选、依赖排序、联动关系输出、风险摘要。

## 调用

```bash
python3 skills/meta-orchestrator/run.py '{"action":"plan","skills":[{"name":"autonomous-scheduler","score":0.8}],"proactive_level":"medium"}'
python3 skills/meta-orchestrator/run.py '{"action":"summarize","plan":{"selected_skills":["meta-orchestrator"]}}'
```
