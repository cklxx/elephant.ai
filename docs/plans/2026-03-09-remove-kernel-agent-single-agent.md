# 2026-03-09 删除独立 Kernel Agent，收敛到单一主 Agent

## Goal
- 删除独立 `kernel` agent runtime、命令入口和 bootstrap/DI wiring。
- 保留定时触发能力，但统一由主 `AgentCoordinator` 执行。
- 对定时触发任务启用无人值守上下文，保证自动执行语义仍成立。

## Constraints
- 只修改与 kernel runtime 收敛直接相关的文件。
- 不保留兼容 shim；删除无效代码与入口。
- 保持 `internal/**` 分层边界清晰，避免遗留空壳抽象。

## Implementation Plan
1. 移除 `internal/app/agent/kernel`、`internal/domain/kernel`、`internal/infra/kernel` 的 runtime 依赖入口。
2. 删除 `cmd/alex-server` 的 `kernel-daemon` / `kernel-once` 路由和对应 bootstrap。
3. 删除 DI 中的 `KernelEngine` 与 kernel-specific prompt/context 注入。
4. 让 `scheduler` / `timer` 在触发主 agent 时显式进入 unattended context。
5. 清理本地 Lark supervisor / component 中的 kernel 组件管理代码。
6. 运行定向测试、lint、code review，并提交。

## Progress
- 2026-03-09 目前已确认：`scheduler` 与 `timer` 早已直接调用 `AgentCoordinator.ExecuteTask`，独立 `kernel` 只是另一套循环/状态/调度外壳。
