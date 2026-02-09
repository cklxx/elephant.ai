---
name: a2ui_templates
description: On-demand A2UI templates for flowchart, form, dashboard, info cards, and gallery.
triggers:
  intent_patterns:
    - "a2ui|flowchart|form|dashboard|card grid|gallery|UI mock"
  context_signals:
    keywords: ["a2ui", "flowchart", "form", "dashboard", "cards", "gallery", "ui"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# a2ui-templates

Generate A2UI component payloads for visual layouts: flowcharts, forms, dashboards, info cards, and galleries. Templates and rendering logic are handled by run.py.

## 调用

```bash
python3 skills/a2ui-templates/run.py '{"action":"render","template":"flowchart","title":"Release Flow","data":{"steps":["Design","Build","Deploy"]}}'
```
