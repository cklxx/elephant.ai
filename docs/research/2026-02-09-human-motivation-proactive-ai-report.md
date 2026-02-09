# 2026-02-09 人类动机来源与主动性 AI 应用报告

## 1. 目标与范围
本报告将“人类动机来源”转化为可落地的主动性 AI 产品框架，目标是指导 elephant.ai 在以下方面稳定提升：
- 在正确时机触发主动行为（不是过度打扰）。
- 在保持用户自主权的前提下提高执行率与复利行为。
- 将动机支持能力沉淀为可评测、可回归、可治理的工程资产。

范围包含三部分：
- 动机来源模型（心理学 + 行为科学 + 神经机制的工程化抽象）。
- 主动性 AI 应用框架（策略、流程、风险边界）。
- 产品化落地建议（映射到 elephant.ai 的模块与流程）。

## 2. 人类动机来源的工程化模型

### 2.1 基础生理与神经驱动
- 大脑奖励预测与努力分配机制决定“愿不愿意启动行为”。
- 长期压力、睡眠、疲劳会降低执行阈值，导致“知道该做但做不动”。

工程含义：系统应区分“不会做”与“做不动”，低能量状态下优先降低启动成本，而不是增加任务复杂度。

### 2.2 基本心理需要（Autonomy / Competence / Relatedness）
- `Autonomy`：被控制感会损伤持续动机。
- `Competence`：看见进步与可达成性会提升投入。
- `Relatedness`：社会支持与信任关系是稳定动机来源。

工程含义：主动性策略必须默认尊重用户选择权，并显式构造“可完成的下一步”与“被理解感”。

### 2.3 期望-价值评估（Expectancy-Value）
- 用户会同时评估：我能不能做到？这件事值不值得？
- 任何一侧太低，行为都容易中断。

工程含义：每次主动建议都应包含两类信息：
- 成功路径（降低“我做不到”）。
- 价值锚点（降低“做了没意义”）。

### 2.4 时间折扣、拖延与习惯回路
- 远期收益会被主观折价，短期不适更容易主导行为。
- 稳定线索触发的习惯行为可以绕开高认知负担。

工程含义：
- 强化短周期反馈（小时/天维度）。
- 用固定触发器（时间、情境、前序动作）建立自动执行链。

### 2.5 社会与身份因素
- 动机受身份叙事影响：用户更愿意执行“符合我是谁”的行为。
- 同伴承诺与外部可见进展可增强坚持概率。

工程含义：
- 允许用户定义“角色目标”（如 owner、maintainer、mentor）。
- 主动性输出要与用户长期身份叙事一致，而非短期操控。

### 2.6 统一状态抽象（用于系统实现）
建议在策略层维护一个轻量 `MotivationState`：

```yaml
motivation_state:
  stage: "pre_intent|intent|prepare|act|maintain"
  energy_level: "low|medium|high"
  autonomy_sensitivity: "low|medium|high"
  competence_signal: "low|medium|high"
  relatedness_signal: "low|medium|high"
  temporal_focus: "short_term|mixed|long_term"
  confidence_score: 0.0
  risk_flags:
    - "over_nudge"
    - "privacy_sensitive"
    - "consent_required"
```

## 3. 主动性 AI 的应用框架

### 3.1 核心原则
- 主动性不等于替用户决策；目标是降低执行摩擦。
- 默认“最小有效干预”：一次只推进一个最关键下一步。
- 先理解阶段，再触发动作，避免“过早行动”。

### 3.2 策略闭环（Sense -> Decide -> Act -> Learn）
1. `Sense`：读取上下文与近期行为信号（对话、任务状态、节奏偏好）。
2. `Decide`：估计 `MotivationState`，选择对应干预模板。
3. `Act`：执行低风险动作（澄清、计划、提醒、任务/日历写入、进展产物化）。
4. `Learn`：记录反馈（接受、忽略、取消、拒绝），更新下一次触发策略。

### 3.3 分阶段干预策略

| 阶段 | 目标 | 推荐动作 | 需要避免 |
|---|---|---|---|
| pre_intent | 降低心理阻力 | `clarify` + 价值澄清 | 直接下硬指令 |
| intent | 明确起点 | `plan`（最小闭环） | 一次性给过长计划 |
| prepare | 固化执行条件 | `set_timer` / `scheduler_create_job` / `lark_task_manage` | 只给口头建议不落地 |
| act | 保持推进 | `lark_send_message`、短反馈、障碍清理 | 高频催促 |
| maintain | 防复发与复盘 | `memory_search` + `artifacts_write` + 规律化计划 | 仅凭短期成功外推 |

### 3.4 安全与治理边界
- 明确禁止操控性策略（羞辱、恐惧放大、隐性诱导）。
- 涉及敏感偏好或第三方触达时，先 `request_user`。
- 每次主动触发都应可解释：为何现在、依据什么、可如何关闭。
- 保留撤销/退订能力，尊重用户“停止提醒”的明确指令。

