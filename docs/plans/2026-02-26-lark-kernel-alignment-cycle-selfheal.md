# Plan: Lark Kernel Alignment + Cycle Self-Heal Reliability

> Created: 2026-02-26
> Status: completed
> Trigger: 用户反馈 `kernel` 长期与 `main` SHA 不对齐，`cycle watch` 长期失败且无法自救。

## Goal
- Supervisor 在 SHA 漂移时同时升级 `main` 与 `kernel`，避免 `kernel` 长期跑旧版本。
- `loop` 在 test worktree 具备可执行 slow gate 的基础依赖（web lint 不再因缺 `eslint` 立即失败）。
- Shell/Go 两套 supervisor 逻辑保持一致，覆盖关键回归测试。

## Scope
- `internal/devops/supervisor/supervisor.go`
- `internal/devops/supervisor/supervisor_test.go`
- `scripts/lark/supervisor.sh`
- `scripts/lib/common/lark_test_worktree.sh`

## Tasks
1. 扩展 Go supervisor 的 SHA drift 升级目标到 `kernel`，并补测试。
2. 扩展 shell supervisor 的 SHA drift 逻辑到 `kernel`，保持行为一致。
3. 在 test worktree 初始化阶段自动复用 main worktree 的 `web/node_modules`。
4. 运行相关测试与脚本回归，确认不引入新回归。

## Execution Log
- 2026-02-26: 已完成现状确认：`kernel` 升级逻辑仅作用于 `main`；当前 slow gate 失败主因是 test worktree 缺 `web/node_modules` 导致 `eslint: command not found`。
- 2026-02-26: 已实现 `kernel` 参与 SHA drift 自动升级（Go supervisor + shell supervisor）。
- 2026-02-26: 已在 `lark_ensure_test_worktree` 增加 `web/node_modules` 自动链接到 main worktree，避免 slow gate 因 `eslint` 缺失直接失败。
- 2026-02-26: 验证完成：`go test ./internal/devops/supervisor/...`、`./dev.sh lint`、`./dev.sh test` 通过；代码审查工具无 P0/P1。
