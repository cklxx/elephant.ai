# Event Display Architecture V2

## Requirements

1. 按时间顺序展示事件流
2. Subagent 事件按 subagent tool 调用聚合
3. 多个 subagent 执行过程聚合到触发该组 subagent 的事件位置

## 业界最佳实践参考

### 1. 事件驱动架构模式 (Event-Driven Multi-Agent)
- **来源**: [Confluent Event-Driven Multi-Agent Systems](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
- **关键概念**:
  - Orchestrator-Worker 模式: 一个触发事件产生多个工作单元
  - Topic-based 分组: 相关事件共享视觉空间
  - Partition-based 分组: 相关事件在视觉上的邻近性

### 2. 前端可视化模式 (Agentic AI Frontend Patterns)
- **来源**: [LogRocket Agentic AI Patterns](https://blog.logrocket.com/agentic-ai-frontend-patterns/)
- **关键概念**:
  - 双面板界面: 一个面板显示推理过程，另一个显示外部动作
  - 时间线视图: 显示哪个 agent 在何时做了什么
  - 进程图: 可视化 agent 协作和任务流
  - 实时状态指示器: 透明的"思考"状态

### 3. 事件流聚合模式 (Event Stream Aggregation)
- **来源**: [LlamaIndex Multi-Agent Workflows](https://developers.llamaindex.ai/python/framework/understanding/agent/multi_agent/)
- **关键概念**:
  - 按 Agent 分组: consumer group 模式
  - 按任务/工作流分组: Plan → Execute → Review
  - 渐进式披露: 从高层次分组开始，可下钻到单个 agent 事件

## 架构设计

### 核心概念: Trigger-Execution 模式

```
用户输入
    ↓
[Core Agent 执行]
    ↓
Subagent Tool Call (Trigger Event) ← 聚合点
    ↓
[Subagent Group 执行]
    ├─ Subagent 1
    ├─ Subagent 2
    ├─ Subagent 3
    └─ Subagent N
    ↓
继续 Core Agent 执行
```

### 数据流

```
Raw Events → Partition → Build Timeline → Render
                ↓
    ├─ Core Events → mainStream[]
    ├─ Subagent Events → threads[]
    └─ Pending States → pendingTools/nodes Maps
                ↓
    Match Subagent Groups to Trigger Events
                ↓
    Build Unified Timeline
```

### 事件分类

| 层级 | 事件类型 | 处理方式 |
|------|---------|---------|
| **Level 1** | System/Diagnostic | 过滤，不显示 |
| **Level 2** | User Input | 用户消息气泡 |
| **Level 3** | Core Agent Output | AI 输出流 |
| **Level 4** | Core Tool Call | 工具卡片 |
| **Level 5** | Subagent Trigger | 聚合点，展开显示子 agent |
| **Level 6** | Subagent Events | 聚合在卡片内 |

### Subagent 聚合规则

1. **识别 Trigger**: `workflow.tool.completed` + `tool_name: "subagent"`
2. **提取 Parent Task ID**: trigger 事件的 `task_id` 作为 parent_task_id
3. **收集 Subagent 线程**: 所有 `parent_task_id` 匹配的 subagent 事件
4. **聚合位置**: 在 trigger 事件的位置展开显示
5. **时间排序**: Subagent 线程按 `firstSeenAt` 排序

## 组件架构

```typescript
// 统一时间线条目
type TimelineEntry =
  | { kind: "userInput"; event: InputEvent; ts: number }
  | { kind: "aiOutput"; event: OutputEvent; ts: number }
  | { kind: "toolCall"; event: ToolEvent; ts: number }
  | { kind: "subagentGroup"; trigger: ToolEvent; threads: SubagentThread[]; ts: number }
  | { kind: "finalResult"; event: ResultEvent; ts: number };
```

### 渲染流程

```
ConversationEventStream
└── buildUnifiedTimeline(events)
    ├── partitionEvents() → mainStream + subagentThreads + pending
    ├── findSubagentTriggers(mainStream) → trigger[]
    ├── groupSubagentsByTrigger(triggers, subagentThreads) → groups[]
    └── mergeIntoTimeline(mainStream, groups) → TimelineEntry[]

Render
└── TimelineEntry[]
    ├── "userInput" → UserMessage
    ├── "aiOutput" → AIMessage
    ├── "toolCall" → ToolCard
    ├── "subagentGroup" → SubagentGroupCard (多个 AgentCard)
    └── "finalResult" → ResultCard
```

## 关键改进

### 1. 移除时间戳排序混乱
- 所有事件按原始时间顺序排列
- Subagent group 插入在 trigger 事件的位置，而非按 thread 的 firstSeenAt

### 2. 清晰的 Trigger-Execution 关系
- Subagent group 视觉上从属于 trigger tool call
- 可折叠/展开设计（渐进式披露）

### 3. 简化的状态管理
- 不使用 useRef 跟踪已渲染项
- 纯函数构建时间线，相同输入 = 相同输出
- pending 状态单独处理（显示在底部）

### 4. 响应式更新
- 新事件到达时重新构建整个时间线
- React 的 diff 算法优化实际 DOM 更新

## 实现步骤

1. **重构 partitionEvents()**
   - 分离 main stream 和 subagent 聚合
   - 识别 subagent trigger 事件

2. **创建 buildUnifiedTimeline()**
   - 将 subagent groups 绑定到 triggers
   - 构建统一的 TimelineEntry[]

3. **简化渲染逻辑**
   - 根据 entry.kind 渲染不同组件
   - SubagentGroup 组件渲染多个 AgentCard

4. **添加交互**
   - 展开/折叠 subagent group
   - 查看 subagent 详情

## 文件结构

```
components/agent/
├── ConversationEventStream.tsx    # 主组件，统一时间线
├── Timeline/
│   ├── index.tsx                  # 时间线渲染
│   ├── UserInputEntry.tsx         # 用户输入
│   ├── AIOutputEntry.tsx          # AI 输出
│   ├── ToolCallEntry.tsx          # 工具调用
│   └── SubagentGroupEntry.tsx     # Subagent 组（关键）
└── hooks/
    └── useEventPartition.ts       # 事件分区逻辑
```

## 测试策略

| 场景 | 测试内容 |
|------|---------|
| 单 subagent | 1 trigger → 1 subagent card |
| 多 subagent | 1 trigger → 5 subagent cards |
| 多批次 | 2 triggers → 2 groups，每组 N cards |
| 流式更新 | 事件逐步到达，时间线正确更新 |
| 折叠展开 | 用户可折叠/展开 subagent group |
