# Roadmap: Lark-Native Proactive Personal Assistant (OKR-First)

> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-06

---

## Vision

构建一个 **开箱即用的 Lark-native 主动式个人 AI**：用户只需填入 LLM API Key、选择提供商和模型、配置 Lark 机器人 Key，即可拥有一个深度嵌入日常工作流的智能助手。本地有 Codex / Claude Code 订阅？直接就能用。

**产品定位：** 零门槛个人 AI — 不需要部署基础设施、不需要训练模型、不需要写一行代码。**三步启动：**
1. 选择 LLM 提供商（OpenAI / Claude / DeepSeek / ARK / Ollama 等），填入 API Key
2. 在 Lark 开放平台创建机器人，填入 App ID + Secret
3. 启动 elephant.ai — 你的 AI 助手即刻上线

**核心设计原则：**
- **Out-of-the-box** — 最小配置启动，复杂能力渐进解锁（有 Codex 订阅则自动获得编码能力，接入更多 Lark 权限则解锁更多生态能力）
- **Anticipate, don't wait** — 主动注入上下文，而非等待用户检索
- **Channel-native** — Lark 群聊就是主战场，Web/CLI 是补充面
- **Autonomous execution** — ReAct 循环驱动端到端执行，最小化人类干预
- **Compounding intelligence** — 每次交互都让系统变得更聪明
- **Self-evolving** — 影子 Agent 持续驱动系统自我迭代
- **Multi-LLM, no lock-in** — 支持 7+ LLM 提供商，用户自由选择，随时切换

---

## North Star Metrics (NSM)

**北极星指标核心：**任务难度 × 时间 × 准确率。

- **WTCR (Weighted Task Completion Rate)**：按任务难度加权的闭环率
- **TimeSaved**：节省时间（baseline_time − actual_time），关注 p50/p90
- **Accuracy**：结果正确率（自动验证 + 用户确认）

**任务难度定义（建议）**
- L1：单步/低风险（检索、回答、简单说明）
- L2：多步/有写操作（写 Doc、改表格、建日程、改任务）
- L3：跨系统/高风险（多 Agent、代码修改、审批链）

---

## OKR Tree (Global)

### O0 产品北极星
**在 Lark 内完成“日程 + 任务”闭环，并显著提升 WTCR 与 TimeSaved。**
- KR0.1：完成 WTCR/TimeSaved/Accuracy 的**度量口径定义 + 采集**
- KR0.2：建立 L2/L3 的 baseline 及对照流程
- KR0.3：WTCR 与 TimeSaved 相比 baseline **稳定提升**（以趋势为先，不强行定死数值）

### O1 Track 1 — 推理与 Agent 核心
**提升规划与上下文可靠性，使任务执行更准确、更可恢复。**
- KR1.1：Replan + 子目标分解在高失败率任务上可用
- KR1.2：主动上下文注入与记忆结构化提升召回质量
- KR1.3：关键决策具备置信度/证据与澄清路径
- KR1.4：(Steward) 跨轮结构化状态闭环可用 — STATE 自动注入 + 解析 + 持久化 + 再注入

### O2 Track 2 — 系统交互层
**工具链稳定、可度量、可路由，支撑“日程+任务”闭环。**
- KR2.1：核心工具 SLA 基线 + 路由/降级策略可用
- KR2.2：Coding Agent Gateway 可接入 Codex/Claude Code（可选能力）
- KR2.3：Scheduler 支撑提醒/跟进闭环

### O3 Track 3 — Lark 全生态
**Calendar + Tasks 在 Lark 内闭环，审批与权限可控。**
- KR3.1：Calendar/Tasks 读写能力可用且稳定
- KR3.2：写操作全链路审批门禁 + 审计
- KR3.3：主动提醒/跟进形成闭环

### O4 Track 4 — Shadow DevOps
**自我迭代闭环可用，但发布必须人工审批。**
- KR4.1：Shadow Agent 能驱动编码/验证/评测链路
- KR4.2：发布默认人工审批 + 回滚阈值
- KR4.3：评测基线与对照报告可用

### OS 共享基础设施
**事件总线 + 可观测 + 配置/鉴权统一支撑 OKR。**
- KR-S1：Event Bus 支撑任务/会话/系统事件
- KR-S2：Observability 覆盖成本/延迟/成功率
- KR-S3：配置/权限/错误处理统一

---

## Primary Vertical Slice (North-Star Scenario)

**日程 + 任务闭环**（Calendar + Tasks）是短期优先级最高的纵向切片：
- 读取日程/任务 → 生成建议 → 用户确认 → 写入/更新 → 提醒/跟进
- 与 NSM 直接对齐（闭环率、节省时间、准确率）

---

## OKR-Driven Roadmap (Milestones)

