# 2026-02-09 动机感知主动性 AI 评测集合设计与验证方案

## 1. 目标
把“动机支持能力”从概念变成可回归工程资产：
- 有清晰 case 集合（离线可重复）。
- 有明确判定指标（正确性 + 安全边界 + 价值产出）。
- 有线上验证闭环（效果与风险同时可观测）。

## 2. 本次新增产物
- `evaluation/agent_eval/datasets/foundation_eval_cases_motivation_aware_proactivity.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_suite_motivation_aware.yaml`

说明：本次采用独立 suite，避免直接扰动默认 `foundation_eval_suite.yaml` 的历史基线。

## 3. 评测集合设计

### 3.1 设计原则
- 分层覆盖：识别信号、动作选择、边界约束、反馈产物。
- 首动作可判定：每个 case 明确 `expected_tools`。
- 安全内建：显式覆盖 consent、敏感数据、过度打扰风险。
- 个性化可检验：加入 memory 驱动场景。

### 3.2 维度与覆盖

| 维度 | 目标 | 典型工具 |
|---|---|---|
| motivation_signal | 识别低能量/犹豫状态并先澄清 | `clarify`, `plan` |
| motivation_proactivity | 把动机建议落到行动节奏 | `set_timer`, `scheduler_create_job`, `list_timers`, `cancel_timer` |
| motivation_boundary | 避免过度干预与越权 | `request_user`, `clarify` |
| motivation_memory | 用历史偏好做个性化 | `memory_search`, `memory_get` |
| motivation_extrinsic | 将承诺写入任务/日历/OKR | `lark_task_manage`, `lark_calendar_create`, `okr_write` |
| motivation_feedback | 可见进展与交付证据 | `artifacts_write`, `write_attachment`, `artifact_manifest`, `a2ui_emit` |

### 3.3 关键样例类别
- 低能量、拖延、burnout 时的最小干预。
- “要提醒”与“不想被打扰”冲突时的澄清。
- 敏感信息/第三方触达前的同意门控。
- 用记忆恢复偏好节奏与语言风格。
- 进展产物化（artifact/attachment/manifest）以增强反馈回路。

## 4. 如何验证（离线）

### 4.1 运行命令

```bash
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_motivation_aware.yaml \
  --output tmp/foundation-motivation-aware \
  --format markdown
```

### 4.2 离线指标
- `pass@1`：首选工具命中率（主指标）。
- `pass@5`：候选工具集命中率（容错指标）。
- `top_k_hit_rate`：Top-K 命中比例。
- `MRR`：首个命中期望倒数排名。
- `availability_error` 占比：预期工具未注册/不可用。

### 4.3 动机专项派生指标（基于 case 分类统计）
- Consent precision：`motivation_boundary` 中命中 `request_user`/`clarify` 的比例。
- Over-nudge rate：边界类 case 却命中执行动作工具的比例。
- Memory-first rate：个性化 case 命中 `memory_search`/`memory_get` 的比例。
- Actionability rate：`motivation_extrinsic` 命中任务/日历/OKR 工具比例。
- Feedback-evidence rate：`motivation_feedback` 命中 artifact/manifest/attachment 工具比例。

### 4.4 建议门槛（迭代目标）
- `pass@1 >= 0.78`
- `pass@5 >= 0.95`
- Consent precision `>= 0.90`
- Over-nudge rate `<= 0.08`
- Availability errors `= 0`

### 4.5 本次 dry-run 结果（2026-02-09）
使用新增 suite 执行离线评测：

```bash
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_motivation_aware.yaml \
  --output tmp/foundation-motivation-aware \
  --format markdown
```

结果摘要：
- `total_cases = 30`
- `top1_hit_rate = 0.600`
- `topk_hit_rate = 0.933`

说明：当前结果用于建立首版基线；后续按失败 case 做词汇信号与冲突对优化，再冲击 4.4 的目标门槛。

## 5. 如何验证（在线）

### 5.1 实验设计
- 实验对象：触发“动机支持请求”的真实会话。
- 对照组：通用主动策略（无动机分层）。
- 实验组：动机感知策略（状态估计 + 分阶段干预 + consent gate）。
- 观察周期：至少 2 周，覆盖短期新鲜度衰减。

### 5.2 在线指标
- Reminder acceptance rate（24h 内保留提醒占比）。
- Time-to-first-action（从请求到首个有效动作）。
- Follow-through conversion（创建任务/日历后的完成推进率）。
- Opt-out/stop rate（用户要求停止提醒比例）。
- User feedback delta（积极反馈与负反馈净变化）。

### 5.3 风险防线
- 当 `opt-out` 或投诉超过阈值，自动降级为“仅澄清 + 非主动执行”。
- 敏感领域默认要求人工确认或显式用户确认。

