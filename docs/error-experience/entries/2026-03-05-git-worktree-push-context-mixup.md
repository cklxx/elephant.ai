# 2026-03-05 · Git 推送上下文混用导致主线状态紊乱

## Context
- 目标：将修复分支稳定合并回 `main`，并确认远端 CI。
- 实际：在临时 clone、主仓库、worktree 之间交叉执行 rebase/push，导致分支状态与执行上下文混杂。

## Symptom
- `main` 与 `origin/main` 出现非预期分叉。
- 在错误上下文触发 pre-push，出现“需 worktree/上下文不一致”等操作噪音。
- CI 首次失败后未立即转入修复路径，拉长了恢复时间。

## Root Cause
- 缺少“push 只能在主仓库/受管 worktree 执行”的硬性技术门禁。
- 过程上没有把“失败优先修复”作为强约束执行。

## Remediation
- 在 `scripts/pre-push.sh` 增加 `origin` 远端守卫：默认拒绝本地路径远端推送（可显式 `ALLOW_LOCAL_ORIGIN_PUSH=1` 覆盖）。
- 在 `AGENTS.md` 与 `docs/guides/engineering-workflow.md` 增加 push 范围规则。
- 补齐 `check-arch-policy` 需要的白名单，消除确定性 CI 红灯。
- 输出事故复盘，固定“时间线-根因-预防动作”闭环。

## Metadata
- id: err-2026-03-05-git-worktree-push-context-mixup
- tags: [git, worktree, push, ci, user-correction]
- links:
  - docs/error-experience/summary/entries/2026-03-05-git-worktree-push-context-mixup.md
  - docs/postmortems/incidents/2026-03-05-git-worktree-push-flow-break.md