## 4. 落地到 elephant.ai 的实现建议

### 4.1 触发与策略层
- 在 `internal/agent/` 的执行前策略阶段增加 motivation-aware policy 钩子。
- 复用现有 `memory_search` / `memory_get`，优先读取“节奏偏好、触发禁忌、有效策略”类记忆。

### 4.2 工具层动作优先级
- 低风险优先：`clarify` -> `plan` -> `set_timer` / `scheduler_create_job`。
- 执行型动作：`lark_task_manage` / `lark_calendar_create` / `okr_write`。
- 证明型动作：`artifacts_write` / `artifact_manifest` / `write_attachment`。

### 4.3 反馈闭环
将以下信号作为策略训练输入：
- reminder acceptance/cancel rate。
- proactive suggestion acceptance/rejection。
- time-to-first-action。
- follow-through conversion（计划 -> 实际动作）。

## 5. 90 天实施建议

### 阶段 A（第 1-3 周）
- 建立 `MotivationState` 标签与事件埋点。
- 接入最小策略：先做 `clarify/plan`，限制主动动作频率。

### 阶段 B（第 4-8 周）
- 上线节奏控制：提醒/日程/任务联动。
- 引入记忆驱动个性化（偏好节奏、禁忌、有效动作）。

### 阶段 C（第 9-12 周）
- 执行离线评测 + 小流量在线实验。
- 根据“过度干预率”和“持续执行率”双目标迭代。

## 6. 参考来源（用于报告追溯）
- Ryan & Deci (Self-Determination Theory): https://pubmed.ncbi.nlm.nih.gov/11392867/
- Wigfield & Eccles (Expectancy-Value): https://pubmed.ncbi.nlm.nih.gov/10620382/
- Baumeister & Leary (Belongingness): https://pubmed.ncbi.nlm.nih.gov/7777651/
- Wood & Neal (Habit): https://pubmed.ncbi.nlm.nih.gov/17907866/
- Temporal Discounting & Procrastination: https://pubmed.ncbi.nlm.nih.gov/38918442/
- NIST AI RMF 1.0: https://www.nist.gov/publications/artificial-intelligence-risk-management-framework-ai-rmf-10
- EU AI Act (Regulation 2024/1689): https://eur-lex.europa.eu/eli/reg/2024/1689/oj

## 7. 最新评测结论（R3，2026-02-09）

### 7.1 套件规模（x/x）
- Collections: `25/25`
- Cases: `400/400`（已从 `445` 收敛）
- Hard stress dimensions: `3/3` 保留

### 7.2 关键结果（x/x）
- 基线（400-case）pass@1: `339/400`，pass@5: `400/400`
- 二轮优化后 pass@1: `349/400`，pass@5: `400/400`
- Deliverable 检查: Good `18/22`，Bad `4/22`（本轮未变化）

### 7.3 主要发现
- 动机/记忆/审批门控相关冲突显著下降（`request_user`、`memory_search`、`web_fetch` 相关簇已清零）。
- 剩余主失败簇集中在计划语义与任务管理语义重叠（`plan => lark_task_manage`）。
- 复杂可交付任务中，仍存在“文件传输 vs 消息通知”的局部歧义，需要继续增强语义分离。

## 8. R9 Hard-Only 评测结论（2026-02-09）

### 8.1 套件规模（x/x）
- Collections: `17/17`（主 suite 已收敛为 hard-only）
- Cases: `269/269`
- Deliverable cases: `29/269`

### 8.2 关键结果（x/x）
- pass@1: `224/269`（83.0%）
- pass@5: `267/269`（99.1%）
- Deliverable Good: `26/29`
- Deliverable Bad: `3/29`
- 评测产物: `tmp/foundation-suite-r9-hardonly-main-20260209-212003`

### 8.3 结论
- 主评测集合已从“高分但挑战不足”切换到“难题驱动”的健康状态。
- 新增业界 benchmark 迁移题显著拉开难度，能够更稳定暴露真实路由短板。
- 当前剩余失败主要集中在 hardest 子集（多轮企业任务与长上下文推理），适合作为下一轮系统优化入口。

## 9. R11 难度升级与系统优化结论（2026-02-09）

### 9.1 本轮新增高难集合（x/x）
- Collections added: `2/2`
  - `industry_benchmark_implicit_intent_boundary_low_overlap`
  - `industry_benchmark_autonomy_long_horizon_value_delivery`
- New hard cases added: `46/46`（`21 + 25`）
- 主 suite 规模：
  - Collections: `19/19`
  - Cases: `315/315`

