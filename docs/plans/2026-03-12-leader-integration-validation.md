# Leader Agent Integration Validation

## Goal

为 leader agent 的 4 个协同行为补一组集成测试骨架，先固定场景、断言和测试夹具，待功能合入后去掉 `t.Skip(...)` 再跑 integration suite。

## Scope

- 新建 `internal/runtime/leader/leader_integration_test.go`
- build tag: `integration`
- 复用 `hooks` integration 测试的 runtime/event wait 模式
- 复用 `leader` 单测中的 mock runtime / decision history 断言思路

## Steps

1. 阅读现有 leader/hooks 测试和实现，确认可复用 helper 与事件模型。
2. 在 integration 测试中补 `integrationRuntime`、prompt capture、event wait helper。
3. 为 4 个场景写测试骨架，先用 `t.Skip("waiting for implementation")` 挂起。
4. 做轻量校验，提交并 fast-forward 合回 `main`。

## Notes

- 当前实现尚未稳定暴露 `GetRecentToolCall`、`GetIterationCount`、`GetRecentEvents`，测试侧通过可选 interface 断言保持向前兼容。
- 本轮不运行 `go test -tags integration ./internal/runtime/leader/...`，等功能完成后再去掉 skip 并执行。
