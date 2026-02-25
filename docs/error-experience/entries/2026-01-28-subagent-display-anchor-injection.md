# Subagent 展示架构重构：锚点注入法

## 问题背景

### 症状
- 当有 subagent 执行结果时，subagent 卡片被放在最底部
- 主 agent 的事件出现在 subagent 卡片上方，而不是按触发顺序排列
- 时序混乱导致用户难以理解事件因果关系

### 根本原因分析

**初始实现的索引错位问题**:

```typescript
// 错误：buildAnchorMap 使用原始数组索引
function buildAnchorMap(events) {
  events.forEach((event, index) => {  // index 是原始数组索引
    if (isDelegationToolEvent(event)) {
      anchorMap.set(anchorId, { eventIndex: index });  // 记录原始索引
    }
  });
}

// 但 buildInterleavedEntries 遍历的是 displayEntries（经过筛选的）
baseEntries.forEach((entry, currentIndex) => {  // currentIndex 是筛选后数组索引
  const shouldInsert = group.anchorIndex <= currentIndex;  // 索引对比无意义！
});
```

**第二个问题：锚点事件被过滤**:
```typescript
// partitionEvents 中跳过了 delegation tool events
if (isDelegationToolEvent(event)) {
  return;  // 锚点事件没有进入 displayEvents！
}
```

这导致锚点事件根本不在展示列表中，无法作为插入参考点。

**旧架构的分区-合并模式问题** (`ConversationEventStream.tsx:316-441`):

```typescript
// 旧逻辑：将事件严格分区
function partitionEvents(events) {
  const displayEvents = [];      // 主 agent 事件
  const subagentThreads = [];    // subagent 事件线程

  events.forEach(event => {
    if (isSubagentLike(event)) {
      // subagent 事件进入独立线程
      subagentThreads.push(event);
    } else {
      displayEvents.push(event);
    }
  });

  return { displayEvents, subagentThreads };
}

// 合并时仅依赖时间戳
const combined = [...displayEvents, ...subagentThreads]
  .sort((a, b) => a.timestamp - b.timestamp);
```

**问题点**:
1. **索引不对齐**: `anchorMap` 使用原始数组索引，但插入逻辑基于筛选后的 `displayEntries`
2. **锚点事件被过滤**: `isDelegationToolEvent` 的事件被跳过，没有进入 `displayEvents`
3. **时间戳不可靠**: subagent 和主 agent 的时间戳可能来自不同的时钟基准
4. **缺乏因果关联**: subagent 由其父 agent 的工具调用触发，但这种关系在分区后丢失

## 解决方案：锚点注入法

### 修复要点

**1. 修复索引对齐问题**:
```typescript
// 修改 buildAnchorMap 接收 displayEntries 而非原始 events
function buildAnchorMap(displayEntries) {
  displayEntries.forEach((entry, displayIndex) => {  // 使用筛选后的索引
    if (isSubagentToolEvent(entry.event)) {
      anchorMap.set(anchorId, { displayIndex });  // 记录 displayEntries 索引
    }
  });
}
```

**2. 保留锚点事件**:
```typescript
// 不再跳过 subagent tool events
if (isSubagentToolEvent(event)) {
  displayEvents.push(event);  // 让锚点事件显示在时间线中
  return;
}
```

**3. 调整 hooks 调用顺序**:
```typescript
// 必须先构建 displayEntries，再构建 anchorMap
const { displayEvents, subagentThreads } = useMemo(() => partitionEvents(...), [...]);
const displayEntries = useMemo(() => buildDisplayEntriesWithClarifyTimeline(displayEvents), [...]);
const anchorMap = useMemo(() => buildAnchorMap(displayEntries), [displayEntries]);  // 依赖 displayEntries
```

