# Track 1: 推理与 Agent 核心循环 — 详细 ROADMAP

> **Parent:** `docs/roadmap/roadmap-lark-native-proactive-assistant.md`
> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-01

---

## 概述

Online Agent 的推理引擎，是整个系统的心脏。本 Track 覆盖 ReAct 执行循环、LLM 推理引擎、上下文工程、记忆系统四大模块。

**关键路径：** `internal/agent/` · `internal/llm/` · `internal/context/` · `internal/memory/` · `internal/rag/`

---

## 1. ReAct 核心循环

> `internal/agent/domain/react/`

### 现状

- Think → Act → Observe 基础循环已实现
- 状态机管理执行阶段（prepare → execute → summarize → persist）
- 并行工具调用 + 结果去重 + 超时控制
- 结构化事件流（强类型 domain events）
- 基础子 Agent 委派（subagent 工具）

### M0: 循环可靠性

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 异常路径全覆盖 | LLM 超时、工具失败、上下文溢出、token limit 每条路径有明确恢复策略 | ⚙️ 基础实现 | `react/runtime.go`, `react/engine.go` |
| 断点快照 | 可序列化的循环状态快照，支持持久化到磁盘/DB | ⚙️ 部分 | `internal/session/state_store` |
| 重启自动续跑 | 进程重启后从最近快照恢复执行，无需用户重新发送请求 | ❌ 待实现 | `internal/agent/app/coordinator.go` |
| 优雅退出 | SIGTERM 时完成当前工具调用、保存状态、通知用户 | ❌ 待实现 | `react/runtime.go` |
| 事件一致性 | 事件 ID 全局唯一 + 幂等消费，断线重连不丢事件 | ✅ 已实现 | `internal/agent/domain/events.go` |

### M1: 智能规划

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Replan 机制 | 工具失败/结果不符预期时自动 replan，产出 `ReplanEvent` | ❌ 待实现 | `react/engine.go` |
| 子目标分解 | 复杂任务拆分为子目标链（DAG），按拓扑序执行 | ❌ 待实现 | 新增 `internal/agent/planner/` |
| 执行路径评估 | 多条候选路径打分（预期成功率 × 成本），选最优 | ❌ 待实现 | `internal/agent/planner/scorer.go` |
| 计划可视化 | 将执行计划结构化输出给前端展示（步骤/依赖/状态） | ❌ 待实现 | `react/workflow.go` |
| 用户干预点 | 计划生成后暂停等用户确认，支持修改后再执行 | ✅ 已实现 | `react/runtime_user_input.go`, `react/runtime.go:maybeTriggerPlanReview()` |

### M2: 多 Agent 协作

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Agent-as-Tool 增强 | 子 Agent 执行结果结构化返回，支持流式 | ⚙️ 基础实现 | `internal/tools/builtin/orchestration/` |
| Agent 间消息通道 | 结构化消息传递（请求/响应/广播），支持异步 | ❌ 待实现 | 新增 `internal/agent/orchestration/` |
| 任务分配策略 | 按 Agent 能力画像分配子任务，支持负载均衡 | ❌ 待实现 | `internal/agent/orchestration/dispatcher.go` |
| 冲突仲裁 | 多 Agent 结果矛盾时的合并/投票/仲裁策略 | ❌ 待实现 | `internal/agent/orchestration/arbiter.go` |
| Agent 状态同步 | 多 Agent 间共享执行上下文和中间结果 | ❌ 待实现 | `internal/agent/orchestration/state.go` |

---

## 2. LLM 推理引擎

> `internal/llm/`

### 现状

- 5 个 client 实现（Anthropic/OpenAI-compatible/OpenAI-Responses/ARK-Antigravity/Ollama），通过 OpenAI-compatible 覆盖 DeepSeek/OpenRouter 等 7+ 提供商
- Extended Thinking（Claude）、Reasoning Effort（ARK o-series）
- 全提供商 SSE 流式输出
- 指数退避重试 + 速率限制
- 自动工厂选择最佳可用提供商

### M0: 基础稳固

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 统一 LLM 接口 | 所有提供商实现统一 interface | ✅ 已实现 | `internal/llm/` |
| 流式输出 | SSE streaming 到所有交互面 | ✅ 已实现 | `internal/llm/` |
| 重试与降级 | 指数退避、速率限制、provider 级降级 | ✅ 已实现 | `internal/llm/retry_client.go` |
| Extended Thinking | Claude extended thinking 支持 | ✅ 已实现 | `internal/llm/anthropic_client.go` |

