# 2026-02-08 Foundation Badcase & Availability Hardening

## Goal
- 在不依赖模型调用的 foundation 评测中，优化 bad case 并彻底解决工具可用性问题。
- 明确要求：不可通过将不可用工具标记为 N/A 来规避失败。

## Scope
- 工具可用性（registration/preset visibility）
- 隐式工具排序（heuristics）
- 关键工具可发现性（definition/metadata）
- 评测失败分类与报告可观测性
- 回归测试 + 多模式评测复跑

## Plan
1. 修复工具可用性断点并显式标记 availability 失败类型
2. 定位剩余 bad case 并做定向排序优化
3. 通过工具描述/标签增强真实可发现性（非仅评测器调分）
4. 补充单测并回归
5. 复跑三组评测并产出详细报告

## Progress
- [x] 移除 preset 工具封锁（改为 unrestricted，保留 preset label）
- [x] `lark-local` 补齐 `write_attachment` 注册
- [x] 新增本地 `write_attachment` alias 实现
- [x] foundation case 结果增加 `failure_type`，并给出 availability recommendation
- [x] 修复 v2 的 4 个 bad case（search_file/browser_info/write_file/list_dir）
- [x] 修复 v3 回归 5 个 case（artifact/memory/read/list_timers/artifacts_write）
- [x] 新增关键场景 ranking 单测
- [x] 重跑 `web/full`, `cli/full`, `web/lark-local` 三组评测
- [x] 产出优化报告（含失败拆解、成功原因、修复路径）

## Validation
- 定向测试：
  - `go test ./internal/shared/agent/presets ./internal/app/agent/preparation ./internal/app/toolregistry ./internal/infra/tools/builtin/aliases ./evaluation/agent_eval ./cmd/alex`
- 评测：
  - `go run ./cmd/alex eval foundation --mode web --preset full --toolset default --top-k 3 --format markdown --output tmp/eval-foundation-availability-20260208-v4/web-full`
  - `go run ./cmd/alex eval foundation --mode cli --preset full --toolset default --top-k 3 --format markdown --output tmp/eval-foundation-availability-20260208-v4/cli-full`
  - `go run ./cmd/alex eval foundation --mode web --preset full --toolset lark-local --top-k 3 --format markdown --output tmp/eval-foundation-availability-20260208-v4/web-lark-local`

## Result Snapshot
- 三组模式均达到：Top-3 hit rate = 100%，Failed cases = 0，Availability errors = 0。

## Notes
- `./dev.sh lint` 仍失败于仓库存量 `internal/devops` / `cmd/alex dev_*` 的 errcheck/staticcheck。
- `./dev.sh test` 仍失败于仓库存量 race/env-guard（`internal/delivery/server/bootstrap`、`internal/shared/config`）。
