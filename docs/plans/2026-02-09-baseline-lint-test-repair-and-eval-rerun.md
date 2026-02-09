# Plan: Baseline Lint/Test Repair + New Eval Rerun (2026-02-09)

## Status
- completed

## Goal
- 修复仓库当前既有 lint/test 失败，恢复全量质量基线。
- 回归运行新增评测集合，确认扩展集合稳定通过。

## Scope
- `cmd/alex/dev.go`
- `cmd/alex/dev_lark.go`
- `internal/devops/orchestrator.go`
- `internal/devops/config_test.go`
- `internal/devops/port/allocator_test.go`
- `internal/devops/services/*.go` (acp/authdb/backend/sandbox/web)
- `internal/devops/supervisor/*.go` + tests

## Steps
- [x] 复现 lint/test 失败并收集清单
- [x] 修复 errcheck/staticcheck/unused 问题
- [x] 移除受限路径中的 `os.Getenv` 使用
- [x] 跑全量 lint + 全量 test
- [x] 复跑新增 foundation suite 评测集合

## Progress Log
- 2026-02-09 10:26: 复现失败：`golangci-lint` 多处 errcheck/staticcheck；`TestNoUnapprovedGetenv` 失败。
- 2026-02-09 10:39: 完成 devops/supervisor/services/cmd 的批量 errcheck 修复与 gofmt。
- 2026-02-09 10:41: `golangci-lint` 全绿。
- 2026-02-09 10:41: `scripts/go-with-toolchain.sh test ./...` 全绿。
- 2026-02-09 10:42: 新增评测复跑：`17/17` collections，`408/408` cases，availability errors=`0`。
