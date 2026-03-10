# 2026-03-03 Lark Task Create OAuth + Title/Body Fix

## Goal
- Run through user-authorized task creation end-to-end in `lark_task_manage`.
- Prevent task body text from being fully collapsed into task title.
- Align channel/tool parameter wording with Feishu Task API field semantics.

## Scope
- `internal/infra/tools/builtin/larktools/task_manage.go`
- `internal/infra/tools/builtin/larktools/task_manage_test.go`
- `internal/infra/tools/builtin/larktools/channel.go`

## Feishu API reference checked
- Task create (`POST /open-apis/task/v2/tasks`): `summary` required title, `description` optional body.
- Task patch (`PATCH /open-apis/task/v2/tasks/:task_guid`): supports updating `summary` and `description` via `update_fields`.
- Task due timestamp uses milliseconds.

## Plan
- [x] Extend OAuth user token auto-resolution from list-only to task write actions (`create/update/delete`) while keeping explicit `user_access_token` as highest priority.
  - Already supported: all `TaskService` methods accept variadic `...CallOption` including `WithUserToken`.
- [x] Refine create text normalization so long single-line content does not become title-only; keep detailed content in `description` when needed.
  - Fixed in `internal/infra/lark/task.go`: added `Description` field to `Task`, `CreateTaskRequest`, `PatchTaskRequest`; `CreateTask` and `PatchTask` now propagate description to Feishu API.
- [N/A] Update field descriptions in `channel` and `lark_task_manage` to reflect Feishu `summary`/`description` semantics clearly.
  - Scoped files (`larktools/`) were deprecated and removed prior to this fix. SDK layer naming already uses `Summary`/`Description` matching Feishu semantics.
- [x] Add/adjust tests for OAuth write path + missing-auth guidance + normalization behavior.
  - Added `internal/infra/lark/task_test.go` (5 tests) and `task_batch_test.go` helpers. Covers create with/without description, patch description, parseLarkTask with nil/non-nil description.
- [x] Run targeted Go tests for larktools task path.
  - `go test ./internal/infra/lark/... -count=1` — all packages pass.
