# Plan: Foundation Latency Metrics (2026-02-08)

## Status
- completed

## Goal
- 在 foundation / foundation-suite 报告中增加真实评测运行时延指标（总耗时、吞吐、case latency p50/p95/p99）。

## Scope
- `evaluation/agent_eval/foundation_eval.go`
- `evaluation/agent_eval/foundation_suite.go`
- `evaluation/agent_eval/foundation_report.go`
- `cmd/alex/eval_foundation_suite.go`
- `evaluation/agent_eval/*_test.go`

## Steps
- [x] 在 case 级采集 routing latency，并聚合到 implicit summary。
- [x] 在 suite 级聚合 collection/case latency 与吞吐指标。
- [x] 在 markdown/CLI 输出时延字段。
- [x] 更新测试断言并回归。
- [x] 提交并合并回 main。

## Progress Log
- 2026-02-08 22:22: 已实现 case latency 采集与 summary 聚合。
- 2026-02-08 22:24: 已实现 suite latency 聚合与 markdown 表格展示。
- 2026-02-09 00:24: 回归通过（目标包测试通过 + 全量 `go test ./...` 仅剩仓库已知 `internal/shared/config` getenv guard 失败）；真实 suite 运行已产出 latency 指标。
