# Agent Real E2E Eval Run Plan (2026-02-13)

## Goal
执行真实端到端评测（非 dry-run），产出可追溯结果与对照指标，识别失败簇并给出后续优化入口。

## Scope
- Run `foundation_eval_suite_e2e_systematic.yaml`
- Run `foundation_eval_suite.yaml` for baseline comparison
- Run `foundation_eval_suite_basic_active.yaml` for active-tool/skills health
- Run `foundation_eval_suite_motivation_aware.yaml` for motivation-aware proactivity health
- Run `foundation_eval_suite_active_tools_systematic_hard.yaml` for hard active-tool intent mapping
- Output summary and failure focus report under `docs/analysis/`

## Steps
1. 从 `main` 切新 worktree 分支并复制 `.env`（completed）。
2. 执行三套 suite 真跑并保存到 `tmp/`（completed）。
3. 解析核心指标（pass@1/pass@5/failed/availability/deliverable）（completed）。
4. 补跑 motivation-aware + active-tools-hard 两套 suite（completed）。
5. 输出扩展分析报告与后续建议（completed）。

## Acceptance
- 每个目标 suite 都有完整 result/report artifact。
- 至少包含：总览指标、失败集合、对比结论、下一步优化建议。
