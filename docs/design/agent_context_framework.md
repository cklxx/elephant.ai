# 智能体三层 Context 框架技术方案（面向 ALEX 优化）

## 0. 现状评估（2024-05）

| 维度 | 当前实现 | 主要限制 |
| --- | --- | --- |
| Prompt 裁剪 | `internal/context/manager.go` 通过字符计数粗略估算 token，并在超阈值时保留最近 10 条消息 | 未对系统提示、Persona、长期记忆做分层管理，容易造成关键信息被截断 |
| 会话状态 | `internal/session/filestore` 将完整对话序列写入磁盘，缺少结构化的计划/反馈记录 | 难以针对特定类型信息做检索或差异化保留 |
| 配置管理 | 目标、规则、Persona 分散在多处（`configs/`, `internal/prompts`, `internal/agent/app`） | 缺乏统一 Schema 与版本治理，难以做环境级切换 |
| 记忆与知识 | 仅保留压缩后的历史消息，无独立长期记忆或 RAG 接入 | 无法在多会话间共享经验，也缺乏过期管理 |

现有实现可以支撑基本的 ReAct 循环，但随着工具数量、团队协作场景的增加，会暴露出以下问题：

1. **上下文噪音高**：不同类型的内容混杂在消息序列里，无法按目标/规则/计划等维度做裁剪。
2. **缺少跨会话沉淀**：每次会话都从零开始，无法利用 SOP、知识库或长期偏好。
3. **演化能力弱**：缺乏对记忆、人格的维护机制，无法进行 A/B 测试或渐进式优化。

因此，我们需要以“优化当前项目”为目标，引入结构化的三层 Context 体系，并与现有代码库耦合。

## 1. 背景与目标

本文基于“静态 Context / 动态 Context / Meta-Context”三层框架，为公司智能体平台设计一套可描述、可拆解、可演化的上下文系统。目标如下：

- **结构化 Context**：将“token 塞入提示”升级为“可配置的环境模型”，支持多智能体、多任务场景。
- **提升决策质量**：让智能体在明确目标、规则、资源边界的前提下行动，减少幻觉与重复无效交互。
- **支持长期演化**：通过 Meta-Context 机制，保证记忆、知识、人格在长期运行中保持一致性与可维护性。
- **贴合 ALEX 架构**：充分复用 `internal/agent`, `internal/context`, `internal/tools`, `internal/observability` 等现有模块，实现平滑演进。

## 2. 总体架构

```
┌────────────────────────────┐        ┌─────────────────────┐
│        API / Gateway       │        │  Agent CLI / Web UI │
└─────────────┬──────────────┘        └──────────┬──────────┘
              │                                    │
              ▼                                    ▼
┌────────────────────────────┐        ┌─────────────────────┐
│ Session Service            │◄──────►│ Event Streamer (SSE)│
│ (`internal/agent/app`)     │        │ (`internal/output`) │
└───────┬──────────┬─────────┘        └─────────────────────┘
        │          │
        │          ▼
        │   ┌────────────────────────────┐
        │   │ Context Service            │
        │   │ (`ContextComposer`)        │
        │   │  • Static Registry         │
        │   │  • Dynamic Runtime         │
        │   │  • State Store Adapter     │
        │   └──────────┬─────────────────┘
        │              │
        ▼              ▼
┌──────────────┐   ┌────────────────────┐
│ Tool Runtime │   │ LLM Runner         │
│ (`internal/` │   │ (`internal/llm`)   │
│ tools/app)   │   └─────────┬──────────┘
└──────────────┘             │
        ▲                    │
        └─────────┬──────────┘
                  │
          ┌───────▼────────┐
          │ Meta Steward   │
          │ (batch jobs)   │
          └────────────────┘
```

