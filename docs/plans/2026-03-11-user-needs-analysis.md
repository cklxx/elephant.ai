# 需求优先级报告：Elephant.ai 未完成需求与用户痛点分析

**报告日期**: 2026-03-11  
**分析范围**: docs/plans/ 目录下全部 35 份计划文档  
**分析人员**: ckl

---

## 1. 执行摘要

本报告基于对 `docs/plans/` 目录下 35 份计划文档的全面分析，识别出当前产品的 **8 大核心用户痛点** 和 **23 项未完成的 prioritized 需求**。

**关键发现**:
- **P0 需求**: 5 项，涉及产品契约统一、核心架构稳定性和用户体验基础能力
- **P1 需求**: 10 项，涉及功能完善、性能优化和可观测性提升
- **P2 需求**: 8 项，涉及高级功能、代码清理和长期技术债务

---

## 2. 核心用户痛点

### 痛点 1: 产品契约双轨制（最高优先级）

**症状**:
- 对外宣传 CLI-first / Team-first，但历史文档和代码仍围绕 `run_tasks` / `reply_agent`
- LLM 认知分裂，开发者理解不一致
- 用户文档与实际入口不一致

**影响**:
- 理解成本 +40%
- 多 agent 信任感下降
- Debug 成本 +25%

**来源**: `2026-03-06-agent-team-feishu-cli-terminal-integration.md`, `2026-03-06-team-feishu-terminal-backlog.md`

---

### 痛点 2: 飞书能力面双轨制

**症状**:
- 飞书能力同时存在于 channel / Lark delivery / Go tool action matrix
- 两套 schema、两套错误语义、两套 prompt 说明
- 新增飞书能力需要扩散到多套工具面

**影响**:
- Tool selection 准确率下降 20-30%
- 新增飞书能力接入效率低下

**来源**: `2026-03-06-feishu-cli-canonical-surface.md`, `2026-03-06-agent-team-feishu-cli-terminal-integration.md`

---

### 痛点 3: Terminal 体验产品化缺失

**症状**:
- Terminal 更像开发者调试工具，不像用户可见产品
- 用户无法直观看到"谁在执行、干到哪、卡没卡、结果在哪"
- 内部概念（tmux pane、runtime root、sidecar）暴露给用户

**影响**:
- 用户无法有效干预任务执行
- 可委托感、可观察感缺失

**来源**: `2026-03-06-team-terminal-ux-model.md`, `2026-03-08-cli-runtime-kaku-compact-design.md`

---

### 痛点 4: 任务执行不可观察/不可干预

**症状**:
- 用户频繁主动查询任务状态（`isNaturalTaskStatusQuery` 逻辑存在）
- 用户发 /stop 命令，但系统不记录 stop 原因
- Agent 频繁 ask_user，但不做频率统计
- Handoff 通知后，不追踪用户响应行为

**影响**:
- 进度透明度不够
- 交互摩擦大
- 用户焦虑感

**来源**: `2026-03-11-leader-agent-improvement.md` (Lark 渠道用户反馈分析)

---

### 痛点 5: Leader Agent 决策延迟与控制精度不足

**症状**:
- Stall 检测到介入延迟：最坏情况 > 60s
- LLM 决策单点串行
- Stall prompt 缺乏上下文（不含 last tool call、error、session 摘要）
- 决策类型仅有 INJECT/FAIL/ESCALATE，无 RETRY_TOOL、SWITCH_STRATEGY 等

**影响**:
- Stall recovery 成功率不可度量（当前未知）
- Mean time to intervention 过长

**来源**: `2026-03-11-leader-agent-improvement.md`

---

### 痛点 6: 飞书文档编辑能力缺失

**症状**:
- 当前 `channel`/`docx` 实现仅支持 create/read/read_content/list_blocks
- Editing paths 因能力缺口而失败
- 用户反馈飞书文档编辑仍不工作

**来源**: `2026-03-03-docx-update-block-and-inject-e2e.md`

