# 2026-03-03 Non-web Simplify Rescan — Wave J

## Scope
Unify low-risk duplicated truncate helpers in non-`web/` backend code.

## Goals
1. Extract shared helper for the repeated pattern: `TrimSpace + rune truncate + "..."`.
2. Keep existing behavior for edge cases (`limit<=0`) at each call site.
3. Validate touched packages with tests/lint, then commit.

## Plan
1. Add shared helper in `internal/shared/utils`.
2. Repoint `truncateMemorySection` and `truncateSnippet` to shared helper.
3. Keep `truncateText` wrapper behavior in `memory_capture` while delegating common logic.
4. Add focused unit tests for the new helper.
5. Run targeted tests/lint/code-review and commit.

## Progress
- [x] Shared helper added.
- [x] Existing call sites delegated.
- [x] Tests/lint/code-review passed.
- [ ] Wave J commit created.
