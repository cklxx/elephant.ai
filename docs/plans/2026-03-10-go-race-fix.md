# 2026-03-10 go test -race 修复计划

## Goal

- 运行 `CGO_ENABLED=0 go test -race ./...`
- 修复所有 data race / 并发问题
- 完成验证、review、commit，并 fast-forward merge 回 `main`

## Plan

1. 全量运行 race 检测并按失败包归类。
2. 定位共享状态读写路径，分析根因并做最小正确修复。
3. 回归相关包与全量 `-race`。
4. 跑 lint、review、提交并合并。

## Progress

- [x] 创建 worktree
- [x] 首次全量 race 检测
- [x] 修复竞态
- [x] 回归验证
- [x] review / commit / merge

## Notes

- 以 `CGO_ENABLED=0 go test -race ./...` 完整运行了全仓库竞态检测。
- 本轮未发现 data race，也没有触发需要加锁或重构的并发问题。
- 因为没有发现 race，未修改业务代码；本次提交仅记录执行与结论。