---

### 痛点 7: 代码复杂度与技术债务

**症状**:
- 17 个重复的 truncate 函数
- 60+ 处手写的 `strings.ToLower(strings.TrimSpace(x))`
- 12+ 处 ad-hoc `&http.Client{}` 绕过统一客户端
- God struct（RuntimeConfig 54 个字段）
- 370 行的 `Prepare()` 函数

**影响**:
- 维护成本上升
- 新功能开发效率下降

**来源**: `2026-03-03-global-codebase-simplify.md`

---

### 痛点 8: Kernel Dispatch 存储无界增长

**症状**:
- `dispatches.json` 随时间线性增长（每天 240 条记录）
- `ListRecentByAgent()` 和 `RecoverStaleRunning()` 每次扫描全部记录
- 无垃圾回收机制

**影响**:
- O(all-time) 扫描性能
- 内存占用随运行时间无限增长
- Planner 决策质量随历史膨胀而下降

**来源**: `2026-03-09-kernel-dispatch-analysis.md`

---

## 3. 未完成需求优先级列表

### P0: 阻塞性/基础性需求（5 项）

| ID | 需求名称 | 状态 | 预计工期 | 业务价值 | 技术风险 |
|----|---------|------|----------|----------|----------|
| **P0-1** | **Feishu CLI 产品化** - 定义 canonical `feishu-cli` skill contract，统一飞书对象操作入口 | todo | 3-5 天 | ★★★★★ | 中 |
| **P0-2** | **Terminal UX 产品化** - 定义 Team Run view model，实现用户可见执行现场（Live Terminal / Role Cards / Artifacts） | todo | 5-7 天 | ★★★★★ | 中 |
| **P0-3** | **产品契约统一** - 清理 `run_tasks/reply_agent` 用户入口描述，统一为 `alex team ...` 叙事 | done | 1-2 天 | ★★★★☆ | 低 |
| **P0-4** | **Leader Agent Stall 决策历史持久化** - Leader 在第 N 次 stall 时能看到前 N-1 次决策记录 | todo | 2-3 天 | ★★★★☆ | 低 |
| **P0-5** | **富 Stall Prompt** - 增加 last_tool_call、last_error、iteration_count 等上下文，支持 RETRY_TOOL/SWITCH_STRATEGY 决策 | todo | 3-4 天 | ★★★★☆ | 中 |

---

### P1: 重要功能完善（10 项）

| ID | 需求名称 | 状态 | 预计工期 | 业务价值 | 技术风险 |
|----|---------|------|----------|----------|----------|
| **P1-1** | **飞书文档编辑能力** - 实现 docx block text update，支持 Feishu 文档 PATCH 操作 | todo | 3-5 天 | ★★★★☆ | 中 |
| **P1-2** | **Handoff 上下文增强** - 增加 LastToolCall、LastError、SessionTail 等诊断信息 | todo | 3-4 天 | ★★★★☆ | 中 |
| **P1-3** | **Handoff Interactive Card** - Lark 交互卡片支持一键 retry/abort/provide_input | todo | 3-4 天 | ★★★★☆ | 中 |
| **P1-4** | **Stall Recovery 效果评估** - 量化 INJECT 成功率，淘汰无效策略 | todo | 3-4 天 | ★★★☆☆ | 中 |
| **P1-5** | **Kaku Runtime Skeleton** - 多 session 管理、生命周期控制、持久化 | in_progress | 3-5 天 | ★★★★☆ | 中 |
| **P1-6** | **CLI Member Adapter** - Codex / Claude Code / Kimi 统一接入 | todo | 4-6 天 | ★★★★☆ | 中 |
| **P1-7** | **Team Task Runner 容量问题** - 解决 `background task limit reached: 4 active (max=4)` | blocked | 1-2 天 | ★★★★☆ | 低 |
| **P1-8** | **Dispatch Store GC** - 添加 configurable retention，边界存储增长 | todo | 2-3 天 | ★★★★☆ | 低 |
| **P1-9** | **子任务拓扑感知** - EventChildCompleted 携带 sibling_total/completed | todo | 3-4 天 | ★★★☆☆ | 中 |
| **P1-10** | **代码重用清理** - 统一 TrimLower、Truncate、ToolError 等工具函数 | in_progress | 3-5 天 | ★★★☆☆ | 低 |

