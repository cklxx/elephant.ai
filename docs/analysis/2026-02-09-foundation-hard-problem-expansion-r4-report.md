# 2026-02-09 — Foundation Hard Problem Expansion + Full Optimization (R4)

## 1) Objective
- 增加更难问题集合（benchmark 风格映射）并压测隐式意图裂解能力。
- 执行“扩题 -> 全量评测 -> 失败拆解 -> 规则优化 -> 回归验证”闭环。

## 2) x/x Scoreboard

| Stage | Collections | Cases | pass@1 | pass@5 | Failed | Deliverable Good/All |
|---|---:|---:|---:|---:|---:|---:|
| Pre-expand baseline | `22/22` | `608/608` | `526/608` | `608/608` | `0` | `33/44` |
| Post-expand (before optimization) | `22/25` | `656/656` | `556/656` | `648/656` | `8` | `38/50` |
| Final after optimization | `25/25` | `656/656` | `562/656` | `656/656` | `0` | `39/50` |

结论：
- 规模从 `608 -> 656`，挑战强度显著上升。
- 在更难集合下，最终恢复到 `pass@5=656/656`，并将真失败归零。
- 相比扩题前，`pass@1` 比例从 `86.5%` 降到 `85.7%`，仍保持挑战压力。

## 3) Added Collections
- `Sparse-Clue Retrieval Stress` (`16/16`)
- `Stateful Commitment Boundary Stress` (`16/16`)
- `Reproducibility Trace Evidence Stress` (`16/16`)

suite 入口：`evaluation/agent_eval/datasets/foundation_eval_suite.yaml`

## 4) Failure Decomposition (Post-expand)

Top true failures（扩题后首次运行）聚焦 8 个 case，集中在：
- scheduler 语义：`scheduler_list_jobs` / `scheduler_delete_job` 被误路由。
- path/content 检索边界：`find` 与 `search_file` 在多轮状态语义下冲突。
- 证据链工具：`artifacts_list` / `artifacts_delete` 在隐式表达下被非目标工具抢分。

Top1 cluster（post-expand）
- `memory_search => search_file` (`5`)
- `find => search_file` (`5`)
- `web_fetch => web_search` (`3`)
- `plan => lark_task_manage` (`3`)
- `lark_send_message => lark_upload_file` (`3`)

## 5) Optimization Actions
- 强化 `scheduler_list_jobs` 读状态意图和 `scheduler_delete_job` 删除而非重建意图（含词干形态）。
- 对 `scheduler_create_job` 在“remove-not-recreate”语义下增加反向惩罚。
- 强化 `find` 的 path-first-before-content 语义，强化 `search_file` 的 content-inside-files 语义。
- 增强 `artifacts_list` / `artifacts_delete` 在“enumerate outputs / stale failed bundles”语义下的正向权重。
- 增加 `write_file` / `replace_in_file` / `video_generate` 在非写作场景的抑制规则。
- 增加回归断言：`evaluation/agent_eval/foundation_eval_test.go`。

## 6) Final Residual Top1 Backlog (No pass@5 failures)
- `memory_search => search_file` (`5`)
- `find => search_file` (`4`)
- `web_fetch => web_search` (`3`)
- `plan => lark_task_manage` (`3`)
- `lark_send_message => lark_upload_file` (`3`)

这些是下一轮 pass@1 优化优先簇。

## 7) Deliverable Sampling (Final)
Good samples:
- `motivation-progress-artifact-proof` -> `artifacts_write`, coverage=`1.0`
- `opt-hard-artifact-progress-proof` -> `artifacts_write`, coverage=`1.0`
- `artifact-delivery-slides-from-images` -> `pptx_from_images,artifacts_write`, coverage=`1.0`

Bad samples:
- `opt-hard-downloadable-summary-attachment`, coverage=`0.5`
- `artifact-delivery-path-first-content-later-package`, coverage=`0.0`
- `artifact-delivery-scheduler-audit-before-removal`, coverage=`0.0`

## 8) Run Artifacts
- Pre-expand baseline: `tmp/foundation-suite-r4-preexpand-20260209-152913`
- Post-expand baseline: `tmp/foundation-suite-r4-postexpand-20260209-153059`
- Final optimized run: `tmp/foundation-suite-r4-final2-20260209-153437`
