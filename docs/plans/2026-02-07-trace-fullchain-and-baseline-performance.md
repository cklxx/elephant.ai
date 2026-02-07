# 2026-02-07 全链路 Trace + 基础性能优化

## 目标
- 建立 HTTP → Task → ReAct Iteration → LLM → Tool → SSE 的可关联 trace 观测能力，用于性能分析。
- 完成两项低风险基础性能优化，优先减少高频路径 CPU/分配开销。

## Active Memory（本次）
- `agent/ports` 禁止引入 memory/RAG 依赖，避免架构环路。
- 非平凡任务必须在 `docs/plans/` 有计划并持续更新。
- 逻辑修改遵循 TDD；交付前跑完整 lint + tests。
- 追踪链路优先保证 run/session 维度可关联，即使跨 HTTP 请求也可通过属性串联。
- 历史经验：高频事件路径要避免无界增长，优先做上限、去重、批处理。
- 历史经验：subagent/并发链路需要稳态限流与轻量观测，避免高开销埋点。
- 当前仓库已有 observability 基础，但 LLM/Tool/ReAct 粒度 span 接入不完整。
- 当前 SSE 附件去重存在重哈希开销，可做轻量签名替代。

## 范围
- 后端 Go：`internal/domain/agent/react/`、`internal/delivery/server/http/` 相关性能热点。
- 文档：方案与验证记录。

## 进度
- [x] 建立独立 worktree 分支并复制 `.env`。
- [x] 读取工程实践/记忆文档并提炼 active memory。
- [x] 调研现状链路与性能热点（含子代理并行调研）。
- [x] 实现 ReAct/LLM/Tool trace 埋点并保证上下文属性贯穿。
- [x] 实施基础性能优化（SSE 附件去重降本、LLM delta 合批）。
- [x] 补充/更新测试（优先覆盖新逻辑与回归路径）。
- [x] 运行 lint + 全量测试并修复问题（全量 lint 暴露仓库既有问题，见下方说明）。
- [ ] 更新计划与经验记录，提交增量 commits。
- [ ] 合并回 main 并清理临时 worktree/分支。

## 风险与约束
- 埋点不能引入行为变更；默认以可观测属性增强为主。
- 性能优化必须保持 SSE 消息语义一致（只降成本，不改变协议结构）。
- 优先做低风险可验证优化；高并发锁重构单独评估，不在本次范围内。

## 验证记录
- `go test ./...` 通过。
- `./scripts/run-golangci-lint.sh run ./internal/domain/agent/react/... ./internal/delivery/server/http/...` 通过。
- `./scripts/run-golangci-lint.sh run ./...` 失败，存在仓库既有问题（`evaluation/*` 与 `internal/delivery/eval/http/api_handler_rl.go` 的 `errcheck/unused`），与本次改动无关。
