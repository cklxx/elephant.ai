# Leader Agent Integration Validation

## Goal

补齐并跑通 `internal/runtime/leader/leader_integration_test.go`，验证 leader 的 rich stall prompt、handoff diagnostics、child sibling progress、以及 stall recovery + escalation 协同路径。

## Scope

- 新建/恢复 `internal/runtime/leader/leader_integration_test.go`
- build tag: `integration`
- 使用真实 `hooks.NewInProcessBus()` 和 integration 风格 runtime mock
- 跑 `go test -tags integration ./internal/runtime/leader/... -v`

## Steps

1. 对齐当前主线实现的接口和行为。
2. 实现 4 个 integration 场景。
3. 运行 integration 测试并修正失败。
4. 提交并 fast-forward 合回 `main`。

## Notes

- `RuntimeReader` 当前强制要求 `GetRecentEvents(sessionID, n int) []string`。
- `ToolCallReader` 是可选扩展，通过 type assertion 获取。
- heartbeat 会把旧 decision history 标记为 `recovered` 后从 store 删除；生命周期测试按这个真实语义断言。
