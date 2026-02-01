# Roadmap: Lark-Native Proactive Personal Assistant

> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-01

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

## 系统全局架构

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
  │  │ File/Media   │ │ Claude/Kimi) │ │ Tasks/Approval │     │
  │  └─────────────┘ └──────────────┘ └────────────────┘     │
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

**关键架构决策：**

- **Coding Agent Gateway 是共享基础设施**，Online Agent（用户编码请求）和 Shadow Agent（自我迭代编码）都调用它。
- **验证逻辑（build/test/lint/diff review）统一在 `coding/verify` 包中**，Shadow Agent 编排时调用，不重复实现。
- **Observability 是跨 Track 基础设施**，不归属任何单一 Track。

**三 Agent 模型：**

| Agent | 角色 | 生命周期 |
|-------|------|---------|
| **Online Agent** | 在线助手，服务用户请求 | 常驻运行，7×24 |
| **Shadow Agent** | 影子 Agent，驱动 coding iteration（需求→编码→测试→合并→发版） | 按需唤醒或定时运行 |
| **Test Agent** | 被测 Agent，Shadow 产出的新版本在此验证 | 影子 Agent 触发后临时运行 |

---

## 四条主线概览

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Track 1: 推理与 Agent 核心                         │
│   LLM Reasoning · ReAct Loop · Planning · Multi-Agent · Memory      │
├─────────────────────────────────────────────────────────────────────┤
│                    Track 2: 系统交互层                                │
│   Tool Engine · Sandbox · Coding Agent Gateway · Data Processing    │
├─────────────────────────────────────────────────────────────────────┤
│                    Track 3: 人类集成交互 — Lark 全生态                  │
│   IM · Docs · Sheets · Wiki · Calendar · Tasks · Approval · Web    │
├─────────────────────────────────────────────────────────────────────┤
│                    Track 4: 自主迭代升级 — 影子 Agent DevOps           │
│   Shadow Agent · Coding Supervision · Test · Release · Self-Evolve │
└─────────────────────────────────────────────────────────────────────┘
```

**里程碑：**

| 里程碑 | 代号 | 目标 |
|--------|------|------|
| **M0** | Foundation | 开箱即用：填入 API Key + Lark Bot Key 即可运行；核心循环稳固，单 Agent 可靠执行，基础工具链完备 |
| **M1** | Proactive | Lark-native 主动交互，Lark 全生态接入，Coding Agent Gateway 全链路，信号采集体系 |
| **M2** | Autonomous | 多 Agent 协作，Shadow Agent 上线（监督编码→验收→合并→发版→评测→晋升） |
| **M3** | Self-Evolving | Shadow Agent 驱动自我进化闭环，A/B 测试，自愈运维 |

---

## Track 1: 大模型推理与 Agent 核心循环

> 子 ROADMAP: `docs/roadmap/track1-agent-core.md`

Online Agent 的推理引擎，是整个系统的心脏。

### 现状

- ReAct 循环（Think → Act → Observe）已实现，状态机健壮
- 7+ LLM 提供商统一抽象（OpenAI/Claude/ARK/DeepSeek/Ollama）
- 分层上下文工程（System/Policy/Task/Memory）+ 动态压缩
- 持久记忆（Postgres + 文件） + RAG 语义检索
- 基础子 Agent 委派

### M0: Foundation — 开箱即用

- **开箱即用体验** — 用户三步启动：1) 选择 LLM 提供商 + 填入 API Key；2) 配置 Lark Bot Key；3) 启动。即可在 Lark 群聊中使用 AI 助手
- **渐进式能力解锁** — 基础对话能力零门槛；有 Codex/Claude Code 订阅则自动获得编码能力；接入更多 Lark Open API 权限则解锁文档/表格/日历等生态能力
- **ReAct 可靠性** — 所有异常路径（LLM 超时、工具失败、上下文溢出）有明确恢复策略；断点快照后重启可自动续跑
- **结构化事件流** — 所有循环阶段产出强类型事件，前端可消费
- **记忆基线** — 对话/决策记忆自动存储与检索链路跑通

### M1: Proactive

- **Replan 机制** — 工具失败/结果不符预期时自动重新规划
- **子目标分解** — 复杂任务拆分为子目标链，逐步执行
- **智能模型路由** — 按任务类型/复杂度/成本自动选择最优模型
- **主动上下文注入** — 自动检测话题，主动加载相关记忆和 Lark 聊天历史
- **Token 预算管理** — 成本/Token 预算驱动上下文裁剪和模型选择
- **Memory Flush-before-Compaction** _(D3)_ — AutoCompact 前提取关键上下文落盘
- **记忆目录结构化** _(D5)_ — entries/ + daily/ + MEMORY.md 三层记忆

### M2: Autonomous

- **多 Agent 协作** — Agent 间结构化消息传递，任务分配与负载均衡，冲突仲裁
- **高级推理** — 多路径采样投票、置信度建模、不确定性传播
- **学习型记忆** — 用户偏好学习、任务模式识别、记忆纠错与遗忘策略
- **长程规划** — 支持小时/天级别的多阶段目标规划

### M3: Self-Evolving

- 置信度建模全面上线，所有关键决策带证据和评分
- 记忆系统从被动存储进化为主动知识管理

---

## Track 2: 系统交互层

> 子 ROADMAP: `docs/roadmap/track2-system-interaction.md`

Agent 的手脚 — 工具、沙箱、Coding Agent 编排、数据处理。

### 现状

- 69+ 个内置工具（7 层分类），带 schema 注册与权限预设（Full/ReadOnly/Safe/Sandbox/Architect 五档）
- 代码执行沙箱、Shell、浏览器自动化、文件操作已实现
- 图像/视频/PPT 生成已实现
- 12 个 Markdown 驱动技能

### M0: Foundation

- **工具引擎稳固** — 工具 SLA 基线数据采集（延迟/成本/可靠性）
- **Coding Agent Gateway** — 统一接口抽象（Submit / Stream / Cancel / Status），首个 adapter（Claude Code CLI 或 Codex CLI）跑通
- **本地 CLI 自动探测** — 启动时检测本地已安装的 coding agent CLI（`which codex`、`which claude`），有则自动注册为可用 adapter，无则跳过（渐进式能力解锁）
- **工作区管理** — 为 coding 任务准备隔离工作目录 + 变更追踪

### M1: Proactive

- **Coding Agent 全链路** — 任务翻译 → 上下文组装 → Agent 执行 → 结果验收（build/test/lint） → 自动 commit + PR
- **多 Adapter** — Claude Code + Codex + Kimi K2 adapter 全部就绪
- **智能选型** — Agent 能力画像 + 规则路由 + 降级链
- **智能工具路由** — 基于 SLA 画像动态选择工具链，自动降级
- **工具治理** _(D1)_ — Tool allow/deny policy + policyAwareRegistry + group tags + 三档 profile
- **Scheduler 增强** _(D4)_ — JobStore 持久化 + 状态跟踪 + 冷却/并发 + 动态 Job 工具
- **多模态处理** — PDF 解析、Excel/CSV 处理、音频转录
- **用户自定义技能** — 用户通过 Markdown 定义自己的技能

### M2: Autonomous

- **云端隔离环境** — 每个 Agent 独立容器，环境快照与恢复，资源配额
- **多 Agent 并行编码** — 独立模块分发给不同 agent 并行实现 + 冲突合并
- **修复循环自动化** — 验证失败 → 错误注入 → coding agent 修复 → 再验证，无人值守
- **历史效果路由** — 基于历史成功率/速度/成本的 bandit 自适应路由

### M3: Self-Evolving

- 所有核心工具能力缺口补齐
- 工具链完全自主可配置和扩展

---

## Track 3: 人类集成交互 — Lark 全生态

> 子 ROADMAP: `docs/roadmap/track3-lark-ecosystem.md`

**Lark 是主战场**。不止是 IM 聊天，而是深度接入 Lark 全生态 — 文档读写修改评论、表格、知识库、日历、任务、审批 — 让助手成为用户在 Lark 中的数字分身。Web 和 CLI 作为补充交互面。

### 现状

- Lark IM: WebSocket 事件循环、群聊历史、Emoji 进度、审批门禁、富附件、主动发消息 — 已实现
- Web: SSE 流式渲染、对话界面、附件/工具可视化、会话管理、成本追踪 — 已实现
- CLI: TUI 交互、审批、会话持久化 — 已实现

### M0: Foundation

- **Lark IM 稳固** — 现有消息层功能完备（已达成）
- **Web/CLI 基础完备** — 已达成
- **Lark Open API 封装层** — 构建统一的 Lark API client，为后续生态接入做基础
- **优雅降级** — 未配置 Lark key 时自动降级到 Web/CLI 模式；引导式配置流程帮助用户完成 Lark 机器人接入

### M1: Proactive — Lark 全生态接入

**IM 主动交互：**
- 群聊消息自动感知（无需 @mention）
- 长讨论后主动摘要
- 定时提醒与跟进
- 智能卡片交互（Interactive Card + 按钮操作）

**文档生态 (Docs/Docx)：**
- 文档内容**读取**（富文本 → Markdown）
- 文档**创建/追加/修改**（指定位置编辑）
- 文档**评论与批注**（添加/读取/回复评论）
- 文档权限管理

**表格 (Sheets/Bitable)：**
- 电子表格读写（行/列/范围）
- 多维表格 (Bitable) 记录 CRUD（带筛选/排序）
- 表格数据分析与图表生成

**知识库 (Wiki)：**
- 知识库空间浏览与全文读取
- 知识库语义搜索
- 从群聊/会议自动沉淀知识到 Wiki

**日历 (Calendar)：**
- 日程查询/创建/修改/取消
- 空闲时间查找（多人）
- 会议前准备材料自动汇总

**任务与审批 (Tasks/Approval)：**
- 任务 CRUD + 逾期跟进提醒
- 审批发起/查询/状态追踪

**统一工具封装：**
- 上述所有能力封装为 `lark_doc_*` / `lark_sheet_*` / `lark_wiki_*` / `lark_calendar_*` / `lark_task_*` / `lark_approval_*` 工具集

**macOS Node Host 协议** _(D6)_**：**
- Node Host API 契约定义 + Gateway proxy executor + 动态注册/注销

### M2: Autonomous

- **Lark 深度集成** — 多群联动、语音消息理解、基于角色的权限控制
- **文档智能** — 文档摘要/差异对比/模板应用/联动更新
- **表格自动化** — 定时数据同步、变更监听、报表自动生成
- **日历智能** — 会议纪要自动生成、日程冲突预警
- **macOS Companion App** _(D6)_ — menu bar 常驻 + Node Host 本地工具 + TCC 权限管理
- **Web 增强** — 执行回放、时间线可视化、用户纠错入口
- **跨面一致性** — 同一会话 Lark/Web/CLI 无缝切换、统一通知中心

### M3: Self-Evolving

- 协作模式（多用户同时参与会话）
- 移动端适配（PWA）
- 知识库自动治理（过时内容检测、自动归档）

---

## Track 4: 自主迭代升级 — 影子 Agent DevOps

> 子 ROADMAP: `docs/roadmap/track4-shadow-agent-devops.md`

**核心理念：用当前的 Agent 来监督 Coding Agent 写代码、合代码、测试、发版本。**

这不是传统 CI/CD 流水线。这是一个 **Agent-in-the-loop 的软件交付系统**，由三个 Agent 角色协作完成系统的持续进化。

### 三 Agent 架构

```
┌──────────────────────────────────────────────────────────────────┐
│                        用户 (Lark/Web/CLI)                        │
└────────────────────────────┬─────────────────────────────────────┘
                             │ 日常使用 + 反馈
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                   Online Agent (在线助手)                          │
│                                                                  │
│  · 7×24 常驻运行，服务用户请求                                      │
│  · 当前稳定版本                                                    │
│  · 收集用户反馈、使用模式、失败轨迹                                   │
└────────────────────────────┬─────────────────────────────────────┘
                             │ 反馈信号 + 失败轨迹 + 改进需求
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                   Shadow Agent (影子 Agent)                       │
│                                                                  │
│  · 按需唤醒或定时运行                                               │
│  · 分析反馈 → 识别改进方向 → 拆解为编码任务                           │
│  · 调度 Coding Agent (Codex/Claude Code/Kimi) 写代码               │
│  · 验收：build → test → lint → diff review                       │
│  · 合并：创建 PR → 监控 CI → 自动修复失败 → merge                   │
│  · 发版：语义版本 → changelog → 构建 → 部署到 Test Agent 环境        │
│  · 评估：对比 Test Agent vs Online Agent 的评测得分                  │
│  · 决策：通过 → 替换 Online Agent；不通过 → 记录原因 → 下轮迭代       │
└────────────────────────────┬─────────────────────────────────────┘
                             │ 新版本镜像
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                   Test Agent (被测 Agent)                         │
│                                                                  │
│  · Shadow Agent 产出的新版本                                       │
│  · 在隔离环境中运行完整评测套件                                      │
│  · 评测维度：推理质量、工具可靠性、交互体验、成本效率                    │
│  · 对比基线（当前 Online Agent 的评测分数）                           │
│  · 灰度验证：小流量真实请求路由到 Test Agent                          │
│  · 评测通过 → 晋升为新的 Online Agent                               │
└──────────────────────────────────────────────────────────────────┘
```

**迭代循环：**
```
Online Agent 收集信号
  → Shadow Agent 分析 + 拆解任务
    → Coding Agent 写代码
      → Shadow Agent 验收 (build/test/lint)
        → 合并 + 发版
          → Test Agent 评测
            → 通过: 替换 Online Agent
            → 未通过: 记录 → 下轮迭代
