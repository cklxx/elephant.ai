# Plan: Foundation Expansion + SWE-bench Verified Research (2026-02-08)

## Status
- in_progress

## Goal
- 继续增加 foundation 离线评测集合，扩大覆盖面和案例数量。
- 新增 `SWE-bench Verified` 调研文档，沉淀榜单提交流程、限制条件、与本仓库落地方案。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_*.yaml`
- `evaluation/agent_eval/README.md`
- `docs/research/*`
- `docs/plans/*`, `docs/good-experience/*`, `docs/memory/long-term.md`

## Steps
- [x] 创建独立 worktree 分支并复制 `.env`
- [x] 加载工程规范与记忆
- [x] 新增一组 foundation 评测集合并纳入 suite
- [x] 复跑 suite，定位并收敛新增 bad cases
- [x] 新增 SWE-bench Verified 调研文档并更新 research 索引
- [x] 更新文档中的覆盖规模 `x/x`
- [x] 继续新增系统性能力集合（orchestration + safety policy）
- [x] 运行目标测试与评测命令
- [ ] 提交增量 commits 并合并回 `main`

## Progress Log
- 2026-02-08 19:33: worktree `feat/eval-verified-research-20260208` 初始化完成，已复制 `.env`。
- 2026-02-08 19:36: 现状确认：foundation suite 当前 `6` collections，`190` cases。
- 2026-02-08 19:47: 新增 `SWE-bench Verified Readiness` 集合（24 cases），suite 扩展到 `7` collections。
- 2026-02-08 19:49: 首轮扩容结果 `213/214`，修复 1 个词汇冲突 case 后复跑达到 `214/214`（`7/7`，availability errors=0）。
- 2026-02-08 19:53: 新增调研文档 `docs/research/2026-02-08-swebench-verified-submission-research.md`，并更新 research 索引。
- 2026-02-08 20:05: 新增 `multi_step_orchestration`（20 cases）与 `safety_boundary_policy`（20 cases），suite 扩展到 `9` collections、`254` cases。
- 2026-02-08 20:08: 扩容首轮 `252/254`，定向修复 2 个冲突 case 后稳定到 `254/254`（`9/9`，availability errors=0）。
