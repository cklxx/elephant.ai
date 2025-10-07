# MANUS UI Research and Design

**Date:** 2025-10-02
**Purpose:** Capture the research insights and UX design principles for the MANUS agent interface.

---

## 1. Research Summary

The MANUS workflow emphasizes clarity between agent reasoning and actionable tool output. Competitive analysis across Cursor, Windsurf, and GitHub Copilot revealed a shared preference for progressively disclosed context panes and transcript timelines that emphasize causality between prompts, tool invocations, and resulting changes.

Key takeaways:

1. Maintain a persistent reasoning timeline that interleaves thinking steps with executed tools.
2. Promote actionable context (diffs, diagnostics, previews) in dedicated panels adjacent to the transcript.
3. Support iterative refinement by allowing users to rewind to any reasoning step.

---

## 2. Interaction Model

The recommended interaction pattern centers around an event stream describing each action the agent performs. Individual events are rendered within the transcript using a normalized payload:

{% raw %}
```json
{
  "type": "tool-call",
  "timestamp": "2025-10-02T12:04:33Z",
  "payload": {
    "tool": {
      "name": "search_repository",
      "parameters": {
        "query": "react_engine configuration" 
      }
    },
    "result": {
      "summary": "Located configuration defaults in internal/agent/domain/react_engine.go"
    }
  }
}
```
{% endraw %}

This structure ensures the UI can present rich tool metadata without breaking when optional fields are omitted.

---

## 3. Transcript Rendering Strategy

Each event is processed through a renderer that decides whether to surface a reasoning bubble, a tool call preview, or a result summary. The renderer operates on an event descriptor emitted by the agent runtime:

{% raw %}
```js
{
  name: event.tool_name,
  parameters: event.tool_parameters || {}
}
```
{% endraw %}

Wrapping the descriptor in a raw Liquid block prevents Jekyll from attempting to interpret the double braces when generating the documentation site.

---

## 4. Visual Design Notes

- **Color Coding:** Reasoning steps adopt a neutral palette, while tool invocations use accent colors to reinforce their actionable nature.
- **Icons:** Display a distinct glyph per tool family (search, apply, test) to aid quick scanning.
- **Spacing:** Provide generous vertical rhythm between grouped steps to avoid overwhelming the reader during long sessions.

---

## 5. Next Steps

1. Build a prototype transcript view implementing the renderer contract above.
2. Conduct usability testing with internal users solving moderately complex refactors.
3. Iterate on the layout to ensure accessibility compliance (WCAG 2.1 AA) before external release.
