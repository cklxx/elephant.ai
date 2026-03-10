# 2026-03-10 HTTP Server Handler Audit

## Goal

- 审计 `internal/delivery/server/http/`
- 找出可拆分的过长 handler（超过 80 行）
- 提取重复的 request parsing / response writing 模式
- 完成验证、review、commit，并 fast-forward merge 回 `main`

## Plan

1. 统计 handler 长度并审阅相邻调用链。
2. 识别重复请求解析和响应输出模式。
3. 做最小必要重构并补充/更新测试。
4. 跑相关测试、lint、review，提交并合并。

## Progress

- [x] 创建 worktree
- [x] 审计 handler 与重复模式
- [x] 完成重构
- [x] 回归验证
- [x] review / commit / merge

## Notes

- 初始审计中超过 80 行的 handler 仅有：
  - `SSEHandler.HandleSSEStream`
  - `SSEHandler.HandleTaskSSEStream`
- 两个超长 handler 已拆为请求解析、SSE 连接准备、connected 首包写出、历史回放/排空、主循环等私有 helper。
- 提取了共享 `decodeJSONRequest(...)`，替换 `apps/config/context/onboarding/lark inject` 中重复的 JSON 请求解析逻辑。
- `LarkInjectHandler` 的 JSON 响应写回已统一复用 `writeJSON(...)`。
- 验证已通过：
  - `go test ./internal/delivery/server/http/...`
  - `./scripts/run-golangci-lint.sh run ./internal/delivery/server/http/...`
