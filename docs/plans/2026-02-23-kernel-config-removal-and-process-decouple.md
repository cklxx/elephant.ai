# 2026-02-23 Kernel 配置移除与进程解耦

## Goal
- 删除 runtime `proactive.kernel` 配置依赖。
- kernel 仅由提示词与代码内建规则控制。
- kernel 作为独立守护进程运行，与 lark agent 进程分离。
- 在 supervisor 下自动保活，避免非显式停机。

## Constraints & Best Practices
- 保持 `agent/ports` 与 memory/RAG 依赖解耦（架构边界约束）。
- 配置示例使用 YAML，删除无效配置入口，避免“配置存在但不生效”。
- Go 代码遵循 Effective Go / Go Code Review Comments：
  - 明确错误边界，失败尽早暴露。
  - 通过小而可测的函数收敛复杂度。
  - 新增逻辑同步补充测试（TDD）。

## Implementation Plan
1. 移除 `internal/shared/config` 中 kernel 配置结构与 merge 流程。
2. 在 `internal/app/agent/kernel` 引入内建 runtime 配置与默认 agent prompt。
3. 调整 DI：`buildKernelEngine` 只使用内建 kernel 配置。
4. 调整 bootstrap：
   - `RunLark` 不再启动 kernel stage。
   - 新增 `RunKernelDaemon` 独立启动 kernel。
5. 调整 `cmd/alex-server` 命令路由，增加 `kernel-daemon` 子命令。
6. 新增 `scripts/lark/kernel.sh` 并注册到 `dev lark` supervisor 组件。
7. 更新/新增测试并执行 lint + test + review。

## Progress
- 2026-02-23 08:22 已完成现状排查：kernel 仍依赖 `runtime.proactive.kernel`，且在 `RunLark` 进程内启动。
- 2026-02-23 08:22 已建立本次工作分支 worktree：`cklxx/kernel-decouple-20260223`。
- 2026-02-23 08:34 已完成代码改造：
  - 移除 `internal/shared/config` 中 `proactive.kernel` schema/merge/env-expand 逻辑。
  - kernel runtime 改为 `internal/app/agent/kernel/config.go` 内建 `DefaultRuntimeSettings()`。
  - `RunLark` 移除 kernel stage，新增 `RunKernelDaemon` 独立启动路径。
  - `cmd/alex-server` 增加 `kernel-daemon` 子命令。
  - `scripts/lark/kernel.sh` 新增独立守护脚本，`cmd/alex/dev_lark.go` 已注册 `kernel` 组件。
- 2026-02-23 08:39 已通过完整本地 CI：`./scripts/pre-push.sh`（含 `go test -race ./...`、lint、架构检查）。
- 2026-02-23 08:46 增补防误启动修复：`alex-server` 对未知子命令改为显式报错（不再回退 lark 默认模式），并新增对应单测。
- 2026-02-23 08:50 已再次通过完整本地 CI：`./scripts/pre-push.sh` 全绿。
