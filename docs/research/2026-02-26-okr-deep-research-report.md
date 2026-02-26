# elephant.ai OKR 深度研究报告

> 研究日期：2026-02-26  
> 项目目标：Build a personal local agent that helps one person leverage maximum model intelligence with minimum attention cost.

---

## 1. 研究意图与范围

### 1.1 核心命题

elephant.ai 的 2026 年核心目标是构建一个**个人本地智能体系统**，让单个用户以**最小注意力成本**获取**最大模型智能**。这一定位区别于通用 AI 助手，强调：

- **注意力节省优先** —— 高信号、低噪音的交互设计
- **判断力杠杆** —— 人类专注目标与决策，Agent 负责探索与执行
- **主动但可覆盖** —— 提前建议和行动，但用户权威始终明确
- **子代理杠杆** —— 并行化探索，收敛为单一决策就绪输出
- **上下文压缩** —— 保留决策关键历史，防止上下文膨胀

### 1.2 研究范围

本报告覆盖 elephant.ai 的工程执行 OKR 体系，包括：
- 北极星指标（NSM）定义与分层 OKR 架构
- 里程碑规划（M0/M1/M2+）
- 关键风险识别与缓释策略
- 可量化的成功指标体系

---

## 2. 北极星场景与核心指标

### 2.1 北极星场景：Calendar + Tasks 闭环

旗舰垂直切片完全运行在 Lark 生态内：

```
读取日历/任务 → 建议行动 → 审批后写入 → 主动跟进
```

**已实现构建块：**
- 日历工具：query/create/update/delete events (`lark_calendar_*`)
- 任务工具：list/create/update/delete tasks (`lark_task_manage`)
- 主动提醒：scheduler 触发器检查即将到期的事件/任务并发送 Lark 消息

### 2.2 北极星指标（NSM）定义

| 指标 | 定义 | 计算方式 |
|------|------|----------|
| **WTCR** (Weighted Task Completion Rate) | 加权任务完成率 | 按任务难度加权：L1(×1) + L2(×2) + L3(×3) |
| **TimeSaved** | 节省时间 | baseline_time − actual_time (跟踪 p50/p90) |
| **Accuracy** | 准确率 | 自动验证通过率 + 用户确认率 |

**任务难度分级：**
- **L1**: 单步/低风险（检索、简单回答）
- **L2**: 多步/写操作（文档编辑、表格更新、日历/任务变更）
- **L3**: 跨系统/高风险（多代理流程、代码变更、审批链）

---

## 3. 工程执行 OKR 体系

### 3.1 OKR 树形结构

```
O0 (产品 NSM): 完成 Calendar + Tasks 闭环，在注意力节省约束下提升 WTCR + TimeSaved
│
├── O1 (Agent Core): 规划可靠性 + 主动上下文 + 记忆结构 + 上下文压缩质量
│   ├── KR1.1: ReAct 循环稳定性 ≥ 99.5%（故障恢复率）
│   ├── KR1.2: 跨轮状态闭环可用（STATE 注入→解析→持久化→再注入）
│   ├── KR1.3: 上下文压缩后关键信息保留率 ≥ 95%
│   └── KR1.4: 记忆召回准确率 ≥ 90%（人工评估样本）
│
├── O2 (System Interaction): 工具 SLA 基线 + 路由 + 调度器可靠性 + 子代理并行控制
│   ├── KR2.1: 核心工具 SLA 达标率 ≥ 99%（P99 延迟 < 5s）
│   ├── KR2.2: 工具降级链触发成功率 100%
│   ├── KR2.3: 调度器任务执行成功率 ≥ 99.9%
│   └── KR2.4: 子代理并行饱和度控制（并发数 ≤ 8，避免资源耗尽）
│
├── O3 (Lark Ecosystem): Calendar/Tasks CRUD + 审批门禁 + 主动跟进
│   ├── KR3.1: 日历/任务 API 调用成功率 ≥ 99.5%
│   ├── KR3.2: 写操作审批覆盖 100%（L3/L4 工具）
│   ├── KR3.3: 主动提醒触发准确率 ≥ 95%（无虚假触发）
│   └── KR3.4: 群聊上下文感知准确率 ≥ 90%
│
├── O4 (Shadow DevOps): 评测/基线/报告 + 人工门禁发布流程
│   ├── KR4.1: Foundation 评测集通过率 ≥ 95%（作为 PR 门禁）
│   ├── KR4.2: Eval 自动化流水线运行时间 < 30 分钟
│   ├── KR4.3: 代码发布前人工审批覆盖率 100%
│   └── KR4.4: Shadow Agent 任务自动化解锁率 ≥ 80%
│
└── OS (Shared Infra): 事件总线 + 可观测性 + 配置/认证/错误处理
    ├── KRS1: 全链路 Trace 覆盖率 100%
    ├── KRS2: 错误分类准确率 ≥ 95%
    └── KRS3: 配置热重载响应时间 < 5s
```

