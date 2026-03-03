# 2026-03-03 Lark Task List OAuth Fix

## Goal
Fix `channel(action=list_tasks)` returning `user access token is empty` when user OAuth token is available in context, and return actionable guidance when OAuth authorization is missing.

## Scope
- `internal/infra/tools/builtin/larktools/task_manage.go`
- `internal/infra/tools/builtin/larktools/task_manage_test.go`

## Plan
- [x] Compare calendar OAuth flow and design task list token resolution behavior.
- [x] Implement task list request option resolver (explicit arg token > OAuth context token).
- [x] Add tests for OAuth token success and OAuth authorization-needed guidance.
- [x] Run targeted tests and validate behavior from session snapshots.