- **Context Orchestrator（Context Service）**：面向会话循环的核心编排器，负责按需拉取、合成、裁剪上下文，并在回合结束后写回；在 `internal/agent/app` 中以常驻服务启动（系统启动即加载静态配置、初始化状态缓存）。
- **Static Layer Registry**：集中管理目标、规则、SOP、人格、工具配置等静态元素，支持版本化与多租户；通过 `configs/context` + `internal/config` 组合提供访问接口。
- **Dynamic Runtime & Event Bus**：聚合交互反馈、世界状态、内部计划的实时数据，提供订阅/发布接口；复用 `internal/agent/app/runtime` 的事件模型，并在 `internal/session` 中落盘。
- **Meta Layer Steward**：负责记忆筛选、知识库治理、人设演化策略，提供异步批处理能力；实现为 `cmd/context-steward` 后台任务或托管于 `internal/analytics` 服务中。

### 2.1 Agent Server 架构图

```
                ┌───────────────────────────────────────────┐
                │               HTTP/gRPC API               │
                └───────────────┬───────────────────────────┘
                                │
                                ▼
                ┌───────────────────────────────────────────┐
                │ Session Manager (`internal/agent/app`)    │
                │  • Session Registry / Router              │
                │  • SSE Stream hookup                      │
                │  • Turn lifecycle hooks                   │
                └───────────────┬───────────────────────────┘
                                │
       ┌────────────────────────┴────────────────────────┐
       │                                                 │
       ▼                                                 ▼
┌────────────────────┐                         ┌──────────────────────┐
│ Context Service    │                         │ Runtime Executor     │
│ (`ContextComposer`)│                         │ (`internal/agent/    │
│  • Static registry │                         │  runtime`)           │
│  • Dynamic state   │                         │  • Planner           │
│  • LLM msg builder │                         │  • Tool runner       │
└─────────┬──────────┘                         └──────────┬───────────┘
          │                                             │
          │                                             ▼
          │                                   ┌──────────────────────┐
          │                                   │ LLM Client           │
          │                                   │ (`internal/llm`)     │
          │                                   └──────────┬───────────┘
          ▼                                             │
┌────────────────────┐                                   │
│ State Store +      │◄──────────────────────────────────┘
│ Event Bus          │
└────────────────────┘

Meta 组件（记忆/知识/Persona）监听事件 Bus，并写回配置/知识库。
```

### 2.2 运行流程图

```
┌───────────────┐
│ 服务启动      │
└──────┬────────┘
       │ 初始化 Context Service（加载静态配置 + 建立 StateStore 连接）
       ▼
┌───────────────┐
│ 会话创建      │
└──────┬────────┘
       │ Session Manager 注册回合钩子，Context Service 拉取初始状态
       ▼
┌───────────────┐
│ 事件循环开始  │
└──────┬────────┘
       │
       │1. 用户输入或系统事件触发 → 写入 Event Bus / SSE
       │2. Session Manager 通知 Context Service 构造下一条 LLM message
       ▼
┌───────────────┐
│ LLM 调用      │
└──────┬────────┘
       │ Runtime Executor 将 Context Service 输出的 message 送入 LLM
       │ LLM/工具执行结果回传
       ▼
┌───────────────┐
│ 状态写回      │
└──────┬────────┘
       │ Context Service 接收执行结果 → 更新 StateStore/快照
       │ 同步事件给 SSE（保持对用户可见）
       ▼
┌───────────────┐
│ 回合结束?     │◄───否───┐
└──────┬────────┘         │
       │是                 │
       ▼                   │
┌───────────────┐          │
│ 会话结束      │          │
└──────┬────────┘          │
       │ Context Service 输出最终 LLM message（总结/告别），归档快照
       ▼                   │
┌───────────────┐          │
│ Meta 批处理   │◄────────┘
└───────────────┘
```

## 3. 静态 Context 设计

静态层以“配置即代码”的思路管理，建议使用 **YAML/JSON Schema + GitOps**。

