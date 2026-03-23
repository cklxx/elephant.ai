---
name: code-review
description: When coding is done and changes need review → multi-dimensional code review (SOLID, security, quality, edge cases), outputs structured report.
triggers:
  intent_patterns:
    - "review|审查|code review|CR|代码审查|review code"
    - "帮我看看代码|check.*my.*code|看下.*改得.*对不对"
    - "代码.*有没有.*问题|any.*issues|有没有.*bug|security.*check"
    - "merge.*前.*检查|pre.*merge|提交前.*审|before.*commit"
    - "代码.*质量|code.*quality|clean.*code|重构.*建议|refactor.*suggestion"
    - "看下.*diff|review.*changes|这次.*改动|check.*diff"
    - "有没有.*安全.*问题|vulnerability|漏洞|注入|injection"
  context_signals:
    keywords: ["review", "审查", "CR", "code review", "代码质量", "merge", "diff", "安全", "质量", "检查", "vulnerability", "refactor"]
  confidence_threshold: 0.6
priority: 9
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# code-review

Run a multi-dimensional code review (SOLID architecture, security, quality, edge cases, cleanup) on the current diff and output a structured report with severity levels (P0-P3). All review checklists, workflow steps, and report generation are handled by run.py.

## 调用

```bash
python3 skills/code-review/run.py review
```
