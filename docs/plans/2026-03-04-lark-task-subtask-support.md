# 2026-03-04 Lark Task Subtask Support

## Background
- User requested subtask capability for Lark task operations.
- Current `lark_task_manage` only supports `list/create/update/delete` for top-level tasks.
- Unified `channel` task actions also lack subtask routes.

## Goals
- Add subtask creation and listing support in `lark_task_manage`.
- Expose subtask actions through `channel`.
- Keep auth/approval behavior aligned with existing task actions.

## Changes
1. Extend `lark_task_manage` action schema:
   - `create_subtask`
   - `list_subtasks`
   - add `parent_task_id` parameter.
2. Implement subtask handlers using Feishu Task v2 subtask endpoints.
3. Extend `channel` action enum/dispatch/safety mapping for subtask actions.
4. Add/adjust unit tests for both tool layers.

## Verification
- `go test ./internal/infra/tools/builtin/larktools -count=1`
- Run broader test/lint command before delivery.

## Status
- [x] Code changes
- [x] Tests (focused: `TestTaskManage_` + `TestChannel_`)
- [x] Commit
