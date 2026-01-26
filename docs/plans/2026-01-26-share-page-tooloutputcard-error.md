# Plan: Fix share page ToolOutputCard crash (2026-01-26)

## Goal
- Eliminate the share page frontend crash (`toolName.toLowerCase` on undefined) by normalizing shared session events before rendering.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Trace how shared session events differ from live SSE events and identify where `tool_name` is lost.
2. Add a reusable event normalization helper that merges payload fields into the top-level event shape.
3. Apply normalization to share page events before rendering, and reuse it in the SSE pipeline for consistency.
4. Add unit tests for normalization edge cases (payload-only tool fields, invalid events).
5. Run full lint + tests.
6. Commit changes.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Added shared event normalization helper and tests; wired normalization into SSE pipeline + share page ingestion.
- 2026-01-26: Added shared-session final-result dedupe; updated share page to use normalized event list; expanded normalization tests.
- 2026-01-26: Ran `./dev.sh lint` (failed: unused `styleYellow` in `cmd/alex/tui_styles.go`) and `./dev.sh test` (failed: race in `internal/mcp`); logged in error experience entries.
- 2026-01-26: Ran `npm --prefix web run lint` and `npm --prefix web test` (both pass).
