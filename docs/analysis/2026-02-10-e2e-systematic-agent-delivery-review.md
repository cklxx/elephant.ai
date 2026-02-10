# 2026-02-10 E2E 系统化评测与 Agent 交付能力 Review

## 1. 本次范围
- 目标：
  - 基于业界高难 benchmark 家族重建系统化端到端评测。
  - 对当前 agent 交付能力做一次横向 review（老 suite vs 新 e2e suite）。
- 运行命令：
  - `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml --output tmp/foundation-suite-r19-current-20260210-104937 --format markdown`
  - `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml --output tmp/foundation-suite-r19-e2e-systematic-20260210-104937 --format markdown`

## 2. 总体分数（x/x）

| Suite | Collections (passed/total) | Cases (passed/total) | pass@1 (x/x) | pass@5 (x/x) | Failed | Deliverable Good/All |
|---|---:|---:|---:|---:|---:|---:|
| 当前主 suite (`foundation_eval_suite.yaml`) | `28/28` | `269/269` | `224/269` | `269/269` | `0` | `24/30` |
| 新 E2E 系统化 suite (`foundation_eval_suite_e2e_systematic.yaml`) | `25/28` | `360/363` | `312/363` | `361/363` | `3` | `14/20` |

结论：
- 新 suite 难度显著上升并成功暴露真实失败（`failed=3`），不再是“高分但低挑战”。
- pass@1 仍在 85% 左右，说明首选工具路由仍有可优化空间。
- 交付质量在更高难场景下降（`70%`），主要暴露在“产物写入 vs 附件交付”和“证据契约覆盖不足”。

## 3. 新 E2E suite 维度覆盖（x/x）
- Collections: `28/28`
- Cases: `363/363`
- N/A: `0`

关键维度通过情况（pass@5）：
- Foundation Core（tool/prompt/proactivity/memory/safety/availability/speed）整体稳定，均为 `>= 15/16` 或 `15/15` 级别。
- Frontier Transfer 中出现真实失败：
  - `industry-benchmark-agentbench-multidomain-tooluse-hard`: `11/12`
  - `industry-benchmark-agentlongbench-long-context-memory-hard`: `11/12`
  - `motivation-aware-proactivity`: `16/16` 但存在 top-k 边缘失败点（见下）。

## 4. Top 失败 case 拆解

| Collection | Case | Failure Type | Expected | Top Matches | 诊断 |
|---|---|---|---|---|---|
| `industry-benchmark-agentbench-multidomain-tooluse-hard` | `agentbench-hard-cli-evidence-run` | `rank_below_top_k` | `shell_exec` | `browser_screenshot`, `list_dir`, `execute_code`, `find`, `config_manage` | CLI 证据采集语义被视觉证据词干扰，`shell_exec` 被压到 Top-5 外 |
| `industry-benchmark-agentlongbench-long-context-memory-hard` | `agentlongbench-hard-memory-needle-discovery` | `rank_below_top_k` | `memory_search` | `browser_action`, `memory_get`, `find`, `read_file`, `plan` | “查找 needle”语义被操作/读取类工具抢占，memory search 边界词不足 |
| `motivation-aware-proactivity` | `motivation-memory-recall-cadence` | `rank_below_top_k` | `memory_search` | `cancel_timer`, `list_timers`, `plan` | 动机节奏类文本触发 scheduler 词，memory 检索意图被二义化 |

## 5. 交付能力抽样检查（Good/Bad）

Good 样本（交付契约覆盖 100%）：
- `webarena-hard-artifactize-run-evidence`：`artifacts_write` Top1，契约 `5/5`。
- `osworld-g-persist-visual-runbook`：`artifacts_write` Top1，契约 `4/4`。
- `complex-artifact-delivery/artifact-delivery-manifest-audit-before-send`：`artifact_manifest` + `artifacts_list` 双命中，契约 `2/2`。
- `mle-bench-deliver-downloadable-repro-pack`：`write_attachment` Top1，契约 `2/2`。

Bad 样本（契约覆盖不足）：
- `tau2-hard-upload-final-review-pack`：`lark_upload_file` 命中但契约仅 `1/2`。
- `agentbench-hard-delivery-artifact-pack`：`artifacts_write` 预期但 `write_attachment` Top1，契约 `3/4`。
- `cybench-hard-persist-incident-review-pack`：契约 `3/4`，manifest/list/write 链路未完全收敛。

## 6. 主要冲突簇（expected => top1）
- `read_file => memory_get`（6）
- `artifacts_list => artifacts_write`（4）
- `web_fetch => web_search`（3）
- `memory_search => browser_action`（2）
- `shell_exec => browser_screenshot`（2）

这 5 类占比最高，是下一轮产品优化优先级。

## 7. 当前 Agent 交付能力评审结论
1. 交付“可完成性”仍然较好：新 suite `360/363` 已通过，具备端到端执行框架能力。  
2. 交付“首选路径精度”仍不足：pass@1 `312/363`，在高难隐式意图下存在路径漂移。  
3. 交付“契约一致性”需加强：`Deliverable Good 14/20`，尤其在 artifact/write_attachment/lark_upload_file 边界。  
4. 新增 benchmark 族有效提升了挑战强度并暴露真实失败，不再是仅靠 easy case 刷分。  

## 8. 下一轮优化建议（产品向）
1. `shell_exec` vs `browser_screenshot`：增加“CLI/命令/日志/stdout/stderr/runtime snapshot”强触发词，降低视觉证据词对终端语义的抢占。  
2. `memory_search` vs `memory_get`：将“needle discovery / recall / locate prior note”映射到 search，将“open exact note”映射到 get。  
3. `artifacts_write` vs `write_attachment` vs `artifacts_list`：按“创建/更新产物、打包发送、仅枚举选择”三段式强化语义边界。  
4. 对上述冲突簇增加最小回归集，目标是先清零 `failed=3`，再拉升 pass@1。  

## 9. 产物路径
- 旧 suite 结果：
  - `tmp/foundation-suite-r19-current-20260210-104937/foundation_suite_report_foundation-suite-20260210-025000.md`
  - `tmp/foundation-suite-r19-current-20260210-104937/foundation_suite_result_foundation-suite-20260210-025000.json`
- 新 e2e suite 结果：
  - `tmp/foundation-suite-r19-e2e-systematic-20260210-104937/foundation_suite_report_foundation-suite-20260210-025000.md`
  - `tmp/foundation-suite-r19-e2e-systematic-20260210-104937/foundation_suite_result_foundation-suite-20260210-025000.json`
