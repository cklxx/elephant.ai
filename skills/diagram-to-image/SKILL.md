---
name: diagram-to-image
description: 将 Mermaid 流程图/架构图或“图标块”渲染为可直接发到 Lark 的美观 PNG（可选同时输出 SVG）。
triggers:
  intent_patterns:
    - "mermaid|流程图|架构图|时序图|sequence diagram|flowchart"
    - "转成图片|导出图片|render.*(png|svg)|diagram.*(png|图片)"
    - "icon block|图标块|图标卡片|信息卡片"
  tool_signals:
    - diagram_render
  context_signals:
    keywords: ["mermaid", "diagram", "flowchart", "流程图", "架构图", "时序图", "图标", "png", "svg", "render", "导出"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: diagram
max_tokens: 1800
cooldown: 120
output:
  format: markdown
  artifacts: true
  artifact_type: image
---

# Diagram → Image（Mermaid / Icon Blocks）

## When to use this skill
- 用户提供 Mermaid（或希望你把流程整理成 Mermaid），并要求“转成图片/PNG/SVG/发到 Lark”。
- 用户给一组“图标块 / 信息卡片”内容，希望做成美观图片发群里。

## Inputs
### Mermaid
- Mermaid 源码（可包含 ```mermaid code fence）。
- 期望主题：light/dark（默认 light）。
- 是否需要 SVG：默认只出 PNG；需要时出 `png_svg` 或 `svg`。

### Icon Blocks
- 结构化为 `items[]`：
  - `icon`（emoji 或短文本）
  - `title`
  - `description`（可选）
- 可选 `title`（整体标题）。

## Workflow
1) **识别输入类型**
   - 如果用户贴了 Mermaid（或明显是 Mermaid code fence），走 Mermaid 渲染。
   - 如果用户给的是卡片式要点/模块清单（带 icon/title/desc），抽取成 icon blocks。
2) **结构化/最小化**
   - Mermaid：节点名短、边清晰；不要塞大段长文本（必要时用编号+注释）。
   - Icon blocks：每个 item 1 句标题 + 1 句描述即可。
3) **调用 `diagram_render` 产出图片**
   - Mermaid → PNG（默认），必要时同时输出 SVG。
   - Icon blocks → PNG（SVG 不支持）。
4) **最终回复**
   - 文本只写一句说明（例如“已渲染并附上图片”）。
   - 如 Mermaid 渲染失败：返回错误摘要 + 最小可复现 Mermaid + 具体修复建议（语法/字符/未闭合等）。

## Tool call examples (YAML)

### Mermaid → PNG（默认）
```yaml
tool: diagram_render
args:
  format: mermaid
  source: |
    ```mermaid
    graph LR
      A[Client] --> B[API]
      B --> C[(DB)]
    ```
  theme: light
  output: png
  name: diagram
```

### Mermaid → PNG + SVG
```yaml
tool: diagram_render
args:
  format: mermaid
  source: |
    sequenceDiagram
      participant U as User
      participant S as Service
      U->>S: Request
      S-->>U: Response
  theme: dark
  output: png_svg
  name: seq
```

### Icon blocks → PNG
```yaml
tool: diagram_render
args:
  format: icon_blocks
  title: "Release Highlights"
  items:
    - icon: "🚀"
      title: "Ship"
      description: "Deploy to production with checks"
    - icon: "🔍"
      title: "Observe"
      description: "Monitor SLO + errors"
    - icon: "🧯"
      title: "Rollback"
      description: "Fast mitigation when needed"
  theme: light
  output: png
  name: highlights
```