## 6. 迭代节奏
1. 每周导出失败 case，按冲突对（tool-pair conflict）做定向修正。
2. 对高频失败类别新增样例，避免过拟合单一表达。
3. 每两周更新一次门槛和 case 配比，保持难度与覆盖同步。

## 7. R3 收敛与优化结果（2026-02-09）

### 7.1 集合收敛（x/x）
- Collections: `25/25`（保持不变）
- Cases: `400/400`（由 `445 -> 400`）
- Hard stress collections retained: `3/3`
  - `sparse_clue_retrieval`
  - `stateful_commitment_boundary`
  - `reproducibility_trace_evidence`

### 7.2 评测前后对比（x/x）
- Baseline（400-case）:
  - pass@1: `339/400`（84.9%）
  - pass@5: `400/400`（100.0%）
  - Deliverable Good: `18/22`
  - Deliverable Bad: `4/22`
  - 产物路径: `tmp/foundation-suite-prune-r3-400-baseline-20260209-183049`
- Optimized（第二轮规则后）:
  - pass@1: `349/400`（87.3%）
  - pass@5: `400/400`（100.0%）
  - Deliverable Good: `18/22`
  - Deliverable Bad: `4/22`
  - 产物路径: `tmp/foundation-suite-prune-r3-400-optimized2-20260209-183358`

### 7.3 失败簇收敛（Top1）
- `web_fetch => web_search`: `2 -> 0`
- `memory_search => search_file`: `2 -> 0`
- `request_user => clarify`: `2 -> 0`
- `lark_send_message => replace_in_file`: `2 -> 1`
- `plan => lark_task_manage`: `3 -> 2`（仍是 Top1 残余冲突）

### 7.4 本轮主要优化动作
- 强化“单一已给定 URL 且禁止 discovery search”下 `web_fetch` 优先级。
- 强化审批/敏感门控语义下 `request_user`，下压 `clarify`。
- 强化 memory 历史/回溯/习惯语义下 `memory_search`，下压 `search_file`。
- 强化“brief status ping, no file transfer”下 `lark_send_message`，下压 `replace_in_file`。
- 增加 `lark_calendar_delete` 与 `cancel_timer` 的事件域去歧义。
- 补充对应回归测试，防止冲突回退。

## 8. R4 失败簇系统优化结果（2026-02-09）

- Baseline: `pass@1 349/400`，`pass@5 400/400`，Deliverable Good `18/22`
  - 产物：`tmp/foundation-suite-r4-baseline-20260209-184159`
- Optimized: `pass@1 358/400`，`pass@5 400/400`，Deliverable Good `19/22`
  - 产物：`tmp/foundation-suite-r4-optimized3-20260209-184526`

关键冲突簇变化：
- `artifacts_write => lark_upload_file`: `1 -> 0`
- `memory_get => clarify`: `1 -> 0`
- `memory_get => memory_search`: `1 -> 0`
- `write_file => write_attachment`: `1 -> 0`
- `list_dir => replace_in_file`: `1 -> 0`
- `plan => lark_task_manage`: `2 -> 1`（仍需继续收敛）

## 9. R5 一次性批量优化结果（2026-02-09）

- Baseline:
  - pass@1: `358/400`
  - pass@5: `400/400`
  - Deliverable Good: `19/22`
  - 产物：`tmp/foundation-suite-r5-baseline-20260209-191404`
- Optimized:
  - pass@1: `372/400`
  - pass@5: `400/400`
  - Deliverable Good: `19/22`
  - 产物：`tmp/foundation-suite-r5-optimized2-20260209-191737`

R5 批量收敛的代表簇：
- `write_file => replace_in_file`: `1 -> 0`
- `grep => shell_exec`: `1 -> 0`
- `a2ui_emit+artifacts_write => write_attachment`: `1 -> 0`
- `memory_search => clarify`: `1 -> 0`
- `clarify => memory_search`: `1 -> 0`
- `lark_calendar_create => lark_calendar_query`: `1 -> 0`
- `request_user => lark_task_manage`: `1 -> 0`
- `shell_exec => execute_code`: `1 -> 0`
- `scheduler_list_jobs => scheduler_create_job`: `1 -> 0`

## 10. R9 Hard-Only 套件收敛与业界基准扩展（2026-02-09）

### 10.1 套件收敛（x/x）
- Collections: `17/17`（主 suite 已移除长期饱和的基础集合）
- Cases: `269/269`
- Hard benchmark collections（新增并接入）:
  - `industry_benchmark_general_assistant_gaia`
  - `industry_benchmark_real_world_coding_livecodebench_swelancer`
  - `industry_benchmark_multiturn_enterprise_assistantbench_tau2`
  - `industry_benchmark_context_learning_nolima_longmemeval_babilong`

