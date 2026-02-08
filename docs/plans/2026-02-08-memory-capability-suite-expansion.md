# Plan: Memory Capability Suite Expansion (2026-02-08)

## Status
- done

## Goal
- 增加记忆相关的评测集合，系统覆盖 memory 检索、读取、更新、冲突处理、删除边界与记忆驱动执行链路。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_cases_memory_capabilities.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/README.md`
- `docs/plans/*`, `docs/good-experience/*`

## Steps
- [x] 创建独立 worktree 分支并复制 `.env`
- [x] 加载工程规范与长期记忆
- [x] 新增 memory 能力集合并纳入 suite
- [x] 运行 suite，收敛潜在 bad cases
- [x] 更新 README 覆盖规模 `x/x`
- [x] 更新计划/经验记录，提交并合并回 main

## Progress Log
- 2026-02-08 20:12: worktree `feat/memory-eval-suite-20260208` 初始化完成。
- 2026-02-08 20:14: 基线确认：当前 suite 为 10 collections / 274 cases。
- 2026-02-08 21:01: 新增 `memory-capabilities` 集合（20 cases），suite 扩展到 11 collections / 294 cases。
- 2026-02-08 21:03: 首轮即稳定 `11/11`、`294/294`，availability errors=0。
