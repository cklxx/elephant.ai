---
name: diagram-to-image
description: 将 Mermaid 流程图/架构图或"图标块"渲染为可直接发到 Lark 的美观 PNG（可选同时输出 SVG）。
triggers:
  intent_patterns:
    - "mermaid|流程图|架构图|时序图|sequence diagram|flowchart"
    - "转成图片|导出图片|render.*(png|svg)|diagram.*(png|图片)"
    - "icon block|图标块|图标卡片|信息卡片"
  context_signals:
    keywords: ["mermaid", "diagram", "flowchart", "流程图", "架构图", "时序图", "图标", "png", "svg", "render", "导出"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# diagram-to-image

Render Mermaid diagrams or icon-block layouts into PNG/SVG images suitable for Lark sharing. Supports flowcharts, sequence diagrams, architecture diagrams, and styled icon cards. All rendering and error handling logic are in run.py.

## 调用

```bash
python3 skills/diagram-to-image/run.py '{"action":"render","format":"mermaid","source":"graph LR\n  A[Client] --> B[API]\n  B --> C[(DB)]","output":"png"}'
```