```

### 现状

- 评测套件（SWE-Bench、Agent Eval）已实现
- 可观测基础（OpenTelemetry、Prometheus、结构化日志、成本核算）已实现（跨 Track 共享基础设施）
- Coding Agent Gateway 在 Track 2 中规划
- 验证逻辑（build/test/lint/diff review）在 Track 2 `coding/verify` 中统一实现，本 Track 复用

### M0: Foundation

- **评测基础** — 评测套件可在 CI 中自动运行，结果结构化存储

### M1: Proactive — 信号采集与评测自动化

> Shadow Agent 依赖 Track 2 M1（Coding Gateway 全链路）和 Track 3 M1（Lark 卡片/审批），因此 Shadow Agent 原型在 M2 而非 M1。M1 聚焦于为 M2 准备信号和评测基础设施。

**信号采集体系：**
- 失败轨迹自动记录（工具失败、LLM 错误、答案纠错）
- 用户显式反馈采集（满意度/纠错）
- 隐式信号采集（重试、放弃、修改目标）
- 使用模式统计（高频工具、常见任务、痛点）
- 信号结构化存储

**评测自动化：**
- CI 自动回归门禁
- 分维度评分（推理/工具/交互/成本）
- 基线管理（维护 Online Agent 评测分数）
- 评测报告结构化生成
- Benchmark 看板（Web 可视化）

### M2: Autonomous — Shadow Agent + 完整发布流水线

**Shadow Agent 核心：**
- Shadow Agent 基础框架（唤醒/执行/休眠）
- 需求接收 → 任务拆解 → Coding Agent 调度（复用 Track 2 Gateway）
- 增量验收（复用 Track 2 `coding/verify`：build → test → lint → diff review）
- 修复循环编排 — 验证失败 → 错误注入 → Coding Agent 重试 → 再验证
- Lark 进度卡片推送 + 人工审批门禁

**代码合并：**
- 自动创建 PR + 生成描述
- CI 失败自动分析 → Coding Agent 修复 → 重推
- 自动 rebase/merge，简单冲突自动解决

**版本发布：**
- 语义版本自动管理（基于 commit 类型）
- Changelog 自动生成
- 多平台构建 + 产物归档
- 发布通知推送到 Lark 群

**Test Agent 评测：**
- 新版本自动部署到隔离环境
- 运行完整评测套件，多维度打分
- 与 Online Agent 基线对比，生成评测报告
- 通过阈值后自动晋升

**线上监控：**
- 部署后健康检查
- 异常自动回滚
- 事故报告自动生成

### M3: Self-Evolving — 自主进化闭环

**自改代码闭环：**
- Shadow Agent 从信号分析中自动识别改进方向（无需人工输入需求）
- 自主编码 → 测试 → 发布，无人工介入
- 失败轨迹结构化存储为案例库

**A/B 测试闭环：**
- 灰度发布 → 小流量真实请求 → 观测指标 → 自动全量/回滚
- Prompt/策略自动优化迭代

**自愈运维：**
- Agent 自动分析线上错误日志，定位根因
- 常见故障执行修复 Playbook
- 基于负载自动扩缩容
- 事故报告自动生成并推送 Lark

---

## 里程碑验收标准

### M0: Foundation — 开箱即用

| Track | 验收标准 |
|-------|---------|
| 全局 | **三步启动**：填入 LLM API Key + Lark Bot Key → 启动 → 在 Lark 中可用；本地有 Codex/Claude Code 订阅则自动识别并接入编码能力 |
| **共享基础设施** | Event Bus _(D2)_ Phase 0：`internal/events/` 包实现，现有 5 个 Hook 迁移为 bus subscriber，行为不变 |
| Track 1 | ReAct 异常路径全覆盖；断点续跑；记忆链路跑通 |
| Track 2 | 工具 SLA 基线采集；Coding Agent Gateway 首个 adapter 跑通；`coding/verify` 基础验证；本地 CLI 自动探测（`which codex`/`which claude`） |
| Track 3 | Lark IM + Web + CLI 基础功能完备；Lark API client 封装层就绪；引导式配置（无 Lark key 时优雅降级到 Web/CLI） |
| Track 4 | 评测套件 CI 自动化 |

### M1: Proactive

| Track | 验收标准 |
|-------|---------|
| **共享基础设施** | Event Bus 上线 _(D2)_：task/session/system 三级事件，现有 5 个 Hook 迁移为 subscriber，全量测试回归通过 |
| Track 1 | Replan 机制；智能模型路由；主动上下文注入；Memory Flush-before-Compaction _(D3)_：compact 后 `memory.flush.saved_total > 0`；记忆目录结构化 _(D5)_：entries/ + daily/ + MEMORY.md 三层 + recall 分层优先 |
| Track 2 | Coding Agent 全链路（任务→编码→验收→PR）；3 个 adapter；智能选型；Tool allow/deny policy _(D1)_：Lark 通道 denied_total > 0 且无误拒；Scheduler 增强 _(D4)_：动态 job 重启恢复 + 连续失败暂停 |
| Track 3 | Lark 文档/表格/知识库/日历/任务/审批全部接入；Lark 工具注册到 T2 引擎；Node Host API 契约定义 _(D6)_ |
| Track 4 | 信号采集体系上线；评测分维度自动化；基线管理；Benchmark 看板 |

### M2: Autonomous

| Track | 验收标准 |
|-------|---------|
| Track 1 | 多 Agent 协作；高级推理（投票/置信度）；学习型记忆 |
| Track 2 | 云端隔离环境；多 Agent 并行编码；修复循环自动化 |
| Track 3 | Lark 深度集成（多群/语音/权限）；文档/表格智能操作；跨面同步；macOS Companion App MVP _(D6)_：Gateway→Node Host 全链路 e2e + 权限降级正常 |
| Track 4 | Shadow Agent 上线（编码→验收→PR→CI→发版→评测→晋升全流程）；Test Agent 评测 |

### M3: Self-Evolving

| Track | 验收标准 |
|-------|---------|
| Track 1 | 置信度全面上线；记忆从被动存储进化为主动知识管理 |
| Track 2 | 核心工具能力无缺口；工具链自主可配置 |
| Track 3 | 协作模式；移动端；知识库自动治理 |
| Track 4 | 自改代码闭环运转；A/B 自动决策；自愈运维 |

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

## 跨 Track 边界约定

| 边界点 | 约定 |
|--------|------|
| **Event Bus** _(D2)_ | `internal/events/` 是共享基础设施，不归属任何单一 Track。T1 消费 `context.compact` / `session.ended`，T2 消费 `tool.failed`，T4 消费信号事件。各 Track 只发布/订阅自己领域的事件。 |
| **验证逻辑** | 统一在 Track 2 的 `coding/verify` 包中实现（Build/Test/Lint/DiffReview）。Track 4 Shadow Agent 复用该接口，仅编排重试策略和决策逻辑，不重复实现。 |
| **Coding Agent Gateway** | Track 2 构建能力（gateway + adapters + 验证），Track 4 构建工作流（Shadow Agent 编排）。Gateway 是共享基础设施，Online Agent 和 Shadow Agent 均为消费者。 |
| **Lark 工具封装** | `larktools/` 的工具实现属于 Track 3（Lark 领域），但注册到 Track 2 的工具引擎中。Track 2 提供端口（工具注册接口），Track 3 提供适配器（Lark 工具实现）。 |
| **Node Host** _(D6)_ | `nodehost/` proxy executor 属于 Track 2（工具引擎注册），companion app 实现属于 Track 3（人类交互面）。Track 2 提供端口（动态工具注册），Track 3 提供宿主（macOS app + HTTP server）。 |
| **主动性** | "主动" 是跨 Track 能力：T1 的主动上下文注入、T3 的群聊感知/摘要/提醒、T4 的自动信号分析。各 Track 各自负责本领域的主动性实现。 |

---

## 跨 Track 依赖关系

```
M0 Foundation
├── T1: ReAct 稳固 ─────────────── 所有 Track 的基础
├── T2: 工具 + Gateway 基础 ────── T2.M1 全链路的前置
├── T3: Lark API client ────────── T3.M1 全生态接入的前置
├── T4: 评测 CI ─────────────────── T4.M1 信号/评测的前置
└── 共享: Event Bus (D2) ─────────── M1 所有事件驱动功能的前置
         │
         ▼
