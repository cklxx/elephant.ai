# Lark Task Title/Content Mapping Fix (2026-03-03)

## Goal
- Fix the bug where task body/content is overwritten into title when creating/updating tasks after Lark authorization.
- Verify end-to-end task list retrieval works after OAuth.
- Align implementation with Feishu/Lark official API field semantics.

## Scope
- `internal/delivery/channels/lark/attachment_handler.go`
- `internal/delivery/channels/lark/task_manager.go`
- `internal/delivery/channels/lark/gateway_test.go`
- `internal/delivery/channels/lark/attachment_handler_test.go`
- Any minimal adjacent file required by type/test fixes.

## Steps
- [x] Inspect current task compose/send flow and identify title/content write path.
- [x] Cross-check with official Feishu docs for task title/content fields and payload shape.
- [x] Implement minimal fix preserving current behavior for attachments/media.
- [x] Add/update tests to lock title/content contract.
- [x] Run targeted tests + lint/typecheck gate for changed packages.

## Validation
- `go test ./internal/delivery/channels/lark -run 'Task|Attachment|Gateway'`
- `go test ./internal/delivery/channels/lark`
- `go test ./...` (if required by changed shared code)

## Progress
- 2026-03-03: plan created; investigation started.
- 2026-03-03: implemented attachment partition + content-dedup logic and updated summary behavior to include only non-uploadable text-only attachments.
- 2026-03-03: updated related tests and verified with package tests and full repo checks.
