# Track 4: 自主迭代升级 — 影子 Agent DevOps — 详细 ROADMAP

> **Parent:** `docs/roadmap/roadmap-lark-native-proactive-assistant.md`
> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-01

---

## 概述

**核心理念：用当前的 Agent 来监督 Coding Agent 写代码、合代码、测试、发版本。**

这不是传统 CI/CD。这是一个 **Agent-in-the-loop 的软件交付系统**，由三个 Agent 角色协作完成系统的持续进化。Shadow Agent 是这条 Track 的主角 — 它是 elephant.ai 实现自我进化的关键基础设施。

**关键路径：** `internal/devops/` (新增) · `internal/coding/` (Track 2 共享) · `evaluation/`

---

## 三 Agent 架构详解

### 角色定义

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Online Agent (在线助手)                        │
│                                                                     │
│  角色：服务用户请求的稳定版本                                           │
│  生命周期：7×24 常驻运行                                               │
│  职责：                                                              │
│    · 接收并执行用户请求（Lark/Web/CLI）                                 │
│    · 收集隐式信号（失败轨迹、重试、放弃、超时）                           │
│    · 收集显式反馈（满意度评分、纠错）                                    │
│    · 定期导出信号给 Shadow Agent                                      │
│  输出：反馈信号流、失败轨迹日志、使用模式统计                             │
├─────────────────────────────────────────────────────────────────────┤
│                        Shadow Agent (影子 Agent)                     │
│                                                                     │
│  角色：系统的 "开发者"，驱动代码迭代                                     │
│  生命周期：按需唤醒（手动触发或定时 cron）                                │
│  职责：                                                              │
│    · 消费 Online Agent 的反馈信号                                     │
│    · 分析 → 识别改进方向 → 生成 Issue/需求                              │
│    · 拆解需求为编码子任务                                               │
│    · 调度 Coding Agent (Codex/Claude Code/Kimi) 执行编码               │
│    · 验收：build → test → lint → diff review                        │
│    · 代码合并：PR → CI → 修复 → merge                                │
│    · 版本发布：semver → changelog → build → deploy                   │
│    · 部署新版本到 Test Agent 环境                                      │
│    · 对比评测结果，决策是否晋升                                         │
│  输出：PR、Release、评测报告、晋升决策                                  │
├─────────────────────────────────────────────────────────────────────┤
│                        Test Agent (被测 Agent)                       │
│                                                                     │
│  角色：Shadow Agent 产出的候选版本                                     │
│  生命周期：Shadow Agent 触发后临时运行                                  │
│  职责：                                                              │
│    · 在隔离环境中运行 Shadow Agent 构建的新版本                          │
│    · 执行完整评测套件（SWE-Bench / Agent Eval / 自定义场景）             │
│    · 多维度打分：推理质量、工具可靠性、交互体验、成本效率                   │
│    · 可选：接收小流量真实请求（灰度验证）                                 │
│    · 与 Online Agent 基线对比，生成评测报告                              │
│  输出：评测报告（通过/未通过 + 分维度得分 + 基线对比）                    │
└─────────────────────────────────────────────────────────────────────┘
```

### 完整迭代循环

```
  ┌─── Online Agent (生产环境) ◄──────────────────────────────────┐
  │                                                               │
  │  收集信号                                                      │
  │  · 失败轨迹 (tool failures, LLM errors, wrong answers)         │
  │  · 用户反馈 (thumbs up/down, corrections)                      │
  │  · 使用模式 (popular tools, common tasks, pain points)         │
  │                                                               │
  ▼                                                               │
  Shadow Agent (唤醒)                                              │
  │                                                               │
  ├── 1. 分析信号                                                  │
  │     · 聚合失败模式 → 识别 top-N 改进方向                         │
  │     · 检查 issue backlog → 选择高优先级需求                      │
  │                                                               │
  ├── 2. 拆解为编码任务                                             │
  │     · 需求 → 子任务序列（含依赖关系）                             │
  │     · 每个子任务包含：目标、约束、验收标准                         │
  │                                                               │
  ├── 3. 调度 Coding Agent                                        │
  │     · 选择最优 agent (Claude Code / Codex / Kimi)              │
  │     · 传递上下文（项目结构、相关文件、编码规范）                    │
  │     · 流式监控执行过程                                          │
  │                                                               │
  ├── 4. 增量验收                                                  │
  │     · build → test → lint → diff review                       │
  │     · 失败 → 注入错误信息 → Coding Agent 修复 → 再验证           │
  │     · 最多 N 轮重试，超限则标记失败并记录                         │
  │                                                               │
  ├── 5. 代码合并                                                  │
  │     · 创建 PR + 生成描述                                       │
  │     · CI 监控 → 失败自动修复 → 重推                             │
  │     · 可选：人工审批门禁（关键变更）                              │
  │     · Merge                                                   │
  │                                                               │
  ├── 6. 版本发布                                                  │
  │     · semver 计算（patch/minor/major）                         │
  │     · Changelog 生成                                          │
  │     · 多平台构建 + 产物归档                                     │
  │     · 部署到 Test Agent 隔离环境                                │
  │                                                               │
  ├── 7. Test Agent 评测                                          │
  │     · 运行完整评测套件                                          │
  │     · 多维度打分 + 基线对比                                     │
  │     · 可选灰度：小流量真实请求路由到 Test Agent                   │
  │                                                               │
  └── 8. 晋升决策                                                  │
        · 评测通过 → Test Agent 晋升为新 Online Agent ──────────────┘
        · 评测未通过 → 记录失败原因 → 回到步骤 1
