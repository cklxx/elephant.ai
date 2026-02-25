---
name: okr-management
description: 创建和管理 OKR（目标与关键结果），支持创建、回顾和进度更新工作流。
triggers:
  intent_patterns:
    - "OKR|okr|目标|关键结果|key result|季度目标|quarterly goal"
  context_signals:
    keywords: ["OKR", "okr", "目标", "KR", "关键结果", "进度"]
  confidence_threshold: 0.6
priority: 8
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# okr-management

Create, review, and update OKRs (Objectives and Key Results) with automatic progress calculation and status dashboards. Supports creation workflows, review ticks, and batch KR updates. All OKR logic and formatting are handled by run.py.

## 调用

```bash
python3 skills/okr-management/run.py '{"action":"create","objective":"Increase Q1 revenue","key_results":[{"metric":"MRR","baseline":100,"target":150}]}'
```
