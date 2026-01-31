# Plan: Tool resurvey + overlap matrix

**Created**: 2026-02-01
**Status**: Completed (2026-02-01)
**Author**: cklxx + Codex

## Goals
- Resurvey current builtin tool surface from registry + definitions.
- Produce layered classification and overlap table.

## Plan
1. Extract registered tool names from `internal/toolregistry/registry.go`.
2. Cross-check tool definitions in `internal/tools/builtin/**` (non-test).
3. Group by layer/category and identify functional overlap pairs.
4. Deliver structured classification + overlap table.

## Progress Updates
- 2026-02-01: Plan created.
- 2026-02-01: Collected registry tool list and produced classification + overlap matrix in docs/reference/TOOLS.md.
