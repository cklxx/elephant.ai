# Json-Render UI Cleanup (2026-01-27)

## Goal
Remove raw attachment filename placeholders from the UI and simplify the preview to a single view with better height behavior.

## Plan
1. Strip UI attachment placeholders from rendered markdown content.
2. Remove Server Preview/Interactive tabs and render json-render directly with auto height.
3. Update tests and run lint/test.

## Progress
- [x] Strip UI attachment placeholders
- [x] Simplify preview UI
- [x] Tests
- [x] Lint/test
