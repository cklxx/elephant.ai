# 2026-03-10 logging / token 审计

## Goal

- 审计 `internal/shared/logging/` 和 `internal/shared/token/`
- 统一日志格式风格，删除 dead code
- 简化 token 解析与校验逻辑
- 完成验证、review、commit，并 fast-forward merge 回 `main`

## Plan

1. 盘点两个目录的导出 API、调用点和测试覆盖。
2. 定位日志格式不一致、未使用代码和 token 解析/校验复杂路径。
3. 做最小正确重构并补充/更新测试。
4. 跑相关测试、lint、review，提交并合并。

## Progress

- [x] 创建 worktree
- [x] 审计代码与调用点
- [x] 完成重构
- [x] 回归验证
- [x] review / commit / merge

## Notes

- `logging.Multi(...)` 之前不会把 `WithLogID(...)` 下传到子 logger，导致组合 logger 的 log id 注入格式退化为消息前缀；已改为对子 logger 逐个打标，保持格式一致。
- 删除了 `logging.FromUtils(...)` 和 `readRequestLogMatches(...)` 两处无调用/无实际增益的包装代码。
- request log 的 JSON 解码与 log_id 推导已抽成共享 helper，`log_structured` 与 `log_index` 不再各自维护一套解析逻辑。
- `tokenutil` 去掉了多余的 `sync.Once + init` 组合，改成单次加载 encoding，并把编码/截断 fallback 提取为共享 helper。
- 验证已通过：
  - `go test ./internal/shared/logging ./internal/shared/token`
  - `./scripts/run-golangci-lint.sh run ./internal/shared/logging/... ./internal/shared/token/...`
