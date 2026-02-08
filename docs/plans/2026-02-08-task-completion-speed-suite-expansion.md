# Plan: Task Completion Speed Suite Expansion (2026-02-08)

## Status
- done

## Goal
- 增加“任务完成速度”专项评测集合，验证模型在速度敏感场景下是否优先选择最短可行工具路径。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_cases_task_completion_speed.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/README.md`
- `docs/plans/*`, `docs/good-experience/*`, `docs/memory/long-term.md`

## Steps
- [x] 创建独立 worktree 分支并复制 `.env`
- [x] 新增 task completion speed 集合并纳入 suite
- [x] 运行 suite 并收敛潜在 bad cases
- [x] 更新 README 覆盖规模 `x/x`
- [x] 更新计划/经验记录并合并回 main

## Progress Log
- 2026-02-08 21:15: worktree `feat/speed-eval-suite-20260208` 初始化完成。
- 2026-02-08 22:10: 新增 `task-completion-speed` 集合（20 cases），suite 扩展到 13 collections / 334 cases。
- 2026-02-08 22:11: 首轮即稳定 `13/13`、`334/334`，availability errors=0。
