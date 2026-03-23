---
name: json-render-templates
description: When structured data needs visual presentation → use json-render templates (flowchart, form, dashboard, cards, gallery, table, kanban, diagram).
triggers:
  intent_patterns:
    - "json.render|visual ui|diagram|layout|flowchart|form|dashboard|card grid|gallery|kanban|table"
    - "做个.*看板|make.*kanban|做个.*表格|create.*table|展示.*数据"
    - "卡片.*布局|card.*layout|画廊|gallery.*view|网格.*展示"
    - "仪表盘|dashboard|数据.*面板|data.*panel|监控.*大屏"
    - "表单|form.*ui|输入.*界面|input.*form|问卷"
    - "可视化.*展示|visual.*display|好看.*一点|pretty.*format"
  context_signals:
    keywords: ["json-render", "flowchart", "form", "dashboard", "cards", "gallery", "table", "kanban", "diagram", "看板", "仪表盘", "表单", "卡片", "可视化", "展示"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# json-render-templates

Generate json-render protocol payloads for visual layouts: flowcharts, forms, dashboards, info cards, galleries, tables, kanban boards, and diagrams. All templates and serialization logic are handled by run.py.

## 调用

```bash
python3 skills/json-render-templates/run.py render --template dashboard --title 'Product Dashboard' --data '{"metrics":[{"label":"Active users","value":12450}]}'
```
