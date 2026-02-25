# Tool Call Timeline UI Best Practices Research

## Overview

This document captures a survey of established patterns from industry tools that present streaming automation activity—specifically GitHub Actions, Vercel Deployments, LangSmith traces, Slack Workflow executions, and Linear issue timelines. The findings guide the console redesign so we can deliver a predictable, legible experience while matching user expectations from adjacent products.

## Key References

| Product | Why it matters |
| --- | --- |
| GitHub Actions live logs | Sets expectations for sticky autoscroll, manual pause, badge semantics, and timestamp density.
| Vercel deployment inspector | Demonstrates collapsible sections for stdout/stderr, contextual metadata chips, and copy affordances.
| LangSmith trace viewer | Highlights pairing of tool starts/completions, nested timelines, and JSON pretty-print defaults.
| Slack Workflow run history | Inspires progressive disclosure of parameters, localized status pills, and friendly empty states.
| Linear issue history | Reinforces human-readable timestamps, hover affordances, and subtle motion to communicate progress.

## Consolidated Best Practices

1. **Keep the latest activity in view by default but let the user opt out.** Every reference implementation maintains autoscroll until the visitor scrolls up, then surfaces an explicit “jump to latest” control.
2. **Display paired events as a single narrative unit.** Start/complete or request/response pairs should be grouped with consistent iconography and color accents to reduce cognitive load.
3. **Expose raw arguments and results with progressive disclosure.** Collapsible panels or expandable cards prevent overwhelming the main feed while keeping debugging tools one click away.
4. **Offer quick copy and export actions on structured payloads.** Engineers expect to copy JSON blobs, command invocations, or error traces without selecting text manually.
5. **Respect accessibility, especially for live regions.** Announce new items via `aria-live`, preserve keyboard focus order, and gate high-motion flourishes behind `prefers-reduced-motion` checks.
6. **Show canonical timestamps and durations together.** Present absolute timestamps alongside relative duration badges so users can reconstruct execution order quickly.
7. **Surface metadata chips for traceability.** Call identifiers, tool names, and execution contexts belong in lightweight badges to support cross-referencing with backend logs.
8. **Handle long outputs gracefully.** Syntax highlighting should be optional, and scrollable panels need predictable height caps with visible scrollbars.
9. **Provide empty and error states that build confidence.** Friendly illustrations and localized copy inform users that the system is ready even when no events have streamed yet.
10. **Support auditability.** Preserve a lightweight timeline sidebar or breadcrumb trail summarizing the macro steps of the research plan.

## Recommendations for the Console

- Adopt clipboard controls for tool arguments/results and ensure the JSON formatter preserves key order.
- Add reduced-motion fallbacks so live pulses downgrade to static indicators for motion-sensitive users.
- Promote the research timeline as the left rail on large screens, synchronized with the virtualized stream.
- Keep the autoscroll heuristic we shipped, but instrument analytics to monitor manual scroll frequency in production.
- Expand i18n coverage to every new label so we stay localization-ready.

These practices bridge our long-term plan with concrete UI affordances proven across developer tooling ecosystems.
