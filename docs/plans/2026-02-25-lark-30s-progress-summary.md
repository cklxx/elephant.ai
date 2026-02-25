# 2026-02-25 Lark 30s Progress Summary

## Goal
- 当 Lark 前台任务超过 30 秒仍未返回、且 agent loop 仍在运行时，自动发送一条“最近进展摘要”。
- 摘要优先由 LLM 生成，失败时回退为规则摘要，确保用户可见阶段性反馈。

## Constraints
- 不修改 ReAct 核心循环；仅在 Lark channel 层接入。
- 不影响正常短任务的现有回复行为。
- 配置默认可用，支持 YAML 覆盖。

## Plan
1. `completed`：实现慢任务摘要监听器（30s one-shot，收集事件，判断任务是否仍运行）。
2. `completed`：接入 LLM 摘要与规则回退，并通过 Gateway 发送进展消息。
3. `completed`：扩展 Lark 配置链路（runtime file config → bootstrap → lark gateway config）。
4. `completed`：补充单元测试并跑相关 go test 验证。

## Validation
- `go test ./internal/delivery/channels/lark -run SlowProgressSummaryListener -count=1`
- `go test ./internal/delivery/channels/lark -count=1`
- `go test ./internal/delivery/server/bootstrap -run 'Lark|Config' -count=1`
- `go test ./internal/shared/config -run 'FileConfig|Loader' -count=1`
