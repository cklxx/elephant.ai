# 2026-03-03 Fix Remote CI Arch Violations (httpclient layering)

## Goal
修复远端 CI 中 `check-arch` 与 `check-arch-policy` 失败：非 infra 层直接导入 `internal/infra/httpclient`。

## Context
- 当前失败点：
  - `internal/domain/materials/attachment_migrator.go`（domain -> infra）
  - `internal/app/notification`（app -> infra）
  - `internal/delivery/channels/lark`（delivery -> infra）
  - `internal/shared/config`、`internal/shared/modelregistry`（shared -> infra）
- `httpclient` 是跨层通用能力（proxy policy、限流读取、circuit breaker 包装、URL 校验）。

## Options
1. 仅改 5 个违规文件改为 `net/http`。
   - Pros: 变更小。
   - Cons: 丢失统一 proxy/超时策略，新增重复逻辑。
2. 新增 `shared/httpclient` 并保留 `infra/httpclient` 双实现。
   - Pros: 快速止血。
   - Cons: 代码重复，后续维护成本高。
3. 将 `httpclient` 整体迁移到 `internal/shared/httpclient` 并全量替换导入。
   - Pros: 分层正确、能力保留、后续一致。
   - Cons: 变更面中等。

## Decision
选择 Option 3。

## Steps
- [x] 复现 CI 失败并读取 `artifacts/arch-report.json`。
- [x] 新建 `internal/shared/httpclient`，迁移 `infra/httpclient` 全部实现与测试。
- [x] 全库替换 `alex/internal/infra/httpclient` 为 `alex/internal/shared/httpclient`。
- [x] 删除 `internal/infra/httpclient` 旧目录。
- [x] 执行格式化与针对性测试。
- [x] 执行完整 CI 对齐验证（`./scripts/pre-push.sh`）。
- [x] 跑强制 code review，修复 P0/P1。
- [x] 修复并发 stale-retry 计数逻辑，消除 `go test -race` 稳定失败。
- [ ] 更新计划结果并提交。

## Validation Commands
- `go test ./internal/shared/httpclient ./internal/domain/materials ./internal/shared/modelregistry ./internal/shared/config ./internal/app/notification ./internal/delivery/channels/lark`
- `./scripts/pre-push.sh`
- `python3 skills/code-review/run.py '{"action":"review"}'`

## Result
- 远端同类失败点已消除：`arch boundaries` 与 `arch policy` 均通过。
- `golangci-lint` 通过（修复 Lark post fallback 场景下未使用函数告警）。
- `go test -race` 通过（修复 stale retry 计数按重试链递归增长的问题）。
- 全量 `./scripts/pre-push.sh` 通过（143s）。
