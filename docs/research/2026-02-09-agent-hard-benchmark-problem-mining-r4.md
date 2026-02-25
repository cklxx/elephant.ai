# 2026-02-09 — Agent Hard Benchmark Problem Mining (R4)

## Goal
将公开 benchmark 的高难失败模式映射到本仓库 foundation offline 路由评测，用于持续压测 agent 的隐式意图裂解与工具选择能力。

## Sources (Primary)
- BrowseComp（OpenAI）: https://openai.com/index/browsecomp/
- LongBench v2（arXiv）: https://arxiv.org/abs/2412.15204
- TAU-bench（arXiv）: https://arxiv.org/abs/2406.12045
- SWE-bench Verified（OpenReview）: https://openreview.net/forum?id=VTF8yNQM66

## Hard-Pattern Mapping

### 1) Sparse-Clue Retrieval (BrowseComp-like)
- 信号特征：线索稀疏、目标引用不完整、source-selection 与 retrieval-routing 强耦合。
- 在 foundation 的映射：
  - `web_search` vs `web_fetch`
  - `memory_search` vs `search_file`
  - `find` vs `search_file`
  - `lark_send_message` vs `lark_upload_file` vs `write_attachment`

### 2) Stateful Commitment Boundary (TAU-bench-like)
- 信号特征：多轮状态依赖、承诺边界、读后写约束、禁止突变窗口。
- 在 foundation 的映射：
  - `scheduler_list_jobs` vs `scheduler_create_job` vs `scheduler_delete_job`
  - `request_user` vs `clarify`
  - `plan` vs `lark_task_manage`
  - `find` path-first 与 `search_file` content-first 的顺序语义

### 3) Reproducibility Trace Evidence (WebArena/SWE Verified-like)
- 信号特征：可复现证据链、traceability、发布门控（file-in-thread / downloadable / manifest）。
- 在 foundation 的映射：
  - `artifact_manifest` / `artifacts_list` / `artifacts_delete`
  - `artifacts_write` / `lark_upload_file` / `write_attachment`
  - `browser_info` / `browser_screenshot` 证据采样前置条件

## Resulting Dataset Design
本轮新增 3 个集合（每个 16 case）：
- `foundation_eval_cases_sparse_clue_retrieval.yaml`
- `foundation_eval_cases_stateful_commitment_boundary.yaml`
- `foundation_eval_cases_reproducibility_trace_evidence.yaml`

设计原则：
- 隐式提示词优先，不显式点名工具。
- 每个 case 只考察一个主冲突，避免复合歧义导致不可诊断。
- 产物型任务保留 deliverable contract，用于路由正确性与交付完整度分离评估。
