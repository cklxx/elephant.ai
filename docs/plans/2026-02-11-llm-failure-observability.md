# 2026-02-11 — LLM 失败日志可观测性补强

## Goal
- 补齐 LLM 调用失败后的关键日志，确保在非 debug 级别也能定位失败原因。
- 排查并补齐其他链路的漏日志点，重点覆盖 Lark 聊天报错对应路径。

## Scope
- `internal/infra/llm/openai_client.go`
- `internal/infra/llm/anthropic_client.go`
- `internal/infra/llm/openai_responses_complete.go`
- `internal/infra/llm/openai_responses_stream.go`
- `internal/infra/llm/retry_client.go`（仅在需要时补齐）

## Non-Goals
- 不改业务重试策略与模型路由策略。
- 不变更外部接口契约。

## Checklist
- [x] 建立问题基线与现有日志点盘点。
- [x] 为 HTTP 非 2xx/请求失败路径增加结构化 `Warn/Error` 日志。
- [x] 统一失败日志字段（provider/model/status/request_id/log_id/retry_after/error_type）。
- [x] 检查其他链路漏点并补齐（如重试层最终失败汇总）。
- [x] 增加/更新测试覆盖失败日志行为（最小必要）。
- [x] 运行 `./dev.sh lint`（失败：web 既有 lint 问题，与本次改动无关）。
- [x] 运行 `./dev.sh test`（失败：`internal/infra/analytics` 依赖缺失 `docs/analytics/tracking-plan.yaml`，与本次改动无关）。
- [ ] 执行代码审查清单并修复问题。
- [ ] 分批提交并合并回 `main`（ff 优先）。

## Progress
- 2026-02-11 现状确认：失败时主要错误信息仅在 debug 输出，非 debug 环境可观测性不足。
- 2026-02-11 新增失败日志：OpenAI/Anthropic/Responses（含 stream）在 transport、非 2xx、解析失败路径统一输出 `Warn` 级结构化日志。
- 2026-02-11 补齐重试层摘要：`retry_client` 成功/失败日志补充 `provider/request_id/intent`，便于链路关联。
- 2026-02-11 追加 pre-analysis 意图标记：`task_preanalysis`，用于区分辅助请求来源。
- 2026-02-11 根因确认：`gpt-4o-mini + stream=true + temperature=0.2` 主要来自预分析小模型调用；`stream=false + temperature=0.2` 来自 memory capture。
