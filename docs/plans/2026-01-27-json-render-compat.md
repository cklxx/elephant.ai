# Json-Render Compatibility (2026-01-27)

## Goal
Render json-render protocol payloads (including simple message batches) in the web UI, both SSR preview and interactive, and provide templates for flowchart/form/dashboard/cards/gallery.

## Plan
1. Add json-render payload parser + renderer (client + SSR) and detect payload kind in attachment preview.
2. Add json-render templates skill and update trigger rules to prefer it.
3. Add tests for parser + SSR renderer.
4. Run full lint + tests.

## Progress
- [x] Json-render parser + renderers
- [x] Skills + trigger rules
- [x] Tests
- [x] Lint/test