### 9.2 三轮结果对比（x/x）
- Baseline（加入新难题，未优化）:
  - pass@1: `264/315`（83.8%）
  - pass@5: `302/315`（95.9%）
  - Failed: `13`
  - 产物：`tmp/foundation-suite-r11-hard`
- Optimized-R1（失败簇定向规则）:
  - pass@1: `271/315`（86.0%）
  - pass@5: `311/315`（98.7%）
  - Failed: `4`
  - 产物：`tmp/foundation-suite-r11-hard-opt`
- Optimized-R2（调度冲突继续收敛）:
  - pass@1: `273/315`（86.7%）
  - pass@5: `313/315`（99.4%）
  - Failed: `2`
  - 产物：`tmp/foundation-suite-r11-hard-opt2`

### 9.3 Top1 失败簇演进
- Baseline Top1 cluster:
  - `read_file => memory_get`（2）
  - `web_fetch => browser_screenshot`（2）
  - `scheduler_* => 非目标调度工具`（多条）
- Optimized-R2 残余：
  - `scheduler_delete_job => lark_calendar_update`（1）
  - `scheduler_list_jobs => artifacts_list`（1）

### 9.4 产品能力提升结论
- 本轮不仅“加难题”，还完成了失败簇驱动的真实路由能力提升：
  - 隐式审批门控（`request_user`）召回增强。
  - 单链接摄取（`web_fetch`）与视觉截图（`browser_screenshot`）边界拉开。
  - 低词面重叠下的调度语义（`scheduler_list_jobs/create/delete`）显著提升。
  - 记忆回溯场景（`memory_search`）命中增强。
- 仍保留少量高难失败，避免集合再次“过饱和”。

## 10. R12 业界 hardest 批量扩容结论（2026-02-09）

### 10.1 扩容内容（x/x）
- 新增 benchmark-transfer collections: `6/6`
  - Terminal-Bench
  - MLE-Bench
  - SWE-PolyBench
  - GitTaskBench
  - OSWorld-G
  - FrontierMath + Humanity's Last Exam
- 新增 hard cases: `72/72`
- 主 suite 扩容后：`25` collections，`387` cases

### 10.2 本轮分数（x/x）
- pass@1: `330/387`
- pass@5: `380/387`
- failed: `7`
- deliverable good: `34/39`
- 产物：`tmp/foundation-suite-r12-hardbench`

### 10.3 结果解释
- 难度结构明显增强：新增 6 集合贡献了 `5` 个失败 case，且集中在“跨域调度冲突 + 低词面重叠” hardest 区域。
- 新集合未出现工具不可用（N/A）堆积，说明扩容与当前工具生态兼容。
- 失败簇呈系统性分布，适合下一轮按 conflict family 做精准收敛，而非盲目继续加题。

## 11. R13 SOTA Frontier 分层结论（2026-02-10）

### 11.1 分层目标
- 将“业界 hardest”按层级固化，避免只在单一难题簇上过拟合：
  - Core-Hard
  - Frontier-Hard
  - Research-Frontier-Hard

### 11.2 本轮扩容与分数（x/x）
- 新增 Research-Frontier collections: `6/6`
  - RE-Bench（frontier ML R&D）
  - EXP-Bench（autonomous research）
  - ARC-AGI-2（abductive reasoning）
  - PaperBench（end-to-end paper reproduction）
  - MLRC-Bench（open ML research competition）
  - ALE-Bench（long-horizon algorithm engineering）
- 扩容后主 suite：
  - Collections: `31/31`
  - Cases: `457/457`
  - pass@1: `378/457`
  - pass@5: `443/457`
  - failed: `14`
  - 产物：`tmp/foundation-suite-r13-sota-frontier-v2`

### 11.3 结论
- 难度进一步拉高且失败簇更集中，满足“连 SOTA 都困难”的目标方向。
- 当前最值得优先收敛的簇仍是：
  - `read_file => memory_get`
  - `search_file => browser_screenshot`
  - 调度边界簇（`scheduler_*` 与 `calendar/plan/artifacts` 竞争）

## 12. R14 评测集去冗余（淘汰 200 个通过 Case，2026-02-10）

### 12.1 调整目标
- 解决“题目只增不减”导致的信息密度下降问题。
- 一次性移除 `200` 个当前已通过 case，保持失败簇稳定，提升每轮评测有效信号密度。

### 12.2 结果（x/x）
- 调整前：`457/457` cases，pass@5 `443/457`，failed `14`
- 调整后：`257/257` cases，pass@5 `243/257`，failed `14`
- 移除清单：`evaluation/agent_eval/datasets/foundation_eval_prune_manifest_r14.yaml`

### 12.3 结论
- 本轮去冗余未削弱 hardest 信号，失败簇保持一致。
- 评测集合从“规模偏大”收敛为“密度更高”，便于下一轮系统性优化直接对准失败簇。
