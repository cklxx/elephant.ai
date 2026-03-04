# 2026-03-04 Lark Task Tenant Token & Visibility

## Goal
- 调研并落地飞书机器人“创建任务”链路，补齐自动获取 `tenant_access_token` 的逻辑。
- 统一任务/日历/文档的 token 策略：优先用户 OAuth，缺失时安全回退到 tenant 身份。
- 保证由应用身份创建的任务对当前聊天用户可见（符合 task-v2 FAQ 的鉴权模型）。

## Context (official docs)
- Task v2 FAQ 明确：`tenant_access_token` 无租户管理员特权，仅是应用身份；应用需成为任务/清单成员才能访问数据。
- Task v2 概述明确：Task API 支持 `tenant_access_token` 与 `user_access_token`；正式应用需自行处理 token 续期。
- 创建任务 API 明确：请求头支持 tenant/user token；并且成员 `members` 支持 `user`/`app`。

## Scope
- `internal/infra/tools/builtin/larktools/lark_oauth.go`
- `internal/infra/tools/builtin/larktools/task_manage.go`
- `internal/infra/tools/builtin/larktools/task_manage_test.go`
- `docs/plans/2026-03-04-lark-task-tenant-token-and-visibility.md`

## Plan
- [in_progress] 抽取任务 token 解析：显式 `user_access_token` > OAuth 用户 token > tenant fallback（不再因缺失 OAuth 阻断）。
- [pending] 任务创建在 tenant 模式下自动追加当前消息用户为 follower（若未显式传入），提升“用户可见性”。
- [pending] 补充测试：tenant 自动 token、OAuth 缺失回退、tenant 模式成员自动注入。
- [pending] 运行目标测试并复核变更。

## Validation
- `go test ./internal/infra/tools/builtin/larktools -run "TaskManage|Calendar"`
