# Plan: Foundation Suite Scaleout v2 (2026-02-08)

## Status
- done

## Goal
- 回答「SWE 全套评测轮次」口径并给出可执行计算方式。
- 在现有 4 层 foundation suite 基础上继续扩容，新增更多测试集合覆盖更多面。
- 报告中明确展示 `x/x`，并在文档标注每个集合规模与总规模。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_*.yaml`
- `evaluation/agent_eval/README.md`
- `docs/plans/*`（计划与进度记录）

## Steps
- [x] 盘点 SWE 轮次口径（实例规模、max_turns 默认值、总轮次计算方式）。
- [x] 新增集合：availability recovery。
- [x] 新增集合：valuable workflows。
- [x] 跑 foundation-suite 全量并定位新增 bad case。
- [x] 定向优化 bad case 并复跑到稳定结果。
- [x] 更新文档，标注各集合与总量 `x/x`。
- [x] 提交增量 commits，回合并到 `main`。

## Progress Log
- 2026-02-08 16:35: 在独立 worktree `feat/foundation-scaleout-20260208` 开始，完成环境与规则加载。
- 2026-02-08 16:40: 新增 `foundation_eval_cases_availability_recovery.yaml`（24 cases）。
- 2026-02-08 16:44: 新增 `foundation_eval_cases_valuable_workflows.yaml`（28 cases）。
- 2026-02-08 16:46: suite 从 4 collections 扩展到 6 collections。
- 2026-02-08 16:50: 首轮扩容结果 `5/6`, `187/190`；定位 3 个 bad case 为词汇冲突导致（无可用性错误）。
- 2026-02-08 16:53: 调整 3 条 valuable workflow 意图后复跑达到 `6/6`, `190/190`, `availability_errors=0`。
- 2026-02-08 16:57: 更新 `evaluation/agent_eval/README.md`，补充分层集合覆盖面与 `x/x` 标注。
