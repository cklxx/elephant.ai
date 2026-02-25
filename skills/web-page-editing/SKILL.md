---
name: web-page-editing
description: HTML 内容编辑 — 解析/修改/生成 HTML 页面内容。
triggers:
  intent_patterns:
    - "html|网页编辑|web.?page|edit.?html|修改网页"
  context_signals:
    keywords: ["html", "网页", "web page", "编辑"]
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# web-page-editing

HTML 内容编辑：解析、修改、生成 HTML 片段。

## 调用

```bash
python3 skills/web-page-editing/run.py '{"action":"extract","html":"<div><p>Hello</p></div>","selector":"p"}'
python3 skills/web-page-editing/run.py '{"action":"generate","template":"landing","title":"My Page"}'
```