| 模块 | 关键字段 | 存储 | 说明 | ALEX 集成点 |
| --- | --- | --- | --- | --- |
| 目标（Goal Profiles） | `long_term`, `mid_term`, `success_metrics` | `configs/context/goals/*.yaml` | 支持多 persona / 多任务的目标模板；引用 KPI 计算器。 | 在 `internal/agent/app/session_service.go` 合成回合目标提示 |
| 价值与规则（Policy Sets） | `hard_constraints`, `soft_preferences`, `reward_hooks` | Policy Engine（OPA 或自研 DSL） | 硬约束在推理前执行过滤，软偏好在评分阶段调整权重。 | 与 `internal/approval`、`internal/tools/guardrails` 联动 |
| 知识与经验（Knowledge Packs） | `sop_refs`, `rag_collections`, `memory_keys` | 向量库 + 文档仓库（S3/MinIO） | SOP 以 Markdown + 元数据（标签、版本）管理，RAG 通过 collection ID 编排。 | 在 `internal/rag` 注册 collection，输出到 prompt patch |
| 人格与偏好（Persona Config） | `tone`, `risk_profile`, `decision_style` | Config Service | Persona 由用户配置和系统默认合成；支持层级覆盖。 | 由 `internal/prompts` 生成系统消息模板，并暴露至 CLI/Web 预设 |
| 世界与资源（World/Tool Map） | `environment`, `capabilities`, `limits`, `cost_model` | Tool Registry + Feature Flag | 对接内部工具目录，声明额度、速率限制与权限。 | 结合 `internal/toolregistry` 与 `internal/observability/cost` 计算 |

**拉取策略**：在每次会话初始化时，Context Orchestrator 根据 `tenant_id`, `agent_id`, `session_type` 计算一个合并视图，并缓存于 Redis（复用 `internal/cache`），TTL 依据场景（如 1 小时）。缓存未命中时回退到 GitOps 配置，并记录 Prometheus 指标 `context_static_cache_miss_total`。

## 4. 动态 Context 设计

动态层依赖事件驱动架构，将交互数据拆成三个流，并复用 ALEX 现有的事件模型（`internal/agent/domain/events.go`）。所有数据既要满足**实时推理用的快速读取**，也要兼顾**会话回放、故障排查时的历史追溯**，因此读写策略采用“事件日志 + 快照”的双轨结构：

- 事件日志：写入 Kafka/Redpanda，保留完整回合操作；用于回放与审计。
- 会话快照：在 `internal/session/state_store` 中维护每回合后的合成状态，支持随机读取与分页查询，满足 Web 控制台与 API 的 Session 详情需求。

事件日志与快照通过 `session_id + turn_id` 对齐，Meta 层可按需回放或重建任意时刻的内部状态。

**Session / Message 对齐策略**

- **Session 视角**：面向用户的交互行为，以 SSE 流展示每次输入、输出、工具反馈；Session Manager 使用 `session_id + turn_id (SSE)` 来驱动 UI 时间线和回放。
- **LLM 消息视角**：Context Service 内部维护 `llm_turn_seq`，保证每次发往模型的 message 有稳定的编号，并与 `turn_id` 建立映射。Session 记录中增加 `llm_turn_seq` 字段，便于客服/研发定位模型级问题。

最终 Session 记录与用户体验仍以 SSE 为准，同时在每条记录上“关联 LLM message”，满足“Session 是看用户行为，但可以关联模型消息”的要求。

### 4.1 Context 模块与 LLM Message 的关系

Context 模块（Context Service）不仅维护状态，还负责与 LLM 请求 message 的生产强耦合：

