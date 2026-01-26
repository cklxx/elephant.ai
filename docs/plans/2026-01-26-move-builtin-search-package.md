# Plan: Move builtin search tools into package (2026-01-26)

## Goal
Move builtin search tool implementations into `internal/tools/builtin/search` with package rename and import fixes, without touching registry wiring or shared helpers.

## Steps
1. Inventory current builtin search files and references; confirm target package paths.
2. Move files into `internal/tools/builtin/search`, update `package` declarations and imports.
3. Run gofmt if needed; run full lint + tests.
4. Summarize changes and issues.

## Progress
- 2026-01-26: Engineering practices reviewed; plan created.
- 2026-01-26: Located builtin search files and references; moved them into `internal/tools/builtin/search`.
- 2026-01-26: Updated package declarations to `search`, added builtin re-exports to avoid registry wiring changes.
- 2026-01-26: `make fmt` failed with existing typecheck errors (missing helper symbols, missing embedded asset).
- 2026-01-26: `make test` failed with existing build errors (same missing helpers/assets across builtin/execution/sandbox/toolregistry).
- 2026-01-26: Logged lint/test failures in error-experience entries for follow-up.