### M0: Foundation — 最小闭环可用
**目标：** 建立 NSM 度量与“日程+任务”最小闭环。

**Key Results (derived):**
- KR0.1 度量口径定义 + 数据采集上线
- KR3.1 日程/任务读写最小能力可用
- KR3.2 写操作审批门禁可用

**Initiatives by Track:**
- **T1:** ReAct 异常路径覆盖与断点续跑基线
- **T2:** 工具 SLA 采集基线；Scheduler 基础能力可用
- **T3:** Lark Calendar/Tasks API 最小封装；IM 内确认/审批交互
- **T4:** 评测套件 CI 自动化

### M1: Proactive — 主动闭环显著提升
**目标：** 主动提醒/跟进提升闭环率与节省时间。

**Key Results (derived):**
- KR0.3 WTCR/TimeSaved 相比 baseline **趋势提升**
- KR3.3 主动提醒与跟进闭环
- KR2.1 工具 SLA 画像 + 路由/降级策略可用

**Initiatives by Track:**
- **T1:** Replan 机制 + 子目标分解 + 记忆结构化（D3/D5）+ Steward AI 基础（跨轮状态、NEW_STATE 协议、L1-L4 安全分级、三级预算）
- **T2:** Tool allow/deny policy (D1) + Scheduler 增强 (D4)
- **T3:** Calendar/Tasks 完整 CRUD + 主动提醒
- **T4:** 信号采集体系 + 分维度评测 + 基线管理

### M2: Autonomous — Shadow Agent 原型 + 强门禁
**目标：** Shadow Agent 完整闭环可用，但发布必须人工审批。

**Key Results (derived):**
- KR4.1 Shadow Agent 驱动编码/验证/评测链路
- KR4.2 发布默认人工审批 + 回滚阈值
- KR2.2 Coding Gateway 多 adapter 运行

**Initiatives by Track:**
- **T1:** 多 Agent 协作 + 置信度建模
- **T2:** Coding Agent Gateway 全链路 + 修复循环
- **T3:** Lark 深度集成（多群/权限）+ macOS Node Host MVP (D6)
- **T4:** Shadow Agent 上线 + Test Agent 评测

### M3: Self-Evolving — 受控自进化闭环
**目标：** 自我进化闭环形成，但保持人工审批与回滚能力。

**Key Results (derived):**
- WTCR/TimeSaved 在 L3 任务持续优化
- A/B 与自愈运维可控上线

**Initiatives by Track:**
- **T1:** 记忆从被动存储 → 主动知识管理
- **T2:** 工具链自配置与历史效果路由
- **T3:** 协作模式 + 移动端 + 知识库治理
- **T4:** 自改代码闭环 + 自愈运维

---

## 系统全局架构（保留）

```
                           ┌─────────────────────┐
                           │     用户 (cklxx)      │
                           └──────────┬──────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                   │
               ┌────▼───┐      ┌─────▼────┐       ┌─────▼────┐
               │  Lark   │      │   Web    │       │   CLI    │
               │ 全生态   │      │Dashboard │       │   TUI    │
               └────┬───┘      └─────┬────┘       └─────┬────┘
                    │                 │                   │
                    └─────────────────┼───────────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
  ┌───────────▼──────────┐ ┌─────────▼──────────┐            │
  │  Online Agent (在线)   │ │ Shadow Agent (影子)  │            │
  │  服务用户请求           │ │ 驱动自我迭代         │            │
  └───────────┬──────────┘ └─────────┬──────────┘            │
              │                       │                       │
              └───────────┬───────────┘                       │
                          ▼                                   │
  ┌───────────────────────────────────────────────────────────┤
  │              Shared Infrastructure (共享基础设施)            │
  │                                                           │
  │  ┌─────────────┐ ┌──────────────┐ ┌────────────────┐     │
  │  │ Tool Engine  │ │ Coding Agent │ │ Lark API Layer │     │
  │  │ Sandbox      │ │ Gateway      │ │ Docs/Sheets/   │     │
  │  │ Browser      │ │ (Codex/      │ │ Wiki/Calendar/ │     │
  │  └─────────────┘ └──────────────┘ │ Tasks/Approval │     │
  │                                   └────────────────┘     │
  │                                                           │
  │  ┌─────────────┐ ┌──────────────┐ ┌────────────────┐     │
  │  │Observability│ │ Config/Auth  │ │ Storage/Session│     │
  │  └─────────────┘ └──────────────┘ └────────────────┘     │
  └───────────────────────────────────────────────────────────┘
                          │
              ┌───────────▼───────────┐
              │  Test Agent (被测)      │
              │  新版本隔离评测          │
              │  通过 → 替换 Online     │
              └───────────────────────┘
```

---

## 共享基础设施 (Cross-Track)