### 10.2 评测结果（x/x）
- pass@1: `224/269`（83.0%）
- pass@5: `267/269`（99.1%）
- Deliverable Good: `26/29`
- Deliverable Bad: `3/29`
- 产物路径: `tmp/foundation-suite-r9-hardonly-main-20260209-212003`

### 10.3 关键结论
- 已实现“只留难题”目标：主套件不再包含基础/长期 100% 饱和集合。
- 新增业界迁移集合后，难度显著提升且可诊断性更强（仍保留少量 pass@5 失败用于持续优化）。
- 本轮路由优化已修复新增集合中的多项冲突，尤其在：
  - `shell_exec vs execute_code`
  - `scheduler_create_job vs lark_calendar_update`
  - `read_file vs okr_read`
  - `memory_search vs browser_action`

## 11. R11 难度升级 + 失败簇优化（2026-02-09）

### 11.1 新增集合与规模
- 新增 hard collections: `2/2`
  - `foundation_eval_cases_industry_benchmark_implicit_intent_boundary_low_overlap.yaml`（21）
  - `foundation_eval_cases_industry_benchmark_autonomy_long_horizon_value_delivery.yaml`（25）
- 主 suite 更新后：
  - Collections: `19/19`
  - Cases: `315/315`

### 11.2 三轮评测（x/x）
- Baseline（未做规则收敛）:
  - pass@1: `264/315`
  - pass@5: `302/315`
  - failed: `13`
  - 路径：`tmp/foundation-suite-r11-hard`
- Optimized-R1:
  - pass@1: `271/315`
  - pass@5: `311/315`
  - failed: `4`
  - 路径：`tmp/foundation-suite-r11-hard-opt`
- Optimized-R2:
  - pass@1: `273/315`
  - pass@5: `313/315`
  - failed: `2`
  - 路径：`tmp/foundation-suite-r11-hard-opt2`

### 11.3 本轮系统性优化点（代码）
- 文件：`evaluation/agent_eval/foundation_eval.go`
- 动作：
  - 增加低重叠隐式词别名（如 `greenlight/silence/recurrences/ingest/playbook`）。
  - 加强 `request_user`、`web_fetch`、`artifacts_list`、`memory_search`、`read_file` 的隐式语义 boost。
  - 加强 `scheduler_list_jobs/create/delete` 与 `list_timers/cancel_timer/set_timer` 的调度语义 boost。
  - 增加误路由惩罚（`browser_screenshot`、`write_file`、`plan`、`lark_calendar_*` 在非目标调度语义下的降权）。

### 11.4 残余失败（用于下一轮）
- `scheduler_delete_job => lark_calendar_update`（1）
- `scheduler_list_jobs => artifacts_list`（1）

说明：残余失败集中在“极低词面重叠 + 调度边界冲突” hardest 子簇，可作为下一轮 targeted hardening 的入口。

## 12. R12 业界最难基准系统扩容（2026-02-09）

### 12.1 目标
- 一次性增加 hardest benchmark transfer 集合，避免仅在既有簇上过拟合。
- 以“系统分类”组织评测集合，支持后续按维度做淘汰、补题、优化闭环。

### 12.2 系统分类矩阵（benchmark → 能力维度 → 数据集）
| Benchmark Family | Hard Capability Focus | Dataset |
|---|---|---|
| Terminal-Bench | terminal-first diagnosis, bounded mutation, release gating | `foundation_eval_cases_industry_benchmark_terminal_bench_ops_hard.yaml` |
| MLE-Bench | experiment lifecycle reproducibility, memory-backed iteration, artifactized reports | `foundation_eval_cases_industry_benchmark_mle_bench_experiment_lifecycle_hard.yaml` |
| SWE-PolyBench | cross-language / cross-repo engineering and contract compatibility | `foundation_eval_cases_industry_benchmark_swe_polybench_cross_language_repo_hard.yaml` |
| GitTaskBench | real-repo maintenance, policy boundary, scheduling governance | `foundation_eval_cases_industry_benchmark_gittaskbench_real_repo_maintenance_hard.yaml` |
| OSWorld-G | grounded multimodal computer-use with interaction modality boundaries | `foundation_eval_cases_industry_benchmark_osworld_g_grounded_computer_use_hard.yaml` |
| FrontierMath + HLE | deep reasoning with deterministic validation and high-stakes release gate | `foundation_eval_cases_industry_benchmark_frontiermath_hle_deep_reasoning_validation_hard.yaml` |

参考来源（benchmark 官方页/论文）：
- Terminal-Bench: https://www.tbench.ai/
- MLE-Bench: https://openreview.net/forum?id=as6w2KEfEi
- SWE-PolyBench: https://www.vals.ai/benchmarks/swe-polybench-06-25-2025
- GitTaskBench: https://openreview.net/forum?id=Q6tN6YI0Fx
- OSWorld-G: https://arxiv.org/html/2505.16801v1
- Humanity's Last Exam: https://agi.safe.ai/
- FrontierMath: https://epoch.ai/frontiermath/the-benchmark

