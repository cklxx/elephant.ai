# 2026-03-04 Codex System Instructions Dedup

## Goal
避免同一轮请求中重复的 core system prompt 被多次拼接进 codex `instructions`，降低误判与 token 浪费。

## Plan
- [completed] 在 `internal/infra/llm/openai_responses_input.go` 对 codex `system/developer` 指令做去重（保序）。
- [completed] 在 `internal/infra/llm/openai_responses_client_test.go` 增加回归测试，覆盖重复 system/developer 输入。
- [completed] 运行目标测试并确认通过。
- [completed] 记录复盘并提交。

## Notes
- 去重仅作用于 codex `instructions` 构建路径，不改变非 codex 模型的消息行为。
- 目标验证：
  - `go test ./internal/infra/llm -run 'TestOpenAIResponsesClient(SetsInstructionsForCodex|DeduplicatesInstructionsForCodex|SynthesizesInputWhenCodexInputIsEmpty)' -count=1` ✅