### M1: 智能路由

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 动态模型选择 | 基于任务类型/复杂度/上下文长度自动选择最优模型 | ❌ 待实现 | `internal/llm/router.go` |
| Token 预算管理 | 按任务/会话维度的 token 预算，超预算自动降级模型 | ❌ 待实现 | `internal/llm/budget.go` |
| 温度/采样策略 | 按任务类型调整采样参数（创意高温 vs 精确低温） | ❌ 待实现 | `internal/llm/sampling.go` |
| 成本核算增强 | 实时按模型/任务/用户维度统计成本 | ⚙️ 部分 | `internal/observability/` |
| 提供商健康检测 | 实时探测提供商可用性，不可用时自动切换 | ❌ 待实现 | `internal/llm/health.go` |

### M2: 高级推理

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 多路径采样投票 | 关键决策点多次采样 + 投票，提升可靠性 | ❌ 待实现 | `internal/agent/domain/react/voting.go` |
| 置信度建模 | 结论绑定证据 + 置信度评分，低置信触发澄清 | ❌ 待实现 | `internal/agent/domain/confidence.go` |
| 不确定性传播 | "我不确定" 作为合法输出，触发用户澄清流程 | ❌ 待实现 | `react/engine.go` |
| 思维链缓存 | 相同推理模式的中间结果缓存，避免重复推理 | ❌ 待实现 | `internal/llm/thought_cache.go` |

---

## 3. 上下文工程

> `internal/context/`

### 现状

- System/Policy/Task/Memory 四层上下文分层拼装
- 动态摘要与压缩（超出 token 预算时自动压缩）
- Token 预算滑动窗口
- SOP 解析器（基于任务类型加载标准操作流程）
- Lark 聊天历史作为上下文注入

### M0: 基础完备

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 四层分层拼装 | System/Policy/Task/Memory 有序组装 | ✅ 已实现 | `manager.go` |
| 动态压缩 | 超预算自动摘要 | ✅ 已实现 | `manager_compress.go` |
| Token 预算窗口 | 滑动窗口管理上下文大小 | ✅ 已实现 | `manager_window.go` |
| SOP 解析 | 任务类型 → 标准操作流程 | ✅ 已实现 | `sop_resolver.go` |

### M1: 智能上下文

| 项目 | 描述 | 状态 | 路径 | OpenClaw Delta |
|------|------|------|------|------|
| 主动上下文注入 | 自动检测当前话题，主动从记忆中加载相关上下文 | ⚙️ 部分 | `manager_prompt.go` | |
| Lark 聊天历史注入 | 群最近 N 条消息自动加载 | ✅ 已实现 | `internal/channels/lark/chat_context.go` | |
| **Memory Flush-before-Compaction** | `AutoCompact` 前发布 `context.compact` 事件，触发 MemoryFlushHook 将即将压缩的对话提取关键信息落盘 | ❌ 待实现 | `manager_compress.go` → `events/` → `hooks/memory_flush.go` | **D3** |
| 上下文优先级排序 | 按相关性/新鲜度/重要性对上下文片段排序 | ❌ 待实现 | `manager.go` | |
| 成本感知裁剪 | Token 预算驱动的上下文裁剪策略，优先保留高价值内容 | ❌ 待实现 | `manager_window.go` | |
| 跨会话上下文共享 | 相关会话间共享上下文片段 | ❌ 待实现 | `manager.go` | |

### M2: 上下文自治

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 上下文版本管理 | 上下文快照 + diff，支持回溯 | ❌ 待实现 | `manager.go` |
| 自适应压缩策略 | 按内容类型选择最优压缩方式（摘要/截断/采样） | ❌ 待实现 | `manager_compress.go` |
| 多源上下文融合 | Lark + Memory + RAG + 外部文档统一融合排序 | ❌ 待实现 | `manager.go` |

---

## 4. 记忆系统

> `internal/memory/` · `internal/rag/`

### 现状

- Postgres + 文件双存储（hybrid fallback）
- RAG 语义搜索（embedding + pgvector） + BM25 混合排序
- 自动 token 计数（tiktoken）
- 保留策略（自动过期清理）
- 基础 Lark/WeChat 消息自动入库

