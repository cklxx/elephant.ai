# 2026-03-05 Tool Prune and CLI Orchestration

## Goal
按用户要求完成三项变更：
1) 删除内置 `plan` 工具；
2) 将 `run_tasks` / `reply_agent` 从工具面迁移为 CLI 抽象；
3) 在 skills 中补充新的 CLI 使用方式。

## Scope
- `internal/app/toolregistry/*`
- `internal/app/di/*`
- `internal/app/agent/coordinator/*`（如需暴露 CLI 编排能力）
- `cmd/alex/*`（新增/接入 CLI 子命令）
- `skills/*`（编排相关 skill 说明）
- 必要测试与文档同步

## Plan
- [x] 下线 `plan` 工具注册并修正工具清单测试。
- [x] 停止在容器启动时注册 `run_tasks` / `reply_agent` 工具。
- [x] 新增 `alex team` CLI（承接 run/reply 两类编排能力）。
- [x] 在 skills 文档中补充 `alex team` 的使用示例与约束。
- [x] 运行相关测试、代码审查并提交（`alex dev lint` 受本机缺少 `eslint` 限制；Go 侧测试与聚焦回归通过）。