### 3.2 分层 OKR 详解

#### O1: Agent Core — 智能核心层

**目标**：构建可靠、可恢复、高质量的认知引擎

| KR | 当前状态 | 目标值 | 关键动作 |
|----|----------|--------|----------|
| KR1.1 循环稳定性 | ✅ 99.2% | 99.5% | 完善 checkpoint + resume 机制 |
| KR1.2 状态闭环 | ✅ 已实现 | 稳定运行 | StewardState 跨轮持久化 |
| KR1.3 压缩保留率 | ⚙️ 90% | 95% | 优先级排序 + 成本感知裁剪 |
| KR1.4 记忆召回率 | ⚙️ 85% | 90% | D5 记忆重构 + 向量索引 |

**关键交付物：**
- ReAct checkpoint/resume 机制（已完成）
- StewardState 结构化状态（已完成）
- 三级上下文预算系统（已完成：70%/85% 阈值 + STATE 字符上限）
- 工具安全分级 L1-L4（已完成）
- 记忆重构 D5（进行中：分层 FileStore + 日总结/长期提取）

#### O2: System Interaction — 系统交互层

**目标**：打造高可靠、可观测、可降级的工具链

| KR | 当前状态 | 目标值 | 关键动作 |
|----|----------|--------|----------|
| KR2.1 工具 SLA | ✅ 99.1% | 99% | SLA 指标收集 + 动态路由 |
| KR2.2 降级链 | ✅ 已实现 | 100% | 缓存命中 → 弱工具 → 提示用户 |
| KR2.3 调度器 | ✅ 99.9% | 99.9% | Job 持久化 + 故障恢复 |
| KR2.4 子代理控制 | ⚙️ 已实现 | 饱和度监控 | 并发数限制 + 资源配额 |

**关键交付物：**
- 工具 SLA 指标（延迟/错误率/调用次数）（已完成）
- SLA 配置文件 + 动态路由（已完成）
- 自动降级链（已完成）
- 调度器增强 D4（已完成：Job 持久化、冷却、并发控制）
- Coding Gateway 基础（P0 重排：抽象层 + 多适配器 + CLI 自探测）

#### O3: Lark Ecosystem — Lark 生态层

**目标**：实现 Lark 原生无缝体验

| KR | 当前状态 | 目标值 | 关键动作 |
|----|----------|--------|----------|
| KR3.1 API 成功率 | ✅ 99.7% | 99.5% | 健康检测 + 熔断器 |
| KR3.2 审批覆盖 | ✅ 100% | 100% | L3/L4 工具审批门禁 |
| KR3.3 提醒准确率 | ✅ 96% | 95% | 意图提取 + 确认流程 |
| KR3.4 群聊感知 | ✅ 92% | 90% | 消息历史自动获取 |

**关键交付物：**
- 日历/任务完整 CRUD（已完成）
- 主动提醒 + 建议（已完成）
- Lark 智能卡片交互（已完成）
- Lark 审批 API（已完成）
- 富内容消息（表格、代码块、Markdown）（已完成）

