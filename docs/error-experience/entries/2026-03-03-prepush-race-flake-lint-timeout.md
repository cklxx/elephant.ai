# 2026-03-03 · pre-push 门禁出现 race 抖动 + lint 超时

## Context
- 在大规模改动后执行 `./scripts/pre-push.sh`。
- 同时包含 Go 与 web 变更，触发并行门禁（vet/build/race/lint/web）。

## Symptom
- `go test -race` 在 pre-push 内偶发失败，但独立执行 `go test -race -count=1 ./...` 可以通过。
- `golangci-lint` 在 pre-push 固定 `--timeout=3m` 下超时；独立执行 `--timeout=10m` 可通过。

## Root Cause
- pre-push 使用固定短超时与并行负载，容易在重改场景触发资源竞争和时间抖动。
- 门禁脚本尾部仅展示最近日志，定位失败包成本高。

## Remediation
- 先独立复跑失败项确认是否真实回归：
  - `go test -race -count=1 ./...`
  - `./scripts/run-golangci-lint.sh run --timeout=10m ./...`
- 在计划文档记录“脚本失败但独立复跑通过”的证据链，避免误判。

## Follow-up
- 评估将 pre-push lint timeout 从 `3m` 提升为更稳健阈值（或按变更范围自适应）。
- pre-push 可增加失败包名直出，减少排障时间。

## Metadata
- id: err-2026-03-03-prepush-race-flake-lint-timeout
- tags: [pre-push, race, lint, timeout, quality-gate]
- links:
  - docs/plans/2026-03-03-global-codebase-simplify.md
  - docs/error-experience/summary/entries/2026-03-03-prepush-race-flake-lint-timeout.md