### M0: 记忆基线

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 对话记忆存储 | 每次对话自动持久化 | ✅ 已实现 | `memory/service.go` |
| 向量检索 | 语义搜索（chromem-go cosine similarity）；**无 BM25，无 pgvector** | ⚙️ 部分 | `rag/store.go` |
| 保留策略 | 过期自动清理 | ✅ 已实现 | `memory/retention.go` |
| 消息自动入库 | Lark 群聊消息自动存储 | ⚙️ 部分 | `internal/channels/lark/` |

### M1: 主动记忆

| 项目 | 描述 | 状态 | 路径 | OpenClaw Delta |
|------|------|------|------|------|
| **记忆目录结构化** | FileStore 从 ksuid 平铺重构为三层：`entries/`(原始) + `daily/`(日汇总) + `MEMORY.md`(长期事实)；Recall 分层优先：长期事实 > 日汇总 > entries | ❌ 待实现 | `memory/file_store.go` → `LayeredFileStore` | **D5** |
| **日汇总自动生成** | session.ended 事件触发，按日聚合 entries 生成 daily/YYYY-MM-DD.md | ❌ 待实现 | `memory/daily_summarizer.go` | **D5** |
| **长期事实提炼** | 每日 cron 从日汇总中提取重复出现 ≥3 次的事实/偏好写入 MEMORY.md | ❌ 待实现 | `memory/longterm_extractor.go` | **D5** |
| **旧格式迁移** | 启动时自动将 ksuid.md 文件 move 到 entries/ 子目录，idempotent | ❌ 待实现 | `memory/migration.go` | **D5** |
| 决策记忆 | 记录关键决策（选择了什么 + 为什么 + 上下文），供未来参考 | ❌ 待实现 | `memory/decision_store.go` | |
| 实体记忆 | 从对话中提取人/项目/概念实体，构建实体关系 | ❌ 待实现 | `memory/entity.go` | |
| 记忆检索增强 | 结合 recency/frequency/relevance 三维排序 | ⚙️ 部分 | `rag/retriever.go` | |
| 记忆质量评估 | 检测过时/矛盾/冗余记忆，标记待清理 | ❌ 待实现 | `memory/quality.go` | |
| Lark 消息全量入库 | 所有监听群的消息自动结构化存储 | ⚙️ 部分 | `internal/channels/lark/` | |

### M2: 学习型记忆

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 用户偏好学习 | 从交互模式中提取用户偏好（语言/格式/工具/风格） | ❌ 待实现 | `memory/preferences.go` |
| 任务模式识别 | 识别重复任务模式，构建快捷路径 | ❌ 待实现 | `memory/patterns.go` |
| 记忆纠错 | 用户反馈驱动的记忆修正与遗忘 | ❌ 待实现 | `memory/correction.go` |
| 知识蒸馏 | 从大量对话记忆中蒸馏出结构化知识 | ❌ 待实现 | `memory/distill.go` |

### M3: 主动知识管理

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 知识图谱 | 实体/关系/事件三元组存储与推理 | ❌ 待实现 | `memory/knowledge_graph.go` |
| 记忆一致性维护 | 自动检测并解决矛盾记忆 | ❌ 待实现 | `memory/consistency.go` |
| 主动知识推荐 | 基于当前任务主动推荐可能有用的历史知识 | ❌ 待实现 | `memory/recommend.go` |

---

## 进度追踪

| 日期 | 模块 | 更新 |
|------|------|------|
| 2026-02-01 | All | Track 1 详细 ROADMAP 创建。ReAct 循环和 LLM 引擎基础已实现，主要缺口在 replan、智能路由、多 Agent 协作。 |
| 2026-02-01 | All | 实现审计修正：LLM providers 描述更新（5 client 覆盖 7+ 提供商）；事件一致性 ⚙️→✅；用户干预点 ⚙️→✅；向量检索 ✅→⚙️（chromem-go，无 BM25/pgvector）。 |
| 2026-02-01 | 上下文/记忆 | OpenClaw D3 集成：§3 M1 新增 Memory Flush-before-Compaction 项。D5 集成：§4 M1 新增记忆目录结构化四项（LayeredFileStore + 日汇总 + 长期事实提炼 + 旧格式迁移）。 |
