# 2026-03-10 go test ./... 修复计划

## Goal

- 运行 `go test ./...`
- 修复所有失败测试
- 完成验证、review、commit，并 fast-forward merge 回 `main`

## Plan

1. 采集全量失败输出，按失败簇归类。
2. 定位受影响代码与测试，做最小正确修复。
3. 回归相关包与全量测试。
4. 运行 code review，修复高优先级问题后提交与合并。

## Progress

- [x] 创建 worktree
- [x] 首次全量测试
- [x] 修复失败
- [x] 回归验证
- [ ] review / commit / merge

## Notes

- 首轮 `go test ./...` 仅失败于 `internal/domain/agent/react.TestMultipleTasks`。
- 失败表现为并发任务 completion 收集偶发少于 5，属于测试时序抖动。
- 修复方式：先用 `Collect(..., wait=true)` 等待结果完成，再在短 deadline 内 drain completion，避免单次即时断言的 flaky 行为。
- 验证已通过：
  - `go test ./internal/domain/agent/react -run TestMultipleTasks -count=50`
  - `go test ./internal/domain/agent/react`
  - `go test ./...`
  - `alex dev lint`
  - `python3 skills/code-review/run.py review`
