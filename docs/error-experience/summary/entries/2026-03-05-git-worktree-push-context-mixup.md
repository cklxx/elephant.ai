Summary: 这次主线异常的核心是把临时 clone 与主仓库/worktree 的 Git 上下文混用，导致分支状态紊乱与推送路径错误。修复要点是加 push 范围硬门禁、失败优先修复、以及在事故复盘中固化时间线与预防动作。

## Metadata
- id: errsum-2026-03-05-git-worktree-push-context-mixup
- tags: [summary, git, worktree, push, ci]
- derived_from:
  - docs/error-experience/entries/2026-03-05-git-worktree-push-context-mixup.md
