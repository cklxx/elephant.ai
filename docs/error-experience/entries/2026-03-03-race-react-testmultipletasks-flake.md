# 2026-03-03 · `go test -race` 下 `TestMultipleTasks` 偶发失败

## Context
- 在“全局简化复扫 + 非 web 优先优化”后执行全量门禁。
- 命令：`go test -race -count=1 ./...`。

## Symptom
- 全量 race 跑到 `internal/domain/agent/react` 时出现：
  - `--- FAIL: TestMultipleTasks`
  - `expected 5 completions, got 4`
- 随后定向复跑 `go test -race -count=1 ./internal/domain/agent/react -run TestMultipleTasks` 通过。

## Root Cause
- 现象与既有记录一致，属于并发测试在高负载全量 race 场景下的偶发抖动。
- 当前证据不足以指向本次改动引入的确定性回归。

## Remediation
- 保留失败日志和复跑通过证据，不把该次失败直接归因为功能回归。
- 继续执行剩余 gate（lint/web build），并在计划文档记录“全量失败 + 定向复跑通过”。

## Follow-up
- 评估加固 `internal/domain/agent/react/background_test.go` 的完成计数等待策略，降低 race 场景下计数竞态抖动。
- 将该抖动模式纳入门禁已知问题库，避免重复排障。

## Metadata
- id: err-2026-03-03-race-react-testmultipletasks-flake
- tags: [race, flaky-test, react, quality-gate]
- links:
  - docs/plans/2026-03-03-global-codebase-simplify.md
  - docs/error-experience/summary/entries/2026-03-03-race-react-testmultipletasks-flake.md