```

---

## 1. 信号采集与分析

> `internal/devops/signals/`

### M1: 信号采集

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 失败轨迹记录 | 工具失败、LLM 错误、答案纠错等自动记录 | ⚙️ 部分 | `internal/observability/` |
| 用户显式反馈 | 满意度评分 / thumbs up/down / 文字纠错 | ❌ 待实现 | `devops/signals/feedback.go` |
| 隐式信号采集 | 用户重试、放弃、修改目标等行为信号 | ❌ 待实现 | `devops/signals/implicit.go` |
| 使用模式统计 | 高频工具、常见任务类型、痛点 | ❌ 待实现 | `devops/signals/usage.go` |
| 信号结构化存储 | 统一信号格式，持久化到 DB | ❌ 待实现 | `devops/signals/store.go` |

### M2: 信号分析

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 失败模式聚类 | 将失败轨迹按根因聚类 | ❌ 待实现 | `devops/signals/analysis.go` |
| 改进方向识别 | 从聚类结果中自动识别 top-N 改进方向 | ❌ 待实现 | `devops/signals/analysis.go` |
| Issue 自动生成 | 将改进方向转化为结构化 Issue | ❌ 待实现 | `devops/signals/issue_gen.go` |
| 反馈归因 | 将反馈归因到具体的推理步骤/工具调用 | ❌ 待实现 | `devops/signals/attribution.go` |

---

## 2. Shadow Agent 核心

> `internal/devops/shadow/`

### M2: Shadow Agent 原型

> **依赖：** Track 2 M1（Coding Agent Gateway 全链路） + Track 3 M1（Lark 卡片/审批） + Track 4 M1（信号采集体系）

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Shadow Agent 框架 | 生命周期管理（唤醒/执行/休眠）+ 配置 | ❌ 待实现 | `devops/shadow/agent.go` |
| 需求接收 | 从 Issue 列表/手动输入/信号分析结果接收改进需求 | ❌ 待实现 | `devops/shadow/intake.go` |
| 任务拆解 | 将需求拆解为原子编码子任务序列（含依赖） | ❌ 待实现 | `devops/shadow/decomposer.go` |
| Coding Agent 调度 | 调用 Track 2 的 Coding Agent Gateway 执行编码 | ❌ 待实现 | `devops/shadow/dispatcher.go` |
| 上下文注入 | 为 Coding Agent 组装项目上下文、编码规范、相关文件 | ❌ 待实现 | `devops/shadow/context.go` |
| 验收编排 | 调用 Track 2 `coding/verify` 执行 build/test/lint/diff review | ❌ 待实现 | `devops/shadow/verify_orchestrator.go` |
| 修复循环编排 | 验证失败 → 注入错误信息 → Coding Agent 重试 → 再验证（最多 N 轮） | ❌ 待实现 | `devops/shadow/fix_loop.go` |
| 执行监控 | 流式监控 Coding Agent 执行过程 | ❌ 待实现 | `devops/shadow/monitor.go` |
| Lark 进度推送 | 通过 Lark 卡片推送当前步骤/变更文件/验证状态 | ❌ 待实现 | `devops/shadow/notify.go` |

### M3: Shadow Agent 完善

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自动信号消费 | 定时拉取信号 → 自动识别改进方向 → 自动拆解任务 | ❌ 待实现 | `devops/shadow/auto.go` |
| 优先级排序 | 按影响面/频率/严重度对改进项排序 | ❌ 待实现 | `devops/shadow/prioritizer.go` |
| 多任务串行编排 | 按依赖序执行多个编码任务 | ❌ 待实现 | `devops/shadow/pipeline.go` |
| 决策日志 | 记录 Shadow Agent 的所有决策及理由 | ❌ 待实现 | `devops/shadow/decision_log.go` |
| 人工介入点 | 关键决策（架构变更、破坏性修改）暂停等确认 | ❌ 待实现 | `devops/shadow/approval.go` |

---

## 3. 增量验收 (→ Track 2 `coding/verify`)

> **本模块的验证能力实现位于 Track 2 `coding/verify`。** Shadow Agent 作为消费者调用验证接口，仅在 `devops/shadow/verify_orchestrator.go` 和 `devops/shadow/fix_loop.go` 中编排重试策略与决策逻辑。
>
> 详见 `docs/roadmap/track2-system-interaction.md` § 3 Coding Agent Gateway:
> - M0: `coding/verify_build.go` — 构建验证
> - M1: `coding/verify_test.go` — 测试运行 + 结果解析
> - M1: `coding/verify_diff.go` — Diff 审查
> - M2: `coding/fix_loop.go` — 修复循环（验证失败 → 错误注入 → agent 修复 → 再验证）
>
> **Track 4 在本模块的职责仅限于：** 编排验证调用的顺序（build → test → lint → diff review）、决定是否触发修复循环、决定最大重试次数和失败处理策略。这些编排逻辑在 Shadow Agent 核心（Section 2）中实现。

---

## 4. 代码合并

> `internal/devops/merge/`

### M2: PR 自动化

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自动创建 PR | 验收通过后自动创建 PR | ❌ 待实现 | `devops/merge/pr_create.go` |
| PR 描述生成 | 从 diff + commit + issue 自动生成描述 | ❌ 待实现 | `devops/merge/pr_description.go` |
| CI 监控 | 推送后监控 CI 流水线结果 | ❌ 待实现 | `devops/merge/ci_monitor.go` |
| CI 失败修复 | CI 失败 → 分析原因 → Coding Agent 修复 → 重推 | ❌ 待实现 | `devops/merge/ci_fix.go` |
| 人工审批门禁 | 关键变更暂停等 Lark 审批 | ❌ 待实现 | `devops/merge/approval.go` |
| Review 通知 | 通过 Lark 通知 reviewer 并跟踪进度 | ❌ 待实现 | `devops/merge/notify.go` |

### M3: 智能合并

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自动 rebase | 保持分支与 main 同步 | ❌ 待实现 | `devops/merge/auto_rebase.go` |
| 冲突自动解决 | 简单冲突自动解决（import 顺序、格式化等） | ❌ 待实现 | `devops/merge/conflict.go` |
| 影响分析 | 分析变更对下游的影响范围 | ❌ 待实现 | `devops/merge/impact.go` |
| 批量 PR | 多个相关 PR 按依赖序合并 | ❌ 待实现 | `devops/merge/batch.go` |

---

## 5. 版本发布

> `internal/devops/release/`

### M2: 自动化发布

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 语义版本管理 | 基于 commit 类型（feat/fix/break）自动计算 semver | ❌ 待实现 | `devops/release/version.go` |
| Changelog 生成 | 从 commit/PR 历史生成结构化 changelog | ❌ 待实现 | `devops/release/changelog.go` |
| 多平台构建 | `make build-all` 触发 + 产物归档 | ❌ 待实现 | `devops/release/build.go` |
| 发布通知 | Lark 群推送 changelog 摘要 + 下载链接 | ❌ 待实现 | `devops/release/notify.go` |
| Release 标签 | 自动打 git tag + 创建 GitHub Release | ❌ 待实现 | `devops/release/tag.go` |

### M2: 部署

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 部署到 Test Agent 环境 | 将新版本部署到隔离环境 | ❌ 待实现 | `devops/release/deploy.go` |
| 灰度部署 | 按百分比/标签路由流量到新版本 | ❌ 待实现 | `devops/release/canary.go` |
| 部署审计日志 | 记录每次部署的完整操作日志 | ❌ 待实现 | `devops/release/audit.go` |

---

## 6. Test Agent 评测

> `internal/devops/evaluation/`

### M0: 评测基础

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| SWE-Bench 套件 | 代码任务评测 | ✅ 已实现 | `evaluation/` |
| Agent Eval 套件 | Agent 综合能力评测 | ✅ 已实现 | `evaluation/` |
| Lint + 单元测试 | Go/Web 代码质量门禁 | ✅ 已实现 | `scripts/`, `Makefile` |

### M1: 评测自动化

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| CI 评测门禁 | 每次变更自动跑评测 + 结果结构化存储 | ❌ 待实现 | `.github/workflows/` |
| 分维度评分 | 推理质量 / 工具可靠性 / 交互体验 / 成本效率 | ❌ 待实现 | `devops/evaluation/scorer.go` |
| 基线管理 | 维护 Online Agent 的评测基线分数 | ❌ 待实现 | `devops/evaluation/baseline.go` |
| 评测报告生成 | 结构化报告（通过/未通过 + 分数 + 基线对比） | ❌ 待实现 | `devops/evaluation/report.go` |
| Benchmark 看板 | Web 端评测结果趋势可视化 | ❌ 待实现 | `web/app/evaluation/` |

### M2: 高级评测

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自定义场景评测 | 从真实使用日志提取场景，自动回放评测 | ❌ 待实现 | `devops/evaluation/scenarios.go` |
| 灰度验证 | 小流量真实请求路由到 Test Agent | ❌ 待实现 | `devops/evaluation/canary.go` |
| 红队测试 | 自动生成对抗性输入，测试鲁棒性 | ❌ 待实现 | `devops/evaluation/redteam.go` |
| A/B 测试框架 | Online vs Test 的线上 A/B 对比 | ❌ 待实现 | `devops/evaluation/ab_test.go` |
| 晋升自动决策 | 评测通过阈值后自动晋升 Test → Online | ❌ 待实现 | `devops/evaluation/promotion.go` |

---

## 7. 线上监控与自愈

> `internal/devops/ops/`

### M0: 可观测基础 (→ 共享基础设施)

> **可观测性属于跨 Track 共享基础设施**（`internal/observability/`），不归属任何单一 Track。以下列出当前状态仅供参考。

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| OpenTelemetry Traces | 全链路追踪 | ✅ 已实现 | `internal/observability/` |
| Prometheus Metrics | 工具延迟/Token/成本 | ✅ 已实现 | `internal/observability/` |
| 结构化日志 | 带 Context ID | ✅ 已实现 | `internal/observability/` |
| 会话成本核算 | 每次会话费用统计 | ✅ 已实现 | `internal/observability/` |

### M1: Agent 驱动运维

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 异常告警 | 关键指标异常时 Lark 通知 | ❌ 待实现 | `devops/ops/alert.go` |
| 错误日志分析 | Agent 自动分析线上错误，定位根因 | ❌ 待实现 | `devops/ops/log_analysis.go` |
| 成本分析看板 | 按用户/任务/模型的成本趋势 | ❌ 待实现 | `web/` |
| 全链路重放 | 从 trace 重放完整执行过程 | ❌ 待实现 | `internal/observability/` |

### M2: 自愈系统

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自动故障诊断 | 从错误模式自动定位根因 | ❌ 待实现 | `devops/ops/diagnosis.go` |
| 修复 Playbook | 常见故障的预定义自动修复流程 | ❌ 待实现 | `devops/ops/playbooks/` |
| 自动回滚 | 部署后健康检查失败自动回滚 | ❌ 待实现 | `devops/ops/rollback.go` |
| 自动扩缩容 | 基于负载调整资源 | ❌ 待实现 | `deploy/` |
| 事故报告 | 自动生成事故报告（时间线/根因/修复/影响） | ❌ 待实现 | `devops/ops/incident_report.go` |

---

## 8. 自我进化闭环

> `internal/devops/evolution/`

### M3: 自主进化

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自改代码闭环 | Shadow Agent 发现 bug → 编写修复 → 测试 → PR → 合并 → 发布，全自动 | ❌ 待实现 | `devops/evolution/self_fix.go` |
| 新功能自研 | Shadow Agent 基于使用模式，自主提出并实现新功能 | ❌ 待实现 | `devops/evolution/self_feature.go` |
| Prompt 自动优化 | 基于评测结果 + 反馈信号，自动迭代 prompt | ❌ 待实现 | `devops/evolution/prompt_tuner.go` |
| 策略版本管理 | Prompt/策略纳入版本控制，支持回滚 | ⚙️ 部分 | `internal/context/` |
| A/B 闭环 | 灰度 → 观测 → 自动全量/回滚 | ❌ 待实现 | `devops/evolution/ab_loop.go` |
| 案例库构建 | 成功/失败轨迹结构化存储，作为未来决策参考 | ❌ 待实现 | `devops/evolution/case_library.go` |
| 进化日志 | 记录每次自我迭代的改进点、评测变化、决策理由 | ❌ 待实现 | `devops/evolution/changelog.go` |

---

## 关键依赖

| 本 Track 模块 | 依赖 | 所在 Track |
|---------------|------|-----------|
| Shadow Agent 编码调度 | Coding Agent Gateway | Track 2 |
| Lark 进度推送/审批 | Lark IM + 卡片 + 审批 | Track 3 |
| 修复循环 | ReAct 循环可靠性 | Track 1 |
| Test Agent 隔离环境 | 云端容器隔离 | Track 2 |
| 信号归因 | 可观测全链路追溯 | Track 1 |
| 评测套件 | 已有 evaluation/ | 已实现 |

---

## 进度追踪

| 日期 | 模块 | 更新 |
|------|------|------|
| 2026-02-01 | All | Track 4 详细 ROADMAP 创建。评测套件和可观测基础已实现，核心缺口在 Shadow Agent 框架、信号采集、验收循环和自动化发布流水线。 |
| 2026-02-01 | Shadow/Verify | Review 优化：Shadow Agent 从 M1 调整到 M2（依赖 T2.M1 + T3.M1）；增量验收改为引用 Track 2 `coding/verify`（去重）；可观测性标注为共享基础设施。 |
| 2026-02-01 | All | 实现审计：评测套件 ✅ 确认、共享基础设施（Observability/Config/Auth/Errors/Storage/DI）✅ 确认。`internal/devops/` 尚未创建（M2 范畴）。无需修正标注。 |
