# 2026-02-26 Lark Message Flow System Optimization (History Attachments + Thinking)

## Goal
系统性优化消息交互流程，解决两类问题：
1) 历史消息中的旧附件/文件被重复发送到模型上下文
2) 核验并明确 thinking 字段端到端支持状态（LLM -> Agent -> Lark）

## Findings
- 根因：`internal/infra/llm/message_content.go` 的附件 embed 判定过宽，导致历史 user 消息附件也会被重复注入请求。
- 影响面：`openai_client`、`anthropic_client`、`openai_responses_input` 三条消息转换链路。
- `thinking` 支持现状：
  - 请求侧：`internal/domain/agent/react/solve.go` 已默认 `Thinking.Enabled=true`。
  - OpenAI/Codex 侧：`internal/infra/llm/thinking.go` 已在 codex endpoint 下发 reasoning 配置。
  - Lark 侧：`internal/delivery/channels/lark/task_manager.go` 已在回复中附加 thinking（空答兜底 + 非空答附加）。

## Industry Best Practices (research)
- 对多轮上下文使用会话链路能力（如 `previous_response_id`）而非每轮手动重发完整历史，降低重复负载。
- 文件输入优先引用式（`file_id`/`file_url`）而非重复 inline/base64，减少冗余 payload。
- 仅保留“当前轮必要附件”进入模型上下文，历史附件保留引用/摘要语义即可。
- reasoning 字段优先展示“summary/可控输出”，避免直接暴露原始思维 token。

## Plan
- [x] 调研业界在“多轮历史 + 文件附件 + reasoning/thinking 可见性”上的最佳实践
- [x] 设计并落地最小侵入修复：只对“当前应发送的用户附件”做 embed，历史附件降级为引用
- [x] 补充单元测试覆盖历史附件去重和 thinking 透传关键路径
- [x] 运行相关测试，确认无回归
- [x] 提交

## Validation
- `go test ./internal/infra/llm -count=1` ✅
- `go test ./internal/delivery/channels/lark -run "ThinkingFallback|BuildReply" -count=1` ✅
- `./scripts/pre-push.sh` ✅（go mod tidy / vet / build / go test -race / golangci-lint / architecture / web lint+build 全通过）