1. **服务启动即初始化**：`ContextComposer` 在 agent server 启动时即加载静态配置、建立 StateStore / EventBus 连接，并注册到 Session Manager 的生命周期钩子。无需等待第一条请求才能构造上下文，避免冷启动抖动。
2. **事件驱动取数**：当 Session Manager 收到用户输入、工具完成或系统 webhook 时，会发出上下文事件。Context Service 监听事件并聚合“静态设定 + 最新动态状态 + 执行摘要”，即时生成下一条 LLM request message。事件系统继续沿用现有实现（SSE + Kafka），因此对“用户行为展示”的全局能力无侵入改动。
3. **逐轮回写**：Runtime Executor 完成 LLM 调用或工具操作后，将执行结果（输出文本、工具返回、奖励信号）写回 Context Service。后者更新内部 state（计划、信念、世界状态）并生成“下一条 message”或“收敛/结束 message”。这样每一轮都形成闭环：事件 → message → 执行 → 状态更新 → 新 message。
4. **消息模板化**：Context Service 对外只输出结构化的 `ContextMessage`：`{llm_turn_seq, role, content_parts, attachments}`，Runtime Executor 根据模型适配器转换为具体协议（OpenAI/Anthropic 等）。消息生产完全由 Context Service 控制，确保 state 与 prompt 一致。
5. **与 Agent 服务的关系**：Session Manager 控制节奏（何时需要下一条消息），Context Service 提供内容，Runtime Executor 执行。三者通过接口拆分：
   - `SessionHooks`: `OnUserInput`, `OnToolResult`, `OnLLMResult`。
   - `ContextComposer.NextMessage(ctx, sessionID, triggerEvent)` → `ContextMessage`。
   - `ContextComposer.RecordOutcome(ctx, sessionID, outcome)` 写回状态。

### 4.2 ReAct 循环流程

为保证“观察→思考→行动”的链路在 SSE Session、LLM 消息与工具执行之间保持一致，我们将 ReAct 循环具象成以下步骤（每轮均绑定 `turn_id` 与 `llm_turn_seq`）：

1. **Observe（环境/用户输入）**
   - Session Manager 捕获新的 SSE 事件（用户输入、系统回调、工具 streaming output），写入 `context.feedback` 主题，并调用 `ContextComposer.NextMessage`。
   - Context Service 读取事件对应的快照 diff、奖励信号及世界状态增量，生成 `ObservationBundle`（结构化 JSON），同时把 `turn_id ↔ llm_turn_seq` 关系写入 `state_store.turn_index`。

2. **Think（推理/计划）**
   - Context Service 基于 `ObservationBundle`、静态设定和内部计划，生成一条带 `thought` 段落的 LLM message，并附加“候选工具调用计划”（Action Schema）。
   - Runtime Executor 将 message 传入 LLM，若模型返回思考内容 + 工具调用指令，则由 Plan Manager 更新内部 `plans/beliefs` 并写入快照 diff，形成本轮的 `thought_log`。

3. **Act（执行工具/给出回答）**
   - 若需要工具调用，Runtime Executor 依据 Action Schema 调 `internal/tools`，并将执行日志经 SSE 流反馈给用户；执行结果通过 `SessionHooks.OnToolResult` 回传 Context Service。
   - 如果模型直接给出回答，Runtime Executor 将文本写入 SSE output，同时调用 `ContextComposer.RecordOutcome`，把响应、评分、成本等写回状态。

4. **Loop Control（判断是否继续）**
   - Context Service 根据 `thought_log`、工具结果和奖励信号评估是否满足停机条件（例如计划叶节点完成、用户确认、硬性回合上限）；若需继续，生成下一条 LLM message 并递增 `llm_turn_seq`。
   - Session Manager 将该判定同步到 SSE 时间线（例如展示“继续思考”/“等待工具”状态），保障用户视角与内部回合一致。

该流程将 ReAct 的核心阶段与 ALEX 的事件、存储、回放体系对齐，使得任何中间状态都可以通过 `turn_id + llm_turn_seq` 回溯，并在 UI 上直观呈现“观察-思考-行动-再观察”的链路。

该设计使得 Context 模块“包含 state + LLM 消息逻辑”，并与 agent 服务形成清晰编排关系。

在此基础上，我们将交互数据拆成三个流：

1. **交互反馈流（Interaction Feedback Stream）**
   - 来源：LLM 输出、用户输入、工具调用日志（由 `internal/output/streamer` 统一产出）。
   - 事件格式：`{session_id, turn_id, action, result, reward_signal}`。
   - 写入：Kafka/Redpanda 主题 `context.feedback`，供实时评分与回放，同时在 `internal/session/filestore` 中追加结构化 JSON 备份，并触发快照更新（见“Session 快照策略与读取接口”）。