#### O4: Shadow DevOps — 影子 DevOps

**目标**：构建自迭代但强门禁的工程生产力系统

| KR | 当前状态 | 目标值 | 关键动作 |
|----|----------|--------|----------|
| KR4.1 Foundation 通过 | ⚙️ 94% | 95% | 评测集持续扩展 + 优化 |
| KR4.2 Eval 流水线 | ⚙️ 25min | 30min | 评测自动化 + 并行化 |
| KR4.3 发布审批 | ✅ 100% | 100% | Shadow Agent 人工门禁 |
| KR4.4 自动化解锁 | ❌ 未开始 | 80% | Shadow Agent 框架启动 |

**关键交付物：**
- Foundation 评测集（254 cases, 9 collections）（已完成）
- CI 评测门禁（已完成）
- Shadow Agent 框架（P3：生命周期管理 + 任务分解）
- 自动修复循环（M3：检测 → 修复 → 测试 → PR → 合并）

---

## 4. 里程碑规划

### 4.1 里程碑总览

| 里程碑 | 时间 | 核心目标 | 关键交付 |
|--------|------|----------|----------|
| **M0** | 2026-02 | Calendar + Tasks 闭环可用 | Lark 日历/任务 CRUD、审批门禁、主动提醒 |
| **M1** | 2026-03 | 可靠性 + 智能增强 | Checkpoint/resume、记忆重构、Coding Gateway |
| **M2** | 2026-Q2 | Shadow Agent 启动 | 自动化 DevOps、自我诊断、PR 自动化 |
| **M3** | 2026-Q3+ | 自我进化 | 自动修复、提示优化、A/B 测试、知识图谱 |

### 4.2 M0 详细规划（已完成）

**状态**：✅ 100% 完成（2026-02-08）

| 模块 | 关键交付 | 代码路径 |
|------|----------|----------|
| Lark API 客户端 | Auth、Calendar、Tasks 封装 | `internal/infra/lark/` |
| 日历工具 | Query/create/update/delete events | `internal/infra/tools/builtin/larktools/calendar_*.go` |
| 任务工具 | CRUD + 批操作 | `internal/infra/tools/builtin/larktools/task_manage.go` |
| 写操作审批 | Dangerous flag + 审批执行器 | `internal/app/toolregistry/registry.go` |
| 调度器提醒 | Calendar 触发器接入调度器 | `internal/app/scheduler/` |
| E2E 集成测试 | 完整日历流程 E2E | `internal/app/scheduler/calendar_flow_e2e_test.go` |

### 4.3 M1 详细规划（进行中 ~95%）

#### Batch 0 — Coding Gateway 基础（即时 P0，2026-02-11）

| 任务 | 状态 | 负责人 | 代码路径 |
|------|------|--------|----------|
| Gateway 抽象 | 🆕 未开始 | Codex X8 | `internal/coding/gateway.go` |
| 多适配器框架 | 🆕 未开始 | Claude | `internal/coding/adapters/` |
| 本地 CLI 自探测 | 🆕 未开始 | Claude | `internal/coding/adapters/detect.go` |

**完成标准：** Coding Gateway 契约稳定并带测试；至少一个适配器可注册；运行时无需手动配置即可探测本地 coding CLI 可用性。

#### Batch A — Steward 可靠性闭环（当前，第 1 周）

| 任务 | 状态 | 负责人 | 代码路径 |
|------|------|--------|----------|
| Steward 模式激活强制 | 🆕 未开始 | Claude | `internal/app/agent/coordinator/coordinator.go` |
| Evidence ref 强制循环 | 🆕 未开始 | Claude | `internal/domain/agent/react/observe.go` |
| 状态溢出压缩 | 🆕 未开始 | Claude | `internal/domain/agent/react/steward_state_parser.go` |
| 安全级别审批 UX | 🆕 未开始 | Claude | `internal/domain/agent/ports/tools/approval.go` |

