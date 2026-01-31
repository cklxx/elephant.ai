# Plan: json-render chart support (2026-01-31)

## Goal
Render `chart` elements in json-render payloads (line charts) in web UI and SSR.

## Constraints
- Use TDD for logic changes.
- Update both SSR and React renderers.
- Keep CSS and output lightweight; no new dependencies.

## Steps
1. Add SSR chart rendering test coverage (completed).
2. Build shared chart layout helper (completed).
3. Implement chart rendering in SSR + React with styles (completed).
4. Run full lint/tests, update memory timestamp, and refresh plan status (completed).
