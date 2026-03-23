---
name: deep-research
description: When a topic needs thorough investigation from multiple sources → multi-source search + evidence assembly + structured report.
triggers:
  intent_patterns:
    - "research|调研|调查|分析|analysis|market"
    - "帮我查.*背景|background.*research|了解一下|深入了解"
    - "竞品分析|competitive.*analysis|行业.*报告|industry.*report"
    - "总结.*资料|summarize.*sources|综合.*信息|compile.*findings"
    - "这个.*怎么回事|what.*happened|来龙去脉|前因后果"
    - "趋势|trend|现状|current.*state|发展.*方向"
    - "对比.*方案|compare.*options|选型|tech.*selection|evaluate.*alternatives"
    - "帮我.*整理|help.*organize|梳理|sort.*out"
  tool_signals:
    - web_search
    - web_fetch
  context_signals:
    keywords: ["调研", "研究", "analysis", "research", "竞品", "趋势", "选型", "background", "investigate", "对比", "综合", "梳理", "evaluate"]
  confidence_threshold: 0.6
priority: 8
exclusive_group: research
max_tokens: 200
cooldown: 300
requires_tools: [bash]
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# deep-research

多源检索 + 证据汇编 + 结构化报告。

## 调用

```bash
python3 skills/deep-research/run.py --topic '研究主题' --queries '["关键词1","关键词2"]' --max_results 5 --depth basic
```

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| topic | string | 是 | 研究主题 |
| queries | string[] | 否 | 搜索关键词，默认自动生成 3 条 |
| max_results | int | 否 | 每条 query 结果数，默认 5 |
| depth | string | 否 | "basic" 或 "advanced" |
| fetch_urls | string[] | 否 | 额外抓取全文的 URL |

## 输出

返回 JSON，包含 `searches`（搜索结果）、`fetched_pages`（抓取页面）、`summary_prompt`（综合提示）。

LLM 拿到结果后，按「问题→发现/证据→置信度→影响/建议」结构化整理。