以下模块不归属任何单一 Track，为所有 Track 提供基础能力：

| 模块 | 包路径 | 说明 | OpenClaw Delta |
|------|--------|------|------|
| **Event Bus** | `internal/events/` | 统一 pub/sub 事件总线，task/session/system 三级事件，现有 Hook 平滑迁移为 subscriber | **D2** |
| **Observability** | `internal/observability/` | 全链路 Trace、Prometheus Metrics、结构化日志、成本核算 | |
| **Config** | `internal/config/` | YAML 配置管理、环境变量覆盖 | |
| **Auth** | `internal/auth/` | OAuth/Token、路由鉴权 | |
| **Errors** | `internal/errors/` | 错误分类、重试策略 | |
| **Storage** | `internal/storage/` | 通用持久化 | |
| **DI** | `internal/di/` | 依赖注入、服务装配 | |

---

## 跨 Track 边界约定（OKR 对齐）

| 边界点 | 约定 |
|--------|------|
| **Event Bus** _(D2)_ | `internal/events/` 是共享基础设施；为 KR-S1 提供事件能力。 |
| **验证逻辑** | 统一在 Track 2 的 `coding/verify` 包中实现（Build/Test/Lint/DiffReview）；Track 4 仅编排与审批。 |
| **Coding Agent Gateway** | Track 2 构建能力，Track 4 构建工作流；Gateway 同时服务 Online/Shadow。 |
| **Lark 工具封装** | Track 2 提供工具注册端口，Track 3 提供 Lark 工具实现。 |
| **Node Host** _(D6)_ | Track 2 负责 proxy executor，Track 3 负责 macOS companion app。 |
| **审批门禁** | Shadow Agent 发布**必须人工审批**；写操作在 Lark 侧需审批。 |

---

## 跨 Track 依赖关系（OKR 驱动）

```
O0 (日程+任务闭环)
├── O1 (规划与记忆) ────── 提升准确率与可恢复性
├── O2 (工具与执行) ────── 提升可靠性与可路由性
├── O3 (Lark 生态) ─────── 交互面闭环 + 审批门禁
└── O4 (Shadow DevOps) ─── 自我迭代但强审批
```

---

## 子 ROADMAP 索引

| Track | 子 ROADMAP 文件 | 内容 |
|-------|----------------|------|
| Track 1 | `docs/roadmap/track1-agent-core.md` | ReAct 循环、LLM 路由、上下文工程、记忆系统的 OKR 拆解 |
| Track 2 | `docs/roadmap/track2-system-interaction.md` | 工具引擎、沙箱、Coding Agent Gateway、数据处理、技能系统的 OKR 拆解 |
| Track 3 | `docs/roadmap/track3-lark-ecosystem.md` | Calendar/Tasks 优先的 Lark 生态 OKR 拆解 |
| Track 4 | `docs/roadmap/track4-shadow-agent-devops.md` | 影子 Agent DevOps OKR 拆解（强人工审批） |

---

## 进度追踪

| 日期 | 里程碑 | Track | 更新 |
|------|--------|-------|------|
| 2026-02-01 | M0 | All | Roadmap 创建。M0 大部分基础能力已实现（ReAct、69+ 工具、三端交互、可观测性）。主要缺口：断点续跑、Coding Agent Gateway、Lark API client 封装、CI 评测门禁。 |
| 2026-02-01 | M0 | All | Review 优化：更新产品定位为"开箱即用个人 AI"；修正跨 Track 边界；Shadow Agent 从 M1 移至 M2；新增渐进式能力解锁和本地 CLI 自动探测。 |
| 2026-02-01 | M0 | All | 实现审计：对照代码库校验 Roadmap 标注。修正工具数 83→69+、权限预设三档→五档、技能数 13→12、向量检索 ✅→⚙️（chromem-go，无 pgvector/BM25）、事件一致性 ⚙️→✅、用户干预点 ⚙️→✅、群聊自动感知 ❌→✅、消息引用回复 ❌→✅、定时提醒 ❌→⚙️、超时重试限流 ✅→⚙️。详见 `docs/roadmap/implementation-audit-2026-02-01.md`。 |
| 2026-02-01 | M0-M3 | All | **Roadmap 重构为 OKR-First。** 北极星切片聚焦"日程+任务"闭环，NSM 以 WTCR + TimeSaved + Accuracy 为核心。 |
| 2026-02-02 | M1 | All | **Phase 6 complete (C27-C40).** 14 tasks across 3 batches. All P0+P1 done, P2 ~85% complete. |
| 2026-02-06 | M1 | T1 | **Steward AI foundation complete (Phases 1-7).** StewardState + NEW_STATE 协议 + SYSTEM_REMINDER + L1-L4 安全分级 + 三级预算 + 40+ 测试。M1 ~85% → ~95%。 |
