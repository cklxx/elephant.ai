# 2026-02-09 Lark Notice Binding + Autofix Alerting

Status: Completed
Owner: Codex
Branch: feat/lark-notice-alerts-20260209

## Background
- 用户希望在 Lark 群中通过 `/notice` 绑定告警群。
- 当 lark main/test/loop 组件异常时，优先执行 codex 自愈流程，并在状态变化时自动给绑定群发通知。

## Goals
1. 新增 `/notice` 命令：绑定、查看、关闭通知群。
2. supervisor 在 `healthy -> degraded/cooldown` 与 `degraded/cooldown -> healthy` 发送状态通知。
3. 通知发送失败不影响 supervisor 主循环。

## Implementation Plan
- [x] 新增 Lark Notice 状态存储（JSON state file）与 Gateway 读写逻辑。
- [x] 在 Lark 命令路由中接入 `/notice`、`/notice status`、`/notice off`。
- [x] 新增 `scripts/lark/notify.sh`，封装 Lark OpenAPI 发消息。
- [x] 在 `scripts/lark/supervisor.sh` 增加状态变化检测与通知调用。
- [x] 补充/更新单元测试与脚本 smoke tests。
- [x] 运行 lint + tests 验证，并记录结果。

## Validation Checklist
- [x] `go test ./internal/delivery/channels/lark/...`
- [x] `./tests/scripts/lark-supervisor-smoke.sh`
- [x] `./scripts/run-golangci-lint.sh run ./...`
- [x] `CGO_ENABLED=0 go test ./...`

## Progress Log
- 2026-02-09: 创建 worktree 分支并复制 `.env`；创建计划文件。
- 2026-02-09: 完成 `/notice` 命令与 JSON 状态存储（gateway + tests）。
- 2026-02-09: 完成 `notify.sh` 与 supervisor 状态变化通知（degraded/recovered）。
- 2026-02-09: 完成 `lark-supervisor-smoke.sh` 扩展验证通知触发路径。
- 2026-02-09: 验证通过 `go test ./internal/delivery/channels/lark/...`、`./tests/scripts/lark-supervisor-smoke.sh`、`./scripts/run-golangci-lint.sh run ./...`、`CGO_ENABLED=0 go test ./...`。
