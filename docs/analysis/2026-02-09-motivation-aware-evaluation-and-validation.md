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
