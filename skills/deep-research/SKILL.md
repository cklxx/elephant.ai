---
name: deep-research
description: 深度调研技能，多源检索 + 证据汇编 + 结构化报告。
triggers:
  intent_patterns:
    - "research|调研|调查|分析|analysis|market"
  tool_signals:
    - web_search
    - web_fetch
  context_signals:
    keywords: ["调研", "研究", "analysis", "research"]
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
python3 skills/deep-research/run.py '{"topic":"研究主题", "queries":["关键词1","关键词2"], "max_results":5, "depth":"basic"}'
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