2. **世界状态更新流（World State Stream）**
   - 来源：任务管理系统、文档协作、外部 webhook。
   - 状态缓存：使用 Event Sourcing，维护 `world_state` 表（PostgreSQL/Redis JSON）。
   - 每回合拉取时根据订阅的实体（如 TODO 列表、项目元数据）生成增量 diff。

3. **内部状态更新流（Internal State Store）**
   - 结构化表示计划/信念：
     ```json
     {
       "plans": [{"id": "task-1", "status": "in_progress", "children": [...] }],
       "beliefs": [{"statement": "user_prefers_outline", "confidence": 0.8}],
       "uncertainties": [{"topic": "budget", "next_step": "ask_user"}]
     }
     ```
   - 存储：Session Scope 的 KV（Redis JSON / MongoDB），支持部分字段 TTL；优先复用 `internal/storage/state` 接口，提供嵌入式实现兼容离线模式。
   - 快照：每次状态更新时在 `internal/session/state_store` 中持久化 `{turn_id, plans, beliefs, uncertainties}` 的快照，并写出 diff 日志，便于 UI 对比与调试；失败会记录到 `context.snapshot_error_total` 指标并触发告警。
   - 更新：推理循环结束后由 Plan Manager 写回（新增 `internal/agent/app/plan_manager.go`），或由工具执行器根据结果更新。

**合成逻辑**：
- 每回合构造 Prompt 时，Orchestrator 会：
  1. 聚合最新反馈摘要（例如最近 5 条）、关键奖励信号。
  2. 将世界状态的关键信息转化为结构化提示（表格/要点）。
  3. 注入内部计划节点，提醒模型当前进度与下一步。
- 同时产出一个 `turn_journal`（写入 `internal/analytics/journal`），供 Meta 层评估与回放工具使用，并附带最近两次快照的 diff 信息，便于控制台直接展示“本轮更新了哪些状态”。

**Session 快照策略与读取接口**：
- 新增 `internal/session/api/snapshots.go`，提供 REST/gRPC 端点：
  - `GET /sessions/{id}/snapshots?cursor=`：分页返回快照元数据（含时间戳、turn_id（SSE 序列）、变更摘要、`llm_turn_seq` 映射）。
  - `GET /sessions/{id}/turns/{turn_id}`：返回指定回合的完整状态与 diff，若同一 `llm_turn_seq` 包含多条 SSE，则以聚合结果展示并标记关联片段。
  - `POST /sessions/{id}/replay`：触发后台回放，基于事件日志重建状态并写入沙箱（复用 `internal/session/replayer`）。
- 控制台前端复用这些接口实现时间轴视图，支持研发/运营快速定位问题并下载快照；CLI 将新增 `alex sessions pull --turn` 命令，并允许通过 `--llm-turn` 参数按模型回合过滤。

## 5. Meta-Context 机制

Meta 层以批处理和后台服务为主，关键组件如下：

### 5.1 记忆选择器（Memory Selector）

- 输入：`turn_journal`、奖励信号、用户反馈情绪分析（可调用 `internal/analytics/affect`）。
- 策略：
  - 基于启发式（例如高奖励 + 用户明确肯定）或模型评分，将事件提升为“长期记忆候选”。
  - 采用 `write_back_queue` 异步写入长期 Memory（PostgreSQL/Weaviate），队列复用 `internal/observability/queue`；写回前通过 `internal/rag` 进行 embedding 去重。
  - 设置去重与冲突检测（Jaccard/L2 距离 + 知识 ID），冲突事件进入 `internal/diagnostics` 并触发 PagerDuty 告警。

### 5.2 知识库治理器（Knowledge Steward）

- 功能：
  - 对 Knowledge Packs 执行版本扫描，标记过期（基于有效期、引用计数）；结合 `internal/analytics/catalog` 记录引用频率。
  - 检测冲突：对同一标签下的文档运行差异比对，触发人工审核工作流（复用 `scripts/create-doc-pr.sh` 自动生成 PR）。
  - 自动晋升：当临时检索结果被多次写入记忆且通过质量评估时，生成 PR 推送至知识库仓库，并通过 `internal/observability/alerts` 通知文档负责人 Slack webhook。

