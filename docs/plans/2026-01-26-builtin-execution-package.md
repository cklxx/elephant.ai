# Plan: Move builtin execution tools to subpackage (2026-01-26)

## Goal
- Move builtin execution-related tool implementations into `internal/tools/builtin/execution` with updated package/imports and preserved build tags.

## Constraints
- Do not touch registry wiring or shared helpers.
- Keep `local_exec_flag_*` build tags intact.
- No commits.

## Plan
1. Inventory current builtin execution files and imports.
2. Move files into `internal/tools/builtin/execution/` and update package names/imports.
3. Adjust references from other packages if needed (shared/pathutil only).
4. Run full lint + tests.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Moved execution files into `internal/tools/builtin/execution`, updated package names/imports, and added builtin re-exports to keep registry wiring unchanged.