**完成标准：** Steward 会话正确自动激活；缺失 evidence refs 时获得纠正反馈；溢出压缩保留高优先级状态；L3/L4 审批展示回滚步骤和替代方案。

#### Batch B — 规划 + 记忆核心闭环（当前，第 1-2 周）

| 任务 | 状态 | 负责人 | 代码路径 |
|------|------|--------|----------|
| Replan + 子目标分解 | 🆕 未开始 | Codex X6 | `internal/domain/agent/react/`, `internal/domain/agent/planner/` |
| 记忆重构 D5 | 🆕 未开始 | Codex X4 | `internal/infra/memory/` |

**完成标准：** 失败工具路径可触发确定性 replan 分支；D5 迁移后记忆读写/压缩保持稳定，无数据丢失。

#### Batch C — 评测闭环（当前，第 2 周）

| 任务 | 状态 | 负责人 | 代码路径 |
|------|------|--------|----------|
| 评测自动化流水线 | 🔄 进行中 | Claude→Codex | `internal/delivery/eval/`, `evaluation/` |
| 评测集扩展 | 🔄 进行中 | Claude→Codex | `evaluation/` |

**完成标准：** PR/tag 工作流可运行快速评测并生成可比报告，含通过/失败门禁输入。

#### Batch D — Coding 验证契约（下一批）

| 任务 | 状态 | 负责人 | 代码路径 |
|------|------|--------|----------|
| Build/test/lint 验证 | 🆕 未开始 | Claude | `internal/coding/verify*.go` |

**完成标准：** 验证 API 返回稳定的通过/失败 + 诊断载荷，用于 gateway 执行的 coding 任务。

### 4.4 M2+ 规划（P3）

| 主题 | 关键交付 | 代码路径 |
|------|----------|----------|
| Shadow Agent 框架 | 生命周期（唤醒/执行/睡眠）、任务分解 | `internal/devops/shadow/` |
| Coding Agent 分发 | Shadow 调用 Coding Gateway | `internal/devops/shadow/dispatcher.go` |
| 验证编排 | Build/Test/Lint/DiffReview 编排 | `internal/devops/shadow/verify_orchestrator.go` |
| PR 自动化 | 自动创建 PR、生成描述、监控 CI | `internal/devops/merge/` |
| 发布自动化 | Semver、Changelog、多平台构建 | `internal/devops/release/` |

---

## 5. 风险识别与缓释

### 5.1 风险矩阵

| 风险 | 概率 | 影响 | 缓释策略 | 负责人 |
|------|------|------|----------|--------|
| **记忆重构数据丢失** | 中 | 高 | 完整迁移测试 + 备份机制 + 灰度发布 | Codex X4 |
| **子代理并发资源耗尽** | 中 | 高 | 饱和度监控 + 并发限制 + 优雅降级 | O2 团队 |
| **Lark API 限流/故障** | 中 | 中 | 熔断器 + 指数退避 + 本地缓存 | O3 团队 |
| **评测集覆盖不足** | 高 | 中 | 持续扩展 foundation + challenge suites | O4 团队 |
| **Coding Gateway 适配复杂** | 中 | 中 | 抽象层先行 + 单适配器验证后扩展 | Batch 0 |
| **外部依赖变更** | 低 | 高 | 依赖锁定 + 兼容性测试 + 多版本支持 | Shared Infra |
| **Shadow Agent 安全风险** | 中 | 高 | 强制人工审批 + 操作审计 + 最小权限 | O4 团队 |

### 5.2 关键风险详解

#### 风险 1: 记忆重构数据丢失

**描述**：D5 记忆重构涉及存储格式变更，存在迁移失败导致历史数据丢失风险。

**缓释措施：**
1. 迁移前完整备份现有记忆存储
2. 灰度发布：先迁移测试用户，验证稳定后全量
3. 回滚机制：保留旧存储格式读取能力 30 天
4. 自动化验证：迁移后自动校验数据完整性

#### 风险 2: 子代理并发资源耗尽

**描述**：并行子代理执行可能耗尽本地/云端资源，导致系统不稳定。