### 5.3 人格演化管理器（Persona Evolution Manager）

- 依据用户行为（满意度、干预频率）与系统指标（风险事件）评估是否调整 Persona 参数，数据来源为 `internal/analytics/feedback`。
- 采用“渐进权重”策略：`persona = α * user_profile + (1-α) * system_defaults`，α 在 [0.3, 0.7] 内动态调整；变更写回 `configs/context/personas/overrides/*.yaml` 并触发热更新。
- 加入“守护规则”：若连续 N 次（默认 3 次）检测到自相矛盾输出，则触发回滚到上一个 Persona 版本，并在 `docs/changelog/` 生成自动记录。

## 6. 数据模型与存储选型

| 数据类型 | 推荐存储 | 备注 | ALEX 集成 |
| --- | --- | --- | --- |
| 静态配置 | Git + Config Service（etcd/Consul） | 版本化、可审计。 | 通过 `internal/config/loader` 热更新 |
| 会话状态 | Redis JSON / MongoDB + MinIO（快照归档） | 支持部分字段 TTL 与并发修改，并定期将冷数据快照归档到对象存储以供审计。 | 在 `internal/session` 新增 `StateStore` 接口实现；CLI/Web 提供回放与快照导出 |
| 事件流 | Kafka/Redpanda | 提供订阅、回放、监控。 | 借助 `internal/observability/tracing` 打标签，方便回放 |
| 长期记忆 | PostgreSQL（结构化）+ 向量库（Milvus/Weaviate） | 支持检索与精确查询。 | 接入 `internal/rag` 模块，新增 `MemoryCollection` |
| 知识文档 | S3/MinIO + Git（Markdown） | 与 RAG 管道对接。 | 与 `docs/` 仓库同步，结合 `scripts/publish-docs.sh` |

## 7. 编排流程

1. **Session 初始化**
   - Orchestrator 读取静态配置合成初始上下文缓存，并写入 Prometheus 计时指标。
   - 创建 Session Scope 的内部状态存根，调用 `StateStore.Init(sessionID)`。

2. **每轮推理循环**
   - 拉取动态层最新数据 → 构造 prompt → 调用 LLM；Context Composer 将静态/动态/计划片段拼装成分段提示。
   - 工具执行结果、用户反馈写入事件流，并同步到 `internal/session/filestore`。
   - 生成 `turn_journal`，更新内部状态存储（Plan Manager 负责更新计划节点）。

3. **回合后处理**
   - 将 `turn_journal` 投递至 Meta 层队列，并记录在 `internal/analytics/events`。
   - 若会话结束，执行记忆写回、计划归档（生成 `plans/{session}.json`），导出最终快照到对象存储（`snapshots/{session}/final.json`），并释放工具连接、关闭 sandbox session。

4. **异步批处理**
   - Meta 层定时任务（如每小时）执行记忆筛选、知识库治理、人格评估，任务入口为 `cmd/context-steward`。
   - 触发必要的通知（例如需要人工审核的冲突、Persona 回滚），通过 `internal/observability/alerts` 推送到 PagerDuty/Slack。

## 8. 监控与评估

- **上下文完整性指标**：静态合成耗时、配置缺失率、版本漂移率，导出到 `internal/observability/metrics/context.go`。
- **动态响应指标**：反馈写入延迟、世界状态同步延迟、计划一致性检测、快照写入成功率（`context.snapshot_error_total` 反向指标）、回放重建耗时；可在 Grafana 创建“Context Runtime”看板。
- **Meta 质量指标**：记忆写回通过率、知识冲突解决时长、人设回滚次数，纳入每周 `docs/sprints/` 评审。
- **外部成效指标**：任务完成率、用户满意度、平均工具调用成本，结合 `tests/e2e` 回放与 `evaluation/` 基准测试追踪。

