# Plan: Habit + Soul + Memory Suite Expansion (2026-02-08)

## Status
- done

## Goal
- 增加“用户习惯 + Soul + 记忆”相关评测集合，覆盖偏好持续性、persona 连贯性、记忆冲突与记忆驱动执行。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_cases_user_habit_soul_memory.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/README.md`
- `docs/plans/*`, `docs/good-experience/*`, `docs/memory/long-term.md`

## Steps
- [x] 新建 worktree 分支并复制 `.env`
- [x] 新增 habit+soul+memory 集合并纳入 suite
- [x] 运行 suite 并收敛潜在 bad cases
- [x] 更新 README 覆盖规模 `x/x`
- [x] 更新计划/经验记录并合并回 main

## Progress Log
- 2026-02-08 21:07: worktree `feat/habit-soul-memory-suite-20260208` 初始化完成。
- 2026-02-08 21:08: 新增 `user-habit-soul-memory` 集合（20 cases），suite 扩展到 12 collections / 314 cases。
- 2026-02-08 21:09: 首轮 `313/314`，修复 1 个词汇冲突 case 后稳定到 `314/314`，availability errors=0。