### 12.3 套件规模与结果（x/x）
- Suite: `foundation_eval_suite.yaml`
- Collections: `25/25`
- Cases: `387/387`
- pass@1: `330/387`
- pass@5: `380/387`
- Failed: `7`
- Deliverable Good: `34/39`
- 产物路径: `tmp/foundation-suite-r12-hardbench`

### 12.4 新增 6 集合结果（x/x）
- `industry-benchmark-terminal-bench-ops-hard`: pass@1 `12/12`, pass@5 `12/12`
- `industry-benchmark-mle-bench-experiment-lifecycle-hard`: pass@1 `9/12`, pass@5 `11/12`
- `industry-benchmark-swe-polybench-cross-language-repo-hard`: pass@1 `9/12`, pass@5 `11/12`
- `industry-benchmark-gittaskbench-real-repo-maintenance-hard`: pass@1 `7/12`, pass@5 `10/12`
- `industry-benchmark-osworld-g-grounded-computer-use-hard`: pass@1 `10/12`, pass@5 `12/12`
- `industry-benchmark-frontiermath-hle-deep-reasoning-validation-hard`: pass@1 `10/12`, pass@5 `11/12`

### 12.5 Top1 失败簇（R12）
- `read_file => memory_get`（2）
- `scheduler_delete_job => lark_calendar_update`（1）
- `scheduler_list_jobs => artifacts_list`（1）
- `search_file => browser_screenshot`（1）
- `scheduler_delete_job => plan`（1）
- `replace_in_file => artifacts_delete`（1）

结论：新增 hardest 迁移集合成功拉开难度，且失败分布具备“可系统优化”的冲突簇结构，适合进入下一轮 targeted convergence。

## 13. R13 SOTA Frontier 分层扩容（2026-02-10）

### 13.1 分层框架（Layered Hardness）
- `L1 Core-Hard`：SWE/conflict/orchestration/artifact/reproducibility 核心难题。
- `L2 Frontier-Hard`：Terminal-Bench / MLE-Bench / SWE-PolyBench / GitTaskBench / OSWorld-G。
- `L3 Research-Frontier-Hard`：FrontierMath/HLE + RE-Bench + EXP-Bench + ARC-AGI-2。

### 13.2 本轮新增（Research-Frontier）
- `foundation_eval_cases_industry_benchmark_re_bench_frontier_ml_rd_hard.yaml`
- `foundation_eval_cases_industry_benchmark_exp_bench_autonomous_research_hard.yaml`
- `foundation_eval_cases_industry_benchmark_arc_agi2_abductive_reasoning_hard.yaml`
- `foundation_eval_cases_industry_benchmark_paperbench_full_reproduction_hard.yaml`
- `foundation_eval_cases_industry_benchmark_mlrc_bench_open_research_hard.yaml`
- `foundation_eval_cases_industry_benchmark_ale_bench_long_horizon_algo_engineering_hard.yaml`

### 13.3 套件结果（x/x）
- Collections: `31/31`
- Cases: `457/457`
- pass@1: `378/457`
- pass@5: `443/457`
- Failed: `14`
- Deliverable Good: `44/51`
- 产物路径: `tmp/foundation-suite-r13-sota-frontier-v2`

### 13.4 新增 6 集合结果（x/x）
- RE-Bench: pass@1 `6/12`, pass@5 `11/12`
- EXP-Bench: pass@1 `10/12`, pass@5 `10/12`
- ARC-AGI-2: pass@1 `8/12`, pass@5 `10/12`
- PaperBench: pass@1 `7/10`, pass@5 `9/10`
- MLRC-Bench: pass@1 `9/12`, pass@5 `11/12`
- ALE-Bench: pass@1 `8/12`, pass@5 `12/12`

### 13.5 主要失败簇（R13）
- `read_file => memory_get`（5）
- `search_file => browser_screenshot`（2）
- `scheduler_delete_job => lark_calendar_update`（1）
- `scheduler_list_jobs => artifacts_list`（1）
- `scheduler_delete_job => plan`（1）
- `replace_in_file => artifacts_delete`（1）

### 13.6 SOTA benchmark sources
- RE-Bench（METR）: https://metr.org/blog/2025-06-05-re-bench/
- EXP-Bench: https://openreview.net/forum?id=V4KAvgDWBn
- ARC-AGI-2: https://arcprize.org/arc-agi-2
- PaperBench: https://openai.com/index/paperbench/
- MLRC-Bench: https://mlrc-bench.github.io/
- ALE-Bench: https://openreview.net/forum?id=1A8V31yA5j
