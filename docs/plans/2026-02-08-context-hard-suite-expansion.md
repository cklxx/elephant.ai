# Plan: Context-Hard Suite Expansion (2026-02-08)

## Status
- done

## Goal
- 增加更难、系统性的 agent 能力评测集合。
- 参考最新 context/long-context benchmark 的难题模式，将其映射到 foundation 离线评测集合。

## Scope
- `evaluation/agent_eval/datasets/foundation_eval_cases_context_learning_hard.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- `evaluation/agent_eval/README.md`
- `docs/research/*`
- `docs/plans/*`, `docs/good-experience/*`, `docs/memory/long-term.md`

## Steps
- [x] 新建 worktree 分支并复制 `.env`
- [x] 新增 `context_learning_hard` 集合并接入 suite
- [x] 跑 suite 验证新增集合效果
- [x] 补充 context benchmark 调研映射文档并更新索引
- [x] 更新 README 覆盖规模 `x/x`
- [x] 执行目标测试并提交合并

## Progress Log
- 2026-02-08 19:58: worktree `feat/hard-contextbench-suite-20260208` 初始化完成。
- 2026-02-08 20:01: 新增 `context_learning_hard` 集合（20 cases），suite 扩展为 10 collections。
- 2026-02-08 20:05: 首轮即达到 `10/10` collections、`274/274` cases、`availability_errors=0`。
- 2026-02-08 20:09: 新增调研文档 `docs/research/2026-02-08-context-learning-bench-hard-cases-mapping.md`，并更新 research 索引与 README 覆盖规模。