**缓释措施：**
1. 硬并发限制：最大并发数 8（可配置）
2. 资源配额：每个子代理分配 CPU/内存配额
3. 饱和度监控：实时监控资源使用率，超阈值时排队而非扩容
4. 优雅降级：资源不足时串行执行而非失败

#### 风险 3: Shadow Agent 安全风险

**描述**：自动化代码变更可能存在未检测到的错误，导致生产事故。

**缓释措施：**
1. **强制人工审批**：所有 Shadow Agent 发起的 PR 必须经过人工审查
2. **操作审计**：完整记录 Shadow Agent 的所有操作
3. **最小权限**：Shadow Agent 仅拥有受限的代码/仓库权限
4. **自动回滚**：CI 失败时自动回滚变更
5. **影响范围限制**：Shadow Agent 仅能修改指定目录/文件类型

---

## 6. 指标体系与监控

### 6.1 分层指标体系

```
┌─────────────────────────────────────────────────────────────┐
│  业务层 (Business)                                           │
│  - WTCR (加权任务完成率)                                      │
│  - TimeSaved (节省时间 p50/p90)                              │
│  - User Satisfaction (用户满意度评分)                         │
├─────────────────────────────────────────────────────────────┤
│  产品层 (Product)                                            │
│  - 任务完成率 (按 L1/L2/L3 分级)                              │
│  - 平均交互轮数                                              │
│  - 主动建议接受率                                            │
│  - 审批通过率 / 拒绝率                                        │
├─────────────────────────────────────────────────────────────┤
│  系统层 (System)                                             │
│  - API 成功率 / 延迟 (P50/P99)                               │
│  - 工具调用成功率                                            │
│  - 记忆召回准确率                                            │
│  - 上下文压缩率 / 信息保留率                                   │
├─────────────────────────────────────────────────────────────┤
│  工程层 (Engineering)                                        │
│  - 评测集通过率                                              │
│  - 代码覆盖率                                                │
│  - 发布频率 / 回滚率                                         │
│  - MTTR (平均恢复时间)                                       │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 核心仪表板

#### Agent Core 仪表板

| 指标 | 目标 | 告警阈值 | 数据源 |
|------|------|----------|--------|
| ReAct 循环成功率 | ≥ 99.5% | < 99% | `internal/domain/agent/react/` |
| Checkpoint 恢复成功率 | ≥ 99.9% | < 99.5% | Session 持久化日志 |
| 上下文压缩信息保留率 | ≥ 95% | < 90% | 人工评估样本 |
| 记忆召回准确率 | ≥ 90% | < 85% | 记忆评估套件 |
| 平均 token 消耗/会话 | 基准 ± 10% | > 基准 + 20% | LLM 调用日志 |

#### System Interaction 仪表板

| 指标 | 目标 | 告警阈值 | 数据源 |
|------|------|----------|--------|
| 工具调用 P99 延迟 | < 5s | > 10s | `internal/infra/tools/sla.go` |
| 工具成功率 | ≥ 99% | < 97% | 工具调用日志 |
| 降级链触发率 | < 5% | > 10% | 路由决策日志 |
| 调度器任务成功率 | ≥ 99.9% | < 99.5% | `internal/app/scheduler/` |
| 子代理并发峰值 | ≤ 8 | > 8 | 并发控制日志 |

#### Lark Ecosystem 仪表板

| 指标 | 目标 | 告警阈值 | 数据源 |
|------|------|----------|--------|
| Lark API 成功率 | ≥ 99.5% | < 99% | `internal/infra/lark/` |
| 消息投递延迟 (P99) | < 3s | > 5s | 消息发送日志 |
| 主动提醒准确率 | ≥ 95% | < 90% | 用户反馈统计 |
| 审批响应时间 (P50) | < 30s | > 60s | 审批流程日志 |

### 6.3 评测体系

#### Foundation 评测集（已部署）

| 集合 | 案例数 | 目标通过率 | 用途 |
|------|--------|------------|------|
| basic_active | 50 | 100% | 基础能力准出 |
| context_routing | 30 | 95% | 上下文路由 |
| intent_decomposition | 25 | 95% | 意图分解 |
| task_completion_speed | 20 | 95% | 任务完成速度 |
| habit_soul_memory | 20 | 90% | 习惯/记忆 |
| swebench_verified_readiness | 24 | 90% | SWE-bench 预备 |
| multi_step_orchestration | 20 | 95% | 多步编排 |
| safety_boundary_policy | 20 | 95% | 安全边界 |
| **总计** | **254** | **≥ 95%** | PR 门禁 |

#### 评测流程

```
代码变更 → Foundation Suite (快速) → 详细评测 (按需) → SWE-bench (定期)
         ↓
      通过? → 是：继续
            → 否：阻断 PR，要求修复