通过这些指标，可建立 Dashboard（Grafana + Prometheus + OpenSearch），持续评估上下文系统的有效性。

## 9. 实施路线图

1. **Phase 1：静态层上线（Sprint 28）**
   - 搭建 Context Orchestrator MVP，支持静态配置合成，落地在 `internal/agent/app/context_composer.go`。
   - 引入 Policy Engine，确保规则与工具边界生效；完成 CLI/Web 预设文件迁移至 `configs/context/`。

2. **Phase 2：动态层接入（Sprint 29-30）**
   - 引入事件流，完成交互反馈与世界状态同步，扩展 `internal/session/filestore` 支持结构化条目。
   - 实现内部计划数据结构，并在提示中使用；为 Web dashboard 增加“Plan View”模块。
   - 上线 Session 快照与回放能力：落地 `internal/session/state_store`、`internal/session/api/snapshots.go`、CLI `alex sessions pull --turn`。

3. **Phase 3：Meta 层闭环（Sprint 31-32）**
   - 上线记忆筛选与知识库治理流水线，部署 `cmd/context-steward` 定时任务。
   - 引入 Persona 演化机制与保护策略，完善回滚/告警流程。

4. **Phase 4：运营与优化（持续迭代）**
   - 完善监控告警、自动化测试（上下文回放、回归验证）；在 `tests/e2e/context_replay_test.go` 添加回放场景。
   - 根据指标迭代奖励模型、计划管理策略，定期在 `docs/analytics/context_report.md` 输出周报。

## 10. 预期收益

- **决策更稳健**：目标、规则、资源明确后，智能体行为风格更可控。
- **记忆更可控**：通过 Meta 层的筛选与治理，避免“信息垃圾场”。
- **运营更高效**：标准化 Schema + GitOps，让产品、运营、研发协同调整上下文。
- **可持续演进**：三层架构为日后接入多模态、跨平台智能体奠定基础。

## 11. 查漏补缺清单（上线前 Checklist）

| 维度 | 关键检查项 | 负责人 | 状态追踪 |
| --- | --- | --- | --- |
| Context Service | `ContextComposer` 是否在服务启动时完成静态层缓存预热？`SessionHooks` 是否在 `internal/agent/app` 中全部注册？ | 平台组 | `internal/observability/metrics/context.go` 中的 `context_static_cache_miss_total` 需低于阈值 |
| Session/SSE 对齐 | SSE `turn_id` 与 `llm_turn_seq` 映射表是否在 `state_store.turn_index` 中持续写入、可回放？CLI/Web 是否能以任意 ID 检索？ | 体验组 | `internal/session/api/snapshots.go` e2e 回放用例通过 |
| ReAct 循环 | Observe/Think/Act/Loop Control 四阶段的事件是否全部进入 Kafka `context.feedback` 主题，并附带 `turn_journal`？ | Runtime 组 | Grafana“Context Runtime”看板显示全链路延迟 < 500ms |
| 快照/回放 | 每轮 diff 是否在 2 秒内写入 `StateStore`，失败是否触发 `context.snapshot_error_total` 告警？回放 API 是否能重建到指定 `turn_id`？ | Infra 组 | `tests/e2e/context_replay_test.go` 新增用例通过 |
| Meta Steward | 记忆/知识/Persona 三个批处理任务是否具备回滚策略与 PagerDuty 告警？`cmd/context-steward` 是否记录操作日志？ | 数据组 | `docs/analytics/context_report.md` 周报项覆盖三大指标 |
| 运营交付 | 文档、图示是否同步到内网知识库？CLI/Web 说明是否在 `docs/product/` 更新？ | 产品组 | 在发布清单中附带链接，PR Template 勾选完毕 |

> 说明：Checklist 以 `docs/releases/context_upgrade.md` 为基准文档，发布前需逐项勾选并附带验证证据（Grafana 截图、测试日志等），避免遗漏关键依赖。若某项延期需记录 owner、补偿措施及预计完成时间。

