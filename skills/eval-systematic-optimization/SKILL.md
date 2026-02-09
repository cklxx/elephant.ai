---
name: eval-systematic-optimization
description: 系统化梳理 foundation-suite 失败 case，按冲突簇优化 pass@1，并输出标准化 x/x 报告与 good/bad 抽样检查。
triggers:
  intent_patterns:
    - "评测.*优化|pass@1|pass@5|失败case|failure case|系统性评测|benchmark.*optimi"
  context_signals:
    keywords: ["foundation-suite", "pass@1", "pass@5", "x/x", "goodcase", "badcase", "冲突"]
  confidence_threshold: 0.7
priority: 8
requires_tools: [bash]
max_tokens: 200
cooldown: 90
---

# eval-systematic-optimization

Run foundation-suite evaluation baselines, cluster failures by conflict family, apply systematic rule-layer optimizations (not single-case fixes), and produce standardized x/x reports with good/bad sampling. All phases from baseline to regression verification are handled by run.py.

## 调用

```bash
python3 skills/eval-systematic-optimization/run.py '{"action":"optimize","tag":"r12"}'
```
