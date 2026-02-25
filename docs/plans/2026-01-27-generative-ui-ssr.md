# Generative UI SSR Plan (2026-01-27)

## Goal
Support server-side rendering (SSR) for generative UI payloads (A2UI) in Next.js so we can emit a static HTML preview while keeping client-side rendering intact.

## Context
- A2UI payloads are already emitted via `a2ui_emit` and rendered in the web UI with a client-only renderer.
- There is no server-side renderer or preview fallback today.

## Plan
1. Research MCP Apps/generative UI UI-resource patterns; capture SSR-relevant constraints and map to A2UI.
2. Implement a Next.js server-side A2UI → HTML renderer (static, safe, minimal styling).
3. Add a server route to render A2UI payloads into HTML previews.
4. Expose SSR preview in the web UI (A2UI attachment preview tabs).
5. Simplify `a2ui_emit` tool parameters surfaced to the LLM.
6. Add tests (SSR renderer + preview flow).
7. Run full lint + tests; update docs.

## Progress
- [x] Research + summary doc
- [x] Next SSR renderer implementation
- [x] Server preview route
- [x] Web SSR preview UI
- [x] Tool definition simplification
- [x] Tests
- [x] Lint/test + docs