**4. 修复插入顺序（紧跟锚点事件）**:
```typescript
// 修改前：subagent 插入在锚点事件之后的下一个事件
baseEntries.forEach((entry, currentIndex) => {
  // ...检查是否需要插入...
  if (shouldInsert) groupsToInsert.forEach(g => result.push(g));
  // 然后添加当前 entry
  result.push(entry);
});

// 修改后：紧跟锚点事件立即插入
baseEntries.forEach((entry, currentIndex) => {
  // 先添加当前 entry
  result.push(entry);

  // 然后立即检查是否有 subagent 锚定到当前位置
  const shouldInsert = group.anchorDisplayIndex === currentIndex;
  if (shouldInsert) groupsToInsert.forEach(g => result.push(g));
});
```

### 核心思想
每个 subagent 线程绑定到一个"锚点事件"（触发它的工具调用），并插入到该锚点位置。

### 架构变更

```
旧数据流:
Events → partitionEvents → displayEvents + subagentThreads
                       ↓
              combinedEntries (按ts排序)
                       ↓
                 渲染为列表

新数据流:
Events → identifyAnchors → Map<anchorId, position>
                       ↓
              buildInterleavedEntries
                       ↓
    统一时间线: [event, event, subagentGroup@anchor2, event, ...]
                       ↓
                 渲染为列表
```

### 关键实现

1. **锚点识别** (`getSubagentAnchorId`):
```typescript
function getSubagentAnchorId(event): string | undefined {
  // Primary: call_id (if starts with "subagent")
  if (event.call_id?.startsWith("subagent")) {
    return `call:${event.call_id}`;
  }

  // Secondary: parent_task_id + task_id
  if (event.parent_task_id && event.task_id) {
    return `task:${event.parent_task_id}:${event.task_id}`;
  }

  // Tertiary: subtask_index
  if (event.parent_task_id && event.subtask_index !== undefined) {
    return `subtask:${event.parent_task_id}:${event.subtask_index}`;
  }
}
```

2. **锚点映射构建** (`buildAnchorMap`):
```typescript
function buildAnchorMap(events) {
  const map = new Map();
  events.forEach((event, index) => {
    if (isDelegationToolEvent(event)) {
      const anchorId = `call:${event.call_id}`;
      map.set(anchorId, { timestamp: event.timestamp, eventIndex: index });
    }
  });
  return map;
}
```

3. **交错时间线构建** (`buildInterleavedEntries`):
```typescript
function buildInterleavedEntries(displayEntries, subagentThreads, anchorMap) {
  const result = [];
  const inserted = new Set();

  displayEntries.forEach((entry, currentIndex) => {
    // 查找应在此位置后插入的 subagent 组
    const toInsert = subagentThreads.filter(thread => {
      if (thread.anchorEventId) {
        const anchor = anchorMap.get(thread.anchorEventId);
        return anchor && anchor.eventIndex <= currentIndex;
      }
      // Fallback to timestamp
      return thread.firstSeenAt <= entry.timestamp;
    });

    // 插入 subagent 组
    toInsert.forEach(group => {
      if (!inserted.has(group.key)) {
        inserted.add(group.key);
        result.push({ kind: "subagentGroup", ...group });
      }
    });

    // 添加当前主事件
    result.push(entry);
  });

  return result;
}
```

## 经验总结

### 设计原则
1. **因果关系优先于时间顺序**: 在分布式系统中，逻辑因果关系比物理时间戳更可靠
2. **显式关联优于隐式推断**: 通过锚点显式关联 subagent 与其触发事件
3. **降级策略**: 当锚点信息缺失时，优雅回退到时间戳排序

### 工程实践
- 保持 `partitionEvents` 接口不变，仅扩展返回值
- 单元测试覆盖锚点提取和排序逻辑
- 保留旧行为作为 fallback，确保向后兼容

### 文件变更
- `web/components/agent/ConversationEventStream.tsx`: 核心逻辑重构
- `web/components/agent/ConversationEventStream.test.ts`: 新增单元测试

## 相关链接
- 设计计划: `docs/plans/subagent-display-redesign.md`
