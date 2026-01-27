# Json-Render Protocol Compatibility (2026-01-27)

## Context
- Goal: support vercel-labs json-render payloads (content/page/container/grid/tag/etc.) in the Next.js renderer + SSR.
- Engineering practices reviewed (`docs/guides/engineering-practices.md`).

## Plan
1. Extend json-render parsing to unwrap common wrappers (content/data/page/view) and preserve props.
2. Add renderer + SSR support for container/grid/tag and richer text/image/form props.
3. Add unit tests for new payload shapes.
4. Run full lint + tests.

## Progress
- 2026-01-27: Plan created.
- 2026-01-27: Added wrapper parsing + prop preservation; extended renderer/SSR for container/grid/tag and richer text/image/form props; added tests.
- 2026-01-27: Widened workflow tool result typing to allow object payloads used by json-render emitters.
- 2026-01-27: Normalized tool result handling across web UI consumers (summaries, attachments, tool outputs).
- 2026-01-27: Allow a2ui_emit content to accept JSON objects (serialize to JSON) and added coverage.
