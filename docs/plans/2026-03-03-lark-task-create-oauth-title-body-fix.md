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
- [ ] Extend OAuth user token auto-resolution from list-only to task write actions (`create/update/delete`) while keeping explicit `user_access_token` as highest priority.
- [ ] Refine create text normalization so long single-line content does not become title-only; keep detailed content in `description` when needed.
- [ ] Update field descriptions in `channel` and `lark_task_manage` to reflect Feishu `summary`/`description` semantics clearly.
- [ ] Add/adjust tests for OAuth write path + missing-auth guidance + normalization behavior.
- [ ] Run targeted Go tests for larktools task path.
