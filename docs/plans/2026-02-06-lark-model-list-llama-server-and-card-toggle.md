# Plan: Lark 支持 llama_server 模型列出与卡片可选配置确认

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- 在 Lark `/model list` 中支持 `llama_server`：本地服务可用时自动列出模型。
- 确认并固化 “Lark 卡片可选” 能力（配置开关路径清晰可用）。

## Non-goals
- 不改动主执行链路的 LLM provider 选择逻辑。
- 不删除 antigravity 底层 provider 实现（仅 model list 展示层面的行为）。

## Approach
1. 对齐 CLI 已有做法，在 Lark model list 的 catalog service 注入 `WithLlamaServerTargetResolver`。
2. 在 Lark 渠道层补充 llama server 目标解析（`LLAMA_SERVER_BASE_URL` / `LLAMA_SERVER_HOST` / 默认本地）。
3. 新增 Lark 单测，覆盖“llama_server 在线时 /model list 出现 provider 与模型”。
4. 复核 Lark 卡片配置开关文档，必要时补充说明。
5. 运行 lint + tests 回归。

## Milestones
- [x] 代码实现
- [x] 测试补充（TDD）
- [x] 文档确认/更新
- [x] lint + tests 回归

## Progress Log
- 2026-02-06 16: 在 `internal/delivery/channels/lark/model_command.go` 定位到缺失 `WithLlamaServerTargetResolver` 注入，是 Lark 未显示 `llama_server` 的直接原因。
- 2026-02-06 16: 已确认 Lark 卡片能力已有开关：`cards_enabled` / `cards_plan_review` / `cards_results` / `cards_errors`（见 `docs/reference/CONFIG.md`）。
- 2026-02-06 16: 在 Lark gateway 增加可注入的 CLI 凭据加载器与 llama target resolver，并在 `/model list` catalog service 中注入 `WithLlamaServerTargetResolver`。
- 2026-02-06 16: 新增 `internal/delivery/channels/lark/model_command_test.go`，覆盖 llama_server 在线列出与 resolver 解析行为。
- 2026-02-06 16: 完成 `./dev.sh lint` 与 `./dev.sh test` 全量回归通过。
