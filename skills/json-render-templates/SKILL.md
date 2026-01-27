---
name: json-render-templates
description: Json-render protocol templates for flowchart, form, dashboard, info cards, and gallery.
---

# Json-Render Templates (Flowchart, Form, Dashboard, Cards, Gallery)

## When to use this skill
- The user asks for a visual UI, diagram, or layout (flowchart, form, dashboard, card grid, gallery, UI mock).
- The answer is a structured multi-entity summary that benefits from layout rather than long prose.

## How to apply
1. Pick the closest template below.
2. Replace the data values.
3. Serialize the payload to JSON and emit via `a2ui_emit` using `content` (string).

## Template: Flowchart (message bundle)
```yaml
payload:
  type: ui
  version: "1.0"
  messages:
    - type: heading
      text: "Release Flow"
    - type: flow
      direction: horizontal
      nodes:
        - id: design
          label: "Design"
        - id: build
          label: "Build"
        - id: deploy
          label: "Deploy"
      edges:
        - from: design
          to: build
          label: ""
        - from: build
          to: deploy
          label: ""
```

## Template: Form
```yaml
payload:
  root:
    type: form
    props:
      title: "Onboarding Form"
      fields:
        - label: "Full name"
          type: input
          value: ""
        - label: "Email"
          type: input
          value: ""
        - label: "Role"
          type: input
          value: "Engineer"
        - label: "Notes"
          type: textarea
          value: ""
```

## Template: Dashboard
```yaml
payload:
  root:
    type: dashboard
    props:
      title: "Product Dashboard"
      metrics:
        - label: "Active users"
          value: 12450
        - label: "Conversion"
          value: "3.8%"
        - label: "Revenue"
          value: "$128k"
      items:
        - title: "Trial signup"
          meta: "+120 in last 24h"
        - title: "Churn risk"
          meta: "3 accounts flagged"
```

## Template: Info cards
```yaml
payload:
  root:
    type: cards
    props:
      items:
        - title: "Latency"
          subtitle: "p95"
          body: "210 ms"
        - title: "Error rate"
          subtitle: "last 7 days"
          body: "0.12%"
```

## Template: Gallery
```yaml
payload:
  root:
    type: gallery
    props:
      items:
        - url: "https://example.com/image-1.jpg"
          caption: "Homepage hero"
        - url: "https://example.com/image-2.jpg"
          caption: "Pricing layout"
```

## Template: Table
```yaml
payload:
  root:
    type: table
    props:
      headers: ["Service", "Latency(ms)", "Errors"]
      rows:
        - ["api-gateway", 120, "0.2%"]
        - ["recommendation", 180, "0.4%"]
```

## Template: Kanban
```yaml
payload:
  root:
    type: kanban
    props:
      columns:
        - title: "Todo"
          items:
            - title: "Collect requirements"
              meta: "Owner: PM"
        - title: "In Progress"
          items:
            - title: "Implement renderer"
              subtitle: "json-render"
        - title: "Done"
          items:
            - title: "Define templates"
              meta: "v1"
```

## Template: Diagram (nodes + edges)
```yaml
payload:
  root:
    type: diagram
    props:
      nodes:
        - id: service-a
          label: "Service A"
        - id: service-b
          label: "Service B"
        - id: service-c
          label: "Service C"
      edges:
        - from: service-a
          to: service-b
          label: "calls"
        - from: service-b
          to: service-c
          label: "writes"
```