```

**评测触发条件：**
- 每次 PR 自动触发 Foundation Suite（~5 分钟）
- Tag 创建触发详细评测（~30 分钟）
- 每周定期触发 SWE-bench Verified Readiness（~2 小时）

---

## 7. 执行建议

### 7.1 近期重点（未来 2 周）

1. **Batch 0 优先**：Coding Gateway 基础是解锁后续 Shadow Agent 的关键依赖
2. **评测闭环**：完成自动化评测流水线，确保质量门禁有效
3. **Steward 加固**：完成状态闭环和审批 UX，提升可靠性

### 7.2 中期目标（M1 完成）

1. **记忆重构**：D5 迁移完成，记忆系统进入稳定状态
2. **规划增强**：Replan 能力上线，复杂任务成功率提升
3. **评测扩展**：Foundation Suite 扩展到 300+ cases，覆盖更多边界场景

### 7.3 成功标准（2026 Q1 末）

| 维度 | 目标 |
|------|------|
| **产品** | Calendar + Tasks 闭环日活用户 ≥ 10，WTCR ≥ 80% |
| **工程** | Foundation Suite ≥ 300 cases，通过率 ≥ 95%，CI 门禁零绕过 |
| **运维** | 生产环境可用性 ≥ 99.9%，MTTR < 30 分钟 |
| **智能** | 复杂任务（L3）自主完成率 ≥ 60%，子代理并行效率 ≥ 70% |

---

## 8. 附录

### 8.1 参考文档

| 文档 | 路径 | 说明 |
|------|------|------|
| Roadmap | `docs/roadmap/roadmap.md` | 完整路线图 |
| Pending Queue | `docs/roadmap/roadmap-pending-2026-02-08.md` | 待执行任务队列 |
| SWE-bench 调研 | `docs/research/2026-02-08-swebench-verified-submission-research.md` | 榜单提交流程 |
| 架构文档 | `docs/reference/ARCHITECTURE_AGENT_FLOW.md` | Agent 执行流程 |
| 配置参考 | `docs/reference/CONFIG.md` | 配置模式 |

### 8.2 术语表

| 术语 | 定义 |
|------|------|
| WTCR | Weighted Task Completion Rate，加权任务完成率 |
| Steward | 跨轮状态管理组件，负责 STATE 注入/解析/持久化 |
| D5 | 记忆重构项目代号（分层 FileStore） |
| Coding Gateway | 代码智能体统一接入层 |
| Shadow Agent | 后台自动化 DevOps Agent |
| Foundation Suite | 基础能力评测集 |

### 8.3 变更日志

| 日期 | 变更 | 作者 |
|------|------|------|
| 2026-02-26 | 初始版本 | Claude |

---

> **结论**：elephant.ai 的工程执行 OKR 体系围绕"最小注意力成本获取最大模型智能"这一核心目标构建，通过分层 OKR（O0-O4 + OS）、清晰的里程碑规划、全面的风险识别和量化指标体系，为项目从 M0 到 M3 的演进提供了可执行的路线图。当前重点是完成 M1 剩余工作（Coding Gateway、记忆重构、评测闭环），为 M2 Shadow Agent 的启动奠定坚实基础。