---

### P2: 中长期优化（8 项）

| ID | 需求名称 | 状态 | 预计工期 | 业务价值 | 技术风险 |
|----|---------|------|----------|----------|----------|
| **P2-1** | **Blocker Auto-Remediation** - 已知模式 blocker 自动修复（transient error retry、git review nudge） | todo | 5-7 天 | ★★★★☆ | 高 |
| **P2-2** | **Hooks + Scheduler + Leader Agent** - 依赖编排、事件驱动调度 | todo | 4-6 天 | ★★★☆☆ | 中 |
| **P2-3** | **Panel / UX / Team Recipe** - 通用 panel、team recipe 模板 | todo | 5-7 天 | ★★★☆☆ | 中 |
| **P2-4** | **Alert Feedback Tracking** - 建立 alert→user_action 关联，动态调整 cooldown | todo | 3-4 天 | ★★★☆☆ | 中 |
| **P2-5** | **用户行为信号采集** - Response latency、stop frequency、query frequency | todo | 4-5 天 | ★★☆☆☆ | 中 |
| **P2-6** | **架构蓝图实施** - Provider/Channel Capability Registry、Interface Segregation | todo | 2-3 周 | ★★★☆☆ | 高 |
| **P2-7** | **God Struct 拆分** - RuntimeConfig、Overrides、reactRuntime 拆分 | todo | 5-7 天 | ★★☆☆☆ | 中 |
| **P2-8** | **更多 Member 适配** - AnyGen、Colab 等 | todo | 1-2 周 | ★★☆☆☆ | 中 |

---

## 4. 关键依赖关系

```
P0-3 (契约统一) ──────────────────────────────────────────────┐
        │                                                      │
        ▼                                                      ▼
P0-1 (Feishu CLI) ◄────── P0-2 (Terminal UX) ◄────── P1-5 (Kaku Runtime)
        │                       │                              │
        │                       ▼                              ▼
        │              P1-2/3 (Handoff 增强) ◄───── P1-6 (Member Adapter)
        │                       │                              │
        ▼                       ▼                              ▼
P1-1 (Docx 编辑)         P1-4 (Recovery 评估) ◄───── P2-2 (Hooks/Scheduler)
                                                    │
                                                    ▼
                                           P2-3 (Panel/UX/Recipe)
```

**关键路径**: P0-3 → P0-1/P0-2 → P1-5/P1-6 → P2-2/P2-3

---

## 5. 建议执行路线图

### Phase 1: 基础契约统一（第 1-2 周）

**目标**: 消除产品契约双轨，建立统一叙事

**交付项**:
- [x] P0-3: 产品契约统一（已完成）
- [ ] P0-1: Feishu CLI 产品化（启动）
- [ ] P1-7: Team Task Runner 容量问题（解阻塞）

**成功指标**:
- 用户文档与 skill 文档中，Team 的唯一入口是 `alex team ...`
- 飞书操作优先选择 `feishu-cli`

---

### Phase 2: 用户体验产品化（第 3-5 周）

**目标**: 让任务执行从开发者工具变为用户可见体验

**交付项**:
- [ ] P0-2: Terminal UX 产品化
- [ ] P1-5: Kaku Runtime Skeleton
- [ ] P1-6: CLI Member Adapter
- [ ] P1-2: Handoff 上下文增强

**成功指标**:
- 用户 10 秒内能回答：谁在执行？卡住没？结果在哪？
- 用户无需理解 tmux、runtime root、sidecar

---

### Phase 3: Leader Agent 精度提升（第 6-8 周）

