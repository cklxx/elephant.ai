# 2026-03-11 Lark reply table card

## Goal

Fix Lark reply formatting so markdown tables in reply content do not rely on Feishu markdown table rendering and remain readable in the reply/task delivery path.

## Scope

- Inspect `internal/delivery/channels/lark/markdown_to_post.go` reply formatting behavior.
- Update the reply path used by `task_manager_result.go` and `task_manager_delivery.go` via `smartContent()`.
- Add targeted tests for markdown table replies and nearby non-table behavior.

## Plan

1. Read the current formatter, reply callers, and existing table/card tests.
2. Implement a table-safe reply representation in the interactive card path.
3. Run targeted unit tests, lint if needed for scope, and required code review.
4. Commit the scoped change without pushing.

## Result

- `smartContent()` now routes markdown-table replies to a table-safe interactive card that uses plain-text card blocks instead of card markdown tables.
- Non-table markdown around tables is flattened into readable text blocks so the reply path stays readable without touching the generic markdown renderer.
- Interactive-card fallback now extracts text from both markdown and plain-text div cards before degrading to post/text.

## Validation

- `go test ./internal/delivery/channels/lark -run 'Test(SmartContent|HasTableSyntax|BuildTableSafeCard|ExtractCardText|SplitMarkdownTableCells|BuildPostContent|FlattenPostContentToText|HasMarkdownPatterns|ConvertInlineMarkdown)'`
- `go test ./internal/delivery/channels/lark`
- `python3 skills/code-review/run.py review`
