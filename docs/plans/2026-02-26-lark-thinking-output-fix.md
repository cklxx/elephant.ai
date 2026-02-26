# 2026-02-26 Lark Thinking Output Fix

## Goal
恢复 Lark 通道中的 thinking 可见性：
1) 上游 LLM 请求在 codex 路径启用 reasoning/thinking
2) 下游 Lark 回复每次可读取并展示 thinking（非仅空答兜底）

## Findings
- `internal/domain/agent/react/solve.go` 已设置 `Thinking.Enabled=true`
- `logs/requests/llm.jsonl` 最近请求没有 `reasoning` 字段
- `internal/infra/llm/thinking.go` 中 `shouldSendOpenAIReasoning` 未覆盖 codex endpoint
- `internal/delivery/channels/lark/task_manager.go` 仅在 `reply==""` 时使用 thinking fallback

## Plan
- [x] 定位根因与受影响代码
- [x] 修复 codex reasoning 请求下发
- [x] 修复 Lark 回复拼接 thinking 逻辑
- [x] 补充/更新单元测试
- [x] 运行相关测试并校验
- [x] 提交并 push

## Validation
- `go test ./internal/infra/llm -run "Thinking|Codex|Responses"`
- `go test ./internal/delivery/channels/lark -run "ThinkingFallback|BuildReply"`
- `go test ./...`（失败于 `internal/infra/integration: TestPathInjectionE2E_ReadsOutsideWorkspace`，看起来是现有不稳定测试，与本次改动路径无直接关联）