**目标**: 提升 Leader Agent 决策质量和响应速度

**交付项**:
- [ ] P0-4: Stall 决策历史持久化
- [ ] P0-5: 富 Stall Prompt
- [ ] P1-3: Handoff Interactive Card
- [ ] P1-4: Stall Recovery 效果评估
- [ ] P1-9: 子任务拓扑感知

**成功指标**:
- Stall recovery success rate > 60%
- Mean time to stall intervention < 30s

---

### Phase 4: 架构与性能优化（第 9-12 周）

**目标**: 解决技术债务，提升系统可维护性

**交付项**:
- [ ] P1-8: Dispatch Store GC
- [ ] P1-10: 代码重用清理
- [ ] P2-1: Blocker Auto-Remediation
- [ ] P2-2: Hooks + Scheduler
- [ ] P2-6: 架构蓝图实施

**成功指标**:
- `make check-arch` 0 exceptions
- dispatches.json 大小稳定（24h 保留）

---

## 6. 风险与缓解策略

| 风险 | 影响 | 概率 | 缓解策略 |
|------|------|------|----------|
| Feishu CLI 审批流设计复杂 | P0-1 延期 | 中 | 分阶段：先读操作，后写操作 |
| Kaku CLI 依赖外部二进制 | P1-5 不稳定 | 中 | 提供 fallback 到 tmux 模式 |
| LLM 决策不可控 | P0-5 效果不达预期 | 中 | 添加 fuzzy parsing + fallback |
| Auto-remediation 安全风险 | P2-1 引入故障 | 中 | max_auto_retries=2 + cooldown |
| 架构重构范围蔓延 | P2-6 延期 | 高 | 严格分层，逐阶段验收 |

---

## 7. 附录：参考文档

### 核心计划文档
1. `2026-03-06-agent-team-feishu-cli-terminal-integration.md` - Team/Skills/CLI 一体化方案
2. `2026-03-06-feishu-cli-canonical-surface.md` - Feishu CLI 产品面设计
3. `2026-03-06-team-terminal-ux-model.md` - Terminal UX 模型
4. `2026-03-08-cli-runtime-kaku-implementation-plan.md` - Kaku Runtime 实施计划
5. `2026-03-11-leader-agent-improvement.md` - Leader Agent 改进方案
6. `2026-03-09-kernel-dispatch-analysis.md` - Kernel Dispatch 深度分析
7. `2026-03-04-architecture-optimization-blueprint.md` - 架构优化蓝图
8. `2026-03-03-global-codebase-simplify.md` - 全局代码简化报告
9. `2026-03-03-docx-update-block-and-inject-e2e.md` - 飞书文档编辑能力
10. `2026-03-06-team-feishu-terminal-backlog.md` - 集成 Backlog

### 已完成的计划
- `2026-03-05-notebooklm-cli-independent-optimization.md` - NotebookLM CLI 优化 ✓
- `2026-03-02-feature-code-optimization.md` - 特性代码优化 ✓
- `2026-03-03-lark-task-create-oauth-title-body-fix.md` - Lark 任务创建修复 ✓
- 多个测试覆盖计划（已验证通过）

---

## 8. 度量指标跟踪

| 指标 | 当前值 | Phase 1 目标 | Phase 2 目标 | Phase 3 目标 |
|------|--------|--------------|--------------|--------------|
| Arch exceptions | 123 | < 80 | < 30 | 0 |
| Stall recovery success rate | 未知 | 可度量 | 可度量 | > 60% |
| Mean time to stall intervention | threshold + LLM latency | - | - | < 30s |
| User stop command frequency | 基线 | - | - | 下降 > 20% |
| dispatches.json 大小增长 | 线性 | 开始稳定 | 稳定 | 24h 窗口 |
| Feishu 操作入口统一率 | 0% | 50% | 80% | 100% |

---

*报告完成。建议召开需求评审会议，确认 Phase 1 执行计划。*
