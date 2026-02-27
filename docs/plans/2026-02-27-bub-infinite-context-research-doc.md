# 2026-02-27 Bub 无限上下文（Append-only Tape）实现级调研

## Objective
- 使用 sub-agent 深挖 Bub 的“无限上下文”实现逻辑，输出代码实现级调研文档，覆盖存储、运行时链路、边界条件、文档一致性与可迁移实现方案。

## Scope
- 外部仓库：`https://github.com/PsiACE/bub`（调研基线 commit 见报告）
- 重点模块：
  - `src/bub/tape/store.py`
  - `src/bub/tape/service.py`
  - `src/bub/core/router.py`
  - `src/bub/core/agent_loop.py`
  - `src/bub/core/model_runner.py`
  - `src/bub/tools/builtin.py`
  - 相关 tests 与 `republic` 依赖中 tape/chat 关键路径

## Execution Plan
- [completed] 运行 main 分支 pre-work checklist（`git diff --stat`、`git log --oneline -10`）并确认工作区干净。
- [completed] 复核 `docs/guides/engineering-practices.md`。
- [completed] 并行启动 3 个 sub-agent：
  - A：存储层（JSONL 写入、fork/merge、一致性）
  - B：运行时链路（input/router/model/tape）
  - C：测试与文档一致性（命令面、边界条件）
- [completed] 汇总 sub-agent 证据，生成实现级调研报告。

## Progress Log
- 2026-02-27：完成 pre-work 与 guides 复核。
- 2026-02-27：sub-agent A 回传存储层实现细节、复杂度与风险点（含行号）。
- 2026-02-27：sub-agent B 回传端到端运行时调用链与 anchor/handoff 机制证据。
- 2026-02-27：sub-agent C 回传测试覆盖面、文档偏差与边界条件清单。
- 2026-02-27：输出研究文档 `docs/research/2026-02-27-bub-append-only-tape-infinite-context-implementation-deep-dive.md`。