M1 Proactive
├── 共享: Event Bus 上线 ─────────── D3 flush / D5 日汇总 / D4 调度均依赖事件
├── T1: Replan + 路由 + D3 flush + D5 记忆分层
├── T2: Coding 全链路 + verify + D1 工具治理 + D4 Scheduler 增强
├── T3: Lark 全生态 + D6 Node Host 协议 ──── T4.M2 Shadow Agent 通知/审批走 Lark
└── T4: 信号采集 + 评测自动化 ──── 为 M2 Shadow Agent 准备数据和评测基线
         │
         ▼
M2 Autonomous ← Shadow Agent 在此里程碑上线（依赖 T2.M1 + T3.M1 + T4.M1）
├── T1: 多 Agent ────────────────── T2 多 Agent 并行编码
├── T2: 云端隔离 ────────────────── T4 Test Agent 需要隔离环境
├── T3: 深度集成 + D6 macOS Companion App MVP
└── T4: Shadow Agent + 发布流水线  端到端自动化
         │
         ▼
M3 Self-Evolving
└── T4: 自主进化闭环 ────────────── 依赖所有 Track 的 M2 能力
```

---

## 子 ROADMAP 索引

| Track | 子 ROADMAP 文件 | 内容 |
|-------|----------------|------|
| Track 1 | `docs/roadmap/track1-agent-core.md` | ReAct 循环、LLM 路由、上下文工程、记忆系统的详细拆解 |
| Track 2 | `docs/roadmap/track2-system-interaction.md` | 工具引擎、沙箱、Coding Agent Gateway、数据处理、技能系统 |
| Track 3 | `docs/roadmap/track3-lark-ecosystem.md` | Lark Docs/Sheets/Wiki/Calendar/Tasks/Approval API 接入详细方案 |
| Track 4 | `docs/roadmap/track4-shadow-agent-devops.md` | 三 Agent 架构、影子 Agent 监督循环、发布流水线、自愈系统 |

---

## 进度追踪

| 日期 | 里程碑 | Track | 更新 |
|------|--------|-------|------|
| 2026-02-01 | M0 | All | Roadmap 创建。M0 大部分基础能力已实现（ReAct、69+ 工具、三端交互、可观测性）。主要缺口：断点续跑、Coding Agent Gateway、Lark API client 封装、CI 评测门禁。 |
| 2026-02-01 | M0 | All | Review 优化：更新产品定位为"开箱即用个人 AI"（三步启动）；修正跨 Track 边界（验证去重、larktools 归属、可观测共享）；Shadow Agent 从 M1 移至 M2；新增渐进式能力解锁和本地 CLI 自动探测。 |
| 2026-02-01 | M0 | All | 实现审计：对照代码库校验 Roadmap 标注。修正工具数 83→69+、权限预设三档→五档、技能数 13→12、向量检索 ✅→⚙️（chromem-go，无 pgvector/BM25）、事件一致性 ⚙️→✅、用户干预点 ⚙️→✅、群聊自动感知 ❌→✅、消息引用回复 ❌→✅、定时提醒 ❌→⚙️、超时重试限流 ✅→⚙️。详见 `docs/roadmap/implementation-audit-2026-02-01.md`。 |
| 2026-02-01 | M0-M2 | All | **OpenClaw 6 Delta 集成到 Roadmap。** D1(工具治理)→T2§1 M1; D2(事件总线)→共享基础设施 M0→M1; D3(Memory Flush)→T1§3 M1; D4(Scheduler)→T2§6 M1; D5(记忆结构化)→T1§4 M1; D6(macOS)→T3§12 M1→M2。详见 `docs/research/2026-02-01-openclaw-adoption-plan.md`。 |
