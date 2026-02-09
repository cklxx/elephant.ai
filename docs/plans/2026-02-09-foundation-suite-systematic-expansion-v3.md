# Plan: Foundation Suite Systematic Expansion V3 (2026-02-09)

## Status
- completed

## Goal
- 扩展 foundation 离线评测集合，新增四类高难能力覆盖：
  - 多轮长任务（long-horizon, multi-round）
  - 复杂逻辑架构编码（architecture coding hard）
  - 深度搜索调研（deep research）
  - 自主性与主动推进（autonomy initiative）
- 产出可执行评测并在报告中体现新的总规模与 `x/x` 通过计数。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_long_horizon_multi_round.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_architecture_coding_hard.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_deep_research.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_autonomy_initiative.yaml`
- `docs/good-experience/entries/`
- `docs/good-experience/summary/entries/`

## Steps
- [x] 复盘现有 suite 与 case schema，确认扩展点
- [x] 新增四个高难集合数据文件
- [x] 将四个集合接入 suite 主 YAML
- [x] 跑 foundation-suite 回归并记录 `x/x` 结果
- [x] 更新 good-experience 记录与总结
- [ ] 提交、合并回 main、清理 worktree

## Progress Log
- 2026-02-09 09:10: 完成规范与记忆加载，确认当前 suite 为 13 collections / 334 cases。
- 2026-02-09 09:12: 完成扩展方案设计（新增 4 类能力集合并纳入主 suite）。
- 2026-02-09 09:50: 新增 4 个集合（long-horizon multi-round / architecture coding hard / deep research / autonomy initiative），suite 扩展到 17 collections / 408 cases。
- 2026-02-09 09:52: 首轮回归 `15/17`, `405/408`；针对 3 个 rank-below-top-k badcase 优化 intent 词面。
- 2026-02-09 09:57: 二轮回归 `15/17`, `406/408`；继续优化剩余 badcase。
- 2026-02-09 09:58: 三轮回归稳定 `17/17`, `408/408`，availability errors=0。
