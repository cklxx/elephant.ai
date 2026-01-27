# Json-Render Only + Components (2026-01-27)

## Goal
Enforce json-render as the sole UI payload format and extend renderers/templates for tables, kanban boards, and diagram edges.

## Plan
1. Remove A2UI fallback from prompt triggers and UI payload detection/preview path.
2. Add json-render components (table, kanban, diagram edges) in client + SSR renderers.
3. Extend json-render templates skill with table/kanban/diagram examples.
4. Add tests for new render paths.
5. Run full lint + tests.

## Progress
- [x] Remove A2UI fallback (prompt + preview)
- [x] Add json-render components (client + SSR)
- [x] Update templates skill
- [x] Tests
- [ ] Lint/test (Go typecheck fails in unrelated files)
